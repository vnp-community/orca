# BUG-027: `task.execute`'s direct_agent path blocked the client-facing RPC for the whole agent run, timing out — redesigned to async

**Service:** `task-service`
**Severity:** 🔴 Critical — every `task.execute` dispatch that reaches SimpleExecutor (direct_agent engine — the common case for a leaf task) can run the spawned CLI process for up to 15 minutes; the client-facing RPC/WS round-trip died well before that on every real test.
**Status:** ✅ Root cause confirmed via live logs + source trace. ✅ Redesigned to async, unit-tested (including a real-goroutine regression test), deployed. Explicit user request: *"thiết kế lại đi. phải theo async"* (redesign it, it must be async), issued right after this exact failure was reproduced live.

## What was wrong

After BUG-026's 5 fixes cleared every earlier gate (connection resolution, worktree provisioning, tenant/dev-server relay routing), the agent dispatch itself finally reached the Dev Server Agent — and the user's browser then failed with:

```
RuntimeRpcCallError: rpc error: code = DeadlineExceeded desc = context deadline exceeded
```

task-service's own log for the same request:

```
{"level":"WARN","msg":"task: execute dispatch failed","dispatch_error":"simple_executor: relayByDevServer agent.execPrompt: rpc error: code = Canceled desc = context canceled", ...}
{"level":"ERROR","msg":"rpc failed","method":"/orca.task.v1.TaskService/Execute","duration":25000708874, ...}
```

`duration: 25000708874ns ≈ 25.0007s` — a near-exact match to the frontend's WS connection liveness watchdog (`REMOTE_RUNTIME_SOCKET_LIVENESS_TIMEOUT_MS` / `HEARTBEAT_IDLE_MS`, both `25_000`ms, `frontend/src/shared/remote-runtime-socket-liveness.ts` / `frontend/src/renderer/src/web/web-runtime-client.ts`). The client-side connection was torn down mid-run, canceling the still-in-flight request's context, which cascaded down through api-gateway → task-service → infra-fleet-service as `context canceled`, surfaced to the browser as a raw `DeadlineExceeded`.

## Root cause

