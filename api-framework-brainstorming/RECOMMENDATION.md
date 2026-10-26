# Recommendation and common design

## Recommendation for this repository

Start with **Chi on `net/http`**, and keep application logic independent of either. Choose the standard library `ServeMux` instead if the API stays small enough that explicit route registration is comfortable. The repository currently has persistence examples, but no API routes, HTTP dependencies, or API contract. Chi adds useful route composition and scoped middleware while preserving the standard Go handler, request, response, and context types.

Fiber v3 is a valid alternative if its API ergonomics or a representative benchmark justifies the different HTTP stack. Kratos is a better fit if TaskForge becomes a service platform where protobuf contracts, generated HTTP + gRPC bindings, and framework conventions are desired. For the currently stated OpenAPI-first REST workflow, Kratos changes the contract authoring model and adds more framework structure.

## Shared dependency container vs request context

Use a startup-owned dependency container (ordinary struct fields), and a separate standard `context.Context` for each request. If there are multiple DB pools, group those long-lived handles under a `Stores`/`Resources` field on the shared application struct:

```text
main/composition root
  ├─ config and logger
  ├─ Stores{PrimaryDB, ReadOnlyDB, ...} (long-lived pools)
  ├─ repository adapters (hold pool references)
  ├─ application services/use cases (hold repository interfaces)
  └─ HTTP transport (holds application interfaces)

each request: request context → transport → use case → repository → DB call
```

`App`/`Dependencies` is not a context. Do not put pools, repositories, services, or request-specific state in a global, nor use `context.Value` as a service locator. Construct resources once, inject them through constructors, and close every pool during graceful shutdown. Pass the request's `context.Context` to every operation that can block, especially DB calls. Request IDs, authenticated principals, and trace state may be placed into that request context by middleware if downstream calls need them. Use typed private keys for context values.

Do not keep a request context, framework context, body buffer, or mutable request data after the handler returns. For asynchronous work, copy the required values and use a separately managed job context/lifecycle.

Fiber v3's `fiber.Ctx` now implements `context.Context`, but it is a pooled request object. Pass it only through synchronous request processing and do not retain it. For work that outlives the request, derive/copy what is needed into a normal context and owned values. Fiber v2 differs; don't mix its `UserContext` guidance with v3.

## Dependency direction

```text
domain (only if the invariants merit a domain package)
    ↑
application feature (use cases + the repository ports it consumes)
    ↑                                      ↑
HTTP transport                       PostgreSQL adapters (pgx/sqlc/Bob)
    ↑                                      ↑
composition root wires concrete implementations and server lifecycle
```

The HTTP transport owns path/query/body parsing, API DTO conversion, transport validation, response encoding, and mapping application errors to HTTP status/error schema. Application code owns business decisions and uses plain Go input/output types plus `context.Context`. Persistence implementations adapt the existing sqlc/Bob/etc. code to the feature repository interface. Keep generated OpenAPI models at the API edge if their wire shape differs from application types.

## Feature route composition

The root server owns cross-cutting middleware and version prefixes. Each feature owns its routes, handlers, and feature-level middleware needs. A feature registration function receives its constructed handler explicitly; it doesn't open DB connections or fetch services from globals. Add route-level auth only where policy differs, and centralize shared authn/authz mechanisms without forcing every route to use the same policy.

Example route shape:

```text
/health/live                 liveness; no DB dependency
/health/ready                readiness; checks required dependencies
/api/v1/organizations        organizations feature
/api/v1/users                users feature
/api/v1/projects             projects feature
```

Route groups are organizational and middleware scopes, not separate business layers. Avoid one giant routes file and avoid a custom generic router interface unless multiple live transports actually need the same registration contract.

## OpenAPI workflow (Chi or Fiber)

Use `api/openapi.yaml` as canonical HTTP contract and pin `oapi-codegen` in the tool manifest or `go tool` dependency. The configs in these samples are aligned with the documented v2.8.0 schema; pin the corresponding generator version (for example `go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0`) rather than using `@latest`. Commit generator config and generated files. Keep generated files clearly marked; never hand-edit them. Run generation in CI and fail if it changes the working tree. Generate exactly the target used by that sample: `chi-server` for Chi, `fiber-v3-server` for Fiber v3, or `std-http-server` if choosing only standard `net/http`. Current generator docs list Fiber v3 as requiring Go 1.25+.

Example invocation after pinning the selected generator version with a `tool` directive or tools manifest:

```sh
go tool oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml
git diff --exit-code -- api/gen
test -z "$(git status --porcelain --untracked-files=all -- api/gen)"
```

The final status check also catches newly generated untracked files, which `git diff` alone misses. Run these checks from a clean checkout. If the main module shouldn't own the generator dependency, use a pinned tools module/manifest for the API example. Schema validation, backward-compatibility checks, and API behavior tests remain separate: generation doesn't implement business validation or authorization.

## Migration boundary

Switching Chi ↔ `ServeMux` ↔ Fiber changes route registration, middleware, generated server wrapper/config, and server bootstrap. It may change HTTP tests and some transport adapters. It should not change domain invariants, application use cases, repository interfaces, or database adapters. Moving to Kratos is a larger shift: protobuf service definitions, generated adapters, service interfaces, error mapping, and bootstrap all change, while much of the application and persistence logic can remain if contracts are kept out of it.

No framework gives a zero-cost swap. This boundary aims for a small, honest transport rewrite rather than a lowest-common-denominator router abstraction.

