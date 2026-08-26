# TASK-187: Tests for terminal usecases, `devserveragent` PTY adapter, and `wscompat` channels

**From Solution:** SOL-029 (design part 5: tests)
**Priority:** P1
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `internal/usecase/spawn_terminal_session_test.go`, `attach_pty_test.go`, `wait_terminal_session_test.go` (all new, `infra-fleet-service`); `internal/adapter/devserveragent/methods_test.go` (new); `services/api-gateway/internal/adapter/wscompat/channels_terminal_test.go` (new)
**Depends on:** TASK-180, TASK-181, TASK-182, TASK-183, TASK-184, TASK-185, TASK-186
**Status:** `[x]` DONE for the `wscompat` scope (this pass) — `channels_terminal_test.go` now proves the push-bridge wiring genuinely works, not just that it builds: `TestTerminalCreateChannel_ForwardsOutputAndExitPushEvents` drives a fake `AttachPty` stream through `terminal.create`'s returned `<-chan PushEvent` and asserts a real `terminal.output` event for injected `PtyServerFrame_Out` data, a `terminal.exited` event carrying the real exit code for `PtyServerFrame_Exited`, and that the channel closes afterward; `TestTerminalCreateChannel_StreamErrorClosesEventsWithoutHanging` covers the transport-error close path; `TestTerminalCreateChannel_EndToEndPushInterleavesWithConcurrentSend` drives the REAL `Handler.ServeHTTP`/`pipePush`/`handleInvoke` path over an actual `websocket.Conn` pair (same harness `channels_push_test.go`'s `notifications.subscribe` integration test uses) and proves a concurrent `terminal.send` invoke and a `terminal.output`/`terminal.exited` push both arrive intact on the same connection, sharing `writeMu` without corruption. `TestTerminalStreamRegistry_IsSharedAcrossAllConnections` (the old characterization test asserting the FORMER buggy shared-registry, last-writer-wins behavior) is replaced by `TestTerminalStreamRegistry_IsolatesConnectionsWithTheSamePtyID`, which now asserts the FIXED, correct behavior: two simulated connections (two separate `terminalStreamsContext` values, mirroring `Handler.ServeHTTP` constructing one per real WS connection) that create/attach the SAME `ptyId` never cross-talk — `terminal.send`/`terminal.close` on one connection's context only ever reaches that connection's own `AttachPty` stream. `TestTerminalSendChannel_ConcurrentSendsAreSerializedBySendMu` (real `sendMu` race coverage across 32 concurrent `terminal.send` invokes) is retained unchanged from the prior pass. Unknown-`ptyId` send and close-then-send error cases, plus every other terminal.* channel's existing coverage, are retained and updated to the new per-connection-context calling convention. All new and pre-existing tests pass under `-race`.
>
> `infra-fleet-service` usecase layer and `devserveragent` adapter tests (`spawn_terminal_session_test.go`/`attach_pty_test.go`/`wait_terminal_session_test.go`/`methods_test.go`) were NOT touched or re-verified in this pass — out of this pass's file scope (limited to `api-gateway`'s `wscompat` package per this pass's assignment). Carry forward the prior pass's spot-check note: these existing tests already cover `ConnectionStreamLimiter` rejection, ctx-cancellation stream teardown, and exit/timeout/ignored-output-event `Wait` cases; the orphaned-PTY `KillPty`-rollback-on-`TerminalSessionRepository.Create`-failure case is still NOT covered (a real implementation gap in `spawn_terminal_session.go`, not just a missing test) and remains open. `go build ./... && go vet ./... && go test ./... -race` for `api-gateway` (verify command below) passes; the `infra-fleet-service` verify command was not re-run in this pass.

---

## Context

Covers the regression-critical paths this solution's design calls out
explicitly: the orphaned-PTY-on-bookkeeping-failure rollback, the
`MAX_CONCURRENT_STREAMS` cap, `WaitTerminalSession`'s bounded-timeout
guarantee, Stack A/B method-name resolution, and `terminal.send`'s
interleaving with concurrent push frames on the same `writeMu`.

## Changes to make

### `services/infra-fleet-service/internal/usecase/spawn_terminal_session_test.go` (new)

Fake `ConnectionResolver`/`TerminalSessionRepository`/`DevServerAgentClient`.
Cases:

- Happy path: `Execute` returns a `domain.TerminalSession` with the agent's
  returned `ptyId`, and `TerminalSessionRepository.Create` was called
  exactly once with matching fields.
- `TerminalSessionRepository.Create` fails → assert
  `DevServerAgentClient.KillPty` was called with the SAME `ptyId`
  `SpawnPty` returned (regression guard against orphaned agent-side PTYs),
  and the returned error is `TERMINAL_BOOKKEEPING_FAILED`.
- `connectionId == ""` with `serverDeployment: true` → returns
  `TERMINAL_NO_LOCAL_SHELL` without calling `ConnectionResolver` or
  `DevServerAgentClient` at all.

### `services/infra-fleet-service/internal/usecase/attach_pty_test.go` (new)

Fake `TerminalSessionRepository.Get` (returns a fixed session),
`ConnectionResolver.ResolveConnection` (returns `connected: true`), and
`DevServerAgentClient` with a scriptable `StreamPty` channel and recording
`WritePty`/`ResizePty` calls. Cases:

