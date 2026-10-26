# Chi sample — OpenAPI-first REST

Chi is a router over `net/http`; this sample uses the same handler signature as the standard library and composes feature routes with `Route`, `Group`, and `Mount`.

```text
chi/
├── api/openapi.yaml              # source contract
├── api/oapi-codegen.yaml         # pinned generator config
├── api/gen/taskforge.gen.go      # generated; never edit
├── cmd/api/main.go               # config, DB, wiring, server lifecycle
└── internal/
    ├── app/dependencies.go       # long-lived resources, not request context
    ├── organization/{service.go,repository.go}
    ├── persistence/postgres/organization.go
    └── transport/http/
        ├── server.go
        ├── middleware/{request_id.go,access_log.go,auth.go}
        └── organization/handler.go
```

The sample is conceptually connected to the root repository's pgx/sqlc/Bob experiments, but imports are omitted where the specific generated package/module is still undecided.

## Route and dependency wiring

```go
type Dependencies struct {
    Stores Stores // all long-lived DB pools; close each during shutdown
}

type Stores struct {
    Primary *pgxpool.Pool
    // ReadOnly *pgxpool.Pool // add only if deployment/data access needs it
}

func main() {
    ctx := context.Background()
    pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
    if err != nil { log.Fatal(err) }
    defer pool.Close()

    deps := Dependencies{Stores: Stores{Primary: pool}}
    repo := postgres.NewOrganizationRepository(deps.Stores.Primary)
    useCases := organization.NewService(repo)
    apiHandler := organizationhttp.NewHandler(useCases)

    router := httpapi.NewRouter(apiHandler)
    srv := &http.Server{Addr: ":8080", Handler: router, ReadHeaderTimeout: 5 * time.Second}
    // Production code: signal.NotifyContext, srv.Shutdown(deadline), pool.Close.
    log.Fatal(srv.ListenAndServe())
}
```

```go
func NewRouter(orgAPI *organizationhttp.Handler) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
    r.Use(accesslog.New()) // application structured logger adapter

    r.Get("/health/live", live)
    r.Get("/health/ready", readiness) // inject a readiness checker in production
    r.Route("/api/v1", func(api chi.Router) {
        api.Route("/organizations", func(r chi.Router) {
            r.Get("/{organizationID}", orgAPI.Get)
        })
        api.Post("/organizations", orgAPI.Create)
        r.Route("/users", func(r chi.Router) {
            // user feature routes registered here
        })
        r.Route("/projects", func(r chi.Router) {
            // project feature routes registered here
        })
    })
    return r
}
```

## Request path and errors

```go
type OrganizationUseCases interface {
    Get(ctx context.Context, id uuid.UUID) (organization.Organization, error)
}

type Handler struct { app OrganizationUseCases }

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "organizationID"))
    if err != nil { writeProblem(w, http.StatusBadRequest, "invalid_id") ; return }
    item, err := h.app.Get(r.Context(), id)
    if err != nil { writeApplicationError(w, err); return }
    writeJSON(w, http.StatusOK, toOrganizationResponse(item))
}
```

The handler receives a per-request `r.Context()` and maps application errors to HTTP. The use case doesn't import Chi, `net/http`, OpenAPI generated types, or pgx. Repository methods accept that context so query cancellation and deadlines propagate.

## Middleware and generated contract

Keep middleware in `internal/transport/http/middleware`; each middleware uses the standard signature `func(http.Handler) http.Handler`. Request ID, recovery, and logging can wrap all routes. Auth should be attached to the protected API group, not health routes. Trace/metrics can use compatible `net/http` instrumentation. Don't add CORS or rate limiting until deployment requirements call for them.

Use `oapi-codegen`'s `chi-server: true` to generate route binding/server interfaces and models. Adapt generated interfaces in the transport package; do not make use-case signatures generated transport methods. Generation config and an illustrative contract are in `api/`.

Representative middleware:

```go
type requestIDKey struct{}

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" { id = uuid.NewString() }
        ctx := context.WithValue(r.Context(), requestIDKey{}, id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

The value is request-scoped and typed; application dependencies stay constructor-injected. In production, validate/normalize any client-supplied ID before logging it.

