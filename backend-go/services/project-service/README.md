# project-service

See [`specs/backend-go/services/project-service.md`](../../../specs/backend-go/services/project-service.md)
for the full design. Follows the exact package layout and conventions
established by `usage-service` (the Phase 0 pilot) — see that service's
README for the pattern this scaffold copies.

## What's implemented

This service implements the full RPC surface `proto/orca/project/v1/project.proto`
currently exposes: `CreateProject`, `GetProject`, `ListProjects`, `AddMember`,
`RebindDevServer`, `UpdateProject`, `DeleteProject`, the repo catalog
(`AddRepo`/`ListRepos`/`ReorderRepos`/`RemoveRepo`), worktree metadata
(`RecordWorktreeCreated`/`RecordWorktreeRemoved`/`ListWorktrees`/
`SetWorktreeActivation`/`RenameWorktree`), and the project-group tree
(`CreateProjectGroup`/`UpdateProjectGroup`/`DeleteProjectGroup`/
`ListProjectGroups`). The design doc's fuller API (source-project sharing,
`GetProjectContext`, full membership management beyond `AddMember`) is a
documented follow-up — see "Known gaps" below.

- `internal/domain/` — `Project` (now carrying `description`/
  `default_branch`/`visibility`/`created_by`/timestamps),
  `ProjectMember`/`ProjectRole`, `Repo`, `Worktree`, `ProjectGroup` —
  invariant-enforcing constructors, pure unit tests.
- `internal/usecase/` — one usecase type per RPC (21 in total), each tested
  against in-memory fakes (`fakes_test.go`), no real Postgres/gRPC needed.
- `internal/adapter/postgres/` — real `pgx`-backed repositories:
  `Repository` (projects/members), `RepoRepository`, `WorktreeRepository`,
  `ProjectGroupRepository` — one struct per entity, matching task-service's
  postgres package layout.
- `internal/adapter/grpcclient/` — outbound client adapters toward
  workflow-service and task-service for the active-execution guard (real as
  of Epic C, 2026-08-17 — see "Known gaps" for a remaining task-service-side
  limitation).
- `internal/adapter/grpc/` — implements the generated
  `projectv1.ProjectServiceServer`, pure wire<->usecase translation.
- `migrations/` — `0001_init` (projects/project_members), `0002_project_extended_fields`
  (description/default_branch/visibility/created_by — visibility has a
  `CHECK` constraint), `0003_repos`, `0004_worktrees`, `0005_project_groups`.
- `cmd/server/main.go` — composition root: config load, Postgres pool, dials
  to workflow-service/task-service, gRPC server with the shared interceptor
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

### `UpdateProject` — field-mask semantics

`name`/`description`/`default_branch`/`visibility` are updated only where
the request field is non-empty ("" = no change). `dev_server_id` is
deliberately absent from `UpdateProjectRequest` — `RebindDevServer` (with
its active-execution guard) stays the sole path that may change it, so
there's exactly one code path for rebinding, not two that can drift.

### `DeleteProject` — cascade and guard decisions

Two safety decisions were made explicitly for this RPC, not left implicit:

1. **Guarded by the same active-execution check `RebindDevServer` uses.**
   `DeleteProject` reuses `WorkflowExecutionChecker`/`TaskExecutionChecker`
   and the identical fail-closed policy — deleting a project out from under
   a running workflow/task is at least as risky as rebinding its dev server
   (the execution's worktree bookkeeping disappears entirely, not just
   moves host), so it gets the same `FAILED_PRECONDITION
   PROJECT_HAS_ACTIVE_WORKFLOWS` guard before the delete runs.
2. **Cascade via `ON DELETE CASCADE`, not a soft-delete or an app-layer
   fan-out.** `project.repos.project_id`, `project.worktrees.project_id`/
   `repo_id`, and `project.project_members.project_id` all cascade —
   child rows have no independent meaning once their project is gone, so a
   single `DELETE FROM project.projects` is sufficient; no multi-table
   transaction is needed in the repository or usecase. Covered by
   `TestWorktreeRepository_CascadesOnProjectDelete`.

