# workflow-service

Owns workflow templates and executions — see
[`specs/backend-go/services/workflow-service.md`](../../../specs/backend-go/services/workflow-service.md)
for the full design. Built following the exact package layout and
conventions demonstrated by
[`usage-service`](../usage-service/README.md), this repository's Phase 0
reference implementation.

## What's implemented

- `internal/domain/` — `WorkflowTemplate`/`WorkflowExecution` entities with
  invariant-enforcing constructors, the `StepType`/`StepResult`/
  `StepExecutor` strategy-pattern types, `DAGDefinition` parsing +
  structural validation + `BuildWaves` (real Kahn's-algorithm topological
  sort, general cycle detection via `ErrCyclicDependency`), `StepExecution`
  (one step's outcome within a wave-dispatch run), and a real, tested
  boolean expression evaluator for the Condition step type
  (`EvaluateCondition`: `==`/`!=`/`&&`/`||` over a flat key-value context
  map, no `eval`, fail-safe-false on unparseable input — see
  `domain/condition.go`'s doc comment for the full grammar). Pure unit
  tests throughout, no I/O.
- `internal/usecase/` — `CreateTemplate`, `Execute` (real wave-dispatch, see
  "Known gaps" below for the concurrency/failure-semantics/async decision),
  `GetExecution`, `PauseExecution`, `ResumeExecution`, `ExecuteAdHocStep`
  (the port `automation-service`'s `RunNow` calls, per §3.1 — resolves a
  `StepType` to its registered `StepExecutor` and runs it directly, no
  throwaway-template indirection; now persists a synthetic execution +
  step_execution row, see "Known gaps"), `HasActiveExecutions`,
  `RecoverExecutions` (the boot-time recovery scan, §8 — see "Known gaps"
  below for the full resume algorithm and its remaining honest gaps), and
  `waveDispatcher` (the shared wave-dispatch/single-step-run engine
  `Execute`, `ExecuteAdHocStep`, and now `RecoverExecutions` all use). Each
  tested against in-memory
  fakes; the Pause/Resume tests specifically exercise the
  running->paused/paused->running state-machine invariants (e.g. resuming a
  non-paused execution is rejected); the wave-dispatch tests specifically
  prove the wave gate is real (a channel-blocked fake executor, not a
  sleep-based test — see `wave_dispatcher_test.go`).
- **`HasActiveExecutions` (Epic C, `backend-go/docs/execution-plan.md`)** —
  answers "does this project have a non-terminal (pending/running/paused)
  workflow execution", closing `project-service.RebindDevServer`'s
  active-execution guard, which was previously a client-side no-op because
  this RPC didn't exist. `Execute` now persists `ProjectID` on every
  `WorkflowExecution` (`migrations/0002_execution_project_id`, which also
  adds the partial index `idx_workflow_executions_project_active` on
  `(tenant_id, project_id, status)`), and `internal/adapter/postgres`
  exposes `HasActiveExecutions(tenantID, projectID)` as a single `EXISTS`
  query against that index.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  `TemplateRepository`, `ExecutionRepository`, and `StepExecutionRepository`.
- `internal/adapter/stepexecutors/` — the five `StepExecutor`
  implementations:
  - **Condition** (real) — wraps `domain.EvaluateCondition`.
  - **Webhook** (real) — native `net/http`, with SSRF hardening: rejects
    unsupported schemes, resolves the target and rejects private/loopback/
    link-local IPs by default, supports a config-driven hostname allowlist
    (`WEBHOOK_ALLOWLIST_HOSTS`, empty by default) whose entries are trusted
    outright, disables redirect-following outright (rather than
    re-validating each hop), and bounds response size (1 MiB) and request
    time (client timeout).
  - **Agent / Shell / Notification** (real, Epic A's second pass) —
    `internal/adapter/infrafleetclient/` calls infra-fleet-service's generic
    `Relay` RPC with methods `agent.exec`/`shell.exec`/`notification.send`.
    Now reachable both via `ExecuteAdHocStep` and through a real
    template-driven `Execute` run (wave-dispatch, see "Known gaps" below)
    — see that section for the remaining caveats (best-effort method
    shapes, no live Dev Server Agent to verify against).
- `internal/adapter/grpc/` — implements the generated
  `workflowv1.WorkflowServiceServer`: `CreateTemplate`, `Execute`,
  `GetExecution`, `PauseExecution`, `ResumeExecution`, `ExecuteAdHocStep`,
  `HasActiveExecutions`, plus **`CancelExecution`/`ListTemplates`/
  `ResolveTemplate`** (added 2026-08-17, closing the last item Epic C
  originally left deferred — see "Template inheritance" below). The design
  doc's fuller §3 sketch still has `StreamExecutionEvents`, which isn't in
  the generated proto and so isn't implemented here.
- `migrations/0001_init.{up,down}.sql` — `workflow.templates` (id,
  tenant_id, name, dag_json, scope) and `workflow.executions` (id,
  template_id, tenant_id, status, root_trace_id, paused_at) with RLS
  policies, matching usage-service's pattern.
- `migrations/0002_execution_project_id.{up,down}.sql` — adds
  `workflow.executions.project_id` plus the partial index
  `idx_workflow_executions_project_active`, backing `HasActiveExecutions`
  above.
- `migrations/0003_template_parent_chain.{up,down}.sql` — adds
  `workflow.templates.parent_template_id` plus
  `idx_workflow_templates_parent`, backing `ResolveTemplate` below.
- `migrations/0004_step_executions.{up,down}.sql` — adds
  `workflow.step_executions` (id, execution_id, step_id, wave, status,
  dispatch_token, output, error_message), RLS'd via an `EXISTS` join back
  to `workflow.executions` (this table has no `tenant_id` column of its
  own), backing real wave-dispatch above.
- `migrations/0005_execution_ad_hoc_template.{up,down}.sql` — drops
  `workflow.executions.template_id`'s `NOT NULL` constraint and switches
  its FK to `ON DELETE SET NULL`, matching the fuller design doc (§5) —
  needed so `ExecuteAdHocStep`'s synthetic, templateless execution can be
  persisted at all.
- `cmd/server/main.go` — composition root: config load, Postgres pool,
  `StepExecutorRegistry` wired with all five step types (two real, three
  stubs), gRPC server with the shared interceptor chain, health/readiness
  HTTP server, graceful shutdown on SIGTERM.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/workflow-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/workflow-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/workflow?sslmode=disable \
WEBHOOK_ALLOWLIST_HOSTS=hooks.example.com \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/stepexecutors/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

The `step_executions` table/repository and the ad hoc-execution nullable
`template_id` migration both have real integration coverage in
`internal/adapter/postgres/repository_test.go` (`TestRepository_StepExecution_CreateAndUpdateRoundTrip`,
`TestRepository_ListStepExecutions_ScopedByTenant`,
`TestRepository_CreateExecution_AdHocNullTemplateID`) — verified passing
against a real testcontainers Postgres in this pass, not just compiled.

## Known gaps / follow-ups (tracked, not silently skipped)

- **Real DAG wave-dispatch is now implemented.** `domain.DAGDefinition.BuildWaves`
  computes wave-ordered layers via Kahn's algorithm (in-degree map, BFS by
  wave) and detects general (multi-node) cycles Validate's pairwise checks
  can't see, returning `domain.ErrCyclicDependency` naming every step still
  unprocessed when the algorithm stalls. `usecase.Execute` calls
  `Validate` then `BuildWaves` synchronously (a cyclic or otherwise invalid
  DAG fails the RPC immediately, never discovered only once dispatch
  starts), persists the execution as `status=running`, then hands the
  built waves to a `waveDispatcher` (`internal/usecase/wave_dispatcher.go`)
  on a **detached background goroutine and returns immediately** — Execute
  does NOT block the RPC on the whole DAG finishing. This is a deliberate
  architectural decision (documented in `execute.go`'s doc comment): a DAG
  can contain long-running steps (§8's 30-minute default step timeout)
  that would blow past any sane gRPC deadline if Execute waited on them,
  and it directly targets the bug this pass fixes — an execution
  previously never progressed past `running` on its own; now dispatch runs
  independently of the RPC that started it.
  - **Concurrency model**: within a wave, every step dispatches
    concurrently through a bounded worker pool (`defaultMaxConcurrentSteps`
    = 10 in-flight, per §8), persisting one `step_executions` row per step
    (`status=pending` before dispatch, then `running`/`completed`/`failed`
    as it progresses). Wave N+1 is gated on every step in wave N reaching
    a terminal status — `waveDispatcher.dispatchWave`'s `sync.WaitGroup`
    is the gate, proven by `TestWaveDispatcher_WaveGate_Wave1NeverStartsBeforeWave0Terminates`
    (a channel-blocked fake executor, not a sleep-based test).
  - **Failure semantics (a deliberate choice §7's diagram doesn't spell
    out)**: ANY step failing — a business-level "failed" `StepResult` or a
    hard `StepExecutor` error — fails the whole execution
    (`status=failed`). Waves already in flight when a failure is observed
    are not cancelled (their steps still run to their own terminal
    status), but no further wave is dispatched once the current wave
    finishes. This is the simplest correct behavior given
    `domain.Status` has no partial-success state to report.
  - **What's still not implemented**: idempotent re-dispatch after a
    crash (`dispatch_token` is generated and persisted, per §8, but
    nothing consumes it yet — see the boot-time recovery scan gap below),
    and step outputs are not threaded into later steps' config as
    accumulated context (§4 describes Condition steps evaluating "the
    accumulated outputs of earlier steps" — this pass's `ConditionExecutor`
    still only sees whatever `stepConfigJSON` the DAG step itself carries).
- **Agent/Shell/Notification step executors are now real** (Epic A's second
  pass, see [`docs/execution-plan.md`](../../docs/execution-plan.md) §2
  Epic A) — `internal/adapter/infrafleetclient/` calls infra-fleet-service's
  generic `Relay` RPC with methods `agent.exec`/`shell.exec`/
  `notification.send`. `stub.go` (the old `AgentStub`/`ShellStub`/
  `NotificationStub`) was deleted since nothing references it anymore.
  Real DAG wave-dispatch now exists (see the gap above), so these
  executors are reachable both via `ExecuteAdHocStep` and through a
  template-driven `Execute` run. **Known gaps:**
  - The step-config JSON had no field identifying which dev-server/worktree
    binding a step should target — this pass added a `connectionId` field to
    the new `AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig`
    domain types (`internal/domain/step.go`) to close that.
  - Method names/param shapes (`agent.exec`/`shell.exec`/`notification.send`)
    are best-effort, no live Dev Server Agent to verify against. Reading
    `backend/src/main/workflow/StepExecutors.ts` (the old TS implementation)
    surfaced a real discrepancy worth reconciling before production use:
    `agent.exec` there is a generic `{binary,args,cwd,stdin,env,timeoutMs}`
    process-exec RPC, not the prompt-driven step this service's `agent` step
    type is meant to represent (TS ended up needing a separate
    `agent.execPrompt` for that) — see `agent_step_executor.go`'s doc
    comment.
  - Unit- and fake-client-tested only (`internal/adapter/infrafleetclient/*_test.go`)
    — no live infra-fleet-service/Dev Server Agent available to verify
    against.
- **Webhook SSRF allowlist is a basic stub**, not the full §9 posture:
  redirects are refused outright instead of being re-validated per hop,
  there's no per-tenant allowlist (one process-wide list from config), and
  the IP-range block only covers `net.IP`'s loopback/link-local/private/
  unspecified predicates — not, e.g., IPv4-mapped-IPv6 edge cases or a
  cloud metadata-endpoint-specific denylist.
- **`ExecuteAdHocStep`'s persistence gap is now closed.** Per §3.1, it
  persists a synthetic one-step execution (`domain.NewAdHocWorkflowExecution`
  — `TemplateID` deliberately empty, since an ad hoc run has no backing
  `WorkflowTemplate`) plus one `step_executions` row (`wave=0`), reusing
  `waveDispatcher.runStep` — the same single-step run logic Execute's real
  wave dispatch uses — instead of a bespoke ad hoc path. Unlike `Execute`,
  this remains **synchronous**: `automation-service`'s `RunNow` needs the
  result before reporting the run's outcome (§7), and a single step
  doesn't carry the unbounded-latency concern that makes `Execute`
  asynchronous. `workflow.executions.template_id` is now nullable
  (`migrations/0005_execution_ad_hoc_template`) to allow this — the
  original narrowed `0001_init` schema made it `NOT NULL`, which the
  fuller design doc (§5) never actually specified.
- **Template inheritance (company -> team -> personal chain,
  `ResolveTemplate`/`ListTemplates`/`CancelExecution`) is now implemented**
  (2026-08-17) — this closes the last item Epic C originally left
  deferred (`docs/execution-plan.md` §2/§10 explicitly gated it on
  inheritance "actually being built," which hadn't started until this
  pass). `WorkflowTemplate.ParentTemplateID` + migration
  `0003_template_parent_chain` add the column; `ResolveChain`
  (`internal/adapter/postgres/repository.go`) is the `WITH RECURSIVE`
  query §6 sketched, depth-capped at 5. **Resolution policy is a
  deliberate, documented choice, not something §6 pins down**: the
  algorithm returns the closest (most-specific) template in the chain
  whose `dag_json` defines at least one step — see
  `usecase.ResolveTemplate`'s doc comment for the full reasoning (a
  personal template with no steps of its own correctly inherits from its
  team/company parent, rather than resolving to "no steps"). Multi-hop
  cycle detection isn't implemented beyond the depth cap — not needed
  today, since this service's RPC surface has no `UpdateTemplate` to
  rewire an existing template's parent after creation, so a cycle can't
  actually arise through normal use (see `domain.ErrTemplateSelfParent`'s
  doc comment). `ListTemplates` is keyset-paginated, matching
  annotation-service's `ListAnnotations` convention exactly.
  `CancelExecution` was independent of inheritance and is a straightforward
  peer of `PauseExecution`/`ResumeExecution` (`domain.WorkflowExecution.Cancel`,
  pending/running/paused -> cancelled). All three: real unit tests against
  in-memory fakes, plus real integration tests against a live
  testcontainers Postgres (verified passing in this pass, including two
  pre-existing integration tests in this same file that had never actually
  been run before — `TestRepository_CreateAndGetTemplate`/
  `TestRepository_ExecutionPauseResumeRoundTrip` were seeding non-UUID
  literal ids like `"tmpl-1"`/`"exec-1"` against UUID-typed columns, which
  fails at the database, not at compile or unit-test time; fixed
  alongside this pass's own new integration tests since they live in the
  same file).
- **Boot-time recovery scan (§8) is now implemented.**
  `usecase.RecoverExecutions` (`internal/usecase/recover_executions.go`)
  runs once, synchronously, in `cmd/server/main.go` after every dependency
  is constructed and before the gRPC/HTTP listeners start — matching §8's
  "before accepting new `Execute` calls" ordering. It queries
  `ExecutionRepository.ListRunning` (backed by
  `idx_workflow_executions_resumable`), the one deliberately **process-
  wide, non-tenant-scoped** query in this codebase: every other usecase
  resolves a single tenant from the inbound request's context
  (`tenant.RequireTenantID`), but a boot-time scan has no request and must
  see every tenant this instance's database holds in one pass. `paused`
  rows are left alone entirely (a deliberate user/system action must not
  be silently resumed by a restart — `PauseExecution`/`ResumeExecution`/
  `CancelExecution` are untouched by this change); only `status=running`
  rows are recovered.
  - **Resume algorithm**: for each running execution, refetch its template
    and rebuild the DAG exactly as `Execute` does (`ParseDAG` + `Validate`
    + `BuildWaves` — `WorkflowExecution` has no DAG snapshot field of its
    own, so the template is the source of truth both times), then load its
    existing `step_executions` rows and walk waves in order looking for
    the first one that is NOT fully `completed`: a wave where every step
    has a persisted `status=completed` row is skipped entirely; the first
    wave with a missing, `pending`, `running`, or `failed` row is the
    resume point. Dispatch resumes there via `waveDispatcher`'s new
    `dispatchWavesFrom`/`dispatchWave`-with-`existing` machinery (a
    generalization of the existing wave-gate/worker-pool code, not a
    duplicate) instead of `Execute`'s `dispatchWaves`.
  - **Re-dispatch-on-uncertain-status decision**: a step whose persisted
    row is anything other than `completed` — in particular `running`,
    which means the process crashed while a call to the step's executor
    was in flight, with no way to know whether it actually finished — is
    re-dispatched rather than assumed to have succeeded or failed. This
    also applies uniformly to an already-`failed` row (rather than adding
    a second special case): a step recorded as failed already has a known
    outcome, but this pass chose "anything short of a recorded success
    gets one honest retry after a crash-triggered restart" as the simpler,
    single rule. Re-dispatch reuses the SAME `step_executions` row via
    `UpdateStepExecution` (never re-`CreateStepExecution`), since that
    table's `(execution_id, step_id)` `UNIQUE` constraint would otherwise
    reject a second insert for a step already dispatched before the
    crash.
  - **Each recovered execution's dispatch runs on its own detached
    background goroutine** (`tenant.WithTenantID(context.Background(),
    exec.TenantID)`, same pattern `Execute.Execute` already establishes),
    so `RecoverExecutions.Execute`'s synchronous part — the listing plus
    DAG reconstruction — returns quickly and does not block server
    startup on every recovered execution actually finishing.
  - **Known gap: ad hoc executions cannot be recovered.**
    `ExecuteAdHocStep`'s synthetic execution has no backing
    `WorkflowTemplate` (`TemplateID` is empty) and its single step's
    config JSON is never persisted anywhere (`step_executions` has no
    config column, only `output`/`error`) — it lives only in memory for
    the duration of that synchronous call. If a crash leaves an ad hoc
    execution stuck at `status=running`, this scan has no way to know
    what to re-run and deliberately leaves it as-is (logged, not silently
    dropped). Closing this would need `ExecuteAdHocStep` to persist the
    step's config up front, which it doesn't do today.
  - **Known gap: no distributed lock / leader election.** This scan has no
    way to distinguish "this execution is still genuinely running on
    another live replica of this service" from "the replica that was
    running it crashed." In a multi-replica deployment, every replica
    that restarts (or every replica, if they all restart around the same
    deploy) will independently pick up the same `running` executions from
    `ListRunning` and race to redispatch them — `waveDispatcher`'s
    `CreateStepExecution` calls collide safely on the `UNIQUE`
    constraint for any step not yet dispatched, but a step that has NO
    existing row yet could still be double-dispatched to its
    `StepExecutor` (e.g. two replicas both call `infra-fleet-service` for
    the same step) before either one's `CreateStepExecution` lands. §8
    calls this out directly: real idempotency needs `dispatch_token`
    consumed either by `infra-fleet-service`'s relay contract de-duping on
    it, or by this service querying the execution plane for an in-flight
    step with that token before redispatching — neither is implemented;
    `dispatch_token` is still generated and persisted but nothing reads it
    back yet. Single-replica deployments (the only topology exercised by
    this pass's tests) are unaffected.
  - Unit-tested against this package's existing fake
    `ExecutionRepository`/`StepExecutionRepository` doubles
    (`recover_executions_test.go`): resuming past an already-succeeded
    wave 0 into wave 1, re-dispatching a mid-flight (`running`) step onto
    its existing row, leaving `paused` executions untouched, leaving
    already-terminal executions untouched, and leaving ad hoc executions
    untouched. `ExecutionRepository.ListRunning` also has a real
    integration test against testcontainers Postgres
    (`TestRepository_ListRunning_ReturnsOnlyRunningAcrossTenants`),
    proving it returns `running` rows across multiple tenants and excludes
    `paused`/`completed` ones — verified passing in this pass.
- **No event publishing wired.** Unlike usage-service's
  `internal/adapter/eventbus/`, this service doesn't publish
  `workflow.execution.started`/`completed`/`workflow.step_failed` (§7) —
  real wave-dispatch now produces exactly the state transitions §7
  describes publishing on (execution started/completed/failed, step
  failed), so there's something to wire this to now, it's just not wired.
- **No `sqlc` codegen wired** — same rationale as usage-service:
  `adapter/postgres/repository.go` is hand-written SQL via `pgx`.
- **`common/secrets` (Vault) and `common/tracing`'s OTLP exporter are not
  wired**, same as usage-service — see that service's README for the
  general rationale; not repeated here.

## Deviation from the design doc worth flagging explicitly

`specs/backend-go/services/workflow-service.md` §4 places the
`StepExecutor` interface in `internal/usecase/`, per
`architecture/03-clean-architecture-guidelines.md`'s general "interface
lives with its consumer" rule. This build's instructions place it in
`internal/domain/` instead, reasoning that the strategy contract itself
(`Execute(ctx, stepConfigJSON string) (StepResult, error)`) is a pure
domain concept independent of any adapter, even though four of five
concrete implementations need I/O. `usecase.StepExecutorRegistry` — the
port that *resolves* a `StepType` to a `StepExecutor` — still lives in
`usecase/`, matching the general rule for the one piece that's genuinely
usecase-shaped. See `domain/step.go`'s doc comment for the in-code note.
