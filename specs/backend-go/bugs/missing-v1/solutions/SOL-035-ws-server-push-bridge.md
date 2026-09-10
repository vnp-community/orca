# SOL-035: Bridge gRPC server-streaming into `push` frames, per `08-inter-service-communication.md`'s already-specified design

**Resolves:** [BUG-035](../BUG-035-ws-server-push-not-implemented.md)
**Service:** `api-gateway` (bridge logic) — `infra-fleet-service`/`notification-service` supply the streams
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/push_bridge.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (subscribe-channel registration)
**Status:** ✅ Implemented — all 5 task(s) (TASK-012–016) DONE; see each task file's own Status/Verify section for evidence.

---

## The target design is already written down

`specs/backend-go/tdd/architecture/08-inter-service-communication.md`'s
"API Gateway responsibilities" list, item 5, is this bug's fix, verbatim:

> Manages WebSocket sessions for real-time surfaces (terminal streams,
> agent status, notifications) — accepts a WS connection, opens a
> corresponding gRPC server-streaming call to the owning service
> (`infra-fleet-service` for terminal, `notification-service` for
> push/WS events), pipes frames both directions. ... stateless-by-design:
> any `api-gateway` replica can handle any connection, session affinity
> isn't required because there's no per-user forked process to route back
> to — state that mattered lives in the owning service, not in the
> gateway.

This is the missing half of `wscompat.Handler`. Today `ServeHTTP` only
pipes client→server (`invoke`/`send`); this solution adds the
server→client half by opening a gRPC server-streaming call per subscribed
channel and forwarding each received message as a `push` frame — exactly
the "pipe frames both directions" design, and exactly why the design is
"stateless-by-design": no new connection registry is needed (which
BUG-035's own report proposed as one option) — each `push`-capable channel
just holds its own goroutine + gRPC stream for the lifetime of that one WS
connection, scoped to `ServeHTTP`'s existing per-connection goroutine
group, not a cross-connection registry.

---

## Design — protocol

Add a `subscribe`/`unsubscribe` client message type (the frontend's
`WebSocketRpcClient` already knows how to receive `push` frames per
`docs/execution-plan.md` §0's wire-protocol description; it needs a way to
tell the server which channels to start streaming — infer this from
`notifications.subscribe`/`runtime.clientEvents.subscribe`'s existing
`callRuntimeRpc` shape, treated as `invoke` requests that, instead of
returning once, register a stream):

```go
// envelope.go — no new wire message type needed. "subscribe"/"unsubscribe"
// stay ordinary "invoke" channels (notifications.subscribe,
// runtime.clientEvents.subscribe) — their handler's job is to START a
// stream, not return a value. This keeps the existing 3-message-type
// protocol (invoke/send/push) intact per docs/execution-plan.md §0,
// rather than adding a 4th wire-level message type.
```

```go
// push_bridge.go
// StreamHandler opens a gRPC server-streaming call for one subscription
// and forwards each received item as a push frame. Registered per
// streamable channel, same registry pattern channels.go already uses for
// invoke/send handlers — a parallel map, not a rewrite of Registry.
type StreamHandler func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error)

type PushEvent struct {
    Channel string
    Args    any
}

// pipePush reads from a subscription's event channel until ctx is
// cancelled (connection closed or explicit unsubscribe) or the channel
// closes, writing each event as a push frame — serialized through the
// SAME writeMu handleInvoke already uses (handler.go:99), so push frames
// never interleave-corrupt a concurrent invoke response.
func pipePush(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, events <-chan PushEvent) {
    for {
        select {
        case <-ctx.Done():
            return
        case ev, ok := <-events:
            if !ok {
                return
            }
            writeMu.Lock()
            writeCtx, cancel := context.WithTimeout(context.Background(), writeTimeout)
            _ = wsjson.Write(writeCtx, conn, PushMessage{Type: "push", Channel: ev.Channel, Args: ev.Args})
            cancel()
            writeMu.Unlock()
        }
    }
}
```

`ServeHTTP`'s dispatch switch grows a case for stream-registering
channels, keyed the same way `invoke` already is — a channel is either a
normal request/response `ChannelHandler` or a `StreamHandler`, resolved
from a separate map so existing channels are untouched:

```go
case "invoke":
    if sh, ok := h.Registry.StreamHandlers[msg.Channel]; ok {
        subCtx, cancel := context.WithCancel(ctx) // cancelled on conn close (deferred at ServeHTTP's top) or explicit "unsubscribe" invoke
        events, err := sh(subCtx, identity, msg.Args)
        if err != nil { /* write ErrorMessage as usual */ ; cancel(); break }
        go pipePush(subCtx, conn, &writeMu, events)
        // ack the subscribe call itself with an ordinary ResultMessage so
        // the frontend's subscribe() promise resolves — matches "in-memory
        // WS event fan-out" semantics the old backend's notifications.subscribe had.
        continue
    }
    go h.handleInvoke(ctx, conn, &writeMu, identity, msg)
```

---

## Design — where the events come from

Per the TDD, two owning services supply the actual gRPC streams:

- **`notification-service.StreamNotifications`** — already exists and
  already has a working precedent: `internal/adapter/wsbridge/handler.go`
  proves the exact "open server-streaming call, pipe to WS" pattern this
  solution generalizes. `registerNotificationStreamChannel` in
  `wscompat` should reuse `wsbridge`'s `StreamOpener` type rather than
  reimplementing gRPC-stream-to-channel plumbing twice.
- **`infra-fleet-service`** — per the TDD, owns terminal streams too, but
  per BUG-029's own findings, `infrafleet.proto` has no PTY-shaped
  streaming RPC yet (only a generic `Relay` passthrough) — so terminal
  push-piping is blocked on BUG-029's solution (SOL-029) landing a real
  streaming RPC first, not on this bug. Scope this solution to
  `notification-service`-backed channels only; extend the same
  `StreamHandler` pattern to terminal once SOL-029 exists.

`runtime.clientEvents.subscribe`'s "in-memory event bus" (no cross-replica
requirement per the old backend's description) doesn't need a gRPC service
at all — it can be a local `chan PushEvent` fan-out inside `api-gateway`
itself, registered the same way, satisfying the "stateless-by-design...
state that mattered lives in the owning service" principle by simply not
mattering across replicas for this one channel (consistent with
`08-inter-service-communication.md`'s own carve-out for gateway-local,
non-durable state).

---

## Test plan

- `push_bridge_test.go` — `pipePush` forwards N events from a fake channel
  as N `push` frames, in order, and returns promptly when the channel
  closes or `ctx` is cancelled.
- `handler_test.go` — a subscribe-shaped `invoke` (fake `StreamHandler`)
  gets an immediate `ResultMessage` ack AND subsequent `push` frames from
  the same connection, interleaved correctly with a concurrent ordinary
  `invoke` (regression guard on `writeMu` sharing).
- Integration test against the real `notification-service` client (mirrors
  `wsbridge/handler_test.go`'s existing fake-`StreamOpener` pattern) —
  confirms `notifications.subscribe` over `/ws` delivers a push frame when
  `notification-service` emits one.

## References

- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — "API Gateway responsibilities" item 5 (the target design, verbatim)
- `backend-go/services/api-gateway/internal/adapter/wsbridge/handler.go` — existing, working precedent for the exact gRPC-stream→WS pattern
- `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go` — where the new stream-registering case is added
- [BUG-035](../BUG-035-ws-server-push-not-implemented.md), [BUG-029](../BUG-029-terminal-channels-not-implemented.md) — terminal streaming is blocked on infra-fleet-service getting a real streaming RPC first (see SOL-029)
