package usecase

import "testing"

func TestRateLimiter_AllowsBurstThenBlocks(t *testing.T) {
	// 1 request/sec sustained, burst of 3: the first 3 calls for a tenant
	// should succeed immediately (consuming the full bucket), the 4th
	// should be blocked because the bucket hasn't had time to refill.
	rl := NewRateLimiter(1, 3)

	for i := 0; i < 3; i++ {
		if !rl.Allow("tenant-a") {
			t.Fatalf("call %d: expected Allow to succeed within burst, got false", i+1)
		}
	}

	if rl.Allow("tenant-a") {
		t.Fatal("expected 4th call to be blocked once the burst is exhausted")
	}
}

func TestRateLimiter_TracksTenantsIndependently(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	if !rl.Allow("tenant-a") {
		t.Fatal("expected tenant-a's first call to succeed")
	}
	if rl.Allow("tenant-a") {
		t.Fatal("expected tenant-a's second call to be blocked (burst=1)")
	}

	// tenant-b has its own independent bucket, unaffected by tenant-a's
	// consumption — this is the whole point of per-tenant limiting.
	if !rl.Allow("tenant-b") {
		t.Fatal("expected tenant-b's first call to succeed regardless of tenant-a's state")
	}
}

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
