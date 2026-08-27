# task-service

See [`specs/backend-go/services/task-service.md`](../../../specs/backend-go/services/task-service.md)
for the full design and
[`specs/backend-go/services/usage-service.md`](../../../specs/backend-go/services/usage-service.md) /
[`services/usage-service/`](../usage-service/) for the reference package
layout and conventions this service follows exactly.

## What's implemented

- `internal/domain/` — `Task`, `TaskEdge`, `Grant`/`GrantLevel`, and this
  service's two pieces of real, differentiated domain logic:
  - **`DetectCycle`** (`task_edge.go`) — a pure BFS reachability check: a
    proposed `from -> to` depends-on edge is cyclic iff `to` can already
    reach `from` via the existing edge set. Ported faithfully from TS
    `TaskDAGValidator` per the design doc's §10 "faithful port, not a
    gap-fixing rebuild" framing.
  - **`ResolveGrant`** (`grant_resolution.go`) — the §4.1 BFS ancestor
    walk: collects every matching grant across a task's full ancestor
    chain (target task's own grants always count; ancestors' grants count
    only when `apply_tree=true`), then picks the highest-priority match
    (`owner > admin > user > team > company`, priority over proximity).
    Ported faithfully from TS `TaskGrantService.resolvePermission()`.

  Both are pure functions — no DB, no gRPC, no `context.Context` — and
  **this is where this service's real value is**: 20 unit tests in
  `task_edge_test.go` and `grant_resolution_test.go` cover 2/3/4-hop and
  deep-chain cycles, diamond dependency graphs, edge-kind isolation
  (parent_child edges must not leak into a depends_on cycle check),
  `apply_tree` inheritance stopping and continuing past non-inherited
  ancestors, priority-over-proximity, the max-depth guard, and
  default-deny. See "Known deviations" below for the one place this
  scaffold's proto contract differs from the design doc's sketch schema.
- `internal/usecase/` — `CreateTask`, `GetTask`, `AddEdge` (calls
  `domain.DetectCycle` before ever touching the repository, for
  `depends_on` edges only — `parent_child`'s single-parent invariant is a
  different, DB-enforced constraint), `Grant`, `ResolvePermission` (wires
  `TaskRepository.GetAncestors` + `GrantRepository.ListGrantsForAncestors`
  + `TeamScopeResolver` into `domain.ResolveGrant`), `ExecuteTask` —
  §3.1's complexity branch: a task with any `parent_child` or `depends_on`
  edge *from* it dispatches to `ComplexExecutor`, otherwise
  `SimpleExecutor`. The branch decision itself is real and tested
  (`execute_task_test.go`); both executors are stubs (see below). `Execute`
  also now marks the task `StatusInProgress` via `TaskRepository.UpdateStatus`
  before dispatching, and `HasActiveExecutions` queries that same state —
  see "Epic C: `HasActiveExecutions`" below for what this does and does not
  close. Every usecase is tested against in-memory fakes, no real Postgres
  needed (`fakes_test.go`).
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  all three ports (`TaskRepository`, `EdgeRepository`, `GrantRepository`)
  against task-service's own database. `GetAncestors` is one
  `WITH RECURSIVE` query over `tasks.parent_id`, per the design doc §6's
  query-shape note.
- `internal/adapter/grpc/` — implements the generated
  `taskv1.TaskServiceServer`. Note the generated proto names the sixth
  RPC's messages `TaskServiceExecuteRequest`/`TaskServiceExecuteResponse`
  (not `ExecuteTaskRequest`/`ExecuteTaskResponse` as the design doc's proto
  *sketch* implies) — this scaffold follows the actual generated code, the
  authoritative wire contract.
- `internal/adapter/grpcclient/` — the three stubbed cross-service ports
  (see "Known gaps" below).
- `migrations/0001_init.{up,down}.sql` — `task.tasks`, `task.task_edges`,
  `task.task_grants`, `task.task_comments`, RLS policies matching
  usage-service's pattern.
- `migrations/0002_task_project_execution_tracking.{up,down}.sql` — adds
  `task.tasks.project_id` and a partial index on
  `(tenant_id, project_id, status) WHERE status = 'in_progress'` for Epic C
  (see "Epic C: `HasActiveExecutions`" below).
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/task-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/task-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/task?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

Build/lint standalone (this module has its own `go.mod`/`go.work`
replace directives, same as usage-service):

```sh
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./...
```

