# TASK-004: Viết test cho `RateLimiter` accessors

**From Solution:** SOL-002 (TDD test 3)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/usecase/rate_limit_test.go`  
**Depends on:** TASK-003  
**Status:** `[x]` DONE

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/usecase/rate_limit_test.go`

Thêm test function vào cuối file (sau các tests hiện có):

```go
// TestRateLimiterAccessors verifies the RPS() and Burst() read-only methods
// added by SOL-002 / TASK-003 — confirming they return the configured values
// without mutating any state.
func TestRateLimiterAccessors(t *testing.T) {
	rl := NewRateLimiter(50.0, 100)

	if got := rl.RPS(); got != 50.0 {
		t.Errorf("RPS(): want 50.0, got %f", got)
	}
	if got := rl.Burst(); got != 100 {
		t.Errorf("Burst(): want 100, got %d", got)
	}
}

// TestRateLimiterAccessors_ZeroValues verifies edge case with zero/minimal config.
func TestRateLimiterAccessors_ZeroValues(t *testing.T) {
	rl := NewRateLimiter(0, 0)

	if got := rl.RPS(); got != 0 {
		t.Errorf("RPS(): want 0, got %f", got)
	}
	if got := rl.Burst(); got != 0 {
		t.Errorf("Burst(): want 0, got %d", got)
	}
}

// TestRateLimiterAccessors_DoNotMutateState verifies that calling RPS()/Burst()
// multiple times returns the same value (no side effects, safe for concurrent use).
func TestRateLimiterAccessors_DoNotMutateState(t *testing.T) {
	rl := NewRateLimiter(100.0, 50)

	for i := 0; i < 10; i++ {
		if got := rl.RPS(); got != 100.0 {
			t.Errorf("iteration %d: RPS() mutated state, got %f", i, got)
		}
		if got := rl.Burst(); got != 50 {
			t.Errorf("iteration %d: Burst() mutated state, got %d", i, got)
		}
	}
}
```

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./internal/usecase/... -run TestRateLimiterAccessors -v -count=1
```

Expected output:
```
--- PASS: TestRateLimiterAccessors (0.00s)
--- PASS: TestRateLimiterAccessors_ZeroValues (0.00s)
--- PASS: TestRateLimiterAccessors_DoNotMutateState (0.00s)
PASS
```
