package mcpsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"iter"
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
// The Mcp-Session-Id is a bearer secret, so subjects use a derived key.
type JetStreamEventStore struct {
	log      Log
	maxEvent int

	mu   sync.Mutex
	next map[string]int // subject -> next ordinal; only the replica writing a stream appends to it
}

func NewJetStreamEventStore(l Log, maxEventBytes int) *JetStreamEventStore {
	if maxEventBytes <= 0 {
		maxEventBytes = DefaultMaxEventBytes
	}
	return &JetStreamEventStore{log: l, maxEvent: maxEventBytes, next: map[string]int{}}
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
	s.mu.Lock()
	s.next[subj] = 0
	s.mu.Unlock()
	return nil
}

func (s *JetStreamEventStore) Append(ctx context.Context, sessionID, streamID string, data []byte) error {
	subj := subjectFor(sessionID, streamID)
	s.mu.Lock()
	idx := s.next[subj]
	s.next[subj] = idx + 1
	s.mu.Unlock()
	if len(data) > s.maxEvent {
		data = tooLarge(data)
	}
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appendTimeout) // the request may be gone already
	defer cancel()
	if _, err := s.log.Append(actx, subj, map[string]string{idxHeader: strconv.Itoa(idx)}, data); err != nil {
		return err
	}
	if streamID != "" && isResponse(data) {
		s.mu.Lock()
		delete(s.next, subj)
		s.mu.Unlock()
	}
	return nil
}

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
	for k := range s.next {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.next, k)
		}
	}
	s.mu.Unlock()
	return s.log.Purge(ctx, prefix+">")
}

var _ mcpserver.ResumableEventStore = (*JetStreamEventStore)(nil)
