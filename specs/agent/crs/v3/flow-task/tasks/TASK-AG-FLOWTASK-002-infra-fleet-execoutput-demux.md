# TASK-AG-FLOWTASK-002: Demux `agent.execOutput` notifications in `infra-fleet-service`

**Task ID:** TASK-AG-FLOWTASK-002
**Priority:** 🔵 P3 (Phase D of CR-FLOW-TASK-003 — design-only, not scheduled)
**Solution Ref:** [SOL-AG-FLOWTASK-001](../solutions/SOL-AG-FLOWTASK-001-execution-activity-streaming-design.md) §2.2, §3
**Depends on:** TASK-AG-FLOWTASK-001 (agent/ must actually emit `agent.execOutput` first —
otherwise this has nothing to route)
**Status:** [x] DONE — implemented this pass.

`session.go`: added `execOutputMu`/`execOutputSubs map[string][]chan rawExecOutputNotification`
(keyed by stepId) alongside `ptySubs`/`screencastSubs`; `routeNotification` gained a third case
matching the REAL notification method name confirmed against `agent-print-mode-exec.ts` —
**`agent.execPrompt.output`** (exact match, not the `strings.HasPrefix(n.Method,
"agent.execOutput")` this task's sketch used — that guess doesn't match the real method name, and
a broad prefix would also have wrongly captured the unrelated `shell.exec.output` family, keyed by
`traceId` for a different, not-yet-wired workflow-service consumer — Open Question 3 resolved:
no collision, because this demux is an exact-method match). `subscribeExecOutput`/
`unsubscribeExecOutput` mirror `subscribePty`/`subscribeScreencast` exactly, including the 64-slot
channel buffer (Open Question 1: kept the same as the two precedents, no real traffic data yet to
tune against).

`client.go`: added `Client.StreamExecOutput`, gating `relay-ssh` unconditionally like `StreamPty`/
`StreamScreencast` — Open Question 2 resolved per this task's own explicit default stance
("inherit the block"); the `StreamPty` doc-comment contradiction SOL-AG-FLOWTASK-001 §3 flagged
remains **unresolved**, still needs someone with TASK-192 context.

`ports.go`: added `ExecOutputEvent` + `StreamExecOutput` to `DevServerAgentClient`.

