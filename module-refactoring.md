# Multi-Module Architecture & Workspace Refactoring Guide

This document validates the analysis of the `go mod tidy` failure, explains the Go toolchain's workspace mechanics, and resolves the architectural questions surrounding the **Composition Root**, the **Service Layer**, and multi-module boundaries across **sqlc**, **Bun**, and **Ent**.

---

## Table of Contents

- [1. Technical Validation: What the Analysis Got Right & What It Overlooked](#1-technical-validation-what-the-analysis-got-right--what-it-overlooked)
- [2. The Root Cause: Why `go mod tidy` Failed](#2-the-root-cause-why-go-mod-tidy-failed)
  - [Workspace Commands vs. Module Commands](#workspace-commands-vs-module-commands)
  - [Why `go mod tidy` Ignores `go.work`](#why-go-mod-tidy-ignores-gowork)
  - [The Remote Git Fallback](#the-remote-git-fallback)
- [3. The Composition Root Dilemma](#3-the-composition-root-dilemma)
  - [Your Service Layer Was Already Correct!](#your-service-layer-was-already-correct)
  - [The Wiring Responsibility](#the-wiring-responsibility)
- [4. The Two Architectural Solutions](#4-the-two-architectural-solutions)
  - [Option A: Independent Framework Executables (TaskForge Specification)](#option-a-independent-framework-executables-taskforge-specification)
  - [Option B: Root Composition Root with `replace` Directive](#option-b-root-composition-root-with-replace-directive)
- [5. Verification & Live Execution](#5-verification--live-execution)
- [6. Architecture Decision Matrix](#6-architecture-decision-matrix)

---

## 1. Technical Validation: What the Analysis Got Right & What It Overlooked

| Assertion | Verdict | Details |
|---|---|---|
| **"`go.work` is configured correctly"** | **True** | `go.work` correctly includes `.`, `./sqlc`, `./bun`, and `./ent`. `go list -m all` resolves them all locally. |
| **"`go mod tidy` is module-scoped, not workspace-scoped"** | **True** | `go mod tidy` strictly evaluates the current module's `go.mod` and ignores `go.work` for remote resolution. |
| **"The Service layer must not import `sqlc/repository`"** | **True** | Adheres to Dependency Inversion Principle (DIP). Services must depend on interfaces, never concrete DB adapters. |
| **"I suspect `service/organization.go` imports `sqlc/repository`"** | **False** | Verified via grep: [`service/organization.go`](./service/organization.go) **never imported** `sqlc/repository`. It was already clean! |
| **"The root module shouldn't import `sqlc/repository`"** | **Partial** | If the root directory hosts `main.go` as the application entry point, it **must** import the concrete repository to wire it to the service (the Composition Root). |

---

## 2. The Root Cause: Why `go mod tidy` Failed

### Workspace Commands vs. Module Commands

The Go toolchain behaves differently depending on whether a command is **Workspace-Aware** or **Module-Aware**:

```text
┌────────────────────────────────────────┐
│  Workspace-Aware (Read go.work)        │
│  - go build                            │
│  - go run                              │
│  - go test                             │
│  - gopls (IDE Language Server)         │
└────────────────────────────────────────┘
                  vs.
┌────────────────────────────────────────┐
│  Module-Aware (Isolated to go.mod)     │
│  - go mod tidy                         │
│  - go mod download                     │
│  - go get                              │
└────────────────────────────────────────┘
```

### Why `go mod tidy` Ignores `go.work`

By design (see Go issue [#50750](https://github.com/golang/go/issues/50750)), `go mod tidy` is intended to produce a **deterministic, standalone `go.mod` file** that can be published and built outside your local workstation without requiring your personal `go.work` file.

When you ran `go mod tidy` in the root directory:
1. `go mod tidy` inspected [`main.go`](./main.go).
2. It found the import statement:
   ```go
   import "github.com/siddharth-kulshrestha/taskforge/sqlc/repository"
   ```
3. It checked the root [`go.mod`](./go.mod) (`module github.com/siddharth-kulshrestha/taskforge`).
4. It recognized that `sqlc` is a separate module (`github.com/siddharth-kulshrestha/taskforge/sqlc`), but the root `go.mod` had no local mapping or version requirement for it.
5. Because `go mod tidy` ignores `go.work`, it fell back to searching the public internet.

### The Remote Git Fallback

Go saw the path `github.com/siddharth-kulshrestha/taskforge/sqlc/repository` and executed:
```bash
git ls-remote -q https://github.com/siddharth-kulshrestha/taskforge/
```
Because the remote repository does not exist on GitHub (or is a private repository without SSH/OAuth credentials attached), Git aborted with:
```text
exit status 128:
remote: Repository not found.
fatal: Authentication failed for 'https://github.com/siddharth-kulshrestha/taskforge/'
```

---

## 3. The Composition Root Dilemma

### Your Service Layer Was Already Correct!

The hypothesis in the prompt suspected [`service/organization.go`](./service/organization.go) was importing `sqlc`. Checking the code proves it is completely decoupled:

```go
// service/organization.go
package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/siddharth-kulshrestha/taskforge/domain/models"
)

// Clean abstraction! No database or framework dependencies here.
type OrganizationRepository interface {
	Create(ctx context.Context, org *models.Organization) (*models.Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
}

type OrganizationService struct {
	repo OrganizationRepository
}
```

The service layer **only** depends on [`domain/models`](./domain/models) and defines its own [`OrganizationRepository`](./service/organization.go#L11-L14) interface. It has zero coupling to `sqlc`.

---

### The Wiring Responsibility

In software architecture, **someone must instantiate the concrete repository and pass it into the service**. This place is called the **Composition Root**:

```go
// Who writes these lines of code?
repo := repository.NewOrganizationRepositoryImpl(pool) // Needs concrete repository!
service := service.NewOrganizationService(repo)        // Injects repo into service
```

If these lines live in [`main.go`](./main.go) at the root, then the root module **must** import `sqlc/repository`.

This brings us to two architectural options.

---

## 4. The Two Architectural Solutions

### Option A: Independent Framework Executables (TaskForge Specification)

This matches the design laid out in [TaskForge-Database-DX-Requirements.md](./TaskForge-Database-DX-Requirements.md#1-repository--implementation-boundary):

```text
taskforge/ (Root Module: github.com/siddharth-kulshrestha/taskforge)
├── go.work
├── domain/models/            # Shared domain entities
├── service/                  # Shared use cases & repository interfaces
│
├── sqlc/                     # Independent Module: .../taskforge/sqlc
│   ├── cmd/
│   │   └── main.go           # <-- Composition root for sqlc!
│   ├── repository/           # Implements service.OrganizationRepository
│   ├── internal/store/sqlc/  # Generated sqlc code
│   └── go.mod
│
├── bun/                      # Independent Module: .../taskforge/bun
│   ├── cmd/
│   │   └── main.go           # <-- Composition root for Bun!
│   ├── repository/
│   └── go.mod
│
└── ent/                      # Independent Module: .../taskforge/ent
    ├── cmd/
    │   └── main.go           # <-- Composition root for Ent!
    ├── repository/
    └── go.mod
```

#### How it works:
1. The **root module** only contains `domain/` and `service/`. It has **no** `main.go` and zero database dependencies.
2. Each framework module (`sqlc`, `bun`, `ent`) contains its own `cmd/main.go`.
3. `sqlc/cmd/main.go` imports `service` and its local `repository` package, builds the pool, and runs.
4. Running `go mod tidy` inside `sqlc/` resolves `service` via `go.work` without any errors.

---

### Option B: Root Composition Root with `replace` Directive

If you prefer having a **single `main.go` at the root** that drives the application:

```text
Root Module (github.com/siddharth-kulshrestha/taskforge)
   └── main.go (imports service + sqlc/repository)
```

You inform the root [`go.mod`](./go.mod) where to find `github.com/siddharth-kulshrestha/taskforge/sqlc` locally using a `replace` directive:

```go
module github.com/siddharth-kulshrestha/taskforge

go 1.25.6

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/siddharth-kulshrestha/taskforge/sqlc v0.0.0-unpublished
)

replace github.com/siddharth-kulshrestha/taskforge/sqlc => ./sqlc
```

#### Why this works:
- When you run `go mod tidy` in the root directory, Go reads `replace github.com/.../sqlc => ./sqlc`.
- It redirects the lookup to the local `./sqlc` folder instead of making a network call to GitHub.
- `go mod tidy` succeeds immediately with exit code 0.

---

## 5. Verification & Live Execution

We tested and validated Option B with the local `replace` directive:

1. Ran `go mod tidy` in the root directory $\rightarrow$ **Exited with code 0 (clean)**.
2. Verified [`main.go`](./main.go) compiles and connects using `pgxpool.Pool`.
3. Ran `go run main.go`:
   ```text
   starting the execution of Task forge....
   &{37bbc58c-40d9-4be5-a022-d12c94c27e9c SidOrg SidOrg-slug 2026-10-04 05:22:28.955995 +0000 UTC}
   ```

> [!NOTE]
> In [`main.go`](./main.go#L19), update the connection string to match your Docker PostgreSQL container:
> ```go
> connStr := "postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable"
> ```

---

## 6. Architecture Decision Matrix

| Dimension | Option A: Sub-Module `cmd/main.go` | Option B: Root `main.go` with `replace` |
|---|---|---|
| **Alignment with Spec** | 100% matches `TaskForge-Database-DX-Requirements.md` | Convenient for rapid single-file testing |
| **Dependency Isolation** | Complete: `sqlc` dependencies never touch `bun` or `ent` | Root `go.mod` ends up referencing all 3 adapters |
| **`go mod tidy`** | Run per module: `cd sqlc && go mod tidy` | Run at root via `replace` directive |
| **Benchmarking DX** | Ideal: isolates compile times and binary weights per framework | Shared root binary |
| **Recommendation** | **Target Final Architecture** | **Great for initial development / exploration** |

### Next Steps Recommendation
1. For now, keep the root `replace` directive in [`go.mod`](./go.mod) so you can rapidly test your SQLC queries and services directly in [`main.go`](./main.go).
2. When implementing **Bun** and **Ent**, move each framework's entry point to `sqlc/cmd/main.go`, `bun/cmd/main.go`, and `ent/cmd/main.go` as defined in Option A to ensure zero dependency bleeding between the three frameworks.
