# TaskForge — Go Database Frameworks & Developer Experience (DX) Comparison

Comparing different Go database management libraries and approaches (**sqlc**, **Bun**, and **Ent**) using a realistic, multi-tenant project management application interacting with PostgreSQL.

---

## Table of Contents

- [Overview](#overview)
- [TaskForge Domain](#taskforge-domain)
  - [Entity Relationship Diagram](#entity-relationship-diagram)
  - [Core Models](#core-models)
- [Architecture & Repository Pattern](#architecture--repository-pattern)
  - [High-Level Architecture](#high-level-architecture)
  - [Interaction Flow (API to Database)](#interaction-flow-api-to-database)
  - [Separation of Concerns: DTO vs. Domain vs. Persistence](#separation-of-concerns-dto-vs-domain-vs-persistence)
  - [Code Blueprint: Service & Repository Implementation](#code-blueprint-service--repository-implementation)
- [Directory Structure](#directory-structure)
- [Local PostgreSQL Setup](#local-postgresql-setup)
- [SQLC Setup & Workflow](#sqlc-setup--workflow)
- [Database Migrations (golang-migrate)](#database-migrations-golang-migrate)
- [Documentation Links](#documentation-links)

---

## Overview

In this repository, we compare the developer experience (DX), maintainability, type safety, performance, and ergonomics of three distinct Go database libraries:
1. **[sqlc](https://sqlc.dev/)**: Generates type-safe Go code from raw SQL queries.
2. **[Bun](https://bun.uptrace.dev/)**: SQL-first Golang ORM for PostgreSQL, MySQL, and SQLite.
3. **[Ent](https://entgo.io/)**: An entity framework for Go, based on code generation and graph traversal.

We implement the exact same set of realistic business requirements across all three libraries against PostgreSQL 17 to evaluate how each abstraction performs in real-world scenarios.

---

## TaskForge Domain

**TaskForge** is a multi-tenant project management and issue-tracking software (similar to Jira or Linear).

### Entity Relationship Diagram

```text
Organization
    │
    ├── User
    │
    ├── Team
    │     │
    │     └── TeamMember ── User
    │
    └── Label
          │
          └─────────────┐
                        │
Team ── Project ── Issue ── IssueLabel ── Label
                  │
                  ├── Comment ── User
                  │
                  ├── Parent Issue
                  │
                  └── AuditEvent ── User
```

### Core Models

The domain contains exactly **10 database entities**:

| # | Model | Purpose |
|:---:|:---|:---|
| 1 | `Organization` | Multi-tenant root account |
| 2 | `User` | People belonging to an organization |
| 3 | `Team` | Functional teams within an organization |
| 4 | `TeamMember` | Many-to-many (`N:M`) mapping between `User` and `Team` with role |
| 5 | `Project` | Work project owned by a `Team` |
| 6 | `Issue` | Core work item with status, priority, estimates, and parent/child hierarchy |
| 7 | `Label` | Organization-scoped tags for categorizing issues |
| 8 | `IssueLabel` | Many-to-many (`N:M`) relationship linking `Issue` and `Label` |
| 9 | `Comment` | User comments posted on issues |
| 10 | `AuditEvent` | Append-only history and activity log for issues and organizations |

For full specification and constraints, see [TaskForge Database DX Requirements](./TaskForge-Database-DX-Requirements.md).

---

## Architecture & Repository Pattern

To establish strict **Separation of Concerns (SoC)**, the repository layer decouples the persistence engine (`sqlc`, `Bun`, or `Ent`) from the domain models and application business logic.

- **Domain & Service layers** are pure Go and framework-independent.
- **Repository interfaces** define domain-oriented persistence contracts.
- **Library implementations** (`sqlc`, `Bun`, `Ent`) act as interchangeable infrastructure adapters.

### High-Level Architecture

```text
                    Service
                       │
                       ▼
               domain.Organization
                       ▲
                       │
              ┌────────┴────────┐
              │                 │
        sqlc Repository    Bun Repository
              │                 │
         sqlc types          Bun types
```

```text
                       TaskForge
                          │
             ┌────────────┴────────────┐
             │                         │
           Domain                     Service
             │                         │
             └────────────┬────────────┘
                          │
                 Repository Interface
                          │
              ┌───────────┼───────────┐
              │           │           │
             sqlc        Bun         Ent
              │           │           │
              ▼           ▼           ▼
           Postgres    Postgres    Postgres
```

```text
             framework-independent
        ┌───────────────────────────┐
        │                           │
        │     domain → service      │
        │                           │
        └─────────────┬─────────────┘
                      │
             persistence boundary
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
       sqlc          Bun           Ent
```

### Interaction Flow (API to Database)

```text
HTTP Request / API DTO
         │
         ▼
OrganizationHandler
         │
         │ CreateOrganization(...)
         ▼
OrganizationService
         │
         │ business rules & validations
         ▼
OrganizationRepository (Interface)
         │
         │ Create(...)
         ▼
sqlc.Queries (or Bun / Ent Adapter)
         │
         │ INSERT INTO organizations ...
         ▼
    PostgreSQL
```

### Separation of Concerns: DTO vs. Domain vs. Persistence

```text
API DTO
   ↓
Domain Model
   ↓
Database / sqlc Model
```

- **API DTOs**: Structure transport payloads, handle serialization (JSON), and validate wire inputs.
- **Domain Models**: Express core business concepts, entities, and business rules across application layers.
- **Database / SQL Models**: Match the storage engine schemas, tables, and foreign keys.

Even when fields look identical initially, their responsibilities and lifecycle boundaries remain distinct:

#### 1. API Request DTO
```go
type CreateOrganizationRequest struct {
    Name string `json:"name"`
    Slug string `json:"slug"`
}
```

#### 2. Domain Model
```go
type Organization struct {
    ID        uuid.UUID
    Name      string
    Slug      string
    CreatedAt time.Time
}
```

#### 3. SQLC Generated Model
```go
type Organization struct {
    ID        uuid.UUID
    Name      string
    Slug      string
    CreatedAt time.Time
}
```

> [!NOTE]
> While the fields appear similar today, domain entities often contain methods, invariant validations, and rich types, whereas API DTOs handle transport serialization and database models map strictly to table schemas.

---

### Code Blueprint: Service & Repository Implementation

#### Domain Struct Example

```go
type Issue struct {
    ID       uuid.UUID
    Title    string
    Status   IssueStatus
    Priority IssuePriority
}
```

#### Service Layer Example

Services orchestrate use cases and enforce business workflows:

```go
type IssueService struct {
    repo IssueRepository
}

func (s *IssueService) AssignIssue(ctx context.Context, issueID uuid.UUID, assigneeID uuid.UUID) error {
    // business validation & workflow
    return s.repo.Assign(ctx, issueID, assigneeID)
}

func (s *IssueService) ChangeStatus(ctx context.Context, issueID uuid.UUID, status IssueStatus) error {
    return s.repo.UpdateStatus(ctx, issueID, status)
}

func (s *IssueService) CreateIssue(ctx context.Context, req *CreateIssueParams) (*domain.Issue, error) {
    return s.repo.Create(ctx, req)
}
```

```go
func (s *OrganizationService) CreateOrganization(
    ctx context.Context,
    name string,
    slug string,
) (*domain.Organization, error) {
    // Business validation
    if name == "" {
        return nil, ErrInvalidName
    }

    if slug == "" {
        return nil, ErrInvalidSlug
    }

    org := &domain.Organization{
        ID:   uuid.New(),
        Name: name,
        Slug: slug,
    }

    return s.repo.Create(ctx, org)
}
```

#### Repository Interface Example

```go
type OrganizationRepository interface {
    Create(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}
```

#### SQLC Repository Adapter Implementation

The adapter maps domain models to SQLC parameters, executes queries via `*sqlc.Queries`, and maps SQL records back into domain models:

```go
type organizationRepository struct {
    queries *sqlc.Queries
}

func NewOrganizationRepository(queries *sqlc.Queries) OrganizationRepository {
    return &organizationRepository{
        queries: queries,
    }
}

func (r *organizationRepository) Create(
    ctx context.Context,
    org *domain.Organization,
) (*domain.Organization, error) {
    row, err := r.queries.CreateOrganization(
        ctx,
        sqlc.CreateOrganizationParams{
            ID:   org.ID,
            Name: org.Name,
            Slug: org.Slug,
        },
    )
    if err != nil {
        return nil, err
    }

    return &domain.Organization{
        ID:        row.ID,
        Name:      row.Name,
        Slug:      row.Slug,
        CreatedAt: row.CreatedAt,
    }, nil
}

func (r *organizationRepository) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (*domain.Organization, error) {
    row, err := r.queries.GetOrganization(ctx, id)
    if err != nil {
        return nil, err
    }

    return &domain.Organization{
        ID:        row.ID,
        Name:      row.Name,
        Slug:      row.Slug,
        CreatedAt: row.CreatedAt,
    }, nil
}
```

---

## Directory Structure

```text
             TaskForge requirements
                       │
           ┌───────────┼───────────┐
           ▼           ▼           ▼
         sqlc         Bun         Ent
           │           │           │
        go.mod      go.mod      go.mod
           │           │           │
        Postgres    Postgres    Postgres
           │           │           │
        same DB     same DB     same DB
           │           │           │
        same tests  same tests  same tests
```

### Workspace File Tree

```text
go-frameworks-examples/
├── README.md                                 # Overview, setup, and architecture
├── TaskForge-Database-DX-Requirements.md     # Full 26-point specification
├── Feedback.md                               # Architectural feedback and insights
├── PRODUCTIVITY.md                           # Metrics and DX evaluation logs
├── go.work                                   # Go multi-module workspace
│
├── domain/                                   # Framework-independent domain models
│   └── models/
│       └── organization.go
│
├── service/                                  # Business logic and use case services
│
├── sqlc/                                     # sqlc implementation module
│   ├── cmd/
│   ├── internal/
│   │   └── store/
│   │       └── sqlc/                         # Generated sqlc Go code
│   ├── db/
│   │   ├── migrations/                       # PostgreSQL migration files (.sql)
│   │   ├── queries/                          # Model-specific queries (.sql)
│   │   └── seed/                             # Seed data scripts
│   ├── sqlc.yaml                             # sqlc configuration file
│   ├── go.mod
│   └── README.md
│
├── bun/                                      # Bun implementation module
│   ├── cmd/
│   ├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   └── seed/
│   ├── go.mod
│   └── README.md
│
├── ent/                                      # Ent implementation module
│   ├── cmd/
│   ├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   └── seed/
│   ├── ent/
│   │   └── schema/                           # Ent schema definitions
│   ├── go.mod
│   └── README.md
│
└── docs/                                     # Additional documentation
    ├── requirements.md
    └── sql-comparison.md
```

### Go Work Configuration (`go.work`)

```text
taskforge/
├── go.work
├── sqlc/
│   └── go.mod
├── bun/
│   └── go.mod
└── ent/
    └── go.mod
```

---

## Local PostgreSQL Setup

Follow these steps to run PostgreSQL 17 locally with Docker and Colima (on macOS):

### 1. Start Colima (if on macOS)
```bash
colima start
```

### 2. Start PostgreSQL 17 Container
```bash
docker run -d \
  --name taskforge-postgres \
  -e POSTGRES_USER=taskforge \
  -e POSTGRES_PASSWORD=taskforge \
  -e POSTGRES_DB=taskforge \
  -p 5432:5432 \
  -v taskforge-postgres-data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:17
```

### 3. Verify Container Status
Check if the container is running:
```bash
docker ps
```

Verify connection readiness:
```bash
docker exec -it taskforge-postgres \
  pg_isready -U taskforge -d taskforge
```

*Expected output:*
```text
/var/run/postgresql:5432 - accepting connections
```

### 4. Connection String
Go applications and migration tools connect via:
```text
postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable
```

### 5. Reset Database
To completely wipe the database and recreate it fresh:
```bash
docker rm -f taskforge-postgres
docker volume rm taskforge-postgres-data
docker run -d \
  --name taskforge-postgres \
  -e POSTGRES_USER=taskforge \
  -e POSTGRES_PASSWORD=taskforge \
  -e POSTGRES_DB=taskforge \
  -p 5432:5432 \
  -v taskforge-postgres-data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:17
```

### 6. Connect via Local PSQL
```bash
psql "postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable"
```

### 7. Connect via Container PSQL
```bash
docker exec -it taskforge-postgres \
  psql -U taskforge -d taskforge
```

---

## SQLC Setup & Workflow

### 1. Install CLI Tools
```bash
brew install sqlc
sqlc version
```

### 2. Install Go Dependencies
From within the `sqlc` module directory:
```bash
cd sqlc
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
```

### 3. Install Migration CLI (`golang-migrate`)
```bash
brew install golang-migrate
```

### 4. Configure `sqlc.yaml` & Generate Code
Run code generation:
```bash
sqlc generate
```

In `sqlc.yaml`, define:
- `schema`: Path pointing to the `db/migrations` folder containing DDL schema files.
- `queries`: Path pointing to SQL query files per model (`db/queries/*.sql`).

All annotated queries are compiled into type-safe Go methods and constants inside the generated package.

### 5. SQLC Internal Directory Layout
```text
sqlc/
├── db/
│   ├── migrations/                           # DDL schema migrations
│   └── queries/                              # SQL queries annotated for sqlc
├── internal/
│   └── store/
│       └── sqlc/                             # Generated Go structs and query methods
├── sqlc.yaml                                 # sqlc configuration
└── go.mod
```

---

## Database Migrations (`golang-migrate`)

Run schema migrations against the local PostgreSQL instance:

```bash
migrate \
  -path db/migrations \
  -database 'postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable' \
  up
```

To roll back migrations:
```bash
migrate \
  -path db/migrations \
  -database 'postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable' \
  down
```

---

## Documentation Links

- **[TaskForge Database DX Requirements](./TaskForge-Database-DX-Requirements.md)** — Detailed specification covering 10 tables, required indexes, seed data, 26 feature requirements, transactions, analytical queries, CTEs, window functions, and contract tests.
- **[Feedback & Architectural Exploration](./Feedback.md)** — Real-world observations, developer experience insights, OOP design challenges (ISP/SoC), and the Repository pattern solution for SQLC.
