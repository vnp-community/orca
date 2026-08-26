# TASK-014: Register `notifications.subscribe` as a `StreamHandler`

**From Solution:** SOL-035
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-012, TASK-013
**Status:** `[x]` DONE — `registerNotificationStreamChannel` is registered via `RegisterPushChannels` (wired from `main.go` with the same `notificationStreamOpener` instance passed to `wsbridge.New`), and tests cover the happy-path push-frame delivery, opener error propagation, a non-EOF `stream.Recv()` error closing the output channel cleanly, and `ctx` cancellation closing it without leaking the forwarding goroutine.

---

## Context

Reuse `internal/adapter/wsbridge`'s existing, working
`StreamOpener`/gRPC-server-streaming pattern (`notification-service.StreamNotifications`)
rather than reimplementing gRPC-stream-to-channel plumbing a second time.

## Changes to make

```go
// ── notifications.* (stream) ────────────────────────────────────────────
//
// Reuses wsbridge.StreamOpener's exact "open a server-streaming gRPC call,
// forward each item" shape — see internal/adapter/wsbridge/handler.go for
// the proven precedent this generalizes.
func registerNotificationStreamChannel(r *Registry, opener wsbridge.StreamOpener) {
    r.RegisterStream("notifications.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
        stream, err := opener(ctx, id.UserID)
        if err != nil {
            return nil, err
        }
        out := make(chan PushEvent)
        go func() {
            defer close(out)
            for {
                item, err := stream.Recv()
                if err != nil {
                    return // stream closed or ctx cancelled — pipePush's caller sees the closed channel and returns too
                }
                select {
                case out <- PushEvent{Channel: "notifications.event", Args: item}:
                case <-ctx.Done():
                    return
                }
            }
        }()
        return out, nil
    })
}
```

Wire into `RegisterRealChannels`:

```go
func RegisterRealChannels(
    r *Registry,
    // ... existing params ...
    notificationStreamOpener wsbridge.StreamOpener, // NEW
) {
    // ... existing registrations ...
    registerNotificationStreamChannel(r, notificationStreamOpener)
}
```

Update `cmd/server/main.go`'s call site to pass the same `StreamOpener`
instance `wsbridge.New(...)` already constructs for `/v1/notifications/stream`
— don't build a second one.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
