package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestKeyedSerializer_SerializesSameKey is the core correctness test: N
// goroutines call Do concurrently with the SAME key, each performing a
// deliberately-racy, unguarded increment of a shared counter inside fn. If
// KeyedSerializer fails to serialize same-key calls, this increment races
// and `go test -race` reports it; if serialization holds, there is no
// concurrent access to counter at all, no race is reported, and the final
// count is exact.
func TestKeyedSerializer_SerializesSameKey(t *testing.T) {
	s := NewKeyedSerializer(0)

	const goroutines = 50
	const incrementsEach = 100

	counter := 0 // intentionally unguarded by any mutex/atomic

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range incrementsEach {
				err := s.Do(context.Background(), "same-key", func() error {
					current := counter    // read
					counter = current + 1 // write — racy unless Do truly serializes
					return nil
				})
				if err != nil {
					t.Errorf("unexpected error from Do: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	want := goroutines * incrementsEach
	if counter != want {
		t.Errorf("expected counter=%d after serialized increments, got %d", want, counter)
	}
}

// TestKeyedSerializer_PropagatesFnError checks the plumbing of fn's return
// value back through Do.
func TestKeyedSerializer_PropagatesFnError(t *testing.T) {
	s := NewKeyedSerializer(0)
	wantErr := errors.New("boom")

	err := s.Do(context.Background(), "k1", func() error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// TestKeyedSerializer_DifferentKeysRunConcurrently proves the flip side of
// serialization: two DIFFERENT keys must be able to run at the same time,
// not be globally serialized. Each fn signals it has started, then blocks
// until told to proceed; if the implementation wrongly serializes across
// keys, the second Do call would never even start until the first
// completes, and this test would time out.
func TestKeyedSerializer_DifferentKeysRunConcurrently(t *testing.T) {
	s := NewKeyedSerializer(0)

	started := make(chan string, 2)
	release := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)
	for _, key := range []string{"key-a", "key-b"} {
		key := key
		go func() {
			defer wg.Done()
			_ = s.Do(context.Background(), key, func() error {
				started <- key
				<-release
				return nil
			})
		}()
	}

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for range 2 {
		select {
		case k := <-started:
			seen[k] = true
		case <-timeout:
			t.Fatal("timed out waiting for both distinct-key jobs to start concurrently — different keys appear to be serialized against each other")
		}
	}
	close(release)
	wg.Wait()

	if !seen["key-a"] || !seen["key-b"] {
		t.Fatalf("expected both keys' jobs to start, got %v", seen)
	}
}

// TestKeyedSerializer_QueuedCallRespectsContextCancellation ensures a Do
// call that is still queued behind an in-flight job for the same key
// returns promptly on context cancellation instead of blocking until the
// in-flight job completes.
func TestKeyedSerializer_QueuedCallRespectsContextCancellation(t *testing.T) {
	s := NewKeyedSerializer(0)

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	go func() {
		_ = s.Do(context.Background(), "key-1", func() error {
			close(firstStarted)
			<-releaseFirst
			return nil
		})
	}()
	<-firstStarted

	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- s.Do(ctx, "key-1", func() error { return nil })
	}()

	// Give the second Do a moment to actually enqueue behind the first.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued Do call did not respect context cancellation")
	}

	close(releaseFirst)
}

// TestKeyedSerializer_TearsDownIdleWorkers exercises the idle-timeout
// cleanup path: after a short idle timeout with no pending work, a key's
// backing worker/goroutine is removed so long-lived processes handling many
// distinct handles over time don't leak one goroutine per key forever.
func TestKeyedSerializer_TearsDownIdleWorkers(t *testing.T) {
	s := NewKeyedSerializer(20 * time.Millisecond)

	if err := s.Do(context.Background(), "transient-key", func() error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		_, exists := s.workers["transient-key"]
		s.mu.Unlock()
		if !exists {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("expected idle worker to be torn down, but it is still registered")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