## The 3 stubbed cross-service ports

Per the design doc §2/§7's bounded-context rules, task-service must call
out to other services rather than reading their tables — none of these
three calls are wired to a real gRPC client in this scaffold
(`internal/adapter/grpcclient/`):

1. **`TeamScopeResolver`** (`team_scope_resolver.go`) — should call
   tenant-service to resolve which teams a user belongs to, for
   `GrantLevelTeam` grant matching in `ResolveGrant`'s BFS walk. The stub
   always returns an empty team list, so team-scoped grants will silently
   never match any caller until this is wired to a real
   `tenantv1.TenantServiceClient`.
2. **`SimpleExecutor`** (`simple_executor.go`) — should relay `Execute`'s
   simple path to infra-fleet-service (→ Dev Server Agent `agent.exec`).
   The stub returns a synthesized placeholder execution ref without
   calling anything.
3. **`ComplexExecutor`** (`complex_executor.go`) — should hand off
   `Execute`'s complex path to orchestration-service's coordinator. Same
   stub shape as `SimpleExecutor`.

The *branching logic* in `usecase.ExecuteTask` that decides which of
(2)/(3) to call is real and unit-tested — only the two RPC calls
themselves are stubbed.

## Known deviations from the design doc

- **`GrantLevel` is a 5-value enum (`owner/admin/user/team/company`), not
  a 3-value level plus a separate `grantee_type` field.** The design doc's
  §5 sketch schema has `level ∈ {owner,admin,user}` and a separate
  `grantee_type ∈ {user,team,company}` column; the *generated* proto
  (`orca.task.v1.GrantLevel`, the authoritative wire contract — see
  `proto/orca/task/v1/task.proto`) instead folds grantee-kind into the
  level itself. This scaffold follows the generated proto:
  owner/admin/user grants match by user ID, team grants match by team
  membership, company grants match by tenant ID — see
  `internal/domain/grant.go`'s doc comment.
