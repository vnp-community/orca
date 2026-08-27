# SOL-002: Fix BUG-002 — Register `crashReports.getLatestPending` và `rateLimits.get`

**Resolves:** BUG-002  
**Service:** `api-gateway`  
**Affected files:**
- `services/api-gateway/internal/adapter/wscompat/channels.go`
- `services/api-gateway/internal/usecase/rate_limit.go`
- `services/api-gateway/cmd/server/main.go`  
**Status:** ✅ IMPLEMENTED (2026-08-24) — TASK-003 + TASK-004 + TASK-005 + TASK-006 + TASK-007

---

## Implementation Notes

Đã áp dụng đúng như thiết kế. Thay đổi thực tế:

**`usecase/rate_limit.go`:** Thêm `RPS() float64` và `Burst() int` accessor methods.

**`wscompat/channels.go`:**
- Thêm `rateLimitReader` interface và `rateLimitInfo` struct
- Thêm `registerCrashReportChannels` (trả `nil, nil`)
- Thêm `registerRateLimitChannels` (trả `rateLimitInfo{RPS, Burst}`)
- Cập nhật `RegisterRealChannels` thêm tham số `rateLimits rateLimitReader`
- Gọi 2 registrators mới trong `RegisterRealChannels`

**`cmd/server/main.go`:** Thêm `rateLimiter` vào call site `RegisterRealChannels`.

Tests thêm mới:
- `rate_limit_test.go`: `TestRateLimiterAccessors`, `TestRateLimiterAccessors_ZeroValues`, `TestRateLimiterAccessors_DoNotMutateState` ✅
- `channels_test.go`: `TestCrashReportGetLatestPendingChannel_ReturnsNull`, `TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs`, `TestRateLimitsGetChannel_ReturnsConfiguredValues`, `TestRateLimitsGetChannel_JSONFieldNames` ✅

## Solution Design

Đăng ký 2 channels còn thiếu mà frontend gọi trong bootstrap sequence:

1. **`crashReports.getLatestPending`** — Backend-go không có crash reporting service.
   Trả về `null` (no pending crash report). Không fake, không stub — đây là câu trả lời
   trung thực: service này không tồn tại trong kiến trúc backend-go.

2. **`rateLimits.get`** — Trả về config rate limit hiện tại. `api-gateway` đã có
   `usecase.RateLimiter` — thêm 2 read-only accessors `RPS()` và `Burst()` để
   channel handler có thể expose chúng.

Thiết kế tuân thủ Clean Architecture (03-clean-architecture-guidelines.md):
- Business data (`RateLimiter` state) nằm trong `usecase/` — channel handler chỉ
  đọc qua accessors, không import bất kỳ infrastructure layer nào.
- Channel handlers không có business logic — chỉ translate từ wire format sang
  usecase calls.

---

## Code Change 1 — `usecase/rate_limit.go`

Thêm 2 read-only accessors vào `RateLimiter`:

```go
// RPS returns the per-tenant request-per-second limit this limiter enforces.
// Safe for concurrent use (reads immutable fields set at construction).
func (l *RateLimiter) RPS() float64 {
    return float64(l.rps)
}

// Burst returns the per-tenant burst size this limiter allows.
// Safe for concurrent use (reads immutable fields set at construction).
func (l *RateLimiter) Burst() int {
    return l.burst
}
```

---

## Code Change 2 — `wscompat/channels.go`

Thêm 2 function mới `registerCrashReportChannels` và `registerRateLimitChannels`,
sau đó gọi chúng trong `RegisterRealChannels`.

### Thêm `registerCrashReportChannels`

```go
// ── crashReports.* ──────────────────────────────────────────────────────────
//
// backend-go has no crash reporting service — this architecture uses structured
// gRPC error propagation (apperrors.ToGRPCStatus) and OpenTelemetry traces
// instead of a separate crash-report collection service. The frontend calls
// crashReports.getLatestPending on every bootstrap; returning null is the
// honest answer ("no pending crash report"), not a stub — there is genuinely
// nothing to report from backend-go's crash/panic path.
func registerCrashReportChannels(r *Registry) {
    r.Register("crashReports.getLatestPending", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        // null signals "no pending crash report" — matches the frontend's
        // crashReports.getLatestPending contract (nullable return).
        return nil, nil
    })
}
```

### Thêm `registerRateLimitChannels`

```go
// rateLimitInfo is the wire shape rateLimits.get returns — mirrors the
// frontend's RateLimitInfo type (specs/frontend/api/rpc-catalog.md).
type rateLimitInfo struct {
    RequestsPerSecond float64 `json:"requestsPerSecond"`
    Burst             int     `json:"burst"`
}

// rateLimitReader is a minimal read interface over usecase.RateLimiter so
// this file stays testable without importing the full concrete struct.
type rateLimitReader interface {
    RPS() float64
    Burst() int
}

// ── rateLimits.* ────────────────────────────────────────────────────────────
//
// Exposes api-gateway's in-process per-tenant rate limiter configuration.
// The frontend calls rateLimits.get during bootstrap to understand the
// current throttle policy (e.g. for UI-level quota indicators). Returns
// the limiter's configured RPS/burst — not per-tenant counters (those are
// ephemeral per-replica state, not meaningful to expose externally).
func registerRateLimitChannels(r *Registry, rl rateLimitReader) {
    r.Register("rateLimits.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        return rateLimitInfo{
            RequestsPerSecond: rl.RPS(),
            Burst:             rl.Burst(),
        }, nil
    })
}
```

