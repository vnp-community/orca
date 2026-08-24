# BUG-004: `preflight.check` timeout despite being a local no-downstream-call handler

**Service:** `api-gateway`  
**File:** `internal/adapter/wscompat/channels.go` + `handler.go`  
**Severity:** Medium — preflight is called at every bootstrap; its timeout blocks the entire app initialization  
**Symptom:** "Request timed out: preflight.check" in browser console  
**Status:** ✅ Fixed (2026-08-24) — [SOL-004](./solutions/SOL-004-preflight-writemu-starvation.md), TASK-010 (via SOL-001 + SOL-003)

---

## Description

`preflight.check` is a fully local handler — it makes no gRPC call and returns a static
map in microseconds:

```go
r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    return map[string]any{
        "git":  map[string]any{"installed": true},
        "gh":   map[string]any{"installed": false, "authenticated": false},
        "glab": map[string]any{"installed": false, "authenticated": false},
    }, nil
})
```

Yet the frontend reports it timing out. There are two possible causes:

### Cause A: Write-back on cancelled parent context (BUG-001 interaction)

`handleInvoke` spawns one goroutine per `invoke` message. Multiple channels are
invoked simultaneously during bootstrap. If the **parent HTTP request context**
(`r.Context()`) is cancelled (e.g. nginx upstream timeout, client navigation away,
connection drop) BEFORE the `preflight.check` goroutine calls `wsjson.Write`, the
write fails silently for the same reason as BUG-001.

The `invokeTimeout` is `context.WithTimeout(ctx, 25s)` where `ctx = r.Context()`. If
`r.Context()` is already cancelled, `dispatchCtx` is immediately cancelled too, and the
write never happens.

### Cause B: `writeMu` serialization starvation

All concurrent `handleInvoke` goroutines on the same connection share one `writeMu`
mutex. If a slow channel (e.g. `devServer.list` waiting 25s for gRPC) holds `writeMu`
while `preflight.check`'s goroutine is waiting for the lock, `preflight.check`'s own
`ctx` may expire before it can acquire the mutex and write its response.

Current lock-hold time for `devServer.list`: up to 25 seconds (full `invokeTimeout`).
During that 25s window, NO other goroutine can write a response — including
`preflight.check`.

---

## writeMu Hold-Time Problem (Cause B Detail)

```go
func (h *Handler) handleInvoke(...) {
    ctx, cancel := context.WithTimeout(ctx, invokeTimeout) // 25s
    defer cancel()

    result, err := h.Registry.Dispatch(ctx, ...)
    // ^^^ for devServer.list: blocks for up to 25s

    writeMu.Lock()   // acquired AFTER Dispatch returns — up to 25s later
    defer writeMu.Unlock()
    // ... write response
}
```

`writeMu` is held from `writeMu.Lock()` until the function returns. But the **critical
observation** is that `writeMu.Lock()` is called AFTER `Dispatch` — so the lock is only
held for the duration of `wsjson.Write` (milliseconds), not for the full dispatch time.
This means Cause B is less likely than initially appears.

However, if multiple slow channels all hit their 25s timeout simultaneously, they all
call `writeMu.Lock()` at nearly the same time. The last one to acquire the lock will
do so after ~25s + (N-1) × write_duration where N is the number of concurrent timeouts.

### Real Cause B scenario

With 4 concurrent invokes all hitting their 25s timeout:
1. All 4 goroutines call `writeMu.Lock()` at t≈25s
2. One acquires it immediately; the others queue
3. The context for queued goroutines has **already expired** (they're at t=25s,
   and their ctx timeout is exactly 25s)
4. When they finally acquire `writeMu`, `wsjson.Write(ctx, ...)` immediately fails

This is a confirmed amplification of BUG-001 in the concurrent case.

---

## Fix

Both causes are resolved by the BUG-001 fix (use a fresh write context). Additionally,
the `writeMu` lock should be acquired before the Dispatch result is needed — or a
channel-keyed lock per connection should replace the single mutex. For now, the BUG-001
fix is sufficient.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go` lines 482–490 — `preflight.check` handler
- `services/api-gateway/internal/adapter/wscompat/handler.go` lines 99, 127–140 — `writeMu`, `handleInvoke`
- BUG-001 — root cause of silent error drops
- BUG-003 — `devServer.list` slow timeout that contends for `writeMu`