- **The `tasks` table's field set matches the generated proto exactly**
  (`id, tenant_id, title, status, parent_id, project_id` — `project_id`
  added by migration `0002` for Epic C, see below). This still isn't the
  design doc §5's broader sketch (`description, complexity, assignee_id,
  active_execution_id`). `ExecuteTask`'s complexity branch is computed
  dynamically from edges rather than read from a stored `complexity`
  column, which is both simpler and matches the "branching logic must be
  real and tested" build goal more directly than a column the RPC surface
  has no way to set.
- **Only the 6 RPCs the generated proto actually defines are
  implemented** (`CreateTask`, `GetTask`, `AddEdge`, `Grant`,
  `ResolvePermission`, `Execute`) — the design doc §3's sketch lists a much
  larger surface (`UpdateTask`, `DeleteTask`, `ListTasks`, `GetChildren`,
  `GetSubtree`, `RemoveEdge`, `GetDependencies`,
  `RecalculateProgress`, `AddComment`/`ListComments`, `RevokeGrant`,
  `AIDecompose`/`AIApply`) that isn't in `proto/orca/task/v1/task.proto`
  as generated. Extend the `.proto`, then this service's usecase/adapter
  layers together, as more of the surface is needed — same posture as
  usage-service's README on its own partial RPC surface.

## Epic C: `HasActiveExecutions`

`backend-go/docs/execution-plan.md`'s Epic C asked for `HasActiveExecutions`
so `project-service.RebindDevServer`'s active-execution guard (the fix for
the TS `PROJECT_HAS_ACTIVE_WORKFLOWS` gap) could stop being a client-side
no-op — neither task-service's nor workflow-service's proto exposed a way to
ask "does this project/task have an active execution" before now. This
service's half is now real plumbing, not a stub:

- `Task.ProjectID` / `CreateTaskRequest.project_id` (proto field 6/4) let a
  task be associated with a project-service project.
- `ExecuteTask.Execute` now calls `TaskRepository.UpdateStatus(...,
  domain.StatusInProgress)` before dispatching to either executor — a real,
  persisted state transition (migration `0002` adds the `project_id` column
  and a partial index over `(tenant_id, project_id, status)` for it).
- `HasActiveExecutions` (usecase + RPC) runs a real query:
  `EXISTS(... WHERE project_id = $1 AND status = 'in_progress')`.

**What this does NOT close**: task-service has no execution-completion
callback anywhere in this scaffold, and the generated proto has no
`UpdateTask`/`SetStatus` RPC to drive one manually. That means
`status = 'in_progress'` is one-way — set on dispatch, never cleared. Once a
task has ever been executed, `HasActiveExecutions` will keep reporting
"active" for it indefinitely, even long after the underlying work actually
finished (workflow-service has the identical documented limitation for its
own executions — see that service's `internal/usecase/execute.go`). This
closes the plumbing gap (the RPC now exists and answers a real question
about real persisted state), not the semantics gap (the state it reads isn't
kept accurate over time). A real completion/update-status path is separate,
later work — see "Known gaps" below.

## Known gaps / follow-ups (tracked, not silently skipped)

- **The 3 stubbed cross-service ports above.**
- **`HasActiveExecutions`'s `in_progress` status is one-way** — see "Epic C:
  `HasActiveExecutions`" above. Closing this needs a real
  execution-completion callback (from infra-fleet-service /
  orchestration-service back into task-service) plus an
  `UpdateTask`/`SetStatus`-shaped RPC to drive it, neither of which exist in
  the generated proto yet.
- **`AddEdge`'s cycle check and write are not atomic** — the usecase does
  `ListByKind` (read) then, if no cycle, `Add` (write) as two separate
  calls, not one transaction. Design doc §8 calls this out explicitly:
  "the cycle check and the write must happen in one transaction so a
  concurrent `AddEdge` can't slip a cycle in between check and write." Not
  wired in this scaffold — a serializable transaction or `SELECT ... FOR
  UPDATE` over the relevant task rows would close this gap.
- **No `sqlc` codegen wired** — same posture as usage-service:
  `adapter/postgres/` is hand-written SQL via `pgx`. Design doc §6 names
  `sqlc` with hand-written recursive CTEs as this service's chosen
  approach; add a `sqlc.yaml` + regenerate once the query set stabilizes.
- **OPA integration is wired** (`internal/adapter/opaclient/`) — per design
  doc §9, `ResolvePermission` now hands its resolved level, the caller's
  requested action, and tenant context to an in-process OPA evaluation
  (`common/policy.Evaluator` against
  `policy/orca-authz/task_grant.rego`'s `data.orca.authz.task.allow` rule)
  for the actual allow/deny decision, not just the resolved level. A `false`
  decision or an evaluation error both fail closed into the same
  `PermissionDenied`/`TASK_NO_GRANT` error the BFS walk's own not-found case
  already returned — see `internal/usecase/resolve_permission.go`'s doc
  comment and `internal/usecase/resolve_permission_test.go`'s allow/deny/
  eval-error/not-found-skips-OPA cases.
  - **First-cut permission matrix, not a final taxonomy**: `task_grant.rego`
    itself says so — the `level -> allowed actions` mapping
    (owner/admin: everything; user: read/write/execute; team: read/write;
    company: read) is Epic E's starting point, may need product
    refinement, and isn't re-derived from `GrantLevel`'s priority order so
    much as it mirrors it as a widening/narrowing action set.
  - **`ResolvePermissionRequest` has no `action` field on the wire** — the
    generated proto only carries `task_id`/`user_id`. Until it grows one,
    `internal/adapter/grpc.Server.ResolvePermission` hardcodes
    `Action: "read"`, the one action every named `GrantLevel` authorizes —
    so today the OPA check only ever actually denies the already-denied
    not-found case over gRPC; the deny-on-a-real-action path is exercised
    by the usecase's unit tests (which call `Execute` directly with an
    explicit `Action`) but not reachable through the RPC surface yet. Add
    an `action` field to the proto message, thread it through this
    handler, and this gap closes.
- **No event publishing** — design doc §9 says `Grant`/`RevokeGrant`
  should emit structured audit events. No `EventPublisher` port or
  `adapter/eventbus/` package exists in this scaffold; add it following
  usage-service's `adapter/eventbus/` pattern (and its README's own
  outbox-pattern caveat) when audit logging is needed.
- **`AIDecompose`/`AIApply`** (§3.2) aren't implemented — no RPC for them
  in the generated proto yet, and they'd need an `ai-provider-service`
  client plus the relay-to-Dev-Server-Agent `ai.complete` pattern used by
  `git-gateway-service`'s commit-message generation.
- **`common/secrets` (Vault) is not wired into `main.go`** —
  `DATABASE_DSN` is read directly from the environment for local dev, same
  gap as usage-service's README documents.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
