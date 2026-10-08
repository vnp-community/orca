package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// lcStore is an in-memory request store with CAS and transaction rollback, shared by the lifecycle fakes.
type lcStore struct {
	mu          sync.Mutex
	requests    map[string]domain.Request
	events      []domain.OutboxEvent
	history     []domain.ReturnHistoryEntry
	links       []domain.RequestLink
	claims      map[string]string
	counter     int64
	conflicts   int // Update fails with a version conflict this many times
	updateCalls int
	failOutbox  error
	depth       int
}

func newLcStore() *lcStore {
	return &lcStore{requests: map[string]domain.Request{}, claims: map[string]string{}}
}

func (s *lcStore) seed(mod func(r *domain.Request)) domain.Request {
	r := domain.Request{
		ID: uuid.NewString(), TenantID: "t1", ProjectID: uuid.NewString(), Number: s.nextNum(), Title: "t", SourceProvider: domain.SourceProviderManual,
		Status: domain.RequestStatusNew, Urgency: domain.UrgencyNormal, ReporterID: uuid.NewString(), Version: 1,
	}
	if mod != nil {
		mod(&r)
	}
	s.requests[r.ID] = r
	return r
}

func (s *lcStore) nextNum() int64 { s.counter++; return s.counter }

type lcSnapshot struct {
	requests map[string]domain.Request
	events   int
	history  int
	links    int
	claims   map[string]string
	counter  int64
}

func (s *lcStore) snapshot() lcSnapshot {
	cp := map[string]domain.Request{}
	for k, v := range s.requests {
		cp[k] = v
	}
	cl := map[string]string{}
	for k, v := range s.claims {
		cl[k] = v
	}
	return lcSnapshot{cp, len(s.events), len(s.history), len(s.links), cl, s.counter}
}

func (s *lcStore) restore(sn lcSnapshot) {
	s.requests, s.claims, s.counter = sn.requests, sn.claims, sn.counter
	s.events, s.history, s.links = s.events[:sn.events], s.history[:sn.history], s.links[:sn.links]
}

type lcCtxKey struct{}

func (s *lcStore) InTransaction(ctx context.Context) bool { return ctx.Value(lcCtxKey{}) != nil }

func (s *lcStore) InTx(ctx context.Context, fn func(context.Context) error) error {
	if s.InTransaction(ctx) {
		return fn(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sn := s.snapshot()
	s.depth++
	err := fn(context.WithValue(ctx, lcCtxKey{}, true))
	if err != nil {
		s.restore(sn)
	}
	return err
}

func (s *lcStore) Create(_ context.Context, r domain.Request) error { s.requests[r.ID] = r; return nil }
func (s *lcStore) Get(_ context.Context, id string) (domain.Request, error) {
	r, ok := s.requests[id]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return r, nil
}
func (s *lcStore) GetByNumber(context.Context, int64) (domain.Request, error) {
	return domain.Request{}, errors.New("unused")
}
func (s *lcStore) List(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, errors.New("unused")
}
func (s *lcStore) Update(_ context.Context, r domain.Request, expected int64) (domain.Request, error) {
	s.updateCalls++
	if s.conflicts > 0 {
		s.conflicts--
		return domain.Request{}, domain.ErrRequestVersionConflict(r.ID, expected)
	}
	cur, ok := s.requests[r.ID]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(r.ID)
	}
	if cur.Version != expected {
		return domain.Request{}, domain.ErrRequestVersionConflict(r.ID, expected)
	}
	r.Version = expected + 1
	s.requests[r.ID] = r
	return r, nil
}
func (s *lcStore) UpdateSolutionEngine(context.Context, string, domain.EngineName, int64) error {
	return errors.New("unused")
}
func (s *lcStore) NextNumber(context.Context) (int64, error) { return s.nextNum(), nil }

func (s *lcStore) InsertOutboxEvent(_ context.Context, ev domain.OutboxEvent) error {
	if s.failOutbox != nil {
		return s.failOutbox
	}
	s.events = append(s.events, ev)
	return nil
}

func (s *lcStore) subjects() []string {
	var out []string
	for _, e := range s.events {
		out = append(out, e.Subject)
	}
	return out
}

// lcHistory implements ReturnHistoryRepository over the store.
type lcHistory struct{ s *lcStore }

func (h lcHistory) Append(_ context.Context, e domain.ReturnHistoryEntry) error {
	h.s.history = append(h.s.history, e)
	return nil
}
func (h lcHistory) List(_ context.Context, id string) ([]domain.ReturnHistoryEntry, error) {
	var out []domain.ReturnHistoryEntry
	for _, e := range h.s.history {
		if e.RequestID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

// lcLinks implements RequestLinkRepository over the store.
type lcLinks struct{ s *lcStore }

func (l lcLinks) Insert(_ context.Context, link domain.RequestLink) error {
	l.s.links = append(l.s.links, link)
	return nil
}
func (l lcLinks) ListParents(_ context.Context, child string) ([]domain.RequestLink, error) {
	var out []domain.RequestLink
	for _, x := range l.s.links {
		if x.ChildRequestID == child {
			out = append(out, x)
		}
	}
	return out, nil
}
func (l lcLinks) ListChildren(_ context.Context, parent string) ([]domain.RequestLink, error) {
	var out []domain.RequestLink
	for _, x := range l.s.links {
		if x.ParentRequestID == parent {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChildRequestID < out[j].ChildRequestID })
	return out, nil
}
func (l lcLinks) Delete(context.Context, string, string) error { return errors.New("unused") }

// lcIdempotency implements RequestIdempotencyRepository over the store.
type lcIdempotency struct{ s *lcStore }

func (i lcIdempotency) Claim(_ context.Context, provider, site, ref, requestID string) (string, bool, error) {
	k := provider + "|" + site + "|" + ref
	if ex, ok := i.s.claims[k]; ok {
		return ex, false, nil
	}
	i.s.claims[k] = requestID
	return "", true, nil
}
func (i lcIdempotency) Find(context.Context, string, string, string) (string, error) {
	return "", errors.New("unused")
}

type lcGuard struct{ active bool }

func (g lcGuard) HasActiveExecution(context.Context, string) (bool, error) { return g.active, nil }

type lcCanceller struct{ calls []string }

func (c *lcCanceller) CancelPending(_ context.Context, id, why string) error {
	c.calls = append(c.calls, id+":"+why)
	return nil
}

// lcSpyTransitioner records inputs and delegates to the real use case.
type lcSpyTransitioner struct {
	inner *TransitionRequest
	seen  []TransitionInput
}

func (t *lcSpyTransitioner) Execute(ctx context.Context, in TransitionInput) (TransitionResult, error) {
	t.seen = append(t.seen, in)
	return t.inner.Execute(ctx, in)
}

func lcCtx() context.Context { return tenant.WithTenantID(context.Background(), "t1") }

func lcStatus(s domain.RequestStatus) *domain.RequestStatus { return &s }

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

var errBoom = errors.New("boom")

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
