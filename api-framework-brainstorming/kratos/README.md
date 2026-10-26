# Kratos sample — protobuf-first HTTP + gRPC

Kratos uses a different contract workflow from the other two samples. Define a protobuf service, annotate HTTP bindings, generate Go/protobuf/HTTP/gRPC code, then implement the generated service interface. OpenAPI can be generated from protobuf annotations. This makes Kratos a compelling option when protobuf and multiple transports are first-class requirements; it is a less direct fit if the team wants to author OpenAPI YAML as the canonical contract.

```text
kratos/
├── api/taskforge/v1/organization.proto  # canonical protobuf contract
├── cmd/api/main.go                      # config + app lifecycle
├── internal/
│   ├── conf/                             # generated config types
│   ├── server/{http.go,grpc.go,server.go}# transport construction
│   ├── service/organization.go           # generated service interface adapter
│   ├── biz/{organization.go,errors.go}    # use cases and repository port
│   └── data/{data.go,organization.go}    # pgx pool + adapter
├── third_party/google/api/              # HTTP annotation protos
└── openapi.yaml                         # generated API document
```

This follows the recognizable `service`/`biz`/`data` and `server` pattern in Kratos templates. The example targets **Kratos v3** (module imports use `/v3`). Check the template/CLI version before copying it: the tagged `kratos-layout` releases may target a different major. Keep domain decisions in `biz`; `service` translates protobuf transport messages to use-case inputs. Don't put database access in handlers or generated files.

## Contract and HTTP route

```proto
syntax = "proto3";
package taskforge.api.v1;
option go_package = "example.com/taskforge/api/taskforge/v1;v1";

import "google/api/annotations.proto";

service OrganizationService {
  rpc GetOrganization(GetOrganizationRequest) returns (Organization) {
    option (google.api.http) = { get: "/api/v1/organizations/{id}" };
  }
  rpc CreateOrganization(CreateOrganizationRequest) returns (Organization) {
    option (google.api.http) = {
      post: "/api/v1/organizations"
      body: "*"
    };
  }
}
message GetOrganizationRequest { string id = 1; }
message CreateOrganizationRequest { string name = 1; string slug = 2; }
message Organization { string id = 1; string name = 2; string slug = 3; }
```

Generated HTTP registration binds the annotated routes to an implementation. In a normal Kratos service, configure a Kratos HTTP server, register generated bindings, add server middleware, and pass HTTP/gRPC servers to `kratos.New(...).Run()`. The framework owns transport lifecycle and graceful shutdown.

## Wiring and request context

```go
// cmd/api composition root (schematic; Kratos imports use /v3)
pool, err := pgxpool.New(startupCtx, cfg.Data.Database.Source)
if err != nil { return err }
dataLayer := data.NewData(pool)
orgRepo := data.NewOrganizationRepo(dataLayer)
orgUseCase := biz.NewOrganizationUseCase(orgRepo)
orgService := service.NewOrganizationService(orgUseCase)

httpServer := server.NewHTTPServer(cfg, orgService, authMiddleware, accessLog)
grpcServer := server.NewGRPCServer(cfg, orgService, tracing, recovery)
app := kratos.New(kratos.Name("taskforge"), kratos.Server(httpServer, grpcServer))
defer pool.Close()
return app.Run()
```

Each generated RPC method receives the request's standard `context.Context`; pass it through `service` and `biz` into the repository and DB call. Kratos middleware is transport/server middleware and can also be applied at operation/service levels depending on the concern. Long-lived pools belong to `data`/composition, never the request context. Generated message types are transport contract types; map them to application/domain types at the service boundary where responsibilities differ.

## Middleware, routing, and error shape

Organize middleware under `internal/server/middleware` or focused packages (auth, recovery, logging, tracing), and register shared middleware when constructing each transport. HTTP and gRPC can share auth/trace logic but have different transport metadata and error encoding. Feature API definitions remain in protobuf service files, while generated route code should never be hand-edited.

Representative Kratos v3 middleware shape:

```go
func AccessLog(logger *slog.Logger) middleware.Middleware {
    return func(next middleware.Handler) middleware.Handler {
        return func(ctx context.Context, req any) (reply any, err error) {
            started := time.Now()
            reply, err = next(ctx, req)
            logger.InfoContext(ctx, "request complete",
                "duration", time.Since(started), "error", err)
            return reply, err
        }
    }
}
```

In an actual server, prefer the framework's logging/recovery middleware where it meets the policy. Place logging outside recovery if the log should see recovered failures, and redact secrets rather than logging generated request messages blindly.

Kratos route grouping is typically expressed through service definitions and generated bindings, rather than manually authored Chi-style nested route callbacks. For a large surface, split protobuf files/services by feature and keep server registration centralized. Define application error categories in `biz`; map them to Kratos/HTTP/gRPC status representations in the transport service adapter.

## When this sample is preferable

Choose Kratos when you value its HTTP + gRPC integration, protobuf-first contract, middleware conventions, and generated project layout enough to adopt framework-shaped service boundaries. Choose Chi when direct OpenAPI-first REST, plain `net/http`, and low transport coupling matter more. Kratos does not mean no custom code: authorization, business validation, compatibility policy, persistence, and deployment-specific observability still need explicit design.

