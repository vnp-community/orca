package agentwsserver

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRegistry_ConsumeReturnsRegisteredDevServerID covers the core lookup
// contract: a registered token's Consume returns the DevServer ID it was
// registered for.
func TestRegistry_ConsumeReturnsRegisteredDevServerID(t *testing.T) {
	r := NewRegistry(time.Hour)
	t.Cleanup(r.Stop)
	r.Register("tok-1", "ds-1", nil)

	devServerID, ok := r.Consume("tok-1")
	if !ok || devServerID != "ds-1" {
		t.Fatalf("Consume = (%q, %v), want (ds-1, true)", devServerID, ok)
	}
}

func TestRegistry_ConsumeUnknownTokenFails(t *testing.T) {
	r := NewRegistry(time.Hour)
	t.Cleanup(r.Stop)
	if _, ok := r.Consume("never-registered"); ok {
		t.Fatal("Consume = true, want false for a token that was never registered")
	}
}

// TestRegistry_ConsumeIsSingleUse mirrors "slot is consumed after
// successful connection (second connection rejected)" from
// agent-ws-server.test.ts.
func TestRegistry_ConsumeIsSingleUse(t *testing.T) {
	r := NewRegistry(time.Hour)
	t.Cleanup(r.Stop)
	r.Register("tok-1", "ds-1", nil)

	if _, ok := r.Consume("tok-1"); !ok {
		t.Fatal("first Consume should succeed")
	}
	if _, ok := r.Consume("tok-1"); ok {
		t.Fatal("second Consume of the same token should fail — slots are single-use")
	}
}

// TestRegistry_SlotExpiresAndCallsOnExpired mirrors "slot expires after
// AGENT_CONNECT_TIMEOUT_MS with descriptive error message".
func TestRegistry_SlotExpiresAndCallsOnExpired(t *testing.T) {
	r := NewRegistry(30 * time.Millisecond)
	t.Cleanup(r.Stop)

	expired := make(chan string, 1)
	r.Register("expire-tok", "ds-1", func(reason string) { expired <- reason })

	select {
	case reason := <-expired:
		if !strings.Contains(reason, "did not connect") || !strings.Contains(reason, "expire-tok") {
			t.Errorf("reason = %q, want it to mention 'did not connect' and the token", reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onExpired was never called")
	}

	if _, ok := r.Consume("expire-tok"); ok {
		t.Fatal("Consume should fail once the slot has expired")
	}
}

// TestRegistry_DisposerCancelsExpiry mirrors "disposer cancels expiry timer
// (onExpired NOT called after dispose)".
func TestRegistry_DisposerCancelsExpiry(t *testing.T) {
	r := NewRegistry(30 * time.Millisecond)
	t.Cleanup(r.Stop)

	expired := make(chan string, 1)
	unregister := r.Register("cancel-tok", "ds-1", func(reason string) { expired <- reason })
	unregister()

	select {
	case reason := <-expired:
		t.Fatalf("onExpired should not fire after the disposer cancelled the slot, got %q", reason)
	case <-time.After(150 * time.Millisecond):
		// expected: no expiry within well past the (short) ttl.
	}
}

// TestRegistry_ReregisteringSameTokenCancelsPreviousTimer mirrors
// "re-registering same token cancels the previous slot timer".
func TestRegistry_ReregisteringSameTokenCancelsPreviousTimer(t *testing.T) {
	r := NewRegistry(50 * time.Millisecond)
	t.Cleanup(r.Stop)

	expired1 := make(chan string, 1)
	expired2 := make(chan string, 1)
	r.Register("same-tok", "ds-1", func(reason string) { expired1 <- reason })
	r.Register("same-tok", "ds-2", func(reason string) { expired2 <- reason }) // replaces first

	select {
	case <-expired2:
	case <-time.After(2 * time.Second):
		t.Fatal("second registration's onExpired was never called")
	}

	select {
	case <-expired1:
		t.Fatal("first registration's onExpired should have been cancelled by the re-register")
	default:
	}
}

// TestRegistry_DifferentTokensHaveIndependentTimers mirrors "different
// tokens each have independent expiry".
func TestRegistry_DifferentTokensHaveIndependentTimers(t *testing.T) {
	r := NewRegistry(30 * time.Millisecond)
	t.Cleanup(r.Stop)

	expiredA := make(chan struct{}, 1)
	expiredB := make(chan struct{}, 1)
	r.Register("tok-a", "ds-a", func(string) { expiredA <- struct{}{} })
	r.Register("tok-b", "ds-b", func(string) { expiredB <- struct{}{} })

	for name, ch := range map[string]chan struct{}{"tok-a": expiredA, "tok-b": expiredB} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected %s to expire independently", name)
		}
	}
}

// TestRegistry_StopCancelsAllPendingTimers mirrors "cancels all pending
// slot timers on stop()".
func TestRegistry_StopCancelsAllPendingTimers(t *testing.T) {
	r := NewRegistry(30 * time.Millisecond)

	var mu sync.Mutex
	var fired []string
	r.Register("s1", "ds-1", func(string) { mu.Lock(); fired = append(fired, "s1"); mu.Unlock() })
	r.Register("s2", "ds-2", func(string) { mu.Lock(); fired = append(fired, "s2"); mu.Unlock() })

	r.Stop()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(fired) != 0 {
		t.Fatalf("fired = %v, want none — Stop() should cancel every pending timer", fired)
	}
}

// TestRegistry_StopIsIdempotent mirrors "stop() is idempotent (can be
// called multiple times)".
func TestRegistry_StopIsIdempotent(t *testing.T) {
	r := NewRegistry(time.Hour)
	r.Stop()
	r.Stop() // must not panic
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry(time.Hour)
	t.Cleanup(r.Stop)

	if r.Has("tok-1") {
		t.Fatal("Has = true before Register")
	}
	r.Register("tok-1", "ds-1", nil)
	if !r.Has("tok-1") {
		t.Fatal("Has = false right after Register")
	}
	r.Consume("tok-1")
	if r.Has("tok-1") {
		t.Fatal("Has = true after Consume — slots are single-use")
	}
}
