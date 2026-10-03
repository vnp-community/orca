package mcpsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"iter"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

const (
	// StreamName / SubjectPrefix: the bounded resume buffer. The subject root
	// deliberately avoids "orca.mcp.>" (owned by the MCP domain stream; JetStream
	// forbids two streams on overlapping subjects).
	StreamName    = "MCPSSE"
	SubjectPrefix = "orca.sse.mcp"
	idxHeader     = "Orca-Idx"
	appendTimeout = 5 * time.Second

	DefaultMaxMsgsPerStream = 256
	DefaultMaxAge           = 10 * time.Minute
	DefaultMaxBytes         = 256 << 20
	DefaultMaxEventBytes    = 256 << 10
)

// Log is the part of eventbus.BoundedLog the store uses (fakeable).
type Log interface {
	Append(ctx context.Context, subject string, header map[string]string, data []byte) (uint64, error)
	// AppendAfter / Last are the compare-and-set that keeps ordinals unique when
	// several replicas append to one subject.
	AppendAfter(ctx context.Context, subject string, lastSeq uint64, header map[string]string, data []byte) (uint64, error)
	Last(ctx context.Context, subject string) (eventbus.LogEntry, error)
	First(ctx context.Context, subject string) (eventbus.LogEntry, error)
	Follow(ctx context.Context, subject string, fn func(eventbus.LogEntry) (bool, error)) error
	Snapshot(ctx context.Context, subject string) ([]eventbus.LogEntry, error)
	Purge(ctx context.Context, filter string) error
}

// NewLog opens the MCPSSE stream with hard bounds: per-subject message count,
// age and total bytes, Discard=old. A full buffer loses its oldest events and a
// resume that needs them is answered 404 - it never grows without limit.
func NewLog(ctx context.Context, natsURL string, maxBytes int64) (*eventbus.BoundedLog, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return eventbus.NewBoundedLog(ctx, natsURL, eventbus.BoundedLogConfig{
		Stream: StreamName, Subjects: []string{SubjectPrefix + ".>"},
		MaxMsgsPerSubject: DefaultMaxMsgsPerStream, MaxAge: DefaultMaxAge, MaxBytes: maxBytes,
	})
}

// JetStreamEventStore is the cross-replica ResumableEventStore. Event ordinals
// (the "<stream>_<n>" ids of the SDK) are written as a header, so they stay
// valid after the bounded stream drops older messages.
//
// Several replicas can append to one subject (every replica that holds a
// session writes that session's standalone stream). Each append therefore
// derives its ordinal from the subject's newest message and is stored with a
// compare-and-set on that message's sequence: ordinals are unique and
// increasing per stream key whoever writes, and a lost race re-reads and retries.
// A replica that is the only writer (every request stream, and the usual
// standalone stream) never conflicts and pays no extra round trip.
//
// The Mcp-Session-Id is a bearer secret, so subjects use a derived key.
type JetStreamEventStore struct {
	log      Log
	maxEvent int

	mu    sync.Mutex
	tails map[string]tail        // subject -> newest message this replica knows of
	locks map[string]*sync.Mutex // subject -> serializes this replica's own appends
}

// tail is the newest message of a subject: its stream sequence (0 = none) and ordinal.
type tail struct {
	seq uint64
	idx int
}

// maxAppendAttempts bounds CAS retries. A writer that has to re-read the tail
// (one round trip more) can lose to a replica that is appending in a tight
// loop, so retries back off with jitter to let the burst end.
const (
	maxAppendAttempts = 24
	maxAppendBackoff  = 40 * time.Millisecond
)

func NewJetStreamEventStore(l Log, maxEventBytes int) *JetStreamEventStore {
	if maxEventBytes <= 0 {
		maxEventBytes = DefaultMaxEventBytes
	}
	return &JetStreamEventStore{log: l, maxEvent: maxEventBytes, tails: map[string]tail{}, locks: map[string]*sync.Mutex{}}
}

func sessionKey(secret string) string {
	h := sha256.Sum256([]byte("orca-sse:" + secret))
	return hex.EncodeToString(h[:16])
}

func subjectFor(secret, streamID string) string {
	tok := "s" // standalone stream
	if streamID != "" {
		tok = "p" + streamID
	}
	return SubjectPrefix + "." + sessionKey(secret) + "." + tok
}

func (s *JetStreamEventStore) Open(ctx context.Context, sessionID, streamID string) error {
	subj := subjectFor(sessionID, streamID)
	if streamID == "" { // a new standalone stream supersedes the previous generation
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appendTimeout)
		defer cancel()
		if err := s.log.Purge(pctx, subj); err != nil {
			return err
		}
	}
	s.forget(subj)
	return nil
}

// forget drops what this replica remembers of a subject (it was purged or ended).
func (s *JetStreamEventStore) forget(subj string) {
	s.mu.Lock()
	delete(s.tails, subj)
	delete(s.locks, subj)
	s.mu.Unlock()
}

func (s *JetStreamEventStore) lockFor(subj string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.locks[subj]
	if l == nil {
		l = &sync.Mutex{}
		s.locks[subj] = l
	}
	return l
}