### Cập nhật `RegisterRealChannels`

```go
func RegisterRealChannels(
    r *Registry,
    annotationClient annotationv1.AnnotationServiceClient,
    taskClient taskv1.TaskServiceClient,
    gitClient gitgatewayv1.GitGatewayServiceClient,
    automationClient automationv1.AutomationServiceClient,
    infraFleetClient infrafleetv1.InfraFleetServiceClient,
    rateLimits rateLimitReader, // NEW parameter
) {
    registerAnnotationChannels(r, annotationClient)
    registerTaskChannels(r, taskClient)
    registerGitChannels(r, gitClient)
    registerAutomationChannels(r, automationClient)
    registerPreflightChannels(r)
    registerDevServerChannels(r, infraFleetClient)
    registerFleetChannels(r, infraFleetClient)
    registerCrashReportChannels(r)   // NEW
    registerRateLimitChannels(r, rateLimits) // NEW
}
```

---

## Code Change 3 — `cmd/server/main.go`

Cập nhật call site `wscompat.RegisterRealChannels` để truyền thêm `rateLimiter`:

```go
// Before:
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient)

// After:
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

---

## TDD — Test Cases to Add

File: `services/api-gateway/internal/adapter/wscompat/channels_test.go`

### Test 1: `crashReports.getLatestPending` trả về nil

```go
// TestCrashReportGetLatestPendingChannel_ReturnsNull verifies that the channel
// returns nil (JSON null) — the honest answer for a service that has no crash
// reporting backend.
func TestCrashReportGetLatestPendingChannel_ReturnsNull(t *testing.T) {
    r := NewRegistry()
    registerCrashReportChannels(r)

    result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "crashReports.getLatestPending", nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != nil {
        t.Errorf("want nil (no crash report), got %v", result)
    }
}
```

### Test 2: `rateLimits.get` trả về config đúng

```go
// fakeRateLimitReader is a test double for rateLimitReader.
type fakeRateLimitReader struct{ rps float64; burst int }
func (f *fakeRateLimitReader) RPS() float64 { return f.rps }
func (f *fakeRateLimitReader) Burst() int   { return f.burst }

// TestRateLimitsGetChannel_ReturnsConfiguredValues verifies the channel
// exposes the limiter's configured RPS and burst — not counters.
func TestRateLimitsGetChannel_ReturnsConfiguredValues(t *testing.T) {
    r := NewRegistry()
    rl := &fakeRateLimitReader{rps: 100.0, burst: 200}
    registerRateLimitChannels(r, rl)

    result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "rateLimits.get", nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    info, ok := result.(rateLimitInfo)
    if !ok {
        t.Fatalf("unexpected result type %T", result)
    }
    if info.RequestsPerSecond != 100.0 {
        t.Errorf("want rps=100.0, got %f", info.RequestsPerSecond)
    }
    if info.Burst != 200 {
        t.Errorf("want burst=200, got %d", info.Burst)
    }
}
```

### Test 3: `usecase.RateLimiter` accessors đúng

File: `services/api-gateway/internal/usecase/rate_limit_test.go`

```go
// TestRateLimiterAccessors verifies the new RPS()/Burst() read methods.
func TestRateLimiterAccessors(t *testing.T) {
    rl := NewRateLimiter(50.0, 100)
    if got := rl.RPS(); got != 50.0 {
        t.Errorf("want RPS=50.0, got %f", got)
    }
    if got := rl.Burst(); got != 100 {
        t.Errorf("want Burst=100, got %d", got)
    }
}
```

---

## Verification

```bash
cd services/api-gateway
go test ./internal/usecase/... -run TestRateLimiter -v
go test ./internal/adapter/wscompat/... -run "TestCrashReport|TestRateLimits" -v
go build ./...
```

---

## Files Changed

| File | Change |
|------|--------|
| `internal/usecase/rate_limit.go` | Thêm `RPS()` và `Burst()` accessor methods |
| `internal/usecase/rate_limit_test.go` | Thêm `TestRateLimiterAccessors` |
| `internal/adapter/wscompat/channels.go` | Thêm `registerCrashReportChannels`, `rateLimitInfo`, `rateLimitReader`, `registerRateLimitChannels`; cập nhật `RegisterRealChannels` signature |
| `internal/adapter/wscompat/channels_test.go` | Thêm 2 test functions |
| `cmd/server/main.go` | Truyền thêm `rateLimiter` vào `RegisterRealChannels` call |
