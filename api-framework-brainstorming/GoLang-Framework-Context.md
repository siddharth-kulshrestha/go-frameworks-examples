Context for Codex: Go API Framework Evaluation and Architecture
1. Objective
Help me choose and structure an API stack for a Go application. My immediate comparison is:
1. Fiber — a full-featured HTTP framework built on fasthttp.
2. Chi — a lightweight router that uses Go's standard net/http interfaces.
Use the standard library (net/http) as the baseline, not as an afterthought. I want to understand whether a router/framework is needed at all, what it buys us, and what coupling it introduces.

The decision is not simply about raw requests per second. I care about a maintainable production codebase with clear package boundaries, feature-level route modules, dependency injection, a good testing story, and a path to change the router/framework later without rewriting application logic.

I have also considered Go-Kratos, GoFr, and Go kit in earlier exploration. They are useful context, but the current primary scope is Fiber vs Chi vs net/http for REST APIs. Do not expand the evaluation into every Go framework unless doing so answers a concrete architecture question.
2. Project context
I am experimenting with Go backend architecture and persistence choices, including PostgreSQL and approaches such as sqlc, Bun, and Ent. The domain examples discussed so far are Organization and User. The repo may be a TaskForge learning/lab project; inspect the actual repository before assuming its current structure or dependencies.

Likely API requirements include:
- REST endpoints organized by feature/domain, such as organizations and users.
- PostgreSQL-backed application logic.
- API request/response schemas and a consistent error format.
- Validation, authentication/authorization where needed, structured logging, metrics/tracing, health checks, and graceful shutdown as the project grows.
- Tests for application behavior and HTTP behavior.
- A clear API contract that can be reviewed and used to generate clients.
Do not add all operational capabilities preemptively. Distinguish current requirements from possible future requirements.
3. OpenAPI-first is an important preference
I am interested in writing OpenAPI YAML first and generating Go types/server boilerplate, rather than writing handlers first and generating OpenAPI documentation afterward.

The main candidate is oapi-codegen. Its generation options include standard HTTP, Chi, Fiber v2, and Fiber v3 server boilerplate. Select the correct generator target for the actual framework major version and pin the tool version in the project. Verify generator configuration against the chosen version instead of copying an outdated snippet blindly.

Relevant documentation:
- oapi-codegen configuration
- oapi-codegen repository
Treat api/openapi.yaml as the canonical HTTP contract if that is the chosen design. Generated files must be reproducible; do not hand-edit generated code. Explain how regeneration and CI drift checks should work.

OpenAPI-first does not mean OpenAPI automatically implements business validation, authorization, or backwards compatibility. Specify what is handled by generated code, transport adapters, and application logic separately.
4. Framework facts and trade-offs to evaluate
A. Standard library: net/http
- Go 1.22 added method-aware routing patterns and path wildcards to http.ServeMux, including patterns such as GET /items/{id} and access via r.PathValue("id").
- It provides standard http.Handler, http.HandlerFunc, request/response types, middleware composition, contexts, and server lifecycle primitives.
- It is a valid baseline for a fairly substantial API; do not claim it inherently cannot support modular routes or middleware.
- It has fewer routing conveniences than Chi, and some operational concerns require explicit composition or third-party packages.
Official reference: https://go.dev/doc/go1.22
B. Chi
- Chi is a lightweight router based on a Patricia/radix trie and is compatible with net/http.
- Handlers use http.Handler / http.HandlerFunc; it supports route groups, nested routes, mounted subrouters, and scoped middleware.
- It works naturally with standard Go middleware and many standard HTTP observability libraries.
- It does not force a full application architecture, database library, dependency-injection framework, or API contract format.
- Chi is attractive when composable modular routing and broad net/http compatibility matter more than a framework-specific handler API.
Official references:
- https://github.com/go-chi/chi
- https://github.com/go-chi/chi/blob/master/README.md
C. Fiber
- Fiber offers an Express-like handler and routing experience and uses fasthttp rather than the standard net/http server interfaces.
- Fiber can be productive for REST APIs and offers routing, groups, middleware, and convenient request/response APIs.
- Middleware written for net/http is not automatically interchangeable with Fiber middleware. Adapters exist, but verify their capabilities, overhead, and limitations for each integration.
- fiber.Ctx is pooled/reused and must not be retained after the handler returns. Do not pass the context itself into long-lived goroutines. Copy request-derived values needed by asynchronous work and pass an appropriate standard context.Context.
- Be precise about Fiber v2 vs v3 APIs; do not mix handler signatures, middleware, adapters, or generator settings between major versions.
- Do not repeat unqualified marketing claims such as “astronomical RPS.” Any performance advantage must be demonstrated with repeatable, workload-relevant benchmarks.
Official references:
- https://github.com/gofiber/fiber
- https://docs.gofiber.io/guide/go-context/
- https://docs.gofiber.io/middleware/adaptor/
D. Routing order and performance caveats
Do not blindly accept these brainstorming claims as universal facts:
- “Fiber always requires every static route to be registered before every parameter route.” Verify routing precedence and middleware ordering against the exact Fiber version and route patterns being considered. Clearly distinguish route matching order from middleware execution order.
- “Fiber's performance advantage is always lost if values are copied into ordinary Go types.” A boundary may incur allocations, but copying a small number of values does not automatically erase a performance advantage. Measure it.
- “Chi is always better for enterprise” or “Fiber is only for proxies/Node.js migrants.” These are overly broad generalizations.
- “Thousands of routes require a framework.” Route count alone does not determine the architecture. Module boundaries, ownership, middleware scope, tests, and the ability to navigate the codebase matter more.
Use benchmarks only with equivalent behavior and configuration. Separate router/serialization microbenchmarks from end-to-end database-backed latency. Measure allocations, latency percentiles, throughput, CPU, and memory with a repeatable methodology before making performance claims.
5. Architecture goals
The key requirement is framework isolation without unnecessary abstraction. I want to be able to change from Fiber to Chi or net/http without rewriting business rules, persistence, or most application tests. I understand that the HTTP adapter, routing, middleware, generated server wrappers, and bootstrap code may need to change; do not promise a zero-change framework migration.