`ExecuteTask.Execute` (task-service's core dispatch usecase) called `uc.simple.Execute(ctx, ...)` **inline, on the same goroutine as the client-facing RPC**, for the `direct_agent` engine. `SimpleExecutor.Execute` blocks synchronously on the Dev Server Agent's `agent.execPrompt` call, which can legitimately run for up to 15 minutes (`agent-print-mode-exec.ts`'s `MAX_TIMEOUT_MS`) — an AI coding agent doing real work. Holding a single client-facing WS/gRPC round-trip open for that entire duration was never viable: any client-side liveness/idle watchdog (this one at 25s) will kill the connection long before the agent finishes, and the failure mode look like an infrastructure bug rather than what it is — a fundamentally wrong RPC shape for a long-running operation.

This was already a known, documented gap: `simple_executor.go`'s own doc comment says *"task-service still has no execution-completion callback [for the direct_agent path]"* — this bug is that gap becoming live-observable once every earlier dispatch bug (BUG-025, BUG-026) was fixed and a real execution could finally reach this point.

Notably, the OTHER two engines (`EngineOrchestration`/`EngineWorkflow`) never had this problem: their own `.Execute()` calls are fast (they just kick off a request to another service and return a ref), and their real completion already arrives later via `ReportTaskExecutionResult` (TASK-TG-04-05) — `ExecuteTask.Execute` already returns `Async: true` immediately for them. Only `direct_agent` was still fully synchronous end-to-end.

## Fix

`ExecuteTask.Execute`'s `EngineDirectAgent` branch now dispatches via a new `dispatchDirectAgentAsync` method instead of inline:

1. All the existing SYNCHRONOUS pre-checks are unchanged and still block the RPC (they're fast — sub-second per logs): permission resolve, connection resolve, worktree provision, `StatusInProgress` write, execution-link create.
2. Once dispatch is durably recorded (execution link created + marked active), `Execute` calls `uc.runAsync(fn)` — real wiring (`NewExecuteTask`) spawns `fn` as a genuine goroutine — and returns `ExecuteResult{Async: true}` **immediately**, without waiting for `SimpleExecutor.Execute` at all.
3. `fn` (running in the background) does the SAME completion bookkeeping this call used to do inline: on failure, revert the task's status to its pre-dispatch value and mark the execution link `"failed"` (same fix as TASK-TG-04-01's existing revert-on-failure posture, just delivered asynchronously); on success, persist `StatusReview` + `actual_hours` and mark the link `"completed"`.
4. `fn` runs against a **detached context** (`dispatchCtx`), not the original request `ctx` — the original ctx dies with the RPC being decoupled from; `dispatchCtx` rebuilds only the identity values `SimpleExecutor.Execute` and its downstream resolvers actually read from context (tenant id, user id) onto a fresh `context.Background()`, with no deadline and no tie to the client connection.

`EngineOrchestration`/`EngineWorkflow` are unchanged — they were never the source of this timeout, so their synchronous-dispatch-error-propagation UX (immediate error toast on a fast orchestration-service/workflow-service rejection) is preserved rather than needlessly widened to "always async, always silent about fast failures."

`TaskServiceExecuteResponse.async` (proto) is now `true` for every engine, including direct_agent — comment updated; no wire-format change (the field already existed and was already read/passed through verbatim by both the gRPC server handler and the wscompat `task.execute` bridge, and the frontend never reads it).

## Known, flagged-not-fixed side effect

`ExecuteBatch` (`execute_batch.go`) calls `ExecuteTask.Execute` per task and waits for each dependency-respecting wave to finish before starting the next, relying on the OLD synchronous-completion behavior for direct_agent tasks (needed so `TASK-TG-04-07`'s `{{outputs.taskId.*}}` prompt interpolation can read a dependency's already-persisted `LastExecutionOutput`). **Confirmed: `ExecuteBatch` is dead code — `NewExecuteBatch` has no caller anywhere in this service outside its own test file, no gRPC RPC exposes it, and `cmd/server/main.go` never constructs one.** This redesign has zero production impact via that path. `ExecuteBatch`'s own tests were updated (two tests that needed a *synchronously observable* dispatch failure now use the orchestration engine for that instead of direct_agent, since that's what they were actually testing — `ExecuteBatch`'s own wave-halting logic, not `SimpleExecutor`'s error-propagation shape) — flagged here for whoever wires `ExecuteBatch` up to a real RPC next: it will need its own completion signal (e.g., a channel from `dispatchDirectAgentAsync`, or synchronous mode opted into per-call) to keep working correctly for direct_agent tasks specifically.

## Testing

- `TestExecuteTask_DirectAgentDispatch_ReturnsBeforeExecutorFinishes` — the core regression, using the REAL production `runAsync` (a genuine `go fn()`, not the test helper's synchronous override): proves `Execute()` returns while `SimpleExecutor.Execute` is still blocked (verified via a channel-gated fake executor), and that completion (status → review) only lands after the fake is unblocked. Passes under `-race`.
- `TestExecuteTask_DirectAgentFailure_RevertsStatusWithoutError` — a direct_agent dispatch failure no longer propagates as `Execute`'s own error (caller already got `Async: true`), but the task's status still reverts and the execution link is still marked `"failed"`, same outcome as before, just asynchronous.
- `TestExecuteTask_SimplePath_NoSubtasksNoDependencies` / `TestExecuteTask_SimplePath_CompletesInlineWithActualHours` updated: the immediate response has an empty `ExecutionRef` and `Async: true`; completion side effects are asserted via the test helper's synchronous `runAsync` override (deterministic, no sleep/poll needed for these).
- `TestExecuteTask_ExecutorFailurePropagates` / `TestExecuteTask_DispatchFailure_RevertsStatusToPrevious` repointed to the orchestration engine, which still propagates a fast dispatch failure synchronously — unchanged behavior for that engine, now tested against it specifically instead of direct_agent.
- All pre-existing `ExecuteTask`/`ExecuteBatch` tests still pass (2 `ExecuteBatch` tests adjusted per the flagged side effect above).
- `go build`, `go vet`, `go test ./services/task-service/...` all pass. `go test -race` passes for every test file this bug touched (`execute_task_test.go`); a pre-existing, unrelated race in `ExecuteBatch`'s own dead-code test fakes (`fakeClock`, `fakeExecutionLinkRepository` have no mutex, and `ExecuteBatch`'s own per-wave fan-out already called `Execute` concurrently before this bug) is out of scope — not introduced by this fix, not fixed here.
- **Not yet live-verified against the real Dev Server Agent** — pending this deploy and a user retry. The next thing to check if a live run still fails: whether the task's status actually flips to `review` once the agent finishes (proving the backgrounded goroutine really completed), via `task.get`/`task.list` after a wait, since the browser will no longer receive a synchronous completion signal — that is the intended, correct new behavior, not a regression.

## Second bite — the async redesign let a user's repeated clicks pile up concurrent dispatches, crashing the dev server connection and leaving the task stuck at `in_progress` forever

The first deploy of this redesign made `task.execute` return in milliseconds instead of blocking — confirmed live (`Execute` RPC durations dropped from 25s+ to 14-120ms). But the user then reported: *"sao t ấn liên tục và k thấy có kết quả trả về gì cả"* (why do I keep clicking and see no result at all).

Reading infra-fleet-service's log for the same window revealed the real sequence:

```
{"level":"ERROR","msg":"apperrors: internal cause (not sent to client)","code":"INFRA_AGENT_EXEC_FAILED","cause":"devserveragent: connection lost: failed to get reader: failed to read frame header: EOF"}
...
{"level":"WARN","msg":"dev server disconnected","devServerId":"a1825a89-...","host":"test-01", ...}
...
{"level":"ERROR","msg":"rpc failed","method":"/orca.infrafleet.v1.InfraFleetService/RelayByDevServer","error":"...INFRA_DEV_SERVER_NOT_CONNECTED: this dev server has no live agent connection right now", ...}
```

**Root cause #2:** making `task.execute` return instantly removed the ONE thing that had implicitly throttled repeat clicks before — a blocking RPC the user couldn't easily re-trigger before the first one either succeeded or visibly timed out. With an instant response, a user who sees no visible feedback (a fair reaction — nothing on screen changes when a background dispatch merely starts) has every reason to click again, and again. Each click fired an independent concurrent `RelayByDevServer` call against the SAME dev server's single agent connection; several of those concurrent calls appear to have destabilized that connection (`connection lost: EOF`), which then genuinely dropped (`"dev server disconnected"`), failing every dispatch in flight and every one after it until the agent reconnected on its own.

Worse, this exposed a SECOND, independent bug in `ExecuteTask.Execute` itself: each concurrent `Execute` call captures its own `previousStatus` snapshot (the task's status at the moment THAT call started) to revert to on failure. Once the first concurrent call had already written `StatusInProgress`, every subsequent concurrent call's `previousStatus` snapshot was **also** `in_progress` — so when those calls' backgrounded dispatches failed, their "revert" was a no-op (reverting to the same in_progress value). The task was left stuck at `in_progress` **permanently**, live-confirmed in the DB (5 execution_links rows, all `status_mirror='failed'`, task status still `in_progress`) — exactly the TASK-TG-04-01 failure mode this usecase had already fixed once, reintroduced by concurrent re-entrancy the synchronous design had never allowed to happen in practice.

**Fix #2 (backend):** `ExecuteTask.Execute` now rejects outright — before any pre-check work, before any status write — whenever the task is already `in_progress` (`TASK_EXECUTE_ALREADY_IN_PROGRESS`, `KindFailedPrecondition`). This still has a narrow TOCTOU race (two calls within microseconds of each other), not worth closing with a CAS-style status write for what's fundamentally a "stop the user from shooting themselves in the foot" guard — it collapses "guaranteed pile-up from 10 clicks over several seconds" (what actually happened) down to "not practically reachable," which is the real goal.

**Fix #2 (frontend):** both "Run with Agent" entry points now disable their dispatch button while the task is `in_progress`, closing the actual UX gap that caused the repeated clicking in the first place:
- `TaskDetail.tsx`'s "Execute with Agent" button: disabled + relabeled "⏳ Agent running…" while `isDispatching` (set immediately on click, closing the gap before polling catches up) OR the polled task's status is `in_progress`.
- `TaskPromptEditor.tsx`'s "Run with Agent"/"Generate Spec"/"Implement Spec" buttons: same guard, driven by a `task.status === 'in_progress'` prop check (`isTaskRunning`) rather than only the local `isRunning` flag, which used to reset right after the now-fast RPC resolved regardless of whether the backgrounded dispatch was still running. `TaskDetail` now passes its polled (live) task into `TaskPromptEditor` instead of the possibly-stale store copy, so this prop actually reflects reality once polling (`useTaskActivity`, every 4s) catches up.

**Data repair:** the one task stuck by this bug during live testing (`fa558891-...`, a test task) had its status manually reset from `in_progress` back to `open` and its stale `active_execution_link_id` cleared — a one-off correction for state this bug corrupted, not a schema or code change.

### Testing (fix #2)

- `TestExecuteTask_AlreadyInProgress_RejectsReDispatch` — a task already `in_progress` gets `TASK_EXECUTE_ALREADY_IN_PROGRESS`/`KindFailedPrecondition`, with NO status write, worktree call, or executor call.
- `TaskDetail.test.tsx`: two new tests — polled `in_progress` disables the button (and relabels it), and clicking disables it immediately even before any poll resolves.
- `TaskPromptEditor.test.tsx`: a new test confirming `task.status === 'in_progress'` disables Run/Generate Spec/Implement Spec independent of local `isRunning`.
- All pre-existing tests in every touched file still pass; frontend `tsc --noEmit` shows no new errors in either touched file.

## Third bite — infra-fleet-service's own devserveragent client capped `agent.execPrompt` at 30 seconds

Even after fix #2's guard stopped the concurrent pile-up, the user's next single (correctly-guarded, single-click) retry still produced no result. task-service's diagnostic log showed the SAME `INFRA_AGENT_EXEC_FAILED` code as before, but infra-fleet-service's own log now showed a different underlying cause and a suspiciously exact duration:

```
{"level":"ERROR","msg":"apperrors: internal cause (not sent to client)","code":"INFRA_AGENT_EXEC_FAILED","cause":"devserveragent: request \"agent.execPrompt\" timed out: context deadline exceeded"}
{"level":"ERROR","msg":"rpc failed","method":"/orca.infrafleet.v1.InfraFleetService/RelayByDevServer","duration":30001891558, ...}
```

`duration ≈ 30.0019s` — infra-fleet-service's own outbound client to the Dev Server Agent (`devserveragent` package) killed the request itself, independent of anything upstream (browser, task-service) this time.

**Root cause #3:** `Client.Exec` (`devserveragent/client.go`) — the shared dispatch path both `Relay` and `RelayByDevServer` funnel every method through — called `session.call(ctx, method, params)`, which always applies `cfg.RequestTimeout` (30s, sized for short calls like `ports.scan`/`preflight.check`) via `context.WithTimeout(ctx, s.cfg.RequestTimeout)`. `agent.execPrompt` is the one method this codebase's OWN documentation already says blocks until the spawned CLI process exits — up to 15 minutes (`agent-print-mode-exec.ts`'s `MAX_TIMEOUT_MS`) — but `Exec` never special-cased it: every real agent run was killed client-side at 30 seconds, long before the CLI could produce any output at all. This is the actual reason no run ever "finished" or showed a result, regardless of how correctly task-service's own async dispatch and re-entrancy guard worked.

**Fix #3:** `session.call` now delegates to a new `session.callWithTimeout(ctx, method, params, timeout)`, which takes an explicit per-call timeout instead of always reading `cfg.RequestTimeout` (a `timeout<=0` falls back to `cfg.RequestTimeout`, so `call()`'s own behavior is unchanged for every existing caller). `Client.Exec` now calls `callWithTimeout` with a new `execTimeoutForMethod(method)` helper that returns a 15-minute cap (`execPromptTimeout`, matching `MAX_TIMEOUT_MS` exactly) for `"agent.execPrompt"` specifically, and `0` (→ the existing 30s default) for every other method — not widened to every method, since nothing else in this codebase documents a similarly long real duration; flagged for whoever adds the next long-running method to extend `execTimeoutForMethod` rather than guessing a codebase-wide value.

### Testing (fix #3)

- `TestClientExec_AgentExecPromptSurvivesLongerThanRequestTimeout` — a fake Dev Server Agent set to respond to `agent.execPrompt` after 300ms, with `cfg.RequestTimeout` set to 100ms (shorter than the delay): `Exec` still succeeds, proving `execTimeoutForMethod` actually routes this method around the flat cap (an ordinary method under the same delay/timeout combination would time out, unchanged — not itself re-tested here since `TestClientExecSucceedsAgainstFakeAgent` already covers the default-timeout path).
- All pre-existing `devserveragent` tests pass unchanged.
- `go build`, `go vet`, `go test ./services/infra-fleet-service/...` all pass.
- **Not yet live-verified against the real Dev Server Agent** — pending this deploy and a user retry. If a real run still doesn't show a result after this, the next thing to check is whether `agent.execPrompt` even reaches the CLI at all on the agent side (agent-print-mode-exec.ts), not another backend-go timeout guess.

## Fourth bite — the agent ran, reported success, and made zero actual changes: no trust preset for a headless dispatch

After fix #3 (the 30s timeout), the user asked for the flow to be tested precisely. `task.execute`'s own RPC and the frontend button both looked broken (no `Execute` RPC ever reached task-service after a click), so to isolate the frontend question entirely, `task.execute` was dispatched **directly via `grpcurl`** against `task-service:9090` (bypassing the browser, api-gateway, and any frontend state entirely), using the EXACT prompt `TaskPromptEditor.tsx`'s "Generate Spec" preset builds (write `specs/generated/TASK-1-spec.md`, commit it).

The RPC returned `{"async": true}` immediately, and the task's status did flip `in_progress` → `review` (execution link `status_mirror="completed"`) — the whole async/timeout/re-entrancy pipeline built in fixes #1-#3 worked exactly as designed. But checking the task's worktree directly on the real dev server (`172.20.2.41:/opt/repos/aiops-v3-task-.../`) showed: `git log` unchanged, `git status` clean, no `specs/` directory at all — the agent did **nothing**, despite task-service reporting success.

**Root cause #4:** `task.tasks.last_execution_output` (persisted by `SimpleExecutor.Execute`, the CLI's real stdout) had the actual explanation, in the agent's own words:

> "I didn't create the spec or make the commit: the save to `specs/generated/TASK-1-spec.md` was blocked because it needs your permission. A permission check also stopped me from finding out whether git ignores that folder... approve the write when it comes up again."

`agentExecPromptParams.TrustPreset` (this file's own doc comment: `"full"` appends the CLI's YOLO flag, anything else is a no-op — `agent-print-mode-exec.ts:44,97-99`) was never set by `SimpleExecutor.Execute` — it stayed at its zero value (empty string → "standard" trust). The CLI's default trust level requires interactive approval for file writes and git commands; `task.execute` is a one-shot, non-interactive RPC dispatch with no human present anywhere in the call path to answer that prompt. The agent correctly refused to write unapproved changes, reported that refusal, and exited 0 (a real, correct CLI behavior) — but `SimpleExecutor.Execute`'s only success check is `exitCode == 0`, so task-service had no way to distinguish "did real work" from "safely declined to write anything," and reported the dispatch as fully successful either way.

This is not new breakage from this bug's own earlier fixes — it is the actual, final reason `task.execute`'s direct_agent path could never have produced a real result for ANY prompt requiring a file write, from the very first attempt in this entire investigation. Every earlier fix (BUG-025, BUG-026, and fixes #1-#3 in this bug) was necessary to even reach this failure mode, since every prior attempt died before the agent ever got this far.

**Fix #4 (explicit user decision, not a silent default):** `SimpleExecutor.Execute` now sets `TrustPreset: "full"` unconditionally for every dispatch — asked and confirmed with the user first, given this is a real security-posture change (every automated task.execute dispatch now writes files/runs git commands without a human approval gate, for every project). Not made configurable per-task/per-project in this pass (no existing schema or UI concept for that on this path); flagged for whoever needs finer-grained control later.

### Testing (fix #4)

- `TestSimpleExecutor_Execute_SendsFullTrustPreset` — asserts the `params_json` sent to `Relay`/`RelayByDevServer` always carries `"trustPreset":"full"`.
- All pre-existing tests pass unchanged (`TrustPreset` was never asserted against before, so no test needed updating for the new value).
- **Live-verified this bite specifically via the grpcurl repro above** — the failure mode was directly observed (not inferred) by reading the agent's own real stdout, a stronger form of verification than every earlier bite in this bug, which relied on service-log inference alone.
- **Fix itself not yet live-verified** — pending this deploy and a retry (either via the UI, once the separate frontend-click question below is also resolved, or another direct grpcurl call).

## Open, not yet resolved — the frontend button still didn't fire

Separately from all four backend bites above, the user reported clicking "Generate Spec" then "Run with Agent" in the AI tab produced literally zero RPC traffic at task-service or api-gateway for ~13 minutes (only routine `task.get` polling, confirmed in both services' logs) — the click never reached the backend at all. This is why fix #4 was verified via a direct `grpcurl` call instead of a UI retry. Not yet diagnosed: needs the browser's own DevTools Console/Network output (asked for, not yet received) — every hypothesis reachable from server-side logs alone (stale `isTaskRunning`/`isDispatching` guard state introduced by this same bug's second bite) was reviewed and could not be confirmed or ruled out without client-side evidence.

## Related

- [BUG-025](./BUG-025-task-graph-tree-board-invisible-and-execute-crash.md), [BUG-026](./BUG-026-task-execute-no-connection-structural-gap.md) — the two prior fixes in this same live-debugging chain that had to land before this bug's failure mode could even be reached (every earlier attempt died before dispatch ever got this far).
- `simple_executor.go`'s own doc comment ("Honest limit carried over unchanged, not solved by this task: task-service still has no execution-completion callback") — this bug is that limit becoming user-visible and, per explicit user request, finally addressed.