// tailOf returns the cached tail or reads it from the log.
func (s *JetStreamEventStore) tailOf(ctx context.Context, subj string) (tail, error) {
	s.mu.Lock()
	t, ok := s.tails[subj]
	s.mu.Unlock()
	if ok {
		return t, nil
	}
	e, err := s.log.Last(ctx, subj)
	switch {
	case errors.Is(err, eventbus.ErrLogEmpty):
		t = tail{seq: 0, idx: -1}
	case err != nil:
		return tail{}, err
	default:
		i, ok := entryIdx(e)
		if !ok { // not one of ours: continue after it without reusing an ordinal
			i = -1
		}
		t = tail{seq: e.Seq, idx: i}
	}
	return t, nil
}

func (s *JetStreamEventStore) Append(ctx context.Context, sessionID, streamID string, data []byte) error {
	subj := subjectFor(sessionID, streamID)
	if len(data) > s.maxEvent {
		data = tooLarge(data)
	}
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appendTimeout) // the request may be gone already
	defer cancel()
	lk := s.lockFor(subj)
	lk.Lock()
	defer lk.Unlock()
	for attempt := 0; attempt < maxAppendAttempts; attempt++ {
		t, err := s.tailOf(actx, subj)
		if err != nil {
			return err
		}
		idx := t.idx + 1
		seq, err := s.log.AppendAfter(actx, subj, t.seq, map[string]string{idxHeader: strconv.Itoa(idx)}, data)
		if errors.Is(err, eventbus.ErrLogConflict) {
			s.mu.Lock()
			delete(s.tails, subj) // another replica appended: re-read the tail
			s.mu.Unlock()
			if !sleepBackoff(actx, attempt) {
				return actx.Err()
			}
			continue
		}
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.tails[subj] = tail{seq: seq, idx: idx}
		s.mu.Unlock()
		if streamID != "" && isResponse(data) {
			s.forget(subj)
		}
		return nil
	}
	return errAppendContended
}

// sleepBackoff waits a random slice of an exponentially growing window.
func sleepBackoff(ctx context.Context, attempt int) bool {
	window := time.Millisecond << min(attempt, 6)
	if window > maxAppendBackoff {
		window = maxAppendBackoff
	}
	t := time.NewTimer(time.Duration(rand.Int64N(int64(window)) + 1))
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

var errAppendContended = errors.New("mcpsession: gave up appending an event after repeated conflicts")

func isResponse(data []byte) bool {
	var m struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	return json.Unmarshal(data, &m) == nil && m.Method == "" && len(m.ID) > 0
}

// tooLarge keeps replay memory bounded: the oversized message is replaced by a
// JSON-RPC error for the same id (the live client already got the real one).
func tooLarge(data []byte) []byte {
	var m struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(data, &m)
	if len(m.ID) == 0 {
		m.ID = json.RawMessage("null")
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": m.ID,
		"error": map[string]any{"code": -32603, "message": "event too large to replay"}})
	return b
}

func entryIdx(e eventbus.LogEntry) (int, bool) {
	n, err := strconv.Atoi(e.Header[idxHeader])
	return n, err == nil
}

// After is the SDK's snapshot read; the gateway resumes through Follow, so this
// only serves callers that use the store as a plain mcp.EventStore.
func (s *JetStreamEventStore) After(ctx context.Context, sessionID, streamID string, index int) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		if err := s.Check(ctx, sessionID, streamID, index); err != nil {
			yield(nil, err)
			return
		}
		entries, err := s.log.Snapshot(ctx, subjectFor(sessionID, streamID))
		if err != nil {
			yield(nil, err)
			return
		}
		for _, e := range entries {
			if i, ok := entryIdx(e); ok && i > index && !yield(e.Data, nil) {
				return
			}
		}
	}
}

func (s *JetStreamEventStore) Check(ctx context.Context, sessionID, streamID string, afterIdx int) error {
	first, err := s.log.First(ctx, subjectFor(sessionID, streamID))
	if errors.Is(err, eventbus.ErrLogEmpty) {
		if afterIdx < 0 {
			return nil // nothing happened on this stream yet
		}
		return mcpserver.ErrResumeGap // everything expired or was purged
	}
	if err != nil {
		return err
	}
	if i, ok := entryIdx(first); !ok || i > afterIdx+1 {
		return mcpserver.ErrResumeGap
	}
	return nil
}

func (s *JetStreamEventStore) Follow(ctx context.Context, sessionID, streamID string, afterIdx int, fn func(int, []byte) (bool, error)) error {
	if err := s.Check(ctx, sessionID, streamID, afterIdx); err != nil {
		return err
	}
	return s.log.Follow(ctx, subjectFor(sessionID, streamID), func(e eventbus.LogEntry) (bool, error) {
		i, ok := entryIdx(e)
		if !ok || i <= afterIdx {
			return false, nil
		}
		return fn(i, e.Data)
	})
}

// SessionClosed is per-connection in the SDK and must not wipe a buffer that
// other replicas still need: authoritative closes call Purge.
func (s *JetStreamEventStore) SessionClosed(context.Context, string) error { return nil }

func (s *JetStreamEventStore) Purge(ctx context.Context, sessionID string) error {
	prefix := SubjectPrefix + "." + sessionKey(sessionID) + "."
	s.mu.Lock()
	for k := range s.tails {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.tails, k)
			delete(s.locks, k)
		}
	}
	s.mu.Unlock()
	return s.log.Purge(ctx, prefix+">")
}

var _ mcpserver.ResumableEventStore = (*JetStreamEventStore)(nil)
