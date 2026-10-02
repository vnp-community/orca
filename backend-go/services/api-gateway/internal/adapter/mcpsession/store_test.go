package mcpsession

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// fakeLog is an in-memory eventbus.BoundedLog stand-in with a per-subject bound.
type fakeLog struct {
	mu      sync.Mutex
	max     int
	subject map[string][]eventbus.LogEntry
	subs    []subjectSub
}
type subjectSub struct {
	subj string
	ch   chan eventbus.LogEntry
}

func newFakeLog(max int) *fakeLog {
	return &fakeLog{max: max, subject: map[string][]eventbus.LogEntry{}}
}

func (f *fakeLog) Append(_ context.Context, subj string, h map[string]string, d []byte) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := eventbus.LogEntry{Subject: subj, Header: h, Data: d}
	l := append(f.subject[subj], e)
	if len(l) > f.max {
		l = l[1:]
	}
	f.subject[subj] = l
	for _, s := range f.subs {
		if s.subj == subj {
			s.ch <- e
		}
	}
	return 1, nil
}
func (f *fakeLog) First(_ context.Context, subj string) (eventbus.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l := f.subject[subj]; len(l) > 0 {
		return l[0], nil
	}
	return eventbus.LogEntry{}, eventbus.ErrLogEmpty
}
func (f *fakeLog) Snapshot(_ context.Context, subj string) ([]eventbus.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]eventbus.LogEntry(nil), f.subject[subj]...), nil
}
func (f *fakeLog) Follow(ctx context.Context, subj string, fn func(eventbus.LogEntry) (bool, error)) error {
	ch := make(chan eventbus.LogEntry, 64)
	f.mu.Lock()
	for _, e := range f.subject[subj] {
		ch <- e
	}
	f.subs = append(f.subs, subjectSub{subj, ch})
	f.mu.Unlock()
	for {
		select {
		case e := <-ch:
			if stop, err := fn(e); err != nil || stop {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (f *fakeLog) Purge(_ context.Context, filter string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := strings.TrimSuffix(filter, ">")
	for k := range f.subject {
		if k == filter || (strings.HasSuffix(filter, ">") && strings.HasPrefix(k, prefix)) {
			delete(f.subject, k)
		}
	}
	return nil
}

func TestSubjectsNeverContainTheSessionSecret(t *testing.T) {
	const secret = "SECRET-SESSION-ID-0123456789"
	for _, stream := range []string{"", "STREAMID"} {
		subj := subjectFor(secret, stream)
		if strings.Contains(subj, secret) || !strings.HasPrefix(subj, "orca.sse.mcp.") || strings.Count(subj, ".") != 4 {
			t.Errorf("subject %q", subj)
		}
	}
	if subjectFor("a", "S1") == subjectFor("b", "S1") {
		t.Error("sessions must not share subjects")
	}
}

func TestJetStreamEventStoreOrdinalsGapAndFollow(t *testing.T) {
	log := newFakeLog(3) // bounded: keeps the newest 3 per subject
	s := NewJetStreamEventStore(log, 0)
	ctx := context.Background()
	if err := s.Open(ctx, "sess", "ST"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{`{"n":0}`, `{"n":1}`, `{"n":2}`, `{"n":3}`, `{"n":4}`} {
		if err := s.Append(ctx, "sess", "ST", []byte(d)); err != nil {
			t.Fatal(err)
		}
	}
	// Ordinals survive the bound: retained are 2,3,4.
	if err := s.Check(ctx, "sess", "ST", 1); err != nil {
		t.Errorf("resume from 1 needs 2..4 which are retained: %v", err)
	}
	if err := s.Check(ctx, "sess", "ST", 0); !errors.Is(err, mcpserver.ErrResumeGap) {
		t.Errorf("resume from 0 needs the dropped event 1: %v", err)
	}
	var got []int
	err := s.Follow(ctx, "sess", "ST", 2, func(i int, d []byte) (bool, error) {
		got = append(got, i)
		return i == 4, nil
	})
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("follow after 2: %v %v", got, err)
	}
	// Unknown stream with an index to resume from is a gap; a brand-new one is fine.
	if err := s.Check(ctx, "sess", "NOPE", 3); !errors.Is(err, mcpserver.ErrResumeGap) {
		t.Errorf("unknown stream: %v", err)
	}
	if err := s.Check(ctx, "sess", "NOPE", -1); err != nil {
		t.Errorf("fresh stream: %v", err)
	}
	// SessionClosed (per connection) must NOT wipe the shared buffer; Purge does.
	_ = s.SessionClosed(ctx, "sess")
	if err := s.Check(ctx, "sess", "ST", 3); err != nil {
		t.Errorf("SessionClosed wiped the buffer: %v", err)
	}
	if err := s.Purge(ctx, "sess"); err != nil {
		t.Fatal(err)
	}
	if err := s.Check(ctx, "sess", "ST", 3); !errors.Is(err, mcpserver.ErrResumeGap) {
		t.Errorf("after purge: %v", err)
	}
}

func TestOversizedEventIsReplacedByAnError(t *testing.T) {
	log := newFakeLog(10)
	s := NewJetStreamEventStore(log, 64)
	ctx := context.Background()
	_ = s.Open(ctx, "x", "S")
	big := `{"jsonrpc":"2.0","id":9,"result":{"pad":"` + strings.Repeat("a", 500) + `"}}`
	if err := s.Append(ctx, "x", "S", []byte(big)); err != nil {
		t.Fatal(err)
	}
	e, _ := log.First(ctx, subjectFor("x", "S"))
	if len(e.Data) > 200 || !strings.Contains(string(e.Data), `"id":9`) || !strings.Contains(string(e.Data), "too large") {
		t.Errorf("stored: %s", e.Data)
	}
}

func TestMapErr(t *testing.T) {
	if !errors.Is(mapErr(status.Error(codes.NotFound, "MCP_NOT_FOUND: not found")), mcpserver.ErrSessionNotFound) {
		t.Error("NotFound")
	}
	if !errors.Is(mapErr(status.Error(codes.FailedPrecondition, "MCP_STREAM_LIMIT: too many open streams")), mcpserver.ErrStreamLimit) {
		t.Error("stream limit")
	}
	if mapErr(nil) != nil {
		t.Error("nil must stay nil")
	}
	if e := mapErr(status.Error(codes.Unavailable, "down")); errors.Is(e, mcpserver.ErrSessionNotFound) {
		t.Error("outage must not look like not-found")
	}
}
