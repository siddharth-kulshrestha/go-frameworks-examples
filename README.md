# Introduction
In this repo we are comparing different database management libraries (sql) like, bun, go ent, sqlc.

We are going to use a sample project to showcase capabilities that these libraries provide while interacting with Postgres (or any other SQL like Database)


## TaskForge 
TaskForge is a project management and issue-tracking software.

### Requirements
```
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


```
| # | Model | Purpose |
|---|---|---|
| 1 | `Organization` | Tenant |
| 2 | `User` | People in an organization |
| 3 | `Team` | Team within organization |
| 4 | `TeamMember` | User ↔ Team N:M relationship |
| 5 | `Project` | Project owned by team |
| 6 | `Issue` | Main work item |
| 7 | `Label` | Issue labels |
| 8 | `IssueLabel` | Issue ↔ Label N:M relationship |
| 9 | `Comment` | Issue comments |
| 10 | `AuditEvent` | Issue/activity history |
```

Requirement document: ./TaskForge-Database-DX-Requirements.md

## Directory Structure
```
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

```
├── README.md
├── PRODUCTIVITY.md
│
├── sqlc/
│   ├── cmd/
│   ├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── seed/
│   ├── sqlc.yaml
│   ├── go.mod
│   └── README.md
│
├── bun/
│   ├── cmd/
│   ├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   └── seed/
│   ├── go.mod
│   └── README.md
│
├── ent/
│   ├── cmd/
│   ├── internal/
│   ├── db/
│   │   ├── migrations/
│   │   └── seed/
│   ├── ent/
│   │   └── schema/
│   ├── go.mod
│   └── README.md
│
└── docs/
    ├── requirements.md
    └── sql-comparison.md
```


```
taskforge/
├── go.work
├── sqlc/
│   └── go.mod
├── bun/
│   └── go.mod
└── ent/
    └── go.mod
```



## Local setup for PG
1. `colima start`
2. Starting postgres
`docker run -d \
  --name taskforge-postgres \
  -e POSTGRES_USER=taskforge \
  -e POSTGRES_PASSWORD=taskforge \
  -e POSTGRES_DB=taskforge \
  -p 5432:5432 \
  -v taskforge-postgres-data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:17`
3. Verify if postgres is running or not:
`docker ps`
` docker exec -it taskforge-postgres \
  pg_isready -U taskforge -d taskforge`
-- Above command should print following:
-- /var/run/postgresql:5432 - accepting connections
4. Go applications can now use:
`postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable`
5. To reset the database:
`docker rm -f taskforge-postgres`
`docker volume rm taskforge-postgres-data`
`docker run -d \
  --name taskforge-postgres \
  -e POSTGRES_USER=taskforge \
  -e POSTGRES_PASSWORD=taskforge \
  -e POSTGRES_DB=taskforge \
  -p 5432:5432 \
  -v taskforge-postgres-data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:17`
6. PSQL command: 
`psql "postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable"`
7. To exec into PSQL:
`docker exec -it taskforge-postgres \
  psql -U taskforge -d taskforge`

## SQLC Setup
1. `brew install sqlc` && `sqlc version`
2. `cd sqlc && go get github.com/jackc/pgx/v5 && go get github.com/jackc/pgx/v5/pgxpool`
3. Install another third-party depenendency: `golang-migrate`
`brew install golang-migrate`
4. Once we have sqlc.yaml file, we can hit command `sqlc generate`
5. In SQLC we have to specify -- schema --> For migrations folder
-- Queries --> For the queries per model (all the queries are created as methods in the generated codebase - which then can be called by the application)

### Internal directory structure
```
sqlc/
├── db/
│   ├── migrations/
│   └── queries/
├── internal/
│   └── store/
│       └── sqlc/
└── go.mod
```


## GOLANG Migrate
1. To migrate up:
```
migrate \
  -path db/migrations \
  -database 'postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable' \
  up
```

## Repository and other packages.
1. We are introducing repository package to establish Separation of concerns in sqlc.
2. Domain and service models should be reusable by each implementation of repository (via sqlc, bun and ent)
3. Basically repository layer will change according to library but domain and service models will be same. 

High level Design:

```
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

```
                       TaskForge
                          │
             ┌────────────┴────────────┐
             │                         │
          Domain                     Service
             │                         │
             └────────────┬────────────┘
                          │
                 Repository interface
                          │
              ┌───────────┼───────────┐
              │           │           │
             sqlc        Bun         Ent
              │           │           │
              ▼           ▼           ▼
           Postgres    Postgres    Postgres
```

```
             framework-independent
        ┌───────────────────────────┐
        │                           │
        │  domain → service         │
        │                           │
        └─────────────┬─────────────┘
                      │
             persistence boundary
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
      sqlc           Bun           Ent
```

##### Difference between domain and services from interaction point of view
```
API
  ↓
Service
  ↓
Domain + Repository
  ↓
sqlc
  ↓
PostgreSQL
```

Domains -> Structs denoting business concepts and entities.
Essentially domain represents common language between your application layers.

But API models ≠ Domain models
Instead, 
```
API DTO
   ↓
Domain model
   ↓
Database/sqlc model
```

eg:
```
type Issue struct {
    ID       uuid.UUID
    Title    string
    Status   IssueStatus
    Priority IssuePriority
}
```

Service = "What can I do with those things?"
```
type IssueService struct {
    repo IssueRepository
}

func (s *IssueService) AssignIssue(...) error {
    ...
}

func (s *IssueService) ChangeStatus(...) error {
    ...
}

func (s *IssueService) CreateIssue(...) error {
    ...
}
```

Service implements use cases/business workflows.


From API level: 
```
                    HTTP Request
                         │
                         ▼
                 OrganizationHandler
                         │
                         │ CreateOrganization(...)
                         ▼
                OrganizationService
                         │
                         │ business rules
                         ▼
             OrganizationRepository
                         │
                         │ Create(...)
                         ▼
                  sqlc.Queries
                         │
                         │ INSERT
                         ▼
                    PostgreSQL
```

Service example:
```
func (s *OrganizationService) CreateOrganization(
    ctx context.Context,
    name string,
    slug string,
) (*domain.Organization, error) {

    // business validation
    if name == "" {
        return nil, ErrInvalidName
    }

    if slug == "" {
        return nil, ErrInvalidSlug
    }

    return s.repo.Create(ctx, ...)
}
```

Sample repo example: 
```
type OrganizationRepository interface {
    Create(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}
```

sqlc implementation of the repo:
```
type organizationRepository struct {
    queries *sqlc.Queries
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
```


API DTO vs Domain vs SQLC Generated:

For example:
API request
```type CreateOrganizationRequest struct {
    Name string `json:"name"`
    Slug string `json:"slug"`
}```

Domain
```type Organization struct {
    ID        uuid.UUID
    Name      string
    Slug      string
    CreatedAt time.Time
}```

sqlc
Generated:
```type Organization struct {
    ID        uuid.UUID
    Name      string
    Slug      string
    CreatedAt time.Time
}```

They may look almost identical right now.
But they have different responsibilities.
