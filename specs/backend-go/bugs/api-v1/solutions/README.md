# Solutions — API v1 Bug Fixes

Thư mục này chứa giải pháp chi tiết (theo chuẩn TDD của dự án) cho các bugs
được phát hiện từ lỗi "Request timed out" trên frontend.

> **Status: ✅ IMPLEMENTED** — Tất cả solutions đã được áp dụng vào codebase
> và verified qua full regression test suite (2026-08-24).

## Quick Start

**Thứ tự áp dụng (đã thực thi theo đúng thứ tự này):**

```
SOL-001 → SOL-002 → SOL-003 → SOL-004
```

SOL-001 là fix bắt buộc trước tiên — nó giải quyết root cause khiến tất cả
error messages bị drop silently, làm cho mọi bug khác trở thành 30s timeout.

---

## Solution Index

| Solution | Bug | Mô tả | Severity | Status | Files thay đổi |
|----------|-----|-------|----------|--------|---------------|
| [SOL-001](./SOL-001-fix-cancelled-context-write.md) | BUG-001 | Tách `dispatchCtx` và `writeCtx` trong `handleInvoke` — root cause fix | P0 | ✅ DONE | `handler.go`, `handler_test.go` |
| [SOL-002](./SOL-002-register-missing-channels.md) | BUG-002 | Đăng ký `crashReports.getLatestPending` (trả nil) và `rateLimits.get` (trả config) | P1 | ✅ DONE | `channels.go`, `rate_limit.go`, `channels_test.go`, `rate_limit_test.go`, `main.go` |
| [SOL-003](./SOL-003-devserver-list-per-rpc-deadline.md) | BUG-003 | Thêm `rpcTimeout=8s` per-RPC deadline cho mọi gRPC call trong channels | P1 | ✅ DONE | `channels.go`, `channels_test.go` |
| [SOL-004](./SOL-004-preflight-writemu-starvation.md) | BUG-004 | Thêm `writeMu` contention logging + regression tests cho `preflight.check` | P2 | ✅ DONE | `handler.go`, `channels.go`, `channels_test.go` |

---

## Impact sau khi áp dụng tất cả solutions

| Frontend Error | Trước | Sau |
|---------------|-------|-----|
| `crashReports.getLatestPending` | ❌ timeout 30s | ✅ Returns `null` ngay lập tức |
| `rateLimits.get` | ❌ timeout 30s | ✅ Returns `{requestsPerSecond, burst}` ngay lập tức |
| `devServer.list` | ❌ timeout 30s khi infra-fleet slow | ✅ Returns lỗi `DeadlineExceeded` sau ≤8s |
| `preflight.check` | ❌ timeout 30s | ✅ Returns trong vòng milliseconds |

---

## Constant Relationships (sau khi áp dụng SOL-001 + SOL-003)

```
writeTimeout  (5s)  ← SOL-001: fresh context cho write-back
rpcTimeout    (8s)  ← SOL-003: per-RPC deadline trước khi fail fast
invokeTimeout (25s) ← handler.go: transport-level dispatch window (existing)
INVOKE_TIMEOUT_MS (30s) ← frontend rpc-client.ts: client-side timeout (không thay đổi)

Invariant: writeTimeout < rpcTimeout < invokeTimeout < INVOKE_TIMEOUT_MS
           5s           < 8s        < 25s            < 30s
```

---

## Actual Code Changes (implemented 2026-08-24)

### Files modified

| File | Changes |
|------|---------|
| `internal/adapter/wscompat/handler.go` | Thêm `writeTimeout = 5s`; tách `dispatchCtx`/`writeCtx` trong `handleInvoke`; thêm `lockStart`/`lock_wait` contention log; cập nhật `handleSend` dùng `context.Background()` cho log |
| `internal/adapter/wscompat/channels.go` | Thêm `rpcTimeout = 8s`; `rateLimitReader` interface; `rateLimitInfo` struct; `registerCrashReportChannels`; `registerRateLimitChannels`; bọc `devServer.list`, `devServer.add`, `fleet.health.checkAll` bằng `rpcCtx`; cập nhật `RegisterRealChannels` signature; cập nhật comment `registerPreflightChannels` |
| `internal/usecase/rate_limit.go` | Thêm `RPS()` và `Burst()` accessor methods |
| `cmd/server/main.go` | Truyền `rateLimiter` vào `RegisterRealChannels` call site |

### New test files

| File | Tests mới | Count |
|------|-----------|-------|
| `internal/adapter/wscompat/handler_test.go` | `TestNotImplementedChannelReturnsErrorFast`, `TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName`, `TestWriteTimeoutConstant_ShorterThanInvokeTimeout` | 3 |
| `internal/adapter/wscompat/channels_test.go` | `TestCrashReportGetLatestPendingChannel_ReturnsNull`, `TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs`, `TestRateLimitsGetChannel_ReturnsConfiguredValues`, `TestRateLimitsGetChannel_JSONFieldNames`, `TestRPCTimeoutConstant_ShorterThanInvokeTimeout`, `TestDevServerListChannel_FailsFastWhenServiceSlow`, `TestDevServerAddChannel_FailsFastWhenServiceSlow`, `TestFleetHealthCheckAll_FailsFastWhenServiceSlow`, `TestPreflightCheckChannel_CompletesInstantly`, `TestPreflightCheckChannel_ReturnsExpectedKeys` | 10 |
| `internal/usecase/rate_limit_test.go` | `TestRateLimiterAccessors`, `TestRateLimiterAccessors_ZeroValues`, `TestRateLimiterAccessors_DoNotMutateState` | 3 |
| **Total** | | **16 new tests** |

---

## Verify Results (2026-08-24)

```
go build ./...           → exit 0 (clean)
go vet ./...             → exit 0 (no warnings)
go test ./internal/...   → PASS (26 wscompat tests + 8 usecase tests)
go test -race (fast)     → PASS (no DATA RACE)

internal/usecase:              ok  (0.749s)
internal/adapter/wscompat:     ok  (24.561s — includes 3×8s rpcTimeout tests)
```

---

## Task Execution Reference

Xem chi tiết từng task đã được thực thi tại:
[`../tasks/`](../tasks/) — 11 tasks, tất cả `[x] DONE`
