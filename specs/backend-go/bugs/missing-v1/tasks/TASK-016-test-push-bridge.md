# TASK-016: Tests for the push bridge

**From Solution:** SOL-035
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/push_bridge_test.go` (new), `handler_test.go`
**Depends on:** TASK-012, TASK-013, TASK-014, TASK-015
**Status:** `[x]` DONE — `TestPipePush_*` (order/cancel/close), `TestHandleSubscribe_AcksThenStreams`, `TestHandleSubscribe_InterleavesWithConcurrentInvoke`, and `TestNotificationsSubscribe_DeliversPushFrame` all exist and pass under `-race`.

---

## Tests to add

### `push_bridge_test.go`

- `TestPipePush_ForwardsEventsInOrder` — feed N events into a fake
  channel, assert N `push` frames written to a fake conn, in order.
- `TestPipePush_ReturnsOnContextCancel` — cancel `ctx` mid-stream, assert
  the goroutine returns promptly (bounded time, not a leak).
- `TestPipePush_ReturnsOnChannelClose` — close the events channel, assert
  the goroutine returns.

### `handler_test.go`

- `TestHandleSubscribe_AcksThenStreams` — a fake `StreamHandler` returning
  a channel with 2 events; assert the connection receives: 1
  `ResultMessage` (the subscribe ack) then 2 `PushMessage` frames, in
  order.
- `TestHandleSubscribe_InterleavesWithConcurrentInvoke` — one subscribe +
  one concurrent ordinary `invoke` on the same connection; assert no
  interleaved/corrupted frames (regression guard on `writeMu` sharing
  between `handleInvoke`/`handleSubscribe`/`pipePush`).

### Integration test (notification stream)

- `TestNotificationsSubscribe_DeliversPushFrame` — using a fake
  `wsbridge.StreamOpener` (mirrors `wsbridge/handler_test.go`'s existing
  pattern) that emits one item, assert `notifications.subscribe` over
  `/ws` delivers a `push` frame with `channel:"notifications.event"`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestPipePush|TestHandleSubscribe|TestNotificationsSubscribe" -v -race
go build ./... && go vet ./...
```
