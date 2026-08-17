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
