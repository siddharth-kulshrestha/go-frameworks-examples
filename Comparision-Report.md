# BobGen and sqlc Comparison Report

**Date:** 2026-10-25  
**Objective:** Explain how BobGen (Bob's code generator) and sqlc approach database access, compare their tradeoffs, and recommend when each is a better fit, including for this repository.

## Executive summary

These tools overlap, but they start from different centers of gravity:

- **sqlc starts with SQL.** You maintain schema definitions and named SQL queries; sqlc analyzes them and generates typed Go methods and result/parameter types. The generated API closely reflects the SQL you chose.
- **BobGen starts with the database schema.** It generates table models, setters, query helpers, relationship loaders, and (optionally) factories. Bob also includes a composable SQL builder and an optional SQL-query code-generation plugin.

Choose **sqlc** when the important part of the persistence layer is a carefully curated set of explicit SQL statements. Choose **Bob** when the application benefits from generated table-oriented CRUD, typed query composition, relationship loading, and schema-derived test factories. Bob can also generate Go APIs for handwritten SQL, so the choice need not be all-or-nothing.

For TaskForge, the current SQLC implementation is a natural fit for a small set of deliberate queries. Bob becomes more compelling as the domain grows toward the multi-entity, relationship-heavy model described in the project requirements, especially if its generated relationship and factory features are used. Keep both behind the existing service repository interface while comparing them against the same behavioral tests and database workloads.

## Objective and scope

This report compares the tools as used to build Go data-access layers, rather than treating either as a complete application framework. It covers:

1. What each generator takes as input and produces.
2. Query authoring, type safety, CRUD, relationships, testing, and operational tradeoffs.
3. Scenarios where one has a practical advantage over the other.
4. A recommendation for this repository and guidance for other project types.

This is a code and capability comparison, not a performance benchmark. The repository currently implements only organization create/read in each adapter, so no throughput or latency winner can be inferred from it.

## How each tool works

### sqlc: SQL source to typed methods

The workflow is query-oriented:

1. Maintain schema SQL (or migrations) and query SQL.
2. Annotate each query with its generated operation name and result cardinality, such as `:one` or `:many`.
3. Run sqlc to analyze the query against the schema and generate Go parameter structs, result structs, and methods.
4. Call those methods through a generated `Queries` type backed by a database interface.

The generated API remains close to the source SQL: each operation is an explicit query with a specific input and result shape. The query author decides which columns to select, how to join, and how to express the database operation. sqlc replaces repetitive Go execution and scanning code; it does not replace SQL with an ORM abstraction. Its documentation describes this SQL → generated typed interface → application workflow in the [sqlc overview](https://docs.sqlc.dev/en/latest/) and [row retrieval guide](https://docs.sqlc.dev/en/latest/howto/select.html).

**In this repository:** [`sqlc/sqlc.yaml`](sqlc/sqlc.yaml) points at migration and query directories, targets PostgreSQL, and emits pgx/v5 code. [`sqlc/db/queries/organizations.sql`](sqlc/db/queries/organizations.sql) defines `CreateOrganization` and `GetOrganization`; generated methods live in `sqlc/internal/store/sqlc/organizations.sql.go`. The adapter in [`sqlc/repository/organization.go`](sqlc/repository/organization.go) converts between generated pgx/PostgreSQL types and the domain model.

### Bob and BobGen: schema-generated toolkit

Bob is a collection of components, and BobGen is the generator within it. Its schema-driven generation can provide:

- Model structs and setters for tables.
- Table operations, primary-key lookup helpers, and typed column expressions.
- Relationship metadata and helpers for loading related records.
- Factories for making model instances and inserting them, including related records.
- Optional generated APIs for handwritten SQL queries.

The generated model layer provides convenience without requiring every operation to be handwritten. The query builder is composable; generated model column/query helpers provide stronger type guidance than manually spelling table and column names. Bob's own documentation lists these pieces as separate plugins, including models, factories, relationship helpers, and query generation ([configuration and plugins](https://bob.stephenafamo.com/docs/code-generation/configuration/), [model usage](https://bob.stephenafamo.com/docs/code-generation/usage/)).

Bob's query-generation plugin also accepts `.sql` files. Those generated queries can be composed with Bob query modifications and executed with common finishers such as `One`, `All`, and `Cursor` ([Bob query generation](https://bob.stephenafamo.com/docs/code-generation/queries/)). Therefore, the distinction is not “Bob cannot use handwritten SQL”; it is that Bob additionally offers a schema-derived model/query layer.

**In this repository:** [`bob/bobgen.yaml`](bob/bobgen.yaml) configures PostgreSQL schema introspection and enables the plugin preset, including models and factories. It maps UUIDs to `github.com/google/uuid`. Generated files include [`bob/models/organizations.bob.go`](bob/models/organizations.bob.go), with an `OrganizationSetter`, `Organizations` table API, and `FindOrganization`; the adapter in [`bob/repository/organization.go`](bob/repository/organization.go) maps those generated types to the domain model. The current config does not point BobGen's query plugin at a SQL query directory, so this checkout currently demonstrates generated models rather than Bob's SQL-query plugin.

## Comparison

| Area | sqlc | Bob / BobGen |
|---|---|---|
| Primary source of truth | Schema SQL plus explicit query SQL | Live database schema for model generation; optional `.sql` files for generated custom queries |
| Main generated API | One typed method per annotated SQL query, with parameter/result types | Table models/setters, generic table operations, primary-key helpers, typed query mods, relationship helpers, and optional custom-query APIs |
| Query style | SQL is written directly and remains visible as the operation's definition | Generated CRUD and typed composable query mods; can also use Bob's SQL builder or query plugin for custom SQL |
| Type safety | Query parsing/analysis checks SQL against schema and derives Go types; Go catches call-site type errors | Generated model fields, setters, and query mods are typed; SQL query generation can infer custom query APIs; raw string expressions bypass some guarantees |
| Dynamic filtering | Usually write separate SQL queries or use supported SQL patterns/macros | Query mods compose at runtime, which is useful for optional filters, ordering, and reusable conditions |
| CRUD boilerplate | Each operation is usually an explicit query and generated method | Common table operations can be generated and reused without defining a dedicated SQL file for every CRUD variant |
| Relationships | Explicit joins and result shapes are authored in SQL; result models reflect the selected columns | Schema relationships can generate relation-aware helpers and loading patterns; supported behavior depends on relation shape and driver |
| Testing support | Generated types are convenient, but test fixture creation is normally project-owned | Optional generated factories can build models and insert related records; useful for integration tests and graph-shaped fixtures ([factory docs](https://bob.stephenafamo.com/docs/code-generation/factories/)) |
| Query review | SQL diffs make exact database operations easy to inspect | Model calls can be concise; for complex behavior, inspect the mods or generated SQL and retain explicit SQL where that aids review |
| Database-specific SQL | Strong fit: the SQL author directly controls database features and syntax | Bob supports multiple dialects, but SQL builder/query-generation behavior and portability vary by dialect; custom SQL remains possible |
| Schema changes | Update schema/migrations and regenerate; incompatible queries can be caught in generation | Update database schema and regenerate; generated models reflect the schema BobGen inspected |
| Extra checks | Includes query linting with `sqlc vet`; `sqlc verify` checks query compatibility against proposed schema changes through sqlc Cloud ([vet](https://docs.sqlc.dev/en/latest/howto/vet.html), [verify](https://docs.sqlc.dev/en/latest/howto/verify.html)) | Generated code and Bob's generated tests can be used in CI; generated relationship and query behavior still needs project-level tests |
| Runtime approach | Generated methods over a driver interface; no general ORM model layer required | Bob executor/scanner and query-builder runtime, plus whatever generated model/query features the app uses |

### Type safety is not identical

Both tools generate compile-time Go types, but the safety is applied at different boundaries:

- With **sqlc**, type analysis is centered on the authored SQL. A query's exact projection, parameters, and result type are checked against the configured schema when generating code. The resulting Go call is explicit and narrow.
- With **Bob**, type safety is strongest when using the generated table/column/query helpers and generated setters. These are derived from the schema and make model-oriented CRUD and query composition convenient. A raw expression or manually assembled SQL string necessarily gives up some generated guarantees.

Neither tool proves that a query implements the intended business rule, that an index is used, or that the query is fast. Both need behavioral tests and production-shaped query review. sqlc's optional `verify` is a useful schema rollout check, but requires the hosted sqlc workflow; it is not a local performance or business-correctness proof.

## Where BobGen shines

Bob is likely to be the better fit when several of these conditions hold:

1. **The application has many tables and ordinary CRUD operations.** Generated table operations reduce repetitive query definitions and adapters.
2. **Queries are assembled from optional filters.** Composable typed query mods can be more convenient than maintaining many near-duplicate SQL statements.
3. **The domain has substantial relationships.** Generated relation helpers can support loading and inserting related records. Bob documents preload and then-load approaches; note that the current docs limit preload to `to-one` relations, so confirm the precise relation pattern required ([relationship guide](https://bob.stephenafamo.com/docs/code-generation/relationships/)).
4. **Integration tests need realistic related fixtures.** Generated factories can create table instances and related data, cutting down handwritten fixture setup.
5. **The team wants a toolkit with multiple levels.** It can use direct SQL, Bob's builder, generated table models, factories, or generated SQL APIs as needed.
6. **The codebase has schema-driven conventions.** Generated models and setters help keep repetitive persistence code consistent across tables.

For TaskForge's planned project-management domain, Bob's advantage would grow as Organizations, Users, Teams, Projects, Issues, Comments, Labels, and joins are implemented. Relationship loading and graph fixture creation could remove significant repeated plumbing compared with writing and mapping every operation manually.

## Where sqlc outdoes Bob

sqlc is likely to be the better fit when:

1. **The workload is SQL-heavy or query-specific.** Reports, CTEs, window functions, complex joins, carefully tuned updates, and PostgreSQL-specific features stay first-class source code rather than being expressed through a builder.
2. **The team wants a small, explicit persistence surface.** Each use case has a named query with a deliberate input and result rather than exposing a broader model API.
3. **Query review and database control are priorities.** Reviewers can inspect the exact SQL that executes, and database specialists can tune it without translating from a higher-level abstraction.
4. **The schema and query files are already migration-driven.** sqlc works naturally when schema history and SQL statements are versioned and reviewed as the contract.
5. **Avoiding ORM-like conventions is important.** Generated code stays focused on execution and scanning; the application chooses its own domain model and repository boundaries.
6. **You want query-level linting/schema rollout tooling.** sqlc provides `vet`, and its hosted `verify` workflow can detect some backwards-incompatible changes to existing queries.

For an application with a small number of high-value SQL operations, sqlc can be simpler to reason about and maintain than generating a broad model API that most features do not need.

## When to use what

| Project shape | Recommended starting point | Reason |
|---|---|---|
| Small service with a few hand-tuned queries | **sqlc** | Minimal abstraction; direct SQL and precise generated method signatures |
| Analytics/reporting service with complex SQL | **sqlc** | SQL itself is the core product logic and deserves to be explicit |
| CRUD-heavy internal app with many related entities | **Bob** | Generated models, typed query mods, relationship helpers, and factories can reduce repetitive work |
| API with many optional filters and reusable query conditions | **Bob** | Composable query mods are useful when query structure changes from request to request |
| Legacy database where schema must be introspected | **Bob**, if model generation fits | BobGen can derive model code from the inspected schema; first verify custom type and naming mappings |
| Domain where each use case has bespoke transactional SQL | **sqlc** | Explicit statements make transaction scope and database behavior easy to audit |
| Mixed CRUD and complex bespoke SQL | **Either, or both behind repository interfaces** | Use Bob for generated table workflows and sqlc for especially complex statements, while guarding against duplicated ownership of the same operations |

## Recommendation for this repository

The repository already separates the service from persistence through [`service.OrganizationRepository`](service/organization.go). [`main.go`](main.go) can select the SQLC or Bob adapter with `-repository=sqlc` or `-repository=bob`, so both implementations can exercise the same service flow.

**Recommended path:**

1. Keep **sqlc** as the reference implementation for explicit SQL and query-centric workflows.
2. Continue the **Bob** implementation for schema-generated models, reusable CRUD, relationships, and test factories as the domain expands.
3. Add the same repository contract and behavioral integration tests for both adapters before using results to choose a default. Run the same scenarios, schema, driver, and transaction behavior; add focused benchmarks only if performance is a decision criterion.
4. Prefer one owner for each query path. If both tools remain, document which adapter owns which operations so the implementations do not drift.

At the current organization-only scope, sqlc's explicit `CreateOrganization` and `GetOrganization` methods are easy to follow and are likely the simpler choice. Bob's benefit is not yet demonstrated by this small example. Its case improves when the repository gains enough tables and relationships for its generated helpers and factories to remove real work.

### Repository-specific setup considerations

These points describe this checkout, not universal properties of either project:

- The SQLC module declares Go `1.25.6`; the Bob module declares Go `1.27.0`. Importing Bob from the root module caused `go mod tidy` to raise the root module's Go directive to `1.27.0` in the current setup.
- SQLC uses the `pgx/v5` package directly. Bob uses `bob/drivers/pgx` to adapt the existing `pgxpool` to `bob.Executor`.
- BobGen's current PostgreSQL config expects access to a database schema when regenerating. The SQLC generation input is stored as migration and query SQL in the repository. Ensure local/CI generation can supply the Bob schema consistently.
- Both projects must keep generated files in sync with their inputs. Do not hand-edit generated files; put custom behavior in adapter or hand-maintained files.

## Conclusion

Use **sqlc** when SQL is the intentional design surface and you want each database operation visible, individually typed, and easy to audit. Use **Bob** when a growing schema benefits from generated CRUD, typed query composition, relationship support, and factories. For TaskForge, keeping both as interchangeable examples is useful while comparing them, but the codebase should choose a primary owner per operation and decide based on repeated integration behavior—not on a generic claim that one tool is always faster or safer.

## Sources

- [sqlc documentation overview](https://docs.sqlc.dev/en/latest/)
- [sqlc query generation and retrieval](https://docs.sqlc.dev/en/latest/howto/select.html)
- [sqlc configuration reference](https://docs.sqlc.dev/en/latest/reference/config.html)
- [sqlc vet](https://docs.sqlc.dev/en/latest/howto/vet.html)
- [sqlc verify](https://docs.sqlc.dev/en/latest/howto/verify.html)
- [BobGen configuration and plugins](https://bob.stephenafamo.com/docs/code-generation/configuration/)
- [Bob generated model usage](https://bob.stephenafamo.com/docs/code-generation/usage/)
- [Bob relationships](https://bob.stephenafamo.com/docs/code-generation/relationships/)
- [Bob factories](https://bob.stephenafamo.com/docs/code-generation/factories/)
- [Bob generated SQL queries](https://bob.stephenafamo.com/docs/code-generation/queries/)
