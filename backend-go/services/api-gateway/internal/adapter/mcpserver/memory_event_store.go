package mcpserver

import (
	"context"
	"iter"
	"sync"
)

// MemoryResumableStore is a bounded in-process ResumableEventStore. It is the
// single-replica default and the shared "bus" of the two-replica unit tests
// (several Handlers may share one instance to simulate a common JetStream
// buffer). When a bound is hit the OLDEST data is dropped and a resume that
// needs it fails with ErrResumeGap (client re-initializes) - it never grows
// without limit.
type MemoryResumableStore struct {
	mu               sync.Mutex
	maxPerStream     int
	maxBytes, nBytes int
	streams          map[memKey]*memStream
	seq              uint64
}

type memKey struct{ session, stream string }

type memStream struct {
	first   int // ordinal of events[0]
	events  [][]byte
	bytes   int
	touched uint64
	notify  chan struct{} // closed and replaced on every append / purge
	purged  bool
}

// NewMemoryResumableStore: maxPerStream events per stream (default 256) and
// maxBytes across all streams (default 32 MiB).
func NewMemoryResumableStore(maxPerStream, maxBytes int) *MemoryResumableStore {
	if maxPerStream <= 0 {
		maxPerStream = 256
	}
	if maxBytes <= 0 {
		maxBytes = 32 << 20
	}
	return &MemoryResumableStore{maxPerStream: maxPerStream, maxBytes: maxBytes, streams: map[memKey]*memStream{}}
}

func (s *MemoryResumableStore) wake(st *memStream) {
	close(st.notify)
	st.notify = make(chan struct{})
}

// Open starts (or restarts, for the standalone stream "") a stream.
func (s *MemoryResumableStore) Open(_ context.Context, sessionID, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memKey{sessionID, streamID}
	if old, ok := s.streams[k]; ok {
		s.nBytes -= old.bytes
		old.purged = true
		s.wake(old)
	}
	s.seq++
	s.streams[k] = &memStream{notify: make(chan struct{}), touched: s.seq}
	return nil
}

func (s *MemoryResumableStore) Append(_ context.Context, sessionID, streamID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.streams[memKey{sessionID, streamID}]
	if !ok {
		return nil // purged by a close: the session is gone, nothing to buffer
	}
	st.events = append(st.events, append([]byte(nil), data...))
	st.bytes += len(data)
	s.nBytes += len(data)
	s.seq++
	st.touched = s.seq
	for len(st.events) > s.maxPerStream {
		s.drop(st)
	}
	for s.nBytes > s.maxBytes {
		if !s.evictOldest(st) {
			break
		}
	}
	s.wake(st)
	return nil
}

func (s *MemoryResumableStore) drop(st *memStream) {
	st.bytes -= len(st.events[0])
	s.nBytes -= len(st.events[0])
	st.events[0] = nil
	st.events = st.events[1:]
	st.first++
}

// evictOldest drops the oldest event of the least recently used stream other
// than keep; false when nothing else is left to evict.
func (s *MemoryResumableStore) evictOldest(keep *memStream) bool {
	var victim *memStream
	for _, st := range s.streams {
		if st != keep && len(st.events) > 0 && (victim == nil || st.touched < victim.touched) {
			victim = st
		}
	}
	if victim == nil {
		if len(keep.events) > 1 {
			s.drop(keep)
			return true
		}
		return false
	}
	s.drop(victim)
	return true
}

func (s *MemoryResumableStore) After(_ context.Context, sessionID, streamID string, index int) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		s.mu.Lock()
		st, ok := s.streams[memKey{sessionID, streamID}]
		if !ok {
			s.mu.Unlock()
			yield(nil, ErrResumeGap)
			return
		}
		if index+1 < st.first {
			s.mu.Unlock()
			yield(nil, ErrResumeGap)
			return
		}
		var out [][]byte
		if off := index + 1 - st.first; off < len(st.events) {
			out = append(out, st.events[off:]...)
		}
		s.mu.Unlock()
		for _, d := range out {
			if !yield(d, nil) {
				return
			}
		}
	}
}

// SessionClosed is called by the SDK when ONE connection closes. With sessions
// shared across replicas that is not the end of the session, so it is a no-op;
// the host calls Purge on authoritative closes.
func (s *MemoryResumableStore) SessionClosed(context.Context, string) error { return nil }

func (s *MemoryResumableStore) Purge(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, st := range s.streams {
		if k.session == sessionID {
			s.nBytes -= st.bytes
			st.purged = true
			s.wake(st)
			delete(s.streams, k)
		}
	}
	return nil
}

func (s *MemoryResumableStore) Check(_ context.Context, sessionID, streamID string, afterIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.streams[memKey{sessionID, streamID}]
	if !ok || afterIdx+1 < st.first {
		return ErrResumeGap
	}
	return nil
}

func (s *MemoryResumableStore) Follow(ctx context.Context, sessionID, streamID string, afterIdx int, fn func(int, []byte) (bool, error)) error {
	next := afterIdx + 1
	first := true
	for {
		s.mu.Lock()
		st, ok := s.streams[memKey{sessionID, streamID}]
		if !ok || st.purged {
			s.mu.Unlock()
			if first {
				return ErrResumeGap
			}
			return nil // purged while following: the session was closed
		}
		if next < st.first {
			s.mu.Unlock()
			return ErrResumeGap
		}
		var batch [][]byte
		if off := next - st.first; off < len(st.events) {
			batch = append(batch, st.events[off:]...)
		}
		wait := st.notify
		s.mu.Unlock()
		first = false
		for _, d := range batch {
			stop, err := fn(next, d)
			next++
			if err != nil || stop {
				return err
			}
		}
		if len(batch) > 0 {
			continue
		}
		select {
		case <-wait:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

var _ ResumableEventStore = (*MemoryResumableStore)(nil)
