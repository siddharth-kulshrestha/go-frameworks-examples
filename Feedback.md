# Exploration 

## Golang Migrate
1. Golang Migrate too much of handwritten sql queries. 
2. Need to manage the sql migrations via sql files 
3. If the project is already using some tool which can directly consume sql files then golang migrate would fly. Because the same sql files can be reused while migrating the project to / from golang. 
4. 

## SQLC
1. `db/queries/*.sql` seems very isolated from the application's codebase. 2. No intrinsic feel for developers to think about sql queries and the application business logic together. 
- Developer will first write something in the application code and then he will move to queries part and write those sql statements....
3. Probable advantage would be to know the queries instead of predicting how these queries should look like, probably a better scenario for the AI driven development because no magic is happening around by the ORM. -- AI will be happy... Debuggability would be great. 
4. SQLC Generate will generate methods corresponding to each schema and queries. These generated methods can directly be ported in the application code, also each query mentioned in our files directly ported as constant variables inside the generated file. 
5. We could have explored composing over Organization or model structs but right now all the queries get associated with -- Queries object which is global if we can separate out the Queries and associate them with the models then it would be great because then queries can be tightly associated with the models -- "the One good idea could be to compose over these structs for example CreateOrganizationParams or maybe embed CreateOrganizationParams as a member of Organization" -- TODO: Check if there is any option or not.
6. Good part related to sqlc is that it is a development only tool doesn't come in our development binaries. 
-- SQLC Generates all the methods linked to Queries struct - Low cohesion, SOC not present, ISP (Interface Segregation Principle) these are the OOPs level concepts which are missing in the codebase.
-- We can remediate and control the access from the layer *above sqlc*
```
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

##### Let sqlc have:
```
SQL
 ↓
sqlc
 ↓
one generated Queries
```


Then put your architectural boundary above sqlc.
For example:
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

Example
sqlc generates:
```
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
You then define your own interfaces:
```
type OrganizationRepository interface {
    CreateOrganization(ctx context.Context, ...) (...)
    GetOrganization(ctx context.Context, id uuid.UUID) (...)
}
```

And implement it:
```
type organizationRepository struct {
    queries *sqlc.Queries
}

func NewOrganizationRepository(
    queries *sqlc.Queries,
) OrganizationRepository {
    return &organizationRepository{
        queries: queries,
    }
}

func (r *organizationRepository) CreateOrganization(...) {
    return r.queries.CreateOrganization(...)
}

func (r *organizationRepository) GetOrganization(...) {
    return r.queries.GetOrganization(...)
}
```

Then:
```
type UserRepository interface {
    CreateUser(...)
    GetUser(...)
    ListUsersByOrganization(...)
}
```
and:
```
type userRepository struct {
    queries *sqlc.Queries
}
```

Same underlying *sqlc.Queries, but different exposed abstractions.

--> In the nutshell, repo layer is taking care of separation of concerns (SOC).
