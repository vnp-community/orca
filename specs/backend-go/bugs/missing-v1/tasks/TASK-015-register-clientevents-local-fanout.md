# TASK-015: Register `runtime.clientEvents.subscribe` as a local in-process fan-out

**From Solution:** SOL-035
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-012, TASK-013
**Status:** `[ ]` TODO

---

## Context

Per the old backend's design (no cross-replica requirement — "in-memory
event bus"), this channel doesn't need a gRPC service at all. Consistent
with `08-inter-service-communication.md`'s carve-out for gateway-local,
non-durable state.

## Changes to make

```go
// clientEventBus is a tiny in-process pub/sub for gateway-local events —
// deliberately NOT cross-replica (matches the old backend's "in-memory WS
// event fan-out" description for this exact channel). Any api-gateway
// replica handling this connection sees only events published on that
// same replica — acceptable per 08-inter-service-communication.md's
// stateless-by-design principle, since this is UI-convenience signaling,
// not state that must be consistent cluster-wide.
type clientEventBus struct {
    mu   sync.Mutex
    subs map[chan PushEvent]struct{}
}

func newClientEventBus() *clientEventBus {
    return &clientEventBus{subs: make(map[chan PushEvent]struct{})}
}

func (b *clientEventBus) Subscribe() (<-chan PushEvent, func()) {
    ch := make(chan PushEvent, 16)
    b.mu.Lock()
    b.subs[ch] = struct{}{}
    b.mu.Unlock()
    unsubscribe := func() {
        b.mu.Lock()
        delete(b.subs, ch)
        close(ch)
        b.mu.Unlock()
    }
    return ch, unsubscribe
}

func (b *clientEventBus) Publish(ev PushEvent) {
    b.mu.Lock()
    defer b.mu.Unlock()
    for ch := range b.subs {
        select {
        case ch <- ev:
        default: // slow subscriber — drop rather than block Publish
        }
    }
}

func registerClientEventsChannel(r *Registry, bus *clientEventBus) {
    r.RegisterStream("runtime.clientEvents.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
        ch, unsubscribe := bus.Subscribe()
        go func() { <-ctx.Done(); unsubscribe() }()
        return ch, nil
    })
}
```

Construct one `clientEventBus` in `main.go`'s composition root, pass it
into `RegisterRealChannels`, and expose it (or a narrow `Publish`-only
interface) to whatever other code needs to publish client events —
identify those call sites as a follow-up if none exist yet; this task only
wires the subscribe side.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