### Repo/worktree/project-group design notes

- **Repo ordering** (`position`) is not contiguous or unique —
  `RemoveRepo` leaves a gap rather than renumbering the rest;
  `ReorderRepos` takes the full ordered id list and rejects a
  partial/mismatched list with `INVALID_ARGUMENT` (validated in the usecase
  layer, before any write) rather than silently reordering a subset.
- **`RecordWorktreeRemoved` hard-deletes** the worktree row rather than a
  soft-removed flag — `project.worktrees` is disposable metadata (never
  authoritative for on-disk existence, per `domain.Worktree`'s doc comment)
  with git-gateway-service as the source of truth for lineage/history, so
  there's no reporting/audit need for a tombstone row here.
- **`UpdateProjectGroup` renames only — it never rewires `parent_group_id`.**
  Mirrors workflow-service's `WorkflowTemplate` precedent (no
  `UpdateTemplate` RPC to rewire a template's parent): allowing an existing
  node to move to a new parent reopens a cycle-detection problem (a group
  rewired to become a descendant of its own descendant) that
  `CreateProjectGroup`'s simple "parent must already exist" check avoids
  entirely by construction. If parent-rewiring is ever needed, it should
  ship as a dedicated RPC with its own cycle check, not folded into this
  one. See `domain.ErrGroupSelfParent`'s doc comment.
- **`DeleteProjectGroup` cascades to descendant groups** (`ON DELETE
  CASCADE` on `parent_group_id`) — a folder-tree node's children have no
  independent meaning once their parent folder is gone, mirroring
  `DeleteProject`'s cascade rationale.

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

112 unit tests (`domain`/`usecase`), 17 integration tests
(`adapter/postgres`, real Postgres via testcontainers-go) — all pass as of
this change. `policy/orca-authz/project_test.rego`'s 10 cases (23 total in
the shared bundle) also pass via `make opa-test` / `opa test
policy/orca-authz/ -v` from `backend-go/`.

## Known gaps / follow-ups (tracked, not silently skipped)

