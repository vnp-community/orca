# BUG-001: Error response written on cancelled context causes silent drops

**Service:** `api-gateway`  
**File:** `internal/adapter/wscompat/handler.go`  
**Severity:** High — root cause of ALL "Request timed out: \<channel\>" frontend errors  
**Symptom:** Frontend receives "Request timed out: `\<channel\>`" instead of the actual error  
**Status:** ✅ Fixed (2026-08-24) — [SOL-001](./solutions/SOL-001-fix-cancelled-context-write.md), TASK-001 + TASK-002

---

## Description

`handleInvoke` wraps each channel dispatch in a 25-second `context.WithTimeout`. When
the dispatch context expires (downstream gRPC timeout) OR when a channel handler returns
an error quickly (e.g. `notImplementedHandler`), the error is written back to the
WebSocket connection **using the already-cancelled/expired `ctx`**:

```go
func (h *Handler) handleInvoke(...) {
    ctx, cancel := context.WithTimeout(ctx, invokeTimeout) // shared for dispatch AND write
    defer cancel()

    result, err := h.Registry.Dispatch(ctx, ...)

    writeMu.Lock()
    defer writeMu.Unlock()
    if err != nil {
        // BUG: if ctx expired during Dispatch, wsjson.Write sees ctx.Done()
        // and returns ctx.Err() immediately — the ErrorMessage is NEVER sent.
        _ = wsjson.Write(ctx, conn, ErrorMessage{...})
        return
    }
    _ = wsjson.Write(ctx, conn, ResultMessage{...})
}
```

When `wsjson.Write` is called with a cancelled context, it returns `ctx.Err()` without
writing any bytes. The frontend's pending `invoke` call is left with no response until
its own 30s `INVOKE_TIMEOUT_MS` fires, producing:

```
Uncaught (in promise) Error: Request timed out: <channel>
```

---

## Affected Channels (from browser console errors)

| Channel | Situation | Why ctx is cancelled at write time |
|---------|-----------|-----------------------------------|
| `crashReports.getLatestPending` | Unregistered → `notImplementedHandler` returns error instantly | Parent HTTP ctx may be cancelled if the WS connection itself is unstable; OR context timeout hits 25s while other parallel invokes are running |
| `devServer.list` | Registered; calls `infra-fleet-service.ListDevServers` via gRPC | gRPC call hangs/times out → ctx hits 25s deadline → cancelled before write |
| `rateLimits.get` | Unregistered → `notImplementedHandler` returns error instantly | Same as `crashReports.getLatestPending` |
| `preflight.check` | Registered; local handler, zero downstream calls | ctx may be cancelled by parent WS connection ctx (e.g. HTTP/2 stream reset) before goroutine writes |

> **Note on `preflight.check`:** It makes no gRPC call. If it also times out, the
> parent `r.Context()` (the HTTP connection context) is likely being cancelled due to
> proxy/nginx timeout before the goroutine completes its write. The fundamental problem
> is the same: write context must be independent of the dispatch context.

---

## Proposed Fix

Use a **fresh context** for the write-back, independent of the dispatch context:

```go
func (h *Handler) handleInvoke(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, identity Identity, msg InboundMessage) {
    dispatchCtx, dispatchCancel := context.WithTimeout(ctx, invokeTimeout)
    defer dispatchCancel()

    result, err := h.Registry.Dispatch(dispatchCtx, identity, msg.Channel, msg.Args)

    writeMu.Lock()
    defer writeMu.Unlock()

    // Use a fresh context for the write so a timed-out dispatch context
    // does not silently drop the error or result frame.
    writeCtx, writeCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer writeCancel()

    if err != nil {
        _ = wsjson.Write(writeCtx, conn, ErrorMessage{Type: "error", ID: msg.ID, Message: err.Error()})
        return
    }
    _ = wsjson.Write(writeCtx, conn, ResultMessage{Type: "result", ID: msg.ID, Result: result})
}
```

---

## References

- `services/api-gateway/internal/adapter/wscompat/handler.go` — `handleInvoke`
- `services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `services/api-gateway/internal/adapter/wscompat/handler.go` line 125 — `invokeTimeout = 25 * time.Second`
