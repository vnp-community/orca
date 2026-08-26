# TASK-013: Route subscribe-shaped `invoke` messages to `StreamHandler`s

**From Solution:** SOL-035
**Priority:** P0
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/handler.go`, `registry.go`
**Depends on:** TASK-012
**Status:** `[ ]` TODO

---

## Changes to make

### `registry.go` — add the parallel stream-handler map

```go
type Registry struct {
    handlers       map[string]ChannelHandler
    streamHandlers map[string]StreamHandler // NEW
}

func NewRegistry() *Registry {
    return &Registry{
        handlers:       make(map[string]ChannelHandler),
        streamHandlers: make(map[string]StreamHandler),
    }
}

// RegisterStream adds a StreamHandler for a push-capable channel (e.g.
// notifications.subscribe). Distinct from Register — a channel is either
// request/response or stream-registering, never both.
func (r *Registry) RegisterStream(channel string, h StreamHandler) {
    r.streamHandlers[channel] = h
}

func (r *Registry) StreamHandler(channel string) (StreamHandler, bool) {
    h, ok := r.streamHandlers[channel]
    return h, ok
}
```

### `handler.go` — `ServeHTTP`'s dispatch switch

```go
case "invoke":
    if sh, ok := h.Registry.StreamHandler(msg.Channel); ok {
        go h.handleSubscribe(ctx, conn, &writeMu, identity, msg, sh)
        continue
    }
    go h.handleInvoke(ctx, conn, &writeMu, identity, msg)
```

### New method `handleSubscribe`

```go
// handleSubscribe opens a StreamHandler's subscription, acks the
// subscribe call itself with an ordinary ResultMessage (so the frontend's
// subscribe() promise resolves), then pipes events until the connection
// closes. subCtx is tied to ctx (the connection's lifetime, from
// ServeHTTP), NOT invokeTimeout — a subscription is meant to live for the
// whole connection, unlike a normal request/response invoke.
func (h *Handler) handleSubscribe(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, identity Identity, msg InboundMessage, sh StreamHandler) {
    events, err := sh(ctx, identity, msg.Args)
    writeCtx, cancel := context.WithTimeout(context.Background(), writeTimeout)
    defer cancel()
    writeMu.Lock()
    if err != nil {
        _ = wsjson.Write(writeCtx, conn, ErrorMessage{Type: "error", ID: msg.ID, Message: err.Error()})
        writeMu.Unlock()
        return
    }
    _ = wsjson.Write(writeCtx, conn, ResultMessage{Type: "result", ID: msg.ID, Result: nil})
    writeMu.Unlock()

    pipePush(ctx, conn, writeMu, events)
}
```

Note: `ctx` here is `ServeHTTP`'s connection-lifetime context, already
available in scope — no new cancellation wiring needed; when the
connection closes, `ServeHTTP` returning cancels `ctx` (it derives from
`r.Context()`), which stops `pipePush`'s loop.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/... && go vet ./internal/adapter/wscompat/...
```
