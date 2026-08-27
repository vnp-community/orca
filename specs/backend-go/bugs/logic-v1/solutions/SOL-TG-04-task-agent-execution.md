# SOL-TG-04: Real `ComplexExecutor`, rich context preamble, worktree/session linkage, inline completion for the simple path, and a flagged streaming gap

**Resolves:** [BUG-TG-04](../BUG-TG-04-task-agent-execution-partial.md)
**Service:** `task-service` (primary) + `orchestration-service` (new client) + `project-service`/`git-gateway-service` (worktree reuse-or-create) — **Dev Server Agent (`agent/`) changes required for output streaming, flagged explicitly below**
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto` (`Execute` request/response widen, new `ReportTaskExecutionResult` RPC)
- `backend-go/proto/orca/orchestration/v1/orchestration.proto` (if not already generated — `StartCoordinatorRun` client stub)
- `backend-go/services/task-service/internal/usecase/execute_task.go` (pre-checks, status-revert-on-failure, inline completion for simple path)
- `backend-go/services/task-service/internal/usecase/execute_batch.go` (new — topological batch execution)
- `backend-go/services/task-service/internal/usecase/report_execution_result.go` (new — inbound completion callback for the complex path)
- `backend-go/services/task-service/internal/usecase/ports.go` (`WorktreeProvisioner`, widen `ComplexExecutor`)
- `backend-go/services/task-service/internal/adapter/grpcclient/complex_executor.go` (real implementation, replaces `StubComplexExecutor`)
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go` (context preamble, env vars, inline completion)
- `backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go` (new — calls `git-gateway-service.CreateWorktree`)
- `backend-go/services/task-service/internal/adapter/grpc/server.go`
- `backend-go/services/task-service/cmd/server/main.go` (dial orchestration-service, git-gateway-service)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`task-service.md §3.1` states the branch precisely: "a **simple** task...
relays directly to `infra-fleet-service` → Dev Server Agent `agent.exec`;
a **complex** task... hands off to `orchestration-service`'s coordinator...
`task-service` records a logical FK (`active_execution_id`)... it does not
track live execution state itself" (`task-service.md:76-85`). The current
`ExecuteTask` branch logic (`execute_task.go:80-94`) already matches this
correctly (confirmed real and unit-tested per BUG-TG-04's own finding) —
this solution's job is making both dispatch targets real, not changing
the branch.

`orchestration-service.md §2.2`'s dependency-direction note is the load-bearing
citation for this solution's completion-callback design: "`task-service`
calls in to *start* a run and *read* its terminal result; it never touches
this service's tables directly" and the dependency graph shows
`orch --> task` (`02-microservices-decomposition.md:162`), matched by
`orchestration-service.md`'s own sketch flow diagram
(`orchestration-service.md:53-59`: `ts -->|StartCoordinatorRun| orch`,
`orch -->|result| ts`). **This is the design task-service.md §7 already
names for the complex path**: "`orchestration-service`... calls back into
`task-service`... to read/update task state as it progresses;
`task-service` never calls back for the same request, avoiding a
synchronous cycle" (`task-service.md:276-280`). This solution's
`ReportTaskExecutionResult` inbound RPC is that exact callback, not a new
architectural pattern.

**A key finding from reading the current code, worth surfacing
explicitly**: the simple path does *not* need an async completion
callback at all. `SimpleExecutor.Execute` already blocks synchronously
until the Dev Server Agent's `agent.execPrompt` call returns
(`simple_executor.go:92-98`'s own doc comment: "`agent.execPrompt`
actually blocks until the CLI process exits... and returns its exit
status synchronously in the same RPC response"). That means `ExecuteTask`
already *knows* the simple path finished, successfully or not, before its
own `Execute` call returns to the caller — the "no completion callback"
gap BUG-TG-04 names is real for the *complex* path (which dispatches
asynchronously to a coordinator that may run for a long time) but is a
**design choice already available and unused** for the *simple* path.
This solution closes the simple-path gap inline, with no new RPC, and
reserves the async-callback machinery for the complex path where it's
structurally required.

## Design — a correctness bug found while grounding this design: status never reverts on dispatch failure

`ExecuteTask.Execute` calls `repo.UpdateStatus(..., StatusInProgress)`
**before** determining complexity or attempting dispatch
(`execute_task.go:57-59`), and never reverts that status if the
subsequent `simple.Execute`/`complex.Execute` call fails
(`execute_task.go:66-75`: the error is wrapped and returned, but no
compensating status write happens). Concretely: a task whose project's
dev server is offline gets marked `in_progress` permanently on every
`Execute` attempt, even though `SimpleExecutor` correctly returns
`TASK_EXECUTE_NO_CONNECTION` (`simple_executor.go:141-148`) — the error
surfaces to the caller, but the task is left in a false "running" state
indefinitely, with no RPC to clear it (the exact `README.md:177-188`
one-way-transition gap, but reachable via a guaranteed *failure* path,
not just the documented success path). This solution fixes it as part of
the same pre-check/status-transition rework the spec's "pre-check
(permission + dev-server-online)" step already calls for.

## Design — `usecase/execute_task.go` rework

```go
func (uc *ExecuteTask) Execute(ctx context.Context, in ExecuteTaskInput) (ExecuteResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    userID, _ := tenant.UserID(ctx)
    task, err := uc.repo.Get(ctx, tenantID, in.TaskID)
    if err != nil { return ExecuteResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", ...) }

    // Pre-check 1: permission — spec's "pre-check (permission +
    // dev-server-online)". Every mutating RPC calls ResolvePermission
    // internally first per task-service.md §3 (task-service.md:70-72);
    // ExecuteTask never has, despite dispatching real work — closed here.
    if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: userID, Action: "execute"}); err != nil {
        return ExecuteResult{}, err // PermissionDenied, unchanged shape
    }

    complex, err := uc.isComplex(ctx, tenantID, in.TaskID) // unchanged, computed BEFORE any status write now
    if err != nil { return ExecuteResult{}, ... }

    // Pre-check 2: dev-server-online — resolved once here (not duplicated
    // inside SimpleExecutor/WorktreeProvisioner), so a disconnected
    // project fails BEFORE the in_progress write, not after.
    connectionID, worktreePath, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
    if err != nil || !connected {
        return ExecuteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", err)
    }

    // Worktree reuse-or-create — spec's "IF task.worktreeId exists: use
    // existing worktree ELSE: create one" (see WorktreeProvisioner below).
    worktreeID, worktreePath, err := uc.worktrees.EnsureWorktree(ctx, tenantID, task, worktreePath)
    if err != nil { return ExecuteResult{}, ... }
    if worktreeID != task.WorktreeID {
        if err := uc.repo.UpdateWorktreeID(ctx, tenantID, in.TaskID, worktreeID); err != nil { return ExecuteResult{}, ... } // SOL-TG-01's worktree_id column
    }

    previousStatus := task.Status
    if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, domain.StatusInProgress); err != nil {
        return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_STATUS_UPDATE_FAILED", ...)
    }
    dispatchStart := uc.clock.Now() // for actual_hours on the simple-path inline-completion below

    var ref string
    if complex {
        ref, err = uc.complex.Execute(ctx, tenantID, in.TaskID, in.RequestID, worktreeID) // returns immediately — coordinator_run_id as ref
        if err != nil {
            _ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus) // revert — the bug fix
            return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", ..., err)
        }
        // No further status write here — StatusReview/Done arrives later
        // via ReportTaskExecutionResult, called BY orchestration-service.
        return ExecuteResult{ExecutionRef: ref, Async: true}, nil
    }

    // Simple path: SimpleExecutor.Execute blocks until the CLI process
    // exits (see this file's "key finding" above) — the completion
    // transition happens INLINE, in the same call, no separate RPC.
    result, err := uc.simple.Execute(ctx, tenantID, in.TaskID, in.RequestID, connectionID, worktreePath, uc.buildContext(ctx, tenantID, task))
    if err != nil {
        _ = uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, previousStatus) // revert — the bug fix
        return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_FAILED", ..., err)
    }
    actualHours := uc.clock.Now().Sub(dispatchStart).Hours()
    if err := uc.repo.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, actualHours); err != nil { // SOL-TG-01's actual_hours + status=review, agent_session_id cleared per spec
        return ExecuteResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_COMPLETION_WRITE_FAILED", ..., err)
    }
    return ExecuteResult{ExecutionRef: result.Ref, Async: false, Stdout: result.Stdout}, nil
}
```

The pre-existing `execute_task_test.go` complexity-branch tests are
unaffected (the branch decision itself is unchanged, only reordered
relative to the status write) — new tests cover the revert-on-failure and
inline-completion paths (see Test plan).

## Design — real `ComplexExecutor`

```go
// internal/adapter/grpcclient/complex_executor.go
type ComplexExecutor struct {
    orch  orchestrationv1.OrchestrationServiceClient
    tasks usecase.TaskRepository
    edges usecase.EdgeRepository
}

func (c *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
    spec, err := c.buildOrchestrationSpec(ctx, tenantID, taskID) // walks the subtree via SOL-TG-01's GetSubtree + depends_on edges
    if err != nil { return "", fmt.Errorf("complex_executor: build spec: %w", err) }

    resp, err := c.orch.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
        TenantId: tenantID, OriginTaskId: taskID, Spec: spec, WorktreeId: worktreeID,
    })
    if err != nil { return "", fmt.Errorf("complex_executor: start coordinator run: %w", err) }
    return resp.GetId(), nil // coordinator_run_id — task-service's own "logical FK", per task-service.md §3.1's active_execution_id note
}

// buildOrchestrationSpec translates task-service's own subtree (parent_child
// + depends_on edges, this task's own id space) into orchestration-service's
// OrchestrationTask DAG shape (orchestration-service.md §4's `deps` field
// requires only sibling ids WITHIN one coordinator_run — this function's
// whole job is producing that closed id set from a task-service subtree,
// since the two services deliberately use distinct id spaces, per
// orchestration-service.md §2.1's "different id spaces... do not merge").
func (c *ComplexExecutor) buildOrchestrationSpec(ctx context.Context, tenantID, rootTaskID string) (*orchestrationv1.CoordinatorRunSpec, error) {
    subtree, deps, err := c.tasks.GetSubtree(ctx, tenantID, rootTaskID, 0) // SOL-TG-01
    // one spec node per subtree task: {tempId: task.ID, title, promptTemplate/description, deps: [tempId,...]}
    // origin_task_id = rootTaskID; orchestration-service mints its OWN
    // orchestration_tasks.id per node — task-service.Task.ID rides along
    // as each node's origin reference (a field on OrchestrationTask,
    // orchestration-service.md §4), not as the primary key there.
}
```

`orchestration-service`'s own `StartCoordinatorRun` (`orchestration-service.md:64`)
is a genuinely async call — it starts a `coordinator_run` and returns,
it does not block for the DAG to finish (§2.2's table: "Write pattern:
Agent-driven state-machine ticks, continuous during a run" —
fundamentally unlike the simple path's synchronous `agent.execPrompt`
block). This is why `ExecuteTask`'s complex branch above returns
`Async: true` immediately with no inline completion — the eventual
completion write happens only via the inbound callback below.

## Design — `ReportTaskExecutionResult` — the inbound completion callback for the complex path

```protobuf
// task.proto — new RPC, service-to-service only (see security note below)
rpc ReportTaskExecutionResult(ReportTaskExecutionResultRequest) returns (google.protobuf.Empty);

message ReportTaskExecutionResultRequest {
  string task_id = 1;
  string coordinator_run_id = 2; // must match the active_execution_id task-service recorded, see below
  bool success = 3;
  double actual_hours = 4;
  string error_message = 5; // set iff !success
}
```

```go
// internal/usecase/report_execution_result.go
func (uc *ReportTaskExecutionResult) Execute(ctx context.Context, in ReportTaskExecutionResultInput) error {
    tenantID, err := tenant.RequireTenantID(ctx) // from the SERVICE identity (mTLS/service JWT), not a user session — see security note
    task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
    if task.ActiveExecutionID != in.CoordinatorRunID {
        // Stale/duplicate callback (e.g. a retried NATS delivery, or a
        // callback for a run this task was re-dispatched away from) —
        // ignored, not an error, matching at-least-once consumer
        // idempotence per 05-data-architecture.md's outbox-consumer note.
        return nil
    }
    if in.Success {
        return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, in.ActualHours)
    }
    return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusBlocked, in.ActualHours) // failed complex execution -> Blocked, not silently left in_progress or auto-Done
}
```

**Security note**: this RPC is called by `orchestration-service`, never a
browser/mobile client — `api-gateway` never routes to it.
`07-security-architecture.md`'s service-to-service row ("Short-lived JWT,
mTLS as a second factor... mTLS identity comes from the service mesh,"
`07-security-architecture.md:9`) is the enforcement mechanism: this
handler must validate the calling service's mesh identity is
`orchestration-service` specifically (not just "any internal service"),
via whatever `common/grpcmw` interceptor already extracts service
identity from mTLS for other internal-only RPCs in this codebase — flagged
as a wiring detail to confirm against `common/grpcmw`'s existing
interceptor set during implementation, not re-derived here.

`Task` needs an `ActiveExecutionID` field (mirrors `task-service.md §3.1`'s
`active_execution_id` sketch column, `task-service.md:82-85`) to make the
staleness check above possible — add it alongside SOL-TG-01's other field
additions (`worktree_id`/`agent_session_id`), set by `ComplexExecutor`
right after `StartCoordinatorRun` succeeds.

## Design — task-context preamble (`buildExecutePrompt`)

```go
// internal/adapter/grpcclient/simple_executor.go
func buildExecutePrompt(task domain.Task, parent *domain.Task, completedDeps []domain.Task) string {
    var b strings.Builder
    b.WriteString("Complete the following task.\n\n")
    fmt.Fprintf(&b, "Task: %s\n", task.Title)
    if task.Description != "" { fmt.Fprintf(&b, "Description: %s\n", task.Description) }
    if task.AIContext != "" { fmt.Fprintf(&b, "Context: %s\n", task.AIContext) }
    if parent != nil { fmt.Fprintf(&b, "\nParent task: %s\n%s\n", parent.Title, parent.Description) }
    if len(completedDeps) > 0 {
        b.WriteString("\nCompleted dependencies:\n")
        for _, d := range completedDeps {
            fmt.Fprintf(&b, "- %s: %s\n", d.Title, d.Description) // spec's {{#each completedDeps}}, rendered plain-text per this codebase's existing convention (ai_decompose.go's doc comment, no live agent JSON contract to target instead)
        }
    }
    return b.String()
}
```

`ExecuteTask.buildContext` (called once per `Execute`, before dispatch)
assembles `parent`/`completedDeps` via `uc.tasks.GetAncestors(...)[1]`
(if any) and `uc.getDependencies.Execute(...)` filtered to
`Status == StatusDone` — both already-existing usecases/repository calls
(`GetAncestors`: `repository.go:101-143`; `GetDependencies`:
`get_dependencies.go`), reused rather than duplicated.

Uses `task.PromptTemplate` (SOL-TG-01/SOL-TG-02) in place of the generic
"Complete the following task" opener when set — a task with an
AI-generated or user-edited prompt template should use it verbatim
(spec's `prompt_template` field is exactly for this), falling back to the
generic wording only when empty.

## Design — env-var injection (no `agent/` change needed)

`SimpleExecutor`'s own doc comment already establishes that the real
`agent.execPrompt` RPC accepts an optional `env` map
(`simple_executor.go:52-54`: "Optional: `stepId`... `env`, `timeoutMs`") —
this field exists on the live Dev Server Agent contract today and is
simply never populated. Closing this gap needs **no `agent/` change**,
only using an existing, already-supported field:

```go
type agentExecPromptParams struct {
    Prompt       string            `json:"prompt"`
    WorktreePath string            `json:"worktreePath"`
    StepID       string            `json:"stepId,omitempty"`
    Env          map[string]string `json:"env,omitempty"` // new
}

// populated in SimpleExecutor.Execute:
env := map[string]string{
    "ORCA_TASK_ID":    task.ID,
    "ORCA_PROJECT_ID": task.ProjectID,
}
```

`ANTHROPIC_MODEL`/`GH_CONFIG_DIR` from the spec map to the already-supported
`model`/`accountId` top-level params (`simple_executor.go:47-50`'s doc
comment) rather than `env` entries — `SimpleExecutor` still has no
per-task AI-provider-account pin (unchanged from today, per that same
doc comment's "omit when unresolved" convention) — wiring a real pin is
tracked as a follow-up, not solved by this bug's scope (it would need
task-service to call `AIProviderContextResolver` for the execute path the
way `AIDecompose` already does, a reasonable next step but a distinct
change from context-preamble/env-injection).

## Design — worktree reuse-or-create

```go
// internal/adapter/grpcclient/worktree_provisioner.go
type WorktreeProvisioner struct {
    git gitgatewayv1.GitGatewayServiceClient // reused dial, same as SOL-TG-02's TechStackDetector
}

func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task, defaultWorktreePath string) (worktreeID, path string, err error) {
    if task.WorktreeID != "" {
        return task.WorktreeID, defaultWorktreePath, nil // reuse — spec's "IF task.worktreeId exists: use existing worktree"
    }
    // Create — delegates the whole create+record saga to git-gateway-service's
    // EXISTING CreateWorktree RPC (gitgateway.proto:86), which already does
    // "git worktree add" + project-service bookkeeping in one saga
    // (git-gateway-service/internal/usecase/create_worktree.go:16-29) —
    // this port is a thin caller, not a re-implementation of that saga.
    resp, err := p.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
        ProjectId: task.ProjectID, Branch: fmt.Sprintf("task/%s", task.ID),
    })
    if err != nil { return "", "", fmt.Errorf("worktree_provisioner: create worktree: %w", err) }
    return resp.GetWorktreeId(), resp.GetPath(), nil
}
```

`RepoID` resolution (git-gateway-service's `CreateWorktreeRequest` needs
a `repo_id`, per `create_worktree.go:12`'s `CreateWorktreeInput`) is not
yet available from a bare `ProjectID` at this call site — flagged as an
open wiring detail: either `CreateWorktreeRequest` gains a
`project_id`-based overload that resolves the project's default repo
server-side (a `git-gateway-service`-side convenience, since it already
calls `project-service` in the same saga), or `WorktreeProvisioner`
resolves `RepoID` itself via a `project-service` call first. Left as an
implementation-time decision rather than guessed at here, since it
depends on `project-service`'s current repo-per-project cardinality,
outside this bug's read set.

## Design — the streaming gap: flagged as needing Dev Server Agent + infra-fleet-service changes

**This part of the spec cannot be closed with `task-service`-only
changes.** `agent.execPrompt`'s real contract, as already documented in
this codebase (`simple_executor.go:92-98`), is fully synchronous: one
request, one final `{stdout, stderr, exitCode, timedOut}` response, no
incremental delivery. Achieving the spec's "stream PTY output into a Task
Activity Feed over WebSocket" needs:

1. **`agent/`** — a new Dev Server Agent capability that emits incremental
   output (either a new streaming RPC method alongside `agent.execPrompt`,
   or push notifications over the existing relay connection keyed by
   `stepId`) — genuinely new agent-side work, not a backend-go-only gap.
2. **`infra-fleet-service`** — a server-streaming gRPC endpoint to carry
   those chunks from the relay connection to `task-service` (or directly
   to `api-gateway`), analogous to the terminal-data streaming endpoint
   `infra-fleet-service.md` already describes for PTY sessions
   ("a dedicated server-streaming RPC once the route is resolved,"
   `infra-fleet-service.md:363-366`) — this solution's `SimpleExecutor`
   would need to switch from a unary `Relay` call to consuming that
   stream.
3. **`task-service`** — republish received chunks as `task.agent_output`
   events (outbox → NATS → `api-gateway` WS push), the `TaskActivityEvent`
   union BUG-TG-04 names in full.

None of this is designed further here — it is a multi-service,
cross-repo change with its own tradeoffs (buffering, backpressure,
partial-output persistence) that deserves its own design pass once
prioritized. This solution's job is to state precisely why `task-service`
alone cannot close it, per the explicit instruction to flag `agent/`
dependencies.

## Design — batch execution (`ExecuteBatch`)

```go
// internal/usecase/execute_batch.go
func (uc *ExecuteBatch) Execute(ctx context.Context, in ExecuteBatchInput) (ExecuteBatchResult, error) {
    edges, err := uc.edges.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn) // scoped to in.TaskIDs by the caller-supplied set
    order, err := domain.TopologicalWaves(edges, in.TaskIDs) // new pure domain function, reuses buildDependsOnGraph from SOL-TG-02's critical-path work — groups into waves of mutually-independent tasks
    for _, wave := range order {
        // bounded concurrency, e.g. a semaphore-limited errgroup, per spec's
        // "limited parallelism" — limit is a config value, not hardcoded
        results := dispatchWaveConcurrently(ctx, uc.executeTask, wave, in.MaxConcurrency)
        for _, r := range results {
            if r.Err != nil && in.StopOnFailure { return partial, r.Err }
        }
    }
    return complete, nil
}
```

`{{outputs.<taskId>.*}}` interpolation needs each task's last stdout
retained somewhere readable by a later wave's prompt build — this
solution proposes a bounded `last_execution_output TEXT` column
(truncated, e.g. 8KB) on `task.tasks`, set by the same
`CompleteExecution` write the simple-path inline-completion step already
makes, and read by `buildExecutePrompt` when a task's prompt references
another in-batch task by id. Flagged as an additive design choice, not
directly named by BUG-TG-04's "what's missing" list, needed to make the
spec's interpolation feature representable at all — the storage-size
tradeoff (full output vs. truncated) is a product call worth confirming
before implementation.

## Test plan

- `usecase/execute_task_test.go` — new cases: `simple.Execute` returning
  an error reverts status to the pre-dispatch value (fake `TaskRepository`
  asserts `UpdateStatus` called twice: `InProgress` then the original);
  successful simple execution writes `StatusReview` + a non-zero
  `actual_hours` inline, in the same `Execute` call, with no second RPC;
  a `ResolvePermission` denial short-circuits before any `UpdateStatus`
  call at all (regression guard against the false-in_progress bug this
  solution fixes); complex-path success returns `Async: true` and leaves
  status at `InProgress` (no inline completion) with `ActiveExecutionID`
  set to the returned ref.
- `adapter/grpcclient/complex_executor_test.go` — fake
  `OrchestrationServiceClient`: `buildOrchestrationSpec` produces one spec
  node per subtree task with correctly-translated `deps` (index-based,
  not raw task-service IDs, per `orchestration-service.md §2.1`'s
  distinct-id-space rule); a subtree fetch error never reaches
  `StartCoordinatorRun`.
- `usecase/report_execution_result_test.go` — a callback whose
  `coordinator_run_id` doesn't match the task's current
  `ActiveExecutionID` is a silent no-op, not an error (idempotence/staleness
  guard); success → `StatusReview`; failure → `StatusBlocked`, never a
  status revert to the pre-dispatch value (that's the simple path's
  failure semantics, not the complex path's).
- `adapter/grpcclient/simple_executor_test.go` — `buildExecutePrompt`
  golden-output tests: description/aiContext/parent/completedDeps each
  appear when present, are cleanly omitted when absent (no empty
  "Description: " lines); `env` map always contains `ORCA_TASK_ID`/
  `ORCA_PROJECT_ID`.
- `adapter/grpcclient/worktree_provisioner_test.go` — a task with an
  existing `WorktreeID` never calls `CreateWorktree`; an empty one does,
  and the returned ID is what `ExecuteTask` persists back onto the task.
- `domain/topological_waves_test.go` — diamond dependency graph produces
  the correct wave grouping (independent siblings in one wave, not
  serialized); a task with no dependency among the batch set is wave 0.

## References

- `docs/logic/task-graph/BL-TG-04-task-agent-execution.md` — full spec
- `specs/backend-go/tdd/services/task-service.md:74-96` (§3.1/§3.2 dispatch
  design), `:255-284` (§7 dependency graph, `orch --> task` callback
  direction this solution's `ReportTaskExecutionResult` implements)
- `specs/backend-go/tdd/services/orchestration-service.md:1-70` (§1-§3 —
  bounded context, id-space isolation, `StartCoordinatorRun` API this
  solution's `ComplexExecutor` calls)
- `specs/backend-go/tdd/services/infra-fleet-service.md:363-366`
  (server-streaming precedent cited for the flagged streaming gap)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166`
  (dependency graph — `task --> orch`, `orch --> task` edges)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:5-10`
  (service-to-service AuthN — the mechanism `ReportTaskExecutionResult`'s
  security note relies on)
- `backend-go/services/task-service/internal/usecase/execute_task.go:1-94`
  (the status-never-reverts bug this solution fixes, found while grounding
  this design)
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go:1-193`
  (real relay + the `env`-field citation trail this solution's env-injection
  design relies on), `complex_executor.go:1-26` (the stub this solution
  replaces)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:1-60`
  (the existing create+record saga `WorktreeProvisioner` calls into rather
  than reimplementing)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:86` (`CreateWorktree`
  RPC, already landed)
- `specs/backend-go/bugs/missing-v1/BUG-034-task-channels-not-implemented.md`
  — confirms `ComplexExecutor`/`SimpleExecutor`'s stub status pre-dates this
  solution and that `task.execute` isn't yet wired into `wscompat`
- `specs/backend-go/bugs/logic-v1/BUG-TG-01-task-graph-structural-management-partial.md`
  — `worktree_id`/`agent_session_id`/`active_execution_id`/
  `last_execution_output`/`actual_hours` fields this solution depends on
  (see [SOL-TG-01](./SOL-TG-01-task-graph-structural-management.md))
- `specs/backend-go/bugs/logic-v1/BUG-TG-02-ai-task-planning-partial.md` —
  `prompt_template` and the topological-sort building blocks
  (`buildDependsOnGraph`) this solution's batch execution reuses (see
  [SOL-TG-02](./SOL-TG-02-ai-task-planning.md))
