package usecase

import (
	"context"
	"sync"
	"time"
)

// DefaultIdleTimeout is how long a per-key worker goroutine stays alive
// with no pending work before it tears itself down.
const DefaultIdleTimeout = 60 * time.Second

// job is one unit of serialized work submitted to a keyed worker.
type job struct {
	fn   func() error
	done chan error
}

// keyedWorker is the single-goroutine FIFO queue for one key. Every Do call
// for that key hands its job to jobs and waits on the returned done channel
// — strictly one job in flight per key at a time, in submission order.
type keyedWorker struct {
	jobs chan job
	// pending is the number of jobs enqueued-or-executing for this worker.
	// Guarded by KeyedSerializer.mu (not its own lock) so a worker can never
	// decide to tear itself down while a Do call is mid-handoff — see Do
	// and runWorker for the invariant this protects.
	pending int
}

// KeyedSerializer is the production implementation of HandleSerializer —
// the KeyedAsyncQueue port from orchestration-service.md §6. It serializes
// calls sharing the same key while allowing different keys to run
// concurrently, using one buffered-channel-backed worker goroutine per
// active key (spun up lazily, torn down after DefaultIdleTimeout of
// inactivity) rather than a plain map[string]*sync.Mutex: a mutex only
// guarantees exclusion, not ordering — two goroutines racing for Lock() can
// still commit out of order. A worker-per-key channel enforces strict FIFO
// per key, matching the TS KeyedAsyncQueue's queue semantics.
type KeyedSerializer struct {
	mu          sync.Mutex
	workers     map[string]*keyedWorker
	idleTimeout time.Duration
}

// NewKeyedSerializer constructs a KeyedSerializer. Passing idleTimeout <= 0
// uses DefaultIdleTimeout — tests that want to observe worker teardown can
// pass a short one.
func NewKeyedSerializer(idleTimeout time.Duration) *KeyedSerializer {
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}
	return &KeyedSerializer{
		workers:     make(map[string]*keyedWorker),
		idleTimeout: idleTimeout,
	}
}

// Do runs fn, serialized against every other Do call sharing key — calls
// for different keys run concurrently. Returns fn's error, or ctx.Err() if
// ctx is cancelled before fn starts running (a job already handed to the
// worker still runs to completion in the background so the per-key FIFO
// ordering is never broken by a caller's cancellation).
func (s *KeyedSerializer) Do(ctx context.Context, key string, fn func() error) error {
	w := s.acquireWorker(key)

	done := make(chan error, 1)
	select {
	case w.jobs <- job{fn: fn, done: done}:
	case <-ctx.Done():
		s.release(w)
		return ctx.Err()
	}

	select {
	case err := <-done:
		s.release(w)
		return err
	case <-ctx.Done():
		// The job was already handed off and will run regardless — wait
		// for it in the background so `pending` (and thus worker teardown)
		// stays accurate, but don't make this caller block on it.
		go func() {
			<-done
			s.release(w)
		}()
		return ctx.Err()
	}
}

// acquireWorker returns the worker for key, creating and starting it if
// necessary, and increments its pending count — all under s.mu so this can
// never race with runWorker's idle-teardown check (see runWorker).
func (s *KeyedSerializer) acquireWorker(key string) *keyedWorker {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workers[key]
	if !ok {
		w = &keyedWorker{jobs: make(chan job)}
		s.workers[key] = w
		go s.runWorker(key, w)
	}
	w.pending++
	return w
}

func (s *KeyedSerializer) release(w *keyedWorker) {
	s.mu.Lock()
	w.pending--
	s.mu.Unlock()
}

// runWorker is the single goroutine backing one key's FIFO queue. It exits
// (and removes itself from the map) only after idleTimeout with no jobs AND
// pending == 0 — the pending check, taken under the same lock acquireWorker
// increments under, guarantees a Do call that has already been handed a *w
// reference will always find that worker still reading w.jobs.
func (s *KeyedSerializer) runWorker(key string, w *keyedWorker) {
	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()

	for {
		select {
		case j := <-w.jobs:
			j.done <- j.fn()
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(s.idleTimeout)

		case <-timer.C:
			s.mu.Lock()
			if w.pending == 0 {
				delete(s.workers, key)
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			timer.Reset(s.idleTimeout)
		}
	}
}