Prefer boundaries like these:
1. Domain — business concepts and invariants; no fiber.Ctx, http.Request, router imports, or transport concerns. Avoid putting HTTP serialization tags in core domain types unless there is a specific reason.
2. Application/use cases — operations such as GetOrganization, CreateOrganization, GetUser; use ordinary Go types, explicit dependencies, and context.Context. No Fiber/Chi-specific objects. Keep business decisions here, not in handlers.
3. Ports/repository interfaces — defined where useful to the consuming application/use-case code; implementations can use sqlc, Bun, Ent, or another persistence option. Do not create interfaces for every type just for the sake of abstraction.
4. Transport/HTTP adapter — router-specific request extraction, path/query parsing, request validation at the transport boundary, conversion from API DTOs to application inputs, application invocation, response encoding, and mapping application errors to HTTP status codes.
5. Composition/bootstrap — construct repositories, application services, handlers, middleware, and the server; own configuration and graceful shutdown.
Do not call application services “controllers” unless there is a deliberate reason. Keep HTTP handler/transport code distinct from application use cases. Keep API DTOs/generated OpenAPI models distinct from domain entities when their responsibilities or fields differ.

A possible starting layout (adapt to the actual repo and avoid empty packages):
.
├── cmd/
│   └── api/
│       └── main.go
├── api/
│   ├── openapi.yaml
│   └── gen/                 # generated files, if that matches the generator setup
├── internal/
│   ├── domain/              # core entities/invariants, if needed
│   ├── organization/
│   │   ├── service.go       # use cases/application logic
│   │   └── repository.go    # ports consumed by this feature
│   ├── user/
│   │   ├── service.go
│   │   └── repository.go
│   ├── persistence/
│   │   └── postgres/        # concrete SQL/ORM implementations
│   └── transport/
│       └── http/
│           ├── routes.go    # router composition and common middleware
│           ├── organization.go
│           ├── user.go
│           └── errors.go
└── migrations/
This is a proposal, not a mandate. Feature-oriented packages (organization, user) can be better than a giant generic handlers package. Avoid circular imports and avoid importing a web framework from domain/application/persistence packages.
6. Route organization and dependency injection
I want feature-level route modules rather than a very large main.go or one giant routes file. Each feature should expose a small route-registration function or adapter that receives its dependencies explicitly.

For example, with Chi, feature routes may be mounted under /api/v1/organizations and /api/v1/users; Chi supports Route, Group, and Mount. With Fiber, equivalent route groups or registration functions can be used. Keep router syntax inside the HTTP transport package.

Prefer constructor injection in the composition root, e.g. NewOrganizationService(repo) and NewOrganizationHTTPHandler(service). Avoid global database connections, global service state, service locators, or reflection-based DI unless justified.

