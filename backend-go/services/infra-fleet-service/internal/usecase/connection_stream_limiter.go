package usecase

import (
	"fmt"
	"sync"
)

// defaultMaxPtyStreamsPerConnection bounds concurrent AttachPty streams per
// connectionId — a conservative default sufficient for realistic
// multi-pane/multi-terminal usage against one dev server, chosen since the
// (missing, see this package's git history) task spec did not pin an exact
// number.
const defaultMaxPtyStreamsPerConnection = 32

// ConnectionStreamLimiter bounds the number of concurrent AttachPty streams
// per connectionId. Without it, a client that opens AttachPty repeatedly
// without ever closing the stream (a leak, or a hostile client) could pin
// unbounded goroutines/session subscriptions against one dev server —
// AttachPty's Execute acquires a slot before subscribing via
// DevServerAgentClient.StreamPty and releases it when the stream ends.
type ConnectionStreamLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
}

// NewConnectionStreamLimiter builds a limiter capping each connectionId at
// max concurrent streams; max<=0 falls back to defaultMaxPtyStreamsPerConnection.
func NewConnectionStreamLimiter(max int) *ConnectionStreamLimiter {
	if max <= 0 {
		max = defaultMaxPtyStreamsPerConnection
	}
	return &ConnectionStreamLimiter{counts: make(map[string]int), max: max}
}

// ErrStreamLimitReached is returned by Acquire when connectionID already has
// the maximum number of concurrent streams open.
type ErrStreamLimitReached struct {
	ConnectionID string
	Max          int
}

func (e *ErrStreamLimitReached) Error() string {
	return fmt.Sprintf("usecase: connection %q has reached its max concurrent pty streams (%d)", e.ConnectionID, e.Max)
}

// Acquire reserves one stream slot for connectionID. On success it returns a
// release func the caller MUST call exactly once (typically via defer) when
// the stream ends; on failure it returns *ErrStreamLimitReached.
func (l *ConnectionStreamLimiter) Acquire(connectionID string) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counts[connectionID] >= l.max {
		return nil, &ErrStreamLimitReached{ConnectionID: connectionID, Max: l.max}
	}
	l.counts[connectionID]++

	var once sync.Once
	release := func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.counts[connectionID]--
			if l.counts[connectionID] <= 0 {
				delete(l.counts, connectionID)
			}
		})
	}
	return release, nil
}
