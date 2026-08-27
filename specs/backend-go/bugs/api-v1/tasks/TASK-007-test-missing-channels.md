# TASK-007: Viết tests cho `crashReports` và `rateLimits` channels

**From Solution:** SOL-002 (TDD tests 1 & 2)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/channels_test.go`  
**Depends on:** TASK-005  
**Status:** `[x]` DONE

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/channels_test.go`

Thêm vào cuối file (sau các tests hiện có):

```go
// ── Test helpers for SOL-002 channels ───────────────────────────────────────

// fakeRateLimitReader is a test double for the rateLimitReader interface.
type fakeRateLimitReader struct {
	rps   float64
	burst int
}

func (f *fakeRateLimitReader) RPS() float64 { return f.rps }
func (f *fakeRateLimitReader) Burst() int   { return f.burst }

// ── crashReports.* tests ─────────────────────────────────────────────────────

// TestCrashReportGetLatestPendingChannel_ReturnsNull verifies that the channel
// returns nil (JSON null) — the honest answer for a backend that has no crash
// reporting service. Frontend expects a nullable result.
func TestCrashReportGetLatestPendingChannel_ReturnsNull(t *testing.T) {
	r := NewRegistry()
	registerCrashReportChannels(r)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "crashReports.getLatestPending", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("want nil (no crash report in backend-go), got %v", result)
	}
}

// TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs verifies that the
// handler does not panic or error when called with no args or extra args.
func TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs(t *testing.T) {
	r := NewRegistry()
	registerCrashReportChannels(r)

	// no args
	if _, err := r.Dispatch(context.Background(), Identity{}, "crashReports.getLatestPending", nil); err != nil {
		t.Errorf("with nil args: unexpected error: %v", err)
	}
	// extra args (frontend may pass session id etc.)
	args := argsJSON(t, map[string]any{"sessionId": "abc-123"})
	if _, err := r.Dispatch(context.Background(), Identity{}, "crashReports.getLatestPending", args); err != nil {
		t.Errorf("with extra args: unexpected error: %v", err)
	}
}

// ── rateLimits.* tests ───────────────────────────────────────────────────────

// TestRateLimitsGetChannel_ReturnsConfiguredValues verifies the channel exposes
// the limiter's configured RPS and burst — not live per-tenant counters.
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
		t.Fatalf("unexpected result type %T, want rateLimitInfo", result)
	}
	if info.RequestsPerSecond != 100.0 {
		t.Errorf("want RequestsPerSecond=100.0, got %f", info.RequestsPerSecond)
	}
	if info.Burst != 200 {
		t.Errorf("want Burst=200, got %d", info.Burst)
	}
}

// TestRateLimitsGetChannel_JSONFieldNames verifies the JSON wire format has
// the field names the frontend expects (camelCase, matching rpc-catalog.md).
func TestRateLimitsGetChannel_JSONFieldNames(t *testing.T) {
	r := NewRegistry()
	rl := &fakeRateLimitReader{rps: 10.0, burst: 20}
	registerRateLimitChannels(r, rl)

	result, err := r.Dispatch(context.Background(), Identity{}, "rateLimits.get", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := m["requestsPerSecond"]; !ok {
		t.Errorf("JSON field 'requestsPerSecond' missing; got keys: %v", m)
	}
	if _, ok := m["burst"]; !ok {
		t.Errorf("JSON field 'burst' missing; got keys: %v", m)
	}
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... \
    -run "TestCrashReport|TestRateLimits" \
    -v -count=1
```

Expected output:
```
--- PASS: TestCrashReportGetLatestPendingChannel_ReturnsNull (0.00s)
--- PASS: TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs (0.00s)
--- PASS: TestRateLimitsGetChannel_ReturnsConfiguredValues (0.00s)
--- PASS: TestRateLimitsGetChannel_JSONFieldNames (0.00s)
PASS
```
