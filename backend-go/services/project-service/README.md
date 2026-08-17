# project-service

See [`specs/backend-go/services/project-service.md`](../../../specs/backend-go/services/project-service.md)
for the full design. Follows the exact package layout and conventions
established by `usage-service` (the Phase 0 pilot) — see that service's
README for the pattern this scaffold copies.

## What's implemented

This scaffold implements the slice of project-service.md's design that the
current `proto/orca/project/v1/project.proto` surface exercises:
`CreateProject`, `GetProject`, `ListProjects`, `AddMember`, `RebindDevServer`.
The design doc's fuller API (`UpdateProject`, `DeleteProject`, member
management beyond `AddMember`, repo/worktree/project-group/source-project
catalogs) is a documented follow-up — see "Known gaps" below.

- `internal/domain/` — `Project`/`ProjectMember`/`ProjectRole` entities with
  invariant-enforcing constructors, pure unit tests.
- `internal/usecase/` — `CreateProject`, `GetProject`, `ListProjects`,
  `AddMember`, and `RebindDevServer` (the guarded rebind saga — see below),
  each tested against in-memory fakes, no real Postgres/gRPC needed.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  `ProjectRepository` against `project.projects` / `project.project_members`.
- `internal/adapter/grpcclient/` — outbound client adapters toward
  workflow-service and task-service for the rebind guard. **Currently
  STUBs** — see "Known gaps".
- `internal/adapter/grpc/` — implements the generated
  `projectv1.ProjectServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL: `project.projects`,
  `project.project_members`, RLS policies.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, dials to workflow-service/task-service (currently unused by
  the stub checkers, see below), gRPC server with the shared interceptor
  chain, health/readiness HTTP server, graceful shutdown on SIGTERM.

### `RebindDevServer` — the guarded rebind saga

`RebindDevServer` closes the gap TS's `project.update` never guarded: a
project's `dev_server_id` can only change through this one RPC, and only
after confirming — via synchronous calls to `WorkflowExecutionChecker` and
`TaskExecutionChecker` — that no workflow or task execution is currently
active for the project. Either checker reporting `true`, or either checker
erroring (fails closed), rejects the rebind with a `FAILED_PRECONDITION`
`PROJECT_HAS_ACTIVE_WORKFLOWS` error. See
[`project-service.md` §3](../../../specs/backend-go/services/project-service.md#3-api-surface-grpc-sketch)
for the full saga and rationale.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/project-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/project-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/project?sslmode=disable \
WORKFLOW_SERVICE_ADDR=workflow-service:9090 \
TASK_SERVICE_ADDR=task-service:9090 \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **`WorkflowExecutionChecker`/`TaskExecutionChecker` are STUBs** —
  `internal/adapter/grpcclient/workflow_execution_checker.go` and
  `task_execution_checker.go` dial a real `grpc.ClientConn` to
  workflow-service/task-service (config is fully wired), but
  `HasActiveExecutions` unconditionally returns `(false, nil)` because
  neither service defines that RPC yet. **This means
  `RebindDevServer`'s active-execution guard is currently a no-op** — a
  rebind will always be allowed to proceed past the checker step, even with
  a running workflow/task. The usecase logic itself (`internal/usecase/rebind_dev_server.go`)
  is real and unit-tested against fakes that simulate both "active" and
  "checker error" cases — only the outbound gRPC call is stubbed. **Do not
  deploy this stub to production** — replace both adapters' bodies with a
  real unary RPC call once workflow-service/task-service expose
  `HasActiveExecutions(projectId) returns (bool)`. See
  [`project-service.md`'s RebindDevServer section](../../../specs/backend-go/services/project-service.md#3-api-surface-grpc-sketch).
- **`infra-fleet-service`'s `ValidateDevServer` step isn't implemented** —
  the sequence diagram in project-service.md §3 also validates the new
  dev-server id exists before the execution checks; this scaffold's
  `RebindDevServer` only requires it to be non-empty. Add a
  `DevServerValidator` port + adapter alongside the two execution checkers.
- **No `sqlc` codegen wired** — same rationale as `usage-service`'s README:
  hand-written SQL via `pgx` is a valid destination per the tech stack doc,
  not the codegen-checked default.
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  same as `usage-service`; `DATABASE_DSN` is read directly from the
  environment for local dev.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
- **No OPA policy integration** — project-service.md §9 specifies role/global-admin
  checks (owner-only for `RebindDevServer`/`AddMember`) enforced via the Go
  OPA SDK in a gRPC interceptor. Not wired in this scaffold; every RPC here
  only requires an authenticated tenant, not a specific role.
- **No event publishing** — unlike `usage-service`, this scaffold doesn't
  publish `project.created`/`project.rebound`/`member.added` events yet (see
  project-service.md §6's `adapter/eventbus/` note). Add alongside the
  transactional outbox once needed by a consumer.
- Full RPC surface per the service doc's exhaustive API section (§3) isn't
  implemented — only the 5 RPCs the current proto defines. Extend
  `proto/orca/project/v1/project.proto` and this service's usecase/adapter
  layers together as more of the surface (repos, worktrees, project groups,
  source projects, `UpdateProject`/`DeleteProject`, full membership
  management) is needed.
