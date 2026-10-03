# Developer Experience & Architectural Feedback

This document captures engineering observations, architectural analyses, and developer experience (DX) insights gathered while evaluating **golang-migrate**, **sqlc**, and repository design patterns for the TaskForge lab.

---

## Table of Contents

- [1. Database Migrations: Golang Migrate](#1-database-migrations-golang-migrate)
  - [Observations & Trade-offs](#observations--trade-offs)
  - [Portability & Ecosystem Compatibility](#portability--ecosystem-compatibility)
- [2. Query Generation: SQLC](#2-query-generation-sqlc)
  - [Developer Mental Model & Context Switching](#developer-mental-model--context-switching)
  - [Predictability, Debuggability & AI-Assisted Development](#predictability-debuggability--ai-assisted-development)
  - [Code Generation Mechanics & Zero-Runtime Overhead](#code-generation-mechanics--zero-runtime-overhead)
- [3. Architectural Analysis: Monolithic Queries & SOLID Principles](#3-architectural-analysis-monolithic-queries--solid-principles)
  - [The Monolithic Queries Challenge](#the-monolithic-queries-challenge)
  - [Violations of OOP / Clean Architecture Principles](#violations-of-oop--clean-architecture-principles)
  - [Composition & Model Association Ideas](#composition--model-association-ideas)
- [4. The Solution: Architectural Boundary & Repository Adapter Pattern](#4-the-solution-architectural-boundary--repository-adapter-pattern)
  - [Boundary Flow Diagram](#boundary-flow-diagram)
  - [Code Blueprint: Segregated Repositories](#code-blueprint-segregated-repositories)
- [5. Key Takeaways & Summary](#5-key-takeaways--summary)

---

## 1. Database Migrations: Golang Migrate

### Observations & Trade-offs

1. **Heavy Reliance on Handwritten SQL**:
   - `golang-migrate` does not provide an ORM-style schema DSL or auto-generated migrations. Every schema migration (`.up.sql` and `.down.sql`) must be authored by hand.
   - Requires strong familiarity with PostgreSQL DDL, index tuning, foreign key behaviors, and constraints.
2. **File Management Overhead**:
   - Migrations must be managed as sequential numerical/timestamped files on disk (e.g., `000001_create_orgs.up.sql`, `000001_create_orgs.down.sql`).
   - Maintaining synchronization across branches and clean rollback paths requires disciplined team conventions.

### Portability & Ecosystem Compatibility

- **Direct SQL Compatibility Across Stacks**:
  - Because `golang-migrate` consumes raw standard SQL files, it integrates seamlessly into polyglot environments.
  - If a team or project already uses database migration utilities that consume raw SQL (such as Flyway, Liquibase, or db-migrate), `golang-migrate` excels.
  - The exact same SQL migration files can be reused directly when migrating services to or from Go without rewriting schemas into language-specific DSLs.

---

## 2. Query Generation: SQLC

### Developer Mental Model & Context Switching

1. **Isolation from Application Code**:
   - Query files in `db/queries/*.sql` reside in a separate directory structure, physically disconnected from the Go domain and service packages.
2. **Lack of Inline Feedback**:
   - Developers do not get the immediate, unified sensation of authoring SQL queries and application business logic simultaneously.
   - Typical workflow: Write application code $\rightarrow$ switch to SQL files to author query statements $\rightarrow$ run `sqlc generate` $\rightarrow$ return to application code to invoke the generated methods.

### Predictability, Debuggability & AI-Assisted Development

1. **No "ORM Magic"**:
   - You know the exact SQL query being executed. There is no guessing how an ORM will translate complex method chains or joins into SQL.
2. **Excellent for AI-Driven Development**:
   - Because SQL is explicit, AI coding assistants produce fewer hallucinated or inefficient query patterns compared to complex ORM query builders ("AI will be happy").
3. **Superior Debuggability & Observability**:
   - Running `EXPLAIN ANALYZE` on failed or slow queries is instantaneous because the raw SQL is readily available in `.sql` files without needing to dump runtime ORM logs.

### Code Generation Mechanics & Zero-Runtime Overhead

1. **Direct Porting of Types & Constants**:
   - Running `sqlc generate` creates Go methods for every annotated SQL query and Go structs for every table schema.
   - Each SQL query is also embedded as a constant string variable inside the generated Go files for easy inspection.
2. **Zero Runtime Footprint**:
   - `sqlc` is strictly a build-time and development-time tool.
   - It does not introduce reflection, runtime query compilation, or heavy ORM dependencies into the compiled production binaries.

---

## 3. Architectural Analysis: Monolithic Queries & SOLID Principles

### The Monolithic Queries Challenge

By default, SQLC attaches every generated query method to a single global `Queries` struct:

```go
type Queries struct {
    db DBTX
}

func (q *Queries) CreateOrganization(...)
func (q *Queries) GetOrganization(...)
func (q *Queries) CreateUser(...)
func (q *Queries) GetUser(...)
func (q *Queries) CreateIssue(...)
func (q *Queries) GetIssue(...)
```

### Violations of OOP / Clean Architecture Principles

When exposed directly to the application service layer, this monolithic structure exhibits several design shortcomings:

- **Low Cohesion**: Unrelated operations (organizations, users, teams, issues, audit logs) reside on the exact same struct.
- **Missing Separation of Concerns (SoC)**: Application services are coupled directly to the raw database query interface rather than domain-centric persistence operations.
- **Violation of Interface Segregation Principle (ISP)**: A service that only needs `User` persistence must depend on an interface or struct containing every database query across the entire system.

### Composition & Model Association Ideas

- **Model Composition Exploration**:
  - We explored whether query methods could be partitioned and tightly associated with specific models (e.g., tying organization queries directly to `Organization`).
  - One compelling idea: Compose over generated structs—such as embedding `CreateOrganizationParams` directly as a member of `Organization` or vice versa.
  - **TODO**: Check whether `sqlc` configuration options or custom plugins support query partitioning or embedding parameters directly into domain models.

---

## 4. The Solution: Architectural Boundary & Repository Adapter Pattern

To remediate these design challenges, we establish an explicit architectural boundary **above SQLC**.

### Architectural Approach

1. **Let SQLC do what it does best**:
   ```text
   SQL Files
       ↓
     sqlc
       ↓
   Single Generated *sqlc.Queries Struct
   ```
2. **Place the Persistence Boundary Above SQLC**:
   - Introduce domain-specific repository interfaces (`OrganizationRepository`, `UserRepository`, `IssueRepository`).
   - Implement concrete repository adapters that wrap `*sqlc.Queries` internally.
   - Services only interact with their respective segregated repository interface.

### Boundary Flow Diagram

```text
                    PostgreSQL
                         ▲
                         │
                        pgx
                         ▲
                         │
                   sqlc.Queries
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        ▼                ▼                ▼
OrganizationRepo      UserRepo        IssueRepo
        │                │                │
        ▼                ▼                ▼
OrganizationService   UserService   IssueService
```

---

### Code Blueprint: Segregated Repositories

#### 1. Define Narrow Domain Repository Interfaces (Enforcing ISP)

```go
type OrganizationRepository interface {
    CreateOrganization(ctx context.Context, name string, slug string) (*domain.Organization, error)
    GetOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

type UserRepository interface {
    CreateUser(ctx context.Context, params *domain.CreateUserParams) (*domain.User, error)
    GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error)
    ListUsersByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)
}
```

#### 2. Implement Concrete Adapters Wrapping `*sqlc.Queries`

```go
type organizationRepository struct {
    queries *sqlc.Queries
}

func NewOrganizationRepository(queries *sqlc.Queries) OrganizationRepository {
    return &organizationRepository{
        queries: queries,
    }
}

func (r *organizationRepository) CreateOrganization(
    ctx context.Context,
    name string,
    slug string,
) (*domain.Organization, error) {
    row, err := r.queries.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
        ID:   uuid.New(),
        Name: name,
        Slug: slug,
    })
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

func (r *organizationRepository) GetOrganization(
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

```go
type userRepository struct {
    queries *sqlc.Queries
}

func NewUserRepository(queries *sqlc.Queries) UserRepository {
    return &userRepository{
        queries: queries,
    }
}

func (r *userRepository) CreateUser(ctx context.Context, params *domain.CreateUserParams) (*domain.User, error) {
    // Adapter logic delegating to r.queries.CreateUser
    return nil, nil
}

func (r *userRepository) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    // Adapter logic delegating to r.queries.GetUser
    return nil, nil
}

func (r *userRepository) ListUsersByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
    // Adapter logic delegating to r.queries.ListUsersByOrganization
    return nil, nil
}
```

Both repository adapters share the same underlying `*sqlc.Queries` instance, but each exposes only the relevant abstractions to its corresponding domain service.

---

## 5. Key Takeaways & Summary

> [!TIP]
> **Summary Insight**:
> While `sqlc` generates a monolithic `*sqlc.Queries` struct that lacks native cohesion, wrapping it inside a domain **Repository Layer** provides the ideal balance:
> 1. Preserves raw SQL control, compile-time query verification, and zero runtime overhead.
> 2. Enforces **Separation of Concerns (SoC)** and the **Interface Segregation Principle (ISP)** at the application boundary.
> 3. Enables the service layer to remain completely decoupled from underlying database specifics, paving the way for clean comparisons against **Bun** and **Ent**.
