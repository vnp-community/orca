# TASK-003: Thêm `RPS()` và `Burst()` accessors vào `usecase.RateLimiter`

**From Solution:** SOL-002 (Code Change 1)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/usecase/rate_limit.go`  
**Depends on:** Không có dependency  
**Status:** `[x]` DONE

---

## Context

`usecase.RateLimiter` hiện tại chỉ có method `Allow(tenantID string) bool`. Để
channel handler `rateLimits.get` có thể expose config cho frontend mà không vi phạm
Clean Architecture (channel handler không được import internal state trực tiếp),
cần thêm 2 read-only accessors vào `RateLimiter`.

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/usecase/rate_limit.go`

Thêm 2 methods sau method `limiterFor` (cuối file):

```go
// RPS returns the per-tenant request-per-second limit this limiter enforces.
// Safe for concurrent use — reads immutable fields set at construction time,
// no mutex needed.
func (l *RateLimiter) RPS() float64 {
	return float64(l.rps)
}

// Burst returns the per-tenant burst size this limiter allows.
// Safe for concurrent use — reads immutable fields set at construction time,
// no mutex needed.
func (l *RateLimiter) Burst() int {
	return l.burst
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./internal/usecase/...
go vet ./internal/usecase/...
```

Expected: build thành công, không có lỗi.

---

## Test sẽ được thêm trong TASK-004

TASK-003 chỉ thêm production code. Tests cho accessors này nằm trong TASK-004.
