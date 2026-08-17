# annotation-service

The simplest service in the catalog — see
[`specs/backend-go/services/annotation-service.md`](../../../specs/backend-go/services/annotation-service.md)
for the full design. It owns inline code-review comments anchored to a
file+line, behind a thin gRPC face over a single Postgres table. It follows
the same Clean Architecture package layout as
[`usage-service`](../usage-service/README.md), the pilot reference
implementation — see that service's README for the layout rationale in
depth.

## What's implemented

- `internal/domain/` — `Annotation` entity + `Anchor` value object
  (`repo_id`/`file_path`/`line`/`ref`) with invariant-enforcing
  constructors, pure unit tests.
- `internal/usecase/` — `CreateAnnotation`, `ListAnnotations`,
  `UpdateAnnotation`, `DeleteAnnotation`, each tested against an in-memory
  fake `Repository`, no real Postgres needed. Tenant/author identity is
  pulled from context (`common/tenant`), never trusted from the request.
- `internal/adapter/postgres/` — real `pgx`-backed repository, hand-written
  SQL (see `architecture/04-tech-stack.md` — `sqlc` codegen is the eventual
  target; this scaffold writes the equivalent queries directly).
- `internal/adapter/grpc/` — implements the generated
  `annotationv1.AnnotationServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL: `annotation.annotations`,
  a `tenant_id` index, RLS policy.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM. No
  NATS/eventbus wiring — per the design doc, this service publishes no
  events and has no `internal/adapter/eventbus/` package.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/annotation-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/annotation-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/annotation?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **No `sqlc` codegen wired** — same rationale as `usage-service`: valid
  destination per the tech stack doc, just not the codegen-checked default.
- **`common/secrets` (Vault) is not wired into `main.go`** —
  `DATABASE_DSN` is read directly from the environment for local dev; wire
  `secrets.DatabaseCredentialsFromFile` before this service is deployed
  anywhere Vault is actually running.
- ~~`common/tracing` has no OTLP exporter configured~~ — **closed**
  (`docs/execution-plan.md` §3 Phase 1, fixed in shared `common/tracing`):
  `Init` now batches spans to a real `otlptracegrpc` exporter whenever
  `OTLP_ENDPOINT` is set.
- **Author-only edit/delete via OPA is now enforced** —
  `UpdateAnnotation`/`DeleteAnnotation` fetch the target annotation
  (`Repository.GetAnnotation`) after the tenant/ID/content checks, then
  call `internal/adapter/opaclient` (wrapping `common/policy.Evaluator`
  against `data.orca.authz.annotation.allow` in
  `backend-go/policy/orca-authz/annotation.rego`) with the caller's actor
  id and the annotation's author id, fail-closed and before the mutation
  runs. A denial maps to `apperrors.KindPermissionDenied`
  (`ANNOTATION_NOT_AUTHOR`). **Known gap:** the policy input's
  `actor_role` is always sent as `""` — this service's request context
  (`common/tenant`, populated by `grpcmw.TenantExtractionInterceptor`)
  only ever carries `tenant_id`/`user_id` today; no role claim is
  propagated from `api-gateway` (see `api-gateway`'s
  `AttachIdentity`/`grpcmw.MetadataTenantID`/`MetadataUserID` — there is
  no `MetadataUserRole` counterpart anywhere in this codebase yet). The
  Rego rule's admin-override branch (`actor_role == "admin"`) is
  therefore unreachable from this service until role propagation is
  added upstream; it's exercised only at the Rego/evaluator level
  (`policy/orca-authz/annotation_test.rego`'s `test_admin_override`,
  `common/policy/evaluator_test.go`'s `TestEvaluator_AnnotationDecision`),
  not from an annotation-service usecase test, since faking that path
  here would mean fabricating a role the service can't actually observe.
  `OPABundlePath` is configurable via `OPA_BUNDLE_PATH`, defaulting to
  `../../policy/orca-authz`.
- **`project_id` validation against `project-service` is not called** — the
  design doc's §7 notes this service *may* call `GetProject` to validate a
  new annotation's `repo_id` before persisting; not wired in this scaffold.
- ~~The proto's `CreateAnnotationRequest.request_id` field is accepted but
  not used for write idempotency~~ — **closed** (`docs/execution-plan.md`
  §3 Phase 1): migration `0002_annotation_request_id` adds `request_id
  TEXT NOT NULL` + `UNIQUE (tenant_id, request_id)`, matching
  `usage-service`/`automation-service`'s identical convention.
  `CreateAnnotation` now requires `request_id` (`ANNOTATION_NO_REQUEST_ID`
  on empty) and checks `Repository.FindByRequestID` before inserting,
  returning the existing row on a retry instead of a duplicate — including
  a re-check on insert failure to absorb a genuine unique-constraint race,
  the same pattern `automation-service.RunNow` uses.
