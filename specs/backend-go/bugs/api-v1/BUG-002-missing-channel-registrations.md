# BUG-002: Frontend channels not registered — `crashReports.getLatestPending` and `rateLimits.get`

**Service:** `api-gateway`  
**File:** `internal/adapter/wscompat/channels.go` + `cmd/server/main.go`  
**Severity:** Medium — frontend calls these on every bootstrap; they fail immediately with "not yet implemented" error (which is then silently dropped due to BUG-001)  
**Symptom:** Browser console shows "Request timed out: crashReports.getLatestPending" and "Request timed out: rateLimits.get"  
**Status:** ✅ Fixed (2026-08-24) — [SOL-002](./solutions/SOL-002-register-missing-channels.md), TASK-003 → TASK-007

---

## Description

The frontend calls at least these two channels during its bootstrap sequence that have
**no handler registered** in `wscompat.Registry`:

| Channel | Frontend call site (from stack trace) | Expected backend |
|---------|--------------------------------------|-----------------|
| `crashReports.getLatestPending` | `App-B8D_odUb.js:158` — called on initial app load | Should return null/empty if no crash report service exists |
| `rateLimits.get` | `App-B8D_odUb.js:3` — called after preflight | Should return current per-tenant rate limit info |

Both fall through to `registry.go`'s `notImplementedHandler`:

```go
func notImplementedHandler(_ context.Context, _ Identity, channel string) (any, error) {
    return nil, fmt.Errorf("channel %q is not yet implemented in backend-go — ...", channel)
}
```

This returns an error immediately. Due to BUG-001, that error is written back on a
cancelled context and never reaches the client. The client then waits 30s and times out.

---

## Affected Bootstrap Sequence

From the stack trace, the call order is:

1. App boots → calls `preflight.check` ✅ (registered but may hit BUG-001)
2. App calls `crashReports.getLatestPending` ❌ (not registered)
3. App calls `devServer.list` ⚠️ (registered, but may hit gRPC timeout)
4. App calls `rateLimits.get` ❌ (not registered)

All three timeout errors appear before the app fully initializes, suggesting the UI is
blocked waiting for these responses.

---

## Required Channels to Add

### `crashReports.getLatestPending`

Returns the most recent pending crash report for the current user/session. Since
`backend-go` has no crash reporting service, the honest response is `null` (no pending
crash report). This unblocks the frontend without pretending to implement something that
doesn't exist.

```go
r.Register("crashReports.getLatestPending", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    // backend-go has no crash reporting service.
    // Return null — no pending crash report.
    return nil, nil
})
```

### `rateLimits.get`

Returns the current rate limit configuration for the authenticated tenant. The
`api-gateway` already has a real `usecase.RateLimiter` — the channel should expose its
configured limits.

This requires threading `*usecase.RateLimiter` into `RegisterRealChannels` or adding a
dedicated registration function.

```go
r.Register("rateLimits.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    // Return the configured rate limit values.
    // Actual RPS/burst values come from cfg.RateLimitRPS / cfg.RateLimitBurst.
    return map[string]any{
        "requestsPerSecond": rateLimiter.RPS(),
        "burst":             rateLimiter.Burst(),
    }, nil
})
```

> **Note:** `usecase.RateLimiter` currently exposes no `RPS()`/`Burst()` accessors —
> two read-only getters need to be added to `internal/usecase/rate_limit.go`.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go` — `RegisterRealChannels`
- `services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `services/api-gateway/internal/usecase/rate_limit.go` — `RateLimiter`
- `services/api-gateway/cmd/server/main.go` line 241 — `wscompat.RegisterRealChannels(...)`
