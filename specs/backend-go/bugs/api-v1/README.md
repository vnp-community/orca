# API v1 Bug Reports

This directory contains bugs identified from the frontend "Request timed out" errors
observed in the browser console for the deployed dev environment.

> **Resolution Status: ✅ ALL FIXED** — BUG-001–004 implemented 2026-08-24, BUG-005 implemented 2026-08-26. See [`solutions/`](./solutions/) and [`tasks/`](./tasks/).

## Bug Index

| ID | Title | Severity | Status |
|----|-------|----------|--------|
| [BUG-001](./BUG-001-cancelled-context-error-write.md) | Error response written on cancelled context — root cause of all timeout errors | High | ✅ Fixed (SOL-001) |
| [BUG-002](./BUG-002-missing-channel-registrations.md) | Missing `crashReports.getLatestPending` and `rateLimits.get` channel registrations | Medium | ✅ Fixed (SOL-002) |
| [BUG-003](./BUG-003-devserver-list-grpc-timeout.md) | `devServer.list` hangs for full 25s when infra-fleet-service is unreachable | Medium | ✅ Fixed (SOL-003) |
| [BUG-004](./BUG-004-preflight-check-timeout.md) | `preflight.check` timeout despite being a local no-downstream handler | Medium | ✅ Fixed (SOL-004) |
| [BUG-005](./BUG-005-websessionclient-dialect-dropped.md) | `WebSessionClient`'s method/params dialect silently dropped — every real web-mode data call times out | Critical | ✅ Fixed (SOL-005) |

## Fix Priority

**BUG-001 was fixed first.** It was the single root cause that turns every other bug from
"returns a fast error message" into a 30-second client-side timeout. All 4 bugs were
then fixed in sequence: SOL-001 → SOL-002 → SOL-003 → SOL-004.

## Resolution Summary

| Frontend Error (Before) | Behavior After Fix |
|-------------------------|--------------------|
| `crashReports.getLatestPending` → timeout 30s | Returns `null` instantly |
| `rateLimits.get` → timeout 30s | Returns `{requestsPerSecond, burst}` instantly |
| `devServer.list` → timeout 30s | Returns error after ≤8s (rpcTimeout) |
| `preflight.check` → timeout 30s | Returns in milliseconds |

## Affected Files (All Modified)

- `services/api-gateway/internal/adapter/wscompat/handler.go` — BUG-001, BUG-004, BUG-005 ✅
- `services/api-gateway/internal/adapter/wscompat/channels.go` — BUG-002, BUG-003, BUG-004 ✅
- `services/api-gateway/internal/adapter/wscompat/envelope.go` — BUG-005 ✅
- `services/api-gateway/internal/adapter/wscompat/session_dialect.go` — BUG-005 (new file) ✅
- `services/api-gateway/internal/usecase/rate_limit.go` — BUG-002 (RPS/Burst accessors) ✅
- `services/api-gateway/cmd/server/main.go` — BUG-002 (wiring) ✅

## Regression Tests Added

21 new tests across 3 files — all passing as of 2026-08-26:

```
internal/adapter/wscompat/handler_test.go   — 3 tests (BUG-001/002) + 5 tests (BUG-005)
internal/adapter/wscompat/channels_test.go  — 10 tests
internal/usecase/rate_limit_test.go         — 3 tests
```