Use dependency inversion in a pragmatic way:
- Application/use-case code should depend on the interfaces it needs, not on a concrete database or web framework.
- Transport adapters depend on the application service.
- Concrete repositories implement the application's repository interfaces.
- Do not invent a generic Router interface just to claim portability unless it provides real value. Chi, Fiber, and net/http have different handler and middleware models; a few clean adapters are often simpler than a lowest-common-denominator router abstraction.
7. Error handling, contexts, and background work
- Define stable application error categories (for example, not found, invalid input, conflict, unauthenticated/forbidden, internal) without storing HTTP status codes in core domain errors unless there is a compelling reason.
- Map application errors to HTTP status codes and a consistent response schema in the transport layer.
- Ensure logs and responses do not leak SQL errors, stack traces, secrets, or internal implementation details.
- Pass standard context.Context from the transport into application and repository calls for cancellation/deadline propagation.
- With Chi/net/http, use r.Context().
- With Fiber, understand the exact version's context semantics. Do not assume Fiber's handler context is cancellable in the same way as net/http. Follow current Fiber guidance for obtaining a suitable context.Context and copying request values before work outlives a request.
- Do not launch goroutines that capture and retain a request/response context, pooled buffer, or mutable request data beyond its valid lifetime.
8. Middleware and production concerns to compare
Evaluate the minimum set the application genuinely needs:
- Request ID / correlation IDs.
- Structured access logging.
- Panic recovery.
- Authentication and authorization.
- Request size limits and content-type handling.
- CORS only if the API needs it.
- Rate limiting only if required by deployment/security needs.
- OpenTelemetry metrics/tracing and propagation.
- Health/readiness endpoints.
- Timeouts, graceful shutdown, and server configuration.
Compare implementation and operational fit in both frameworks. Do not count middleware as “built in” without specifying which feature/package, limitations, and maintenance implications. Pay particular attention to net/http middleware reuse with Chi versus Fiber adapters or Fiber-native middleware.
9. Frameworks considered previously
These were discussed earlier, but are not the main target unless they change the recommendation:
- Go-Kratos: an integrated, modular microservice framework with a natural Protobuf-first API workflow. Its project template can generate HTTP/gRPC bindings and OpenAPI from annotated .proto files. This differs from an OpenAPI-first workflow.
  - https://go-kratos.dev/docs/guide/openapi/
- GoFr: a more batteries-included microservice framework with integrated operational features and datasource integrations. It can serve Swagger UI for a supplied OpenAPI document, but serving a spec is not the same as OpenAPI-driven handler generation.
  - https://gofr.dev/docs
- Go kit: a composable microservice toolkit with explicit endpoint/service/transport boundaries; likely more wiring than a basic REST router.
  - https://github.com/go-kit/kit
Mention these alternatives only when useful. The user currently wants to compare Fiber and Chi and design an HTTP stack with portable application logic.
10. Comparison criteria
Compare Fiber, Chi, and net/http on the same criteria:
- API ergonomics and amount of boilerplate.
- Fit with OpenAPI-first code generation (oapi-codegen).
- How generated handlers/models are integrated and where generated code lives.
- Ability to compose routes by feature and scope middleware.
- net/http and ecosystem compatibility.
- Context, cancellation, request-body and memory-lifetime semantics.
- Testing of application services and transport handlers.
- Authentication, validation, error handling, observability, and graceful shutdown.
- Version/dependency complexity and operational considerations.
- Ease and cost of switching to a different router.
- Runtime characteristics only after measuring equivalent workloads.
State which capabilities come from the framework, which come from third-party libraries, and which we must implement ourselves.
11. What I want Codex to do
First inspect the actual repository: Go version, current module dependencies, directory layout, API contract files, persistence implementation, test conventions, and existing endpoints. Do not assume this context describes the repo perfectly.

Then provide:
1. A concise recommendation among net/http, Chi, and Fiber, based on the actual repo and these requirements.
2. A concrete architecture proposal with package responsibilities, dependency direction, route module composition, and where generated OpenAPI code belongs.
3. An OpenAPI-first workflow with the appropriate oapi-codegen target, pinned tool/version strategy, generation command, and CI check to detect stale generated code.
4. A representative vertical slice for one resource such as Organization: OpenAPI operation, generated transport boundary, application use case, repository interface, PostgreSQL implementation seam, error mapping, and tests. Adapt examples to actual dependencies rather than dumping generic boilerplate.
5. A migration story: specify which files/packages change if we later switch Fiber ↔ Chi or move to standard net/http, and which layers stay unchanged.
6. A list of trade-offs and unresolved decisions, especially anything that depends on traffic profile, deployment, auth, observability, or API contract ownership.
7. If performance is claimed to matter, propose a fair benchmark plan before asserting which router is faster.
Start by analyzing and recommending. Do not perform a wholesale rewrite or add a complex abstraction layer before explaining why it is needed. If implementation is appropriate, make small, reviewable changes, maintain tests, and follow the repository's existing conventions unless there is a clear reason to change them.
12. Decision principle
Optimize for a simple architecture with stable boundaries, not an imagined guarantee that frameworks can be swapped without any cost.

My likely preference is OpenAPI-first + generated transport boilerplate + framework-independent application logic. Chi is attractive because it stays in the standard net/http ecosystem. Fiber remains a candidate if its developer experience or measured runtime characteristics justify its different context and middleware model. Let the concrete requirements and repository evidence decide; do not choose based solely on framework popularity, marketing benchmarks, or route count.