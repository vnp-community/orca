package usecase

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestIndexEventCoalescer_Debounce(t *testing.T) {
	var mu sync.Mutex
	var flushed []CoalescedEvent
	done := make(chan struct{})

	c := NewIndexEventCoalescer(50*time.Millisecond, 200*time.Millisecond, func(ctx context.Context, e CoalescedEvent) {
		mu.Lock()
		flushed = append(flushed, e)
		mu.Unlock()
		close(done)
	})

	// Ingest 3 events within 20ms
	c.Ingest(context.Background(), "t1", "b1", "gitnexus", "c1", "2026-10-06T10:00:00Z", "changed")
	time.Sleep(10 * time.Millisecond)
	c.Ingest(context.Background(), "t1", "b1", "codegraph", "c2", "2026-10-06T10:01:00Z", "changed")
	time.Sleep(10 * time.Millisecond)
	c.Ingest(context.Background(), "t1", "b1", "git", "c3", "2026-10-06T10:02:00Z", "changed")

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for coalescer flush")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 1 {
		t.Fatalf("expected 1 coalesced event, got %d", len(flushed))
	}
	e := flushed[0]
	if e.NewestCommit != "c3" {
		t.Errorf("expected newest commit 'c3', got %s", e.NewestCommit)
	}
	if len(e.Tools) != 3 {
		t.Errorf("expected 3 tools merged, got %d", len(e.Tools))
	}
}

func TestDevServerRateLimiter_EnforcesLimit(t *testing.T) {
	limiter := NewDevServerRateLimiter(10.0, 5)

	// First 5 allowed (burst capacity)
	for i := 0; i < 5; i++ {
		if !limiter.Allow("dev-1") {
			t.Fatalf("expected call %d to be allowed", i)
		}
	}

	// 6th call should be rejected
	if limiter.Allow("dev-1") {
		t.Fatalf("expected call 6 to be rejected by rate limiter")
	}
}
