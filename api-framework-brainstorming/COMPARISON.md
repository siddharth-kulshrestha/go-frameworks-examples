# Go HTTP framework comparison

## Summary for TaskForge

| Option | Best fit | Contract workflow | HTTP coupling | Developer productivity |
|---|---|---|---|---|
| Standard `net/http` (`ServeMux`) | Small/simple REST surface; minimal dependencies | OpenAPI codegen `std-http-server`, or hand-written | Lowest | High for a small API; route composition and conventions are explicit work |
| Chi | Modular REST API with feature route groups and stdlib integrations | OpenAPI-first works naturally with `chi-server` | Low | Strong balance: concise route composition without an application framework |
| Fiber v3 | Team prefers Fiber ergonomics, or measured performance justifies fasthttp | OpenAPI-first supported with `fiber-v3-server` | Higher | Fast to write handlers/groups; learn Fiber-specific context and middleware lifecycle |
| Go Kratos | Service-oriented system with HTTP + gRPC and protobuf conventions | Protobuf-first; OpenAPI can be generated | Highest of these options | High once conventions/tooling are adopted; more setup and framework concepts initially |

## Developer experience and tradeoffs

### `net/http`

**Productivity:** Go 1.22+ `ServeMux` supports method patterns and path wildcards, so basic REST routing no longer needs a third-party router. Standard middleware, `httptest`, contexts, and observability libraries fit directly. The cost is that grouping, route modules, and middleware scope need to be composed explicitly; this is manageable but verbose as features multiply.

**Strengths:** standard interfaces, minimal dependency risk, straightforward testing and migration, strongest ecosystem compatibility.

**Costs:** fewer ergonomics for nested groups/subrouter mounting; consistent error/logging conventions are your responsibility. It remains fully capable of a substantial API.

### Chi

**Productivity:** Adds convenient `Route`, `Group`, `Mount`, and middleware scoping while handlers remain standard `http.Handler`. Feature packages can register routes without inventing controller discovery or global registries. OpenAPI generation has a Chi server target.

**Strengths:** portable middleware, request context/cancellation semantics, idiomatic handler tests, and easy replacement by `ServeMux` for simpler routing or another `net/http` router later.

**Costs:** it is still a router, not an application framework: DB wiring, dependency injection, validation, auth policy, error schema, and graceful shutdown remain explicit. That is desirable here but means some boilerplate is yours.

### Fiber v3

**Productivity:** Express-like route and response APIs, built-in grouping, and a middleware catalog can make handlers quick to author. Current v3 supports a standard-looking context interface, but its `Ctx` is pooled and transport-specific. Fiber v3 is separate from Fiber v2 in APIs and code generation.

**Strengths:** cohesive native APIs and good ergonomics for teams that like its model; route grouping and OpenAPI server generation are available.

**Costs:** based on fasthttp rather than `net/http`; ordinary stdlib handlers/middleware aren't directly interchangeable. Request context lifetime and integration adapters need care. A later move to Chi requires replacing route/middleware/handler plumbing. Measure before using throughput claims to justify that coupling.

### Go Kratos

**Productivity:** Gives a broader service framework, project layout, HTTP and gRPC transport support, middleware, lifecycle, and code generation patterns. Kratos templates commonly use protobuf-first contracts and layered service/biz/data packages; verify the template/CLI major version because Kratos v2 and v3 differ.

**Strengths:** a coherent path for teams that need protobuf, dual HTTP/gRPC endpoints, and consistent microservice conventions. Generated bindings reduce repetitive transport work.

**Costs:** the framework's service abstractions and protobuf contract workflow shape the codebase. OpenAPI YAML is not the natural source of truth; generated OpenAPI comes from protobuf annotations. It is more framework to learn and a larger migration if only plain REST is needed. The framework doesn't replace business validation or persistence architecture.

Go kit is intentionally not another row here: it is a composable service toolkit rather than a routing framework, and you already have a maintenance concern about it. Its endpoint/transport layering can be borrowed conceptually, but Kratos is the framework candidate in this comparison.