- An `Input` client frame results in exactly one `WritePty` call with the
  frame's bytes, and `TerminalSessionRepository.Touch` is called.
- A `Resize` client frame results in exactly one `ResizePty` call.
- An `Exited` event from `StreamPty`'s channel: `Execute` sends exactly one
  `PtyServerFrame{Exited: ...}` on `serverFrames`, calls
  `TerminalSessionRepository.MarkClosed`, and returns `nil` (loop ends).
- `ConnectionStreamLimiter`: acquire 16 concurrent `Execute` calls for the
  same `connectionId` (blocking each on an unbuffered `clientFrames`
  channel so they stay "in flight"), then start a 17th — assert it returns
  `TERMINAL_TOO_MANY_STREAMS` immediately, without blocking.
- `ctx` cancellation mid-loop: `Execute` returns `ctx.Err()` promptly (no
  goroutine leak — verify via a `WritePty`/`StreamPty` fake that closes its
  channel on ctx.Done, or an explicit goroutine-count assertion if this
  package already has a helper for that).

### `services/infra-fleet-service/internal/usecase/wait_terminal_session_test.go` (new)

Fake `DevServerAgentClient.StreamPty` returning a channel the test
controls. Cases:

- Sending an `Exited` event on the fake channel before the timeout →
  `Execute` returns `{Exited: true, ExitCode: N, TimedOut: false}`
  promptly (assert via a wall-clock bound well under the timeout, e.g.
  `< 500ms` for a 5s test timeout).
- No event sent, timeout set to e.g. `50 * time.Millisecond` → `Execute`
  returns `{TimedOut: true}` at approximately that bound, never blocks
  past `maxWaitTimeout`.
- `timeout` argument `0` or negative → `Execute` uses `maxWaitTimeout` (30s)
  as the effective cap — assert by passing a fake clock or by checking the
  `context.WithTimeout` deadline `Execute` derives internally is
  `~maxWaitTimeout` (may require exposing a testable seam — a `now func()
  time.Time` field, matching this package's existing `clock` convention
  elsewhere, if `wait_terminal_session.go` doesn't already expose the
  deadline for inspection).

### `services/infra-fleet-service/internal/adapter/devserveragent/methods_test.go` (new)

This package's existing tests (`client_test.go`/`session_test.go`, if
present — check before assuming a fake-transport pattern) likely already
have a fake `Transport`/fake agent responder harness — reuse it. Cases:

- `ptyMethodName` resolves `"spawn"` to the expected method name (per
  TASK-183's `TODO(confirm-against-agent)` — write this test to assert
  against whatever name TASK-183 actually shipped, since that task flags
  the real Stack A/B names as unconfirmed; update this test alongside
  resolving that flag, not before).
- `StreamPty` demuxes notifications by `ptyId`: send two `pty.output`
  notifications with different `ptyId`s over one fake session, subscribe to
  only one — assert only the matching one is delivered, the other is
  silently dropped (not delivered to the wrong subscriber, not causing an
  error).
- `SpawnPty` with a response missing `ptyId` returns a clear error, not a
  silently empty string.
- `AgentStatus`/`InspectProcess` against a fake `Exec` that returns a
  JSON-RPC "method not found" (code `-32601`) error → both return
  `(false, ..., nil error)` — the honest-degrade path, not a propagated
  gRPC failure.

### `services/api-gateway/internal/adapter/wscompat/channels_terminal_test.go` (new)

Fake `infrafleetv1.InfraFleetServiceClient` with a scriptable
`AttachPty` bidi-stream fake (mirrors whatever fake-gRPC-stream pattern
`channels_test.go` or `handler_test.go` already uses in this package for
another streaming case — if none exists yet, implement a minimal
`infrafleetv1.InfraFleetService_AttachPtyClient` fake backed by Go channels).
Cases:

- `terminal.create` against the fake: acks with a `TerminalSession`
  containing a `ptyId`, AND the returned `<-chan PushEvent` subsequently
  delivers a `terminal.output` push event when the fake `AttachPty` stream
  sends a `PtyOutput` frame.
- Interleaving regression: while `terminal.create`'s push stream is
  actively delivering `terminal.output` events, a concurrent
  `terminal.send` invoke on the SAME (fake) WS connection completes without
  corrupting the push frame's write — assert both writes go through the
  same `writeMu` (construct the test via `Handler.ServeHTTP` end-to-end
  over a real `websocket.Conn` pair if `handler_test.go` already has that
  harness, rather than re-testing `pipePush`'s internals here — that's
  TASK-016's job).
- `terminal.send` against a `ptyId` with no registry entry (e.g. a stale
  pane after reconnect) returns a clear error, not a panic.
- `terminal.close` removes the registry entry — a subsequent `terminal.send`
  for the same `ptyId` on the same connection then also errors cleanly.
- Two different simulated WS connections (two separate
  `terminalStreamsContext` values) each create a terminal with the SAME
  `ptyId` (unlikely in practice but a real adversarial case for a
  correctness bug): assert `terminal.send` on connection A never reaches
  connection B's stream — the regression guard for TASK-186 Step 5's
  per-connection-registry fix (a shared-registry bug would leak across
  connections here).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go test ./internal/usecase/... ./internal/adapter/devserveragent/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v
```
