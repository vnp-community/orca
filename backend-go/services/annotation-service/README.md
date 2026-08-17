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
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
- **Author-only edit/delete via OPA is not enforced here** — per the design
  doc §9, `UpdateAnnotation`/`DeleteAnnotation` ownership checks are an OPA
  policy decision at the gateway, not an inline usecase check; this
  scaffold enforces tenant isolation only.
- **`project_id` validation against `project-service` is not called** — the
  design doc's §7 notes this service *may* call `GetProject` to validate a
  new annotation's `repo_id` before persisting; not wired in this scaffold.
- The proto's `CreateAnnotationRequest.request_id` field is accepted but
  not used for write idempotency — the `annotations` table has no
  `request_id` column (unlike `usage-service`'s sessions table), matching
  the design doc's minimal §5 schema for this service.