Beyond this task's own listed Files (needed to make TASK-AG-FLOWTASK-003 actually reachable
cross-service, since `Client.StreamExecOutput` above is in-process only): added
`usecase.StreamAgentExecOutput` (resolve-then-subscribe, mirrors `usecase.Relay`'s shape) and a new
gRPC server-streaming RPC `StreamExecOutput` (`infrafleet.proto` + `grpc/server.go`, mirroring
`AttachPty`'s tenant-extraction-workaround pattern) so `task-service` can consume this stream.

Tests added: `session_exec_output_test.go` (demux-by-stepId, stdout/stderr passthrough, unsubscribe
closes+stops routing, `routeNotification` dispatches the new branch without disturbing
`ptySubs`/`screencastSubs`, unrelated methods incl. `shell.exec.output` still fall through to the
silent default) + `stream_agent_exec_output_test.go`. Verified:
`go build`/`go vet`/`go test` clean for `internal/adapter/devserveragent` and `internal/usecase`
(the two packages this task's Files list names). `internal/adapter/grpc` and `cmd/server/main.go`
could NOT be fully `go build`-verified — pre-existing, unrelated breakage already present in this
worktree before this pass (a parallel agent's in-progress `TeardownConnection`/
`GetFleetConnectivitySummary`/ephemeral-VM work references usecases/domain fields that don't exist
yet); confirmed via `git status` (those files were already modified, mine are the only new/touched
ones actually needed here) and via `gofmt -l` (my additions are syntactically valid).

---

## Context

**Files:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`,
`.../client.go`

**Problem:** `session.go`'s `readLoop()` routes notification frames (method, no `id`) through
`routeNotification()` (dòng 319-328), which only recognizes two hardcoded method families:
`pty.data/pty.exit/pty.replay` (demuxed via `ptySubs`) and
`browser.screencastReady/Frame/Ended/Error` (demuxed via `screencastSubs`). Any other notification
method — including a future `agent.execOutput` — hits the silent `default` case and is dropped.
`Client.Exec` (`client.go:243`) also only awaits exactly one `resultCh` response per request
(buffer size 1, `session.go:630`) — no loop for consuming multiple frames tied to one request id.

## Implementation Sketch (from SOL-AG-FLOWTASK-001 §2.2 — not final)

```go
// session.go — add alongside ptySubs/screencastSubs (~dòng 77-95)
execOutputSubs map[string][]chan rawExecOutputNotification   // keyed by stepId

// routeNotification (dòng 319-328) — add a third branch
case strings.HasPrefix(n.Method, "agent.execOutput"):
    s.routeExecOutputNotification(n)
```

```go
// client.go — new method, same shape as StreamPty (dòng 323-333)
func (c *Client) StreamExecOutput(ctx context.Context, devServer domain.DevServer, stepID string) (<-chan usecase.ExecOutputEvent, func(), error) {
    // relay-ssh: see Open Question 2 below — decide whether to gate it the same way StreamPty does today
}
```

This reuses the exact pattern already proven twice (`ptySubs` for PTY, `screencastSubs` for
browser screencast) — a third demux keyed by `stepId` instead of `ptyID`/`worktreeID`, not a new
mechanism.

## Open Questions To Resolve Before Coding

1. **Channel buffer sizing / backpressure** for `execOutputSubs` — decide independently from
   `ptySubs`/`screencastSubs` since chunk frequency/size is not yet bounded (see
   TASK-AG-FLOWTASK-001's granularity question).
2. **`relay-ssh` scope** (SOL-AG-FLOWTASK-001 §3): `StreamPty`/`StreamBrowser` hard-block
   `relay-ssh` today (`client.go:324-326`, `369-371`) citing "no persistent session", but reading
   the package doc comment (dòng 21-29) and `getOrProvisionSession` (dòng 187-214) suggests
   relay-ssh **does** reuse a persistent `c.sessions[devServer.ID]` like the other two modes —
   an apparent contradiction not resolved by this design pass. Someone with TASK-192 context
   (the merge that removed `dialRelaySSH`/`relaySSHHealth`) must confirm the real reason before
   deciding whether `StreamExecOutput` inherits the same block or not. **Default stance until
   resolved: inherit the `relay-ssh` block, same as `StreamPty`** — do not unilaterally widen scope.
3. Confirm `routeExecOutputNotification`'s method-prefix match (`strings.HasPrefix(n.Method,
   "agent.execOutput")`) doesn't collide with any other `agent.*` notification family.

## Tests To Add

File: `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session_test.go`
(or equivalent)

- `routeNotification` dispatches an `agent.execOutput` frame to the correct `execOutputSubs[stepId]`
  channel(s) and to no others (`ptySubs`/`screencastSubs` untouched).
- Unknown/unrelated notification methods still fall through to the silent `default` (no
  regression to existing drop behavior for genuinely unhandled methods).
- `StreamExecOutput` on `relay-ssh` returns the same class of error `StreamPty` returns today,
  unless Open Question 2 above has been resolved and the decision documented in this task first.

## Verification

```bash
cd backend-go && go build ./services/infra-fleet-service/...
cd backend-go && go test ./services/infra-fleet-service/internal/adapter/devserveragent/...
```

---

## Acceptance Criteria

- [ ] `execOutputSubs` demux added following the `ptySubs`/`screencastSubs` pattern exactly.
- [ ] `routeNotification` extended with a third branch, existing two branches unaffected
      (verified by test, not just review).
- [ ] Open Question 2 (`relay-ssh` scope) is explicitly decided and documented, not left implicit
      via silent inheritance of unrelated code.
- [ ] No regression to `StreamPty`/`StreamBrowser` behavior.