- **`WorkflowExecutionChecker`/`TaskExecutionChecker` are real (Epic C,
  docs/execution-plan.md §10, 2026-08-17)** — both now call the real
  `HasActiveExecutions` RPC on workflow-service/task-service respectively.
  `RebindDevServer`'s and `DeleteProject`'s active-execution guards are no
  longer a no-op for the workflow-service half: workflow-service persists
  `project_id` on every execution and answers accurately for
  pending/running/paused status. **The task-service half has a real,
  honestly-documented remaining limit, not a bug**: task-service has no
  execution-completion callback, so "active" there means "a task's status
  is `in_progress`," and nothing in task-service transitions a task back
  out of `in_progress` yet. This means `TaskExecutionChecker` will
  over-report "active" — and therefore `RebindDevServer`/`DeleteProject`
  will fail-closed — for any project with a task ever dispatched, until
  task-service gains a completion/status-update path (tracked in that
  service's own README, not duplicated here).
- **`infra-fleet-service`'s `ValidateDevServer` step isn't implemented** —
  the sequence diagram in project-service.md §3 also validates the new
  dev-server id exists before the execution checks; this service's
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
- **OPA authorization is wired** (Epic E) via the shared embedded evaluator
  (`common/policy`, `internal/adapter/opaclient`, consuming
  `policy/orca-authz/project.rego`'s `data.orca.authz.project.allow` rule) —
  the same in-process pattern auth-service/annotation-service/task-service
  already use, not the gRPC-interceptor approach project-service.md §9's
  prose originally sketched: every other Epic E service enforces its OPA
  check inside the usecase layer (see auth-service's `requireAdminActor`),
  and this service now matches that precedent instead of introducing a
  second enforcement point. `internal/usecase/authorization.go`'s
  `requireProjectAccess` is the shared gate:
  - **Owner-only** (`UpdateProject`/`DeleteProject`/`AddMember`/
    `RebindDevServer`, per the matrix) — caller must be the project's
    `owner`, or a global admin.
  - **Any membership** (`GetProject`/`ListRepos`/`ListWorktrees`) — caller
    must have any membership row (`owner` or `member` — the fuller
    owner/member/viewer model is the same documented follow-up as
    `domain.ProjectRole`'s doc comment), or be a global admin.
  - **Judgment call, not named in the matrix**: `AddRepo`/`RemoveRepo`/
    `ReorderRepos` (the repo-catalog mutation RPCs) got the same owner-only
    rule — a repo belongs to exactly one project, so the project's
    owner-or-admin rule is the natural fit. `RemoveRepo` resolves its
    owning project via a new `RepoRepository.GetRepo(repoID)` lookup, since
    `RemoveRepoInput` only carries a `repo_id`.
  - **Deliberately left unguarded, also a judgment call**: worktree
    mutation RPCs (`RecordWorktreeCreated`/`RecordWorktreeRemoved`/
    `SetWorktreeActivation`/`RenameWorktree`) and the `ProjectGroup` CRUD
    RPCs. Worktree mutations are bookkeeping callbacks from
    git-gateway-service made after the real git operation already
    happened under an already-authorized end-user request —
    git-gateway-service isn't itself a project member, so gating these the
    same way as `AddRepo` risks rejecting legitimate non-owner members'
    git operations, a materially different risk profile from a
    user-invoked catalog mutation. `ProjectGroup` rows aren't linked to
    any single project (`project.project_groups` has no `project_id`,
    it's a tenant-scoped folder tree) so there is no project role to
    resolve at all — these RPCs stay authenticated-only, matching
    `CreateProject`'s existing precedent. Extending authorization to
    either category is future work, not an oversight.
  - **`CreateProject`/`ListProjects` are unchanged** — authentication only,
    per project-service.md §9.
  - **`AddMember`'s bootstrap exception**: a strict owner-only gate on
    `AddMember` creates a deadlock for the very first membership row —
    `CreateProject.Execute`'s doc comment documents that a project's
    creator becomes its first owner via a follow-up `AddMember` call made
    before any membership row exists yet. `AddMember.Execute` special-cases
    this: the project's recorded `created_by` adding **themselves**
    bypasses the owner-only check; adding any other user, or self-adding to
    a project this caller didn't create, still goes through the normal
    `requireProjectAccess` gate.
  - **`caller_global_role` resolution**: no role claim propagates from
    api-gateway into a service's request context yet — the exact same gap
    annotation-service's `OPAClient.Decision` doc comment already
    documents for its own `actor_role` parameter. `authorization.go`'s
    `callerGlobalRole` reuses that documented convention (always returns
    `""`) rather than inventing a new lookup (e.g. a new gRPC call to
    auth-service) — the global-admin-override branch in `project.rego` is
    proven correct at the policy layer (`policy/orca-authz/
    project_test.rego`, run via `opa test`) but not reachable through this
    service's Go code until the upstream claim-propagation gap closes.
  - Every check fails closed on a policy-evaluation error or a
    membership-lookup error — never treated as an allow, matching
    `auth-service.requireAdminActor`'s exact contract.
- **No event publishing** — unlike `usage-service`, this service doesn't
  publish `project.created`/`project.updated`/`project.deleted`/
  `member.added`/etc. events yet (see project-service.md §6's
  `adapter/eventbus/` note). Add alongside the transactional outbox once
  needed by a consumer.
- **Source-project sharing (`LinkSourceProject`/`UnlinkSourceProject`/
  `GetSharedProjectData`), `GetProjectContext`, and membership management
  beyond `AddMember` (`RemoveMember`/`UpdateMemberRole`/`ListMembers`) are
  out of this change's scope** — not half-built, simply not started. The
  proto surface doesn't define these RPCs yet; extend
  `proto/orca/project/v1/project.proto` and this service's usecase/adapter
  layers together when that work is picked up.