## Cross-cutting concerns

| Concern | `net/http` / Chi | Fiber v3 | Kratos |
|---|---|---|---|
| Feature routes | ServeMux patterns or Chi `Route`/`Mount` | Fiber `Group`; registration order matters | Split service/proto files; generated binding registration |
| Shared DB pools | Construct once, inject; pass request context to DB | Same ownership; pass Fiber context synchronously, don't retain pooled `Ctx` | Construct in data/composition; pass RPC context through service/biz/data |
| Middleware reuse | Broad stdlib reuse | Fiber-native or explicit adapter | Kratos transport middleware; HTTP/gRPC metadata differs |
| Per-request context | `r.Context()` with cancellation/deadline | Fiber v3 `Ctx` implements context interface but is pooled | Standard context passed to generated service method |
| Contract generation | `std-http-server` or `chi-server` | `fiber-v3-server` | Protobuf + HTTP annotations; OpenAPI output |
| Handler tests | `httptest` + real router | Fiber test API or test listener | Service/biz unit tests plus transport tests |
| Graceful shutdown | `http.Server.Shutdown` | Fiber shutdown API | Kratos app/server lifecycle |
| Migration cost | Baseline | Medium/high transport rewrite | High transport + contract convention shift |

## Context and DB rules that apply to all

1. A shared dependency struct is for application-lifetime objects, not request context.
2. Pools are created once and closed at shutdown. Repositories and use cases receive dependencies by constructor.
3. Per-request context flows transport → use case → repository → DB and is never stored on a long-lived service.
4. Context values are for request metadata needed across boundaries (trace, identity, correlation), not arbitrary parameters or dependency lookup.
5. Health liveness should not require the DB. Readiness can query dependency state, with a bounded timeout.
6. Do not retain Fiber's pooled context past its handler; no request context should be used as a detached job lifetime.

## Recommendation

For this repository's OpenAPI-first, REST-focused, router-flexible goal, use **Chi**. Use generated Chi transport boilerplate at the edge, hand-write application use cases and repository ports, and wire everything in `cmd/api`. If the preference is fewer dependencies above all, choose `ServeMux`; if the goal changes to protobuf-defined multi-transport services, prototype Kratos. Adopt Fiber only if the team specifically values its ergonomics or equivalent-workload measurement supports the tradeoff.

There is no reason to choose on route count alone. Large services keep route surfaces manageable through feature ownership and centralized composition; Grafana's API code groups routes by resources and scopes middleware, while Argo CD's server builds long-lived service dependencies and constructs HTTP servers around them. Borrow those organization patterns, not their application-specific complexity.

## Sources

- [Go 1.22 release notes: enhanced routing patterns](https://go.dev/doc/go1.22)
- [Chi router README and interfaces](https://github.com/go-chi/chi)
- [Grafana API route registration](https://github.com/grafana/grafana/blob/main/pkg/api/api.go)
- [Argo CD API server composition](https://github.com/argoproj/argo-cd/blob/master/server/server.go)
- [Fiber v3 context changes](https://docs.gofiber.io/whats_new/)
- [Fiber route grouping](https://docs.gofiber.io/guide/grouping/)
- [Fiber v3 pooled-context architecture](https://docs.gofiber.io/extra/internal/)
- [oapi-codegen configuration](https://github.com/oapi-codegen/oapi-codegen/blob/main/docs/configuration.md)
- [oapi-codegen Fiber v3 server](https://github.com/oapi-codegen/oapi-codegen/blob/main/docs/fiber-v3-server.md)
- [Kratos transport overview](https://go-kratos.dev/docs/component/transport/overview/)
- [Kratos project template](https://github.com/go-kratos/kratos-layout)
- [Kratos v2 to v3 migration guide](https://github.com/go-kratos/kratos/blob/main/docs/migration/v2-to-v3.md)

