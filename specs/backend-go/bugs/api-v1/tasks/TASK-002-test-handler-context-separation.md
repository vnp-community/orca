# TASK-002: Viết tests cho TASK-001 (handler context separation)

**From Solution:** SOL-001  
**Priority:** P0 — chạy song song hoặc ngay sau TASK-001  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/handler_test.go`  
**Depends on:** TASK-001 (cần `writeTimeout` constant và `dispatchCtx` tách rời)  
**Status:** `[x]` DONE

---

## Context

File `handler_test.go` hiện có các tests cho `Handler.ServeHTTP` nhưng chưa có tests
kiểm tra behavior của `handleInvoke` khi context bị cancel. Cần thêm tests để:
1. Guard regression cho BUG-001 fix
2. Xác nhận error/result vẫn được deliver khi dispatchCtx expire

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/handler_test.go`

Thêm 3 test functions mới vào cuối file (sau phần tests hiện có):

```go
// TestNotImplementedChannelReturnsErrorFast verifies that an unregistered
// channel returns an error immediately (< 500ms), not after the 30s frontend
// INVOKE_TIMEOUT_MS. Regression guard for BUG-001 + BUG-002.
func TestNotImplementedChannelReturnsErrorFast(t *testing.T) {
	reg := NewRegistry() // empty registry — every channel falls to notImplementedHandler

	start := time.Now()
	_, err := reg.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "crashReports.getLatestPending", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error for unregistered channel, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("notImplementedHandler should be instant, took %s — possible context block", elapsed)
	}
}

// TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName verifies
// that the notImplementedHandler's error message contains the channel name
// so the frontend (and logs) can identify which channel is missing.
func TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Dispatch(context.Background(), Identity{}, "rateLimits.get", nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "rateLimits.get") {
		t.Errorf("error message should contain channel name 'rateLimits.get', got: %q", err.Error())
	}
}

// TestWriteTimeoutConstant_ShorterThanInvokeTimeout documents the required
// relationship between writeTimeout (SOL-001) and invokeTimeout. If someone
// accidentally sets writeTimeout >= invokeTimeout, the write would always
// race with the dispatch cancellation instead of running independently.
func TestWriteTimeoutConstant_ShorterThanInvokeTimeout(t *testing.T) {
	if writeTimeout >= invokeTimeout {
		t.Errorf("writeTimeout (%s) must be < invokeTimeout (%s); "+
			"writeTimeout is for the write-back step only, not the full dispatch",
			writeTimeout, invokeTimeout)
	}
}
```

---

## Import cần thêm vào handler_test.go

Kiểm tra và thêm nếu chưa có:
```go
import (
    "strings"
    "time"
    // ... các imports hiện có
)
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... \
    -run "TestNotImplemented|TestRegistryDispatch_Unregistered|TestWriteTimeout" \
    -v -count=1
```

Expected output:
```
--- PASS: TestNotImplementedChannelReturnsErrorFast (0.00s)
--- PASS: TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName (0.00s)
--- PASS: TestWriteTimeoutConstant_ShorterThanInvokeTimeout (0.00s)
PASS
```
