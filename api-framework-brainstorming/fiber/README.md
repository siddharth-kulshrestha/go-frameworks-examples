# Fiber v3 sample — OpenAPI-first REST

This sample deliberately says **Fiber v3**. It uses `fiber.Ctx`, Fiber-native middleware/grouping, and oapi-codegen's `fiber-v3-server` target. It is not Fiber v2 syntax. Fiber uses fasthttp rather than `net/http`, so standard HTTP handlers and middleware are not drop-in components.

```text
fiber/
├── api/openapi.yaml              # source contract (same shape as Chi sample)
├── api/oapi-codegen.yaml         # fiber-v3-server target
├── api/gen/taskforge.gen.go      # generated; never edit
├── cmd/api/main.go               # config, DB, wiring, Fiber lifecycle
└── internal/
    ├── organization/{service.go,repository.go}
    ├── persistence/postgres/organization.go
    └── transport/fiber/
        ├── server.go
        ├── middleware/{request_id.go,access_log.go,auth.go}
        └── organization/handler.go
```

## Bootstrap and grouped routes

```go
type Stores struct {
    Primary *pgxpool.Pool
    // ReadOnly *pgxpool.Pool
}

func main() {
    startupCtx := context.Background()
    pool, err := pgxpool.New(startupCtx, os.Getenv("DATABASE_URL"))
    if err != nil { log.Fatal(err) }
    defer pool.Close()

    stores := Stores{Primary: pool} // include every long-lived DB pool here
    repo := postgres.NewOrganizationRepository(stores.Primary)
    app := organization.NewService(repo)
    handler := organizationhttp.NewHandler(app)

    f := fiber.New(fiber.Config{ReadTimeout: 10 * time.Second, BodyLimit: 1 << 20})
    f.Use(requestid.New(), accesslog.New(), recover.New())
    f.Get("/health/live", live)
    f.Get("/health/ready", ready)

    api := f.Group("/api")
    v1 := api.Group("/v1")
    orgs := v1.Group("/organizations")
    orgs.Get("/:organizationID", handler.Get)
    v1.Post("/organizations", handler.Create)
    v1.Group("/users") // user feature registers its endpoints here
    v1.Group("/projects")

    log.Fatal(f.Listen(":8080"))
}
```

In a larger service, move each feature registration to its own transport file/function. Fiber groups are virtual prefixes; routes are flattened and execute in declaration order, so keep registration order deliberate where patterns overlap.

## Handler, request context, and middleware

```go
type OrganizationUseCases interface {
    Get(ctx context.Context, id uuid.UUID) (organization.Organization, error)
}

type Handler struct { app OrganizationUseCases }

func (h *Handler) Get(c fiber.Ctx) error {
    id, err := uuid.Parse(c.Params("organizationID"))
    if err != nil { return writeProblem(c, fiber.StatusBadRequest, "invalid_id") }
    // In Fiber v3 Ctx implements context.Context. Pass it synchronously into
    // use-case/repository calls; never retain c after this handler returns.
    item, err := h.app.Get(c, id)
    if err != nil { return writeApplicationError(c, err) }
    return c.Status(fiber.StatusOK).JSON(toOrganizationResponse(item))
}
```

Fiber v3 `Ctx` is pooled/reused. Do not capture it in a goroutine or store it. Any work continuing after the request must receive copied values and its own context/lifecycle. This is one of the main differences from Chi's ordinary `r.Context()` model. Fiber v2 has different context APIs and requires explicit care to get a standard context for downstream calls.

Fiber-native middleware has the signature `fiber.Handler`; standard `net/http` middleware won't compose directly. Keep request ID, access logging, recovery, auth, tracing, and metrics under this transport package and use Fiber-native middleware or audited adapters. The generated Fiber v3 server wrapper is also transport-specific.

Representative middleware sketch:

```go
type requestIDKey struct{}

func RequestID() fiber.Handler {
    return func(c fiber.Ctx) error {
        id := c.Get("X-Request-ID")
        if id == "" { id = uuid.NewString() }
        c.Set("X-Request-ID", id)
        c.SetContext(context.WithValue(c.Context(), requestIDKey{}, id))
        return c.Next()
    }
}
```

Treat this as Fiber v3 code and verify exact imports against the pinned Fiber version. Validate/normalize client-supplied IDs before logging. `SetContext` supplies the base standard context while the request is live; don't retain either it or `c` after the handler returns.

## OpenAPI generation and boundaries

Use the same contract as `chi/api/openapi.yaml` (copy it when turning this sample into a separate module) and set `fiber-v3-server: true` in generator config. Generated types and handler interfaces stay at the HTTP edge. The application service still takes plain inputs and `context.Context`; it must not import Fiber. Be precise when integrating strict server validation: Fiber's request-body handling uses Fiber context locals, so use the matching Fiber v3 middleware/version documented by oapi-codegen.

