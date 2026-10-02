package mcpservertest

import (
	"sync"
	"sync/atomic"
)

// CountingRecorder implements mcpserver.Recorder and mcpserver.SessionRecorder
// with plain counters (tests and ad-hoc debugging).
type CountingRecorder struct {
	Mismatch, RateLimits, Cancelled, Dropped atomic.Int64
	Opened, Closed, StreamsOpen              atomic.Int64

	mu       sync.Mutex
	authFail map[string]int
	resume   map[string]int
	closeWhy map[string]int
}

func (r *CountingRecorder) bump(m *map[string]int, k string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if *m == nil {
		*m = map[string]int{}
	}
	(*m)[k]++
}

func (r *CountingRecorder) get(m map[string]int, k string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return m[k]
}

func (r *CountingRecorder) AuthFailure(reason string) { r.bump(&r.authFail, reason) }
func (r *CountingRecorder) IdentityMismatch()         { r.Mismatch.Add(1) }
func (r *CountingRecorder) RateLimited()              { r.RateLimits.Add(1) }
func (r *CountingRecorder) SessionOpened()            { r.Opened.Add(1) }
func (r *CountingRecorder) SessionClosed(why string)  { r.Closed.Add(1); r.bump(&r.closeWhy, why) }
func (r *CountingRecorder) StreamOpened()             { r.StreamsOpen.Add(1) }
func (r *CountingRecorder) StreamClosed()             { r.StreamsOpen.Add(-1) }
func (r *CountingRecorder) Resume(result string)      { r.bump(&r.resume, result) }
func (r *CountingRecorder) RequestCancelled()         { r.Cancelled.Add(1) }
func (r *CountingRecorder) ProgressCoalesced(n int)   { r.Dropped.Add(int64(n)) }

func (r *CountingRecorder) AuthFailures(reason string) int { return r.get(r.authFail, reason) }
func (r *CountingRecorder) Resumes(result string) int      { return r.get(r.resume, result) }
func (r *CountingRecorder) ClosedFor(reason string) int    { return r.get(r.closeWhy, reason) }
