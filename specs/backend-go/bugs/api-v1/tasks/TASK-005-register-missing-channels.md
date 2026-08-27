# TASK-005: Đăng ký channels `crashReports.getLatestPending` và `rateLimits.get`

**From Solution:** SOL-002 (Code Change 2)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`  
**Depends on:** TASK-003 (cần `RPS()` và `Burst()` methods trên `RateLimiter`)  
**Status:** `[x]` DONE

---

## Context

Frontend gọi `crashReports.getLatestPending` và `rateLimits.get` trong mỗi bootstrap.
Cả hai hiện chưa được đăng ký → rơi vào `notImplementedHandler`. Cần thêm 2 handlers
và cập nhật `RegisterRealChannels` signature.

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Bước 1: Thêm types và `registerCrashReportChannels`

Thêm section mới vào cuối file, **trước** `registerPreflightChannels`:

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

### Bước 2: Thêm `rateLimitInfo`, `rateLimitReader`, và `registerRateLimitChannels`

Thêm tiếp vào cuối file:

```go
// ── rateLimits.* ────────────────────────────────────────────────────────────

// rateLimitInfo is the wire shape rateLimits.get returns — mirrors the
// frontend's RateLimitInfo type.
type rateLimitInfo struct {
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	Burst             int     `json:"burst"`
}

// rateLimitReader is a minimal read interface over usecase.RateLimiter so
// this file stays testable without importing the full concrete struct.
// Satisfied by *usecase.RateLimiter after TASK-003 adds RPS()/Burst().
type rateLimitReader interface {
	RPS() float64
	Burst() int
}

// registerRateLimitChannels exposes api-gateway's in-process per-tenant rate
// limiter configuration. The frontend calls rateLimits.get during bootstrap to
// understand the current throttle policy (e.g. for UI-level quota indicators).
// Returns the limiter's configured RPS/burst — not per-tenant counters (those
// are ephemeral per-replica state, not meaningful to expose externally).
func registerRateLimitChannels(r *Registry, rl rateLimitReader) {
	r.Register("rateLimits.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return rateLimitInfo{
			RequestsPerSecond: rl.RPS(),
			Burst:             rl.Burst(),
		}, nil
	})
}
```

### Bước 3: Cập nhật `RegisterRealChannels` signature và body

Tìm function `RegisterRealChannels` hiện tại:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
) {
```

Thay bằng:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
```

⚠️ Lúc này `cmd/server/main.go` sẽ **fail build** vì `RegisterRealChannels` thiếu
tham số mới. Đó là expected — TASK-006 sẽ fix call site trong `main.go`.
