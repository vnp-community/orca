package devserveragent

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithOnSessionAttached_CalledAfterHandshake_NotBlocking(t *testing.T) {
	released := make(chan struct{})
	got := make(chan string, 1)
	client := New(DefaultConfig(), slog.Default(), WithOnSessionAttached(func(devServerID string) {
		got <- devServerID
		<-released // a slow probe must not hold up the attach
	}))
	t.Cleanup(client.Close)
	t.Cleanup(func() { close(released) })

	start := time.Now()
	client.AttachTransport("ds-hook", "host-1", &stubTransport{}, HandshakeInfo{Platform: "linux"})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("AttachTransport blocked for %v on a slow callback", elapsed)
	}

	select {
	case id := <-got:
		if id != "ds-hook" {
			t.Errorf("callback got %q, want ds-hook", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback was never invoked after the session attached")
	}
	if _, ok := client.LastHandshakeInfo("ds-hook"); !ok {
		t.Error("session must be handshaked before/while the callback runs")
	}
}

func TestWithOnSessionAttached_FiresOnEveryAttach(t *testing.T) {
	var calls atomic.Int32
	client := New(DefaultConfig(), slog.Default(), WithOnSessionAttached(func(string) { calls.Add(1) }))
	t.Cleanup(client.Close)

	client.AttachTransport("ds-a", "h", &stubTransport{}, HandshakeInfo{})
	client.AttachTransport("ds-a", "h", &stubTransport{}, HandshakeInfo{}) // reconnect of the same dev server
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() != 2 {
		t.Errorf("callback calls=%d, want 2", calls.Load())
	}
}

func TestSessionAttached_NoCallbackIsSafe(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)
	client.AttachTransport("ds-none", "h", &stubTransport{}, HandshakeInfo{})
}
