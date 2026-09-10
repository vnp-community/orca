# TASK-AG-FLOWTASK-003: `task-service` consumes `StreamExecOutput`, republishes to `task.activity`

**Task ID:** TASK-AG-FLOWTASK-003
**Priority:** 🔵 P3 (Phase D of CR-FLOW-TASK-003 — design-only, not scheduled)
**Solution Ref:** [SOL-AG-FLOWTASK-001](../solutions/SOL-AG-FLOWTASK-001-execution-activity-streaming-design.md) §2.3
**Depends on:**
- TASK-AG-FLOWTASK-002 (`StreamExecOutput` must exist in `infra-fleet-service` first)
- **CR-FLOW-TASK-001** (`execution_links` — needed as the place to mirror Engine 1's run status)
- **CR-FLOW-TASK-003** (the `task.activity:{taskId}` WS channel + `TaskActivityFrame` shape must
  ship first — this task has nowhere to publish to otherwise)
**Status:** [x] DONE — implemented this pass. Both blocking dependencies were confirmed already
real in this worktree before starting (re-read live code, not assumed from the design doc):
`execution_links`/`ExecutionLinkRepository` (CR-FLOW-TASK-001) and `task.activity:{taskId}` +
`TaskActivityFrame` (`channels_task_activity.go`, CR-FLOW-TASK-003/BE-SOL-003, done by a parallel
agent's TASK-FT-003-01..05 work) both exist and were reused as-is, not rebuilt.

Implementation, real code shape differs from this task's §2.3 sketch in one structural way the
sketch didn't anticipate: `SimpleExecutor` (task-service) and `StreamExecOutput`
(infra-fleet-service, TASK-AG-FLOWTASK-002) are **different OS processes** talking over gRPC — the
sketch's "`SimpleExecutor.Execute` calls `StreamExecOutput` in parallel with `Exec`" reads as an
in-process call, which it cannot be. `SimpleExecutor.Execute` now:
1. Resolves `connectionID` (as before), then opens `StreamExecOutput`'s new gRPC stream (via the new
   `AgentExecOutputRelay` adapter, `usecase.AgentExecOutputStreamer` port) CONCURRENTLY with its own
   unary `Relay('agent.execPrompt')` call — same `stepId = requestID` both calls share.
2. `publishThrottledOutput` drains chunks, batches stdout/stderr into a cumulative buffer, flushes
   at most once per `throttleInterval` (2s default — Open Question 1: no real traffic data existed
   to tune this, 2s is a starting point like the task doc anticipated) via `InsertOutboxEvent`
   (task-service's own new outbox: `task.outbox_events` migration `0005`, `postgres.Repository`
   implements `common/outbox.Store` + `usecase.OutboxWriter`, `cmd/server/main.go` wires an
   `outbox.Relay` publishing subject `orca.task.agent_output_partial` on a new `"TASK"` JetStream
   stream — mirrors `orchestration-service`'s identical wiring exactly).
3. `channels_task_activity.go` (api-gateway) subscribes to that subject, `classifySubject` maps it
   to `Engine: "direct_agent"`, `EventType: "agent_output_partial"` — reused the existing
   `TaskActivityFrame`/`translateToTaskActivity`/`origin_task_id` filter mechanism verbatim, no new
   WS channel (the file's stale "direct_agent never appears here" doc-comment claim was corrected).
   `SimpleExecutor.Execute`'s own success/failure contract is unchanged — streaming a subscribe
   failure never fails or blocks `Execute` (Relay's response stays the sole completion signal).
4. Open Question 2 (model scope): moot in the current real code — `SimpleExecutor` never sends a
   `model` param at all (see `simple_executor.go`'s own doc comment), so every `Execute` call is
   already implicitly claude-only; no gemini/codex/opencode branch exists to guard here.
5. Open Question 3 (`status_mirror` partial-state field shape): NOT touched — this pass publishes
   only to `task.activity`'s outbox/NATS path, it does not write `execution_links.status_mirror`
   for partial progress, so no schema decision on that field was needed.

Files: `migrations/0005_outbox_events.{up,down}.sql`, `internal/adapter/postgres/outbox.go`,
`internal/usecase/ports.go` (`OutboxWriter`, `AgentExecOutputStreamer`, `AgentExecOutputChunk`),
`internal/adapter/grpcclient/agent_exec_output_relay.go` (+ test),
`internal/adapter/grpcclient/simple_executor.go` (+ test), `cmd/server/main.go`; api-gateway's
`channels_task_activity.go` (+ test).

Verified: `cd backend-go && go build ./services/task-service/...` and
`go vet ./services/task-service/...` and `go test ./services/task-service/...` all clean, including
`go test -race` on the new/changed `grpcclient` tests specifically (no data races). api-gateway's
`wscompat` package could not be fully `go build`-verified in place — pre-existing, unrelated
breakage already in this worktree before this pass (a parallel agent's in-progress
`TASK-FT-003-01..05`/channel-registration refactor references `registerInfraFleetChannels`/
`registerStarNagChannels`/`registerStarNagGitHubChannels`/`registerEphemeralVmChannels`, none of
which exist yet anywhere in the tree); confirmed via `git status` (those files were already
modified before this pass) and independently verified by building/testing this task's two changed
files in an isolated scratch copy of the package with the missing functions stubbed out — both
`go build` and the 5 relevant tests (`TestRegisterTaskActivityStreamChannel_*`,
`TestClassifySubject_*`) passed cleanly there.

---

## Context

**Component:** `task-service`'s `SimpleExecutor` (Engine 1 — direct `agent.execPrompt` one-shot
executor)

**Problem:** Today `SimpleExecutor.Execute` calls `Exec` unary and gets exactly one final
`{stdout, stderr, exitCode, timedOut}` response — no mid-run visibility for the frontend, even
for a run approaching `MAX_TIMEOUT_MS = 15 * 60_000`.

## Implementation Sketch (from SOL-AG-FLOWTASK-001 §2.3 — not final)

`SimpleExecutor.Execute` calls `StreamExecOutput` in parallel with `Exec` (mirroring how
`AttachPty`/`WaitTerminalSession` already consume `StreamPty`), then:

- **Does not** publish every chunk as its own outbox event — CR-FLOW-TASK-003 explicitly scoped
  itself to discrete events, not a byte stream; one-event-per-chunk would flood
  `orchestration.messages`/outbox.
- **Should** batch using one of (decide at implementation time, not here):
  - (a) time-based throttle (e.g. every 2s, publish one `orca.task.agent_output_partial` event
    carrying the buffer accumulated so far), or
  - (b) milestone-based (e.g. an OSC 133 boundary) — **not viable for this path**: `--print` mode
    does not run inside a PTY, so no OSC sequences are emitted (unlike `agent.spawn`'s interactive
    PTY path). Option (a) is the only one that's actually usable here without further design work.
- The final payload lands on `task.activity:{taskId}` (CR-FLOW-TASK-003 §3's `TaskActivityFrame`)
  with `Engine: "direct_agent"`, `EventType: "agent_output_partial"` — reusing the already-designed
  frame, not a new WS channel.

## Open Questions To Resolve Before Coding (SOL-AG-FLOWTASK-001 §5)

1. **Throttle policy** — fixed interval vs. milestone-based. Milestone-based is not viable per
   above; fixed-interval needs real data on typical `agent.execPrompt` output length/duration
   before picking an interval.
2. **Scope limitation to acknowledge in the task, not silently ignore:** this design only applies
   to Task runs pinned to model `claude` via Engine 1 (`agent-print-mode-exec.ts:82-94`) — tasks
   using `gemini`/`codex`/`opencode` through `agent.execPrompt` already fail `InvalidParams`
   independent of this work.
3. Confirm `execution_links`'s `status_mirror` (CR-FLOW-TASK-001) has a field shape that can
   accept partial/in-progress state, not only terminal state, before wiring this in.

## Tests To Add

- `SimpleExecutor.Execute` publishes throttled `agent_output_partial` events to `task.activity`
  while a run is in progress, and the existing terminal event (completion/failure) is unaffected.
- No partial event is published faster than the chosen throttle interval, even under a fast/dense
  `agent.execOutput` stream from TASK-AG-FLOWTASK-002.
- A `gemini`/`codex`/`opencode`-pinned task's pre-existing `InvalidParams` failure path is
  unaffected by this change.

## Verification

```bash
cd backend-go && go build ./services/task-service/...
cd backend-go && go test ./services/task-service/...
```

---

## Acceptance Criteria

- [ ] `SimpleExecutor.Execute` consumes `StreamExecOutput` in parallel with `Exec`, without
      changing `Exec`'s own unary completion contract.
- [ ] Chunks are batched (never 1 outbox event per raw chunk) per the chosen throttle policy,
      documented explicitly in code.
- [ ] Final `task.activity:{taskId}` payload matches CR-FLOW-TASK-003's `TaskActivityFrame` shape
      exactly (`Engine: "direct_agent"`, `EventType: "agent_output_partial"`) — no new channel.
- [ ] Verified only after CR-FLOW-TASK-001 and CR-FLOW-TASK-003 are actually deployed, not against
      a stub.
