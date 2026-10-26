# API framework samples for TaskForge

These are **architecture samples**, not a second production application. They show how the same Organization/User API might be assembled with Chi, Fiber v3, or Kratos. The existing repository is a persistence comparison lab; these samples do not alter its root `go.mod`, existing database examples, or root `main.go`.

## Contents

- [`GoLang-Framework-Context.md`](GoLang-Framework-Context.md) — original project context and goals.
- [`RECOMMENDATION.md`](RECOMMENDATION.md) — recommendation, shared architecture, and portability boundaries.
- [`COMPARISON.md`](COMPARISON.md) — feature and developer productivity comparison.
- [`chi/`](chi/) — OpenAPI-first REST sample on `net/http` + Chi.
- [`fiber/`](fiber/) — OpenAPI-first REST sample on Fiber v3.
- [`kratos/`](kratos/) — protobuf-first HTTP/gRPC service sample.

The snippets intentionally stop at seams where a real implementation depends on authentication policy, API semantics, or the persistence experiment. They are illustrative, and aren't claimed as independently buildable modules.

