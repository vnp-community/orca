package usecase

import (
	"context"
	"errors"
	"sync"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// baseRefStore is an in-memory Request store for the base ref use cases. A transaction holds txMu for its whole run, which is
// what FOR UPDATE does to the row: a second transaction waits, and a failed one is rolled back.
type baseRefStore struct {
	mu       sync.Mutex
	txMu     sync.Mutex
	requests map[string]domain.Request
	events   []domain.OutboxEvent
	failSet  error
	failEv   error
	txOpened int
	setCalls int
}

type baseRefTxKey struct{}

func newBaseRefStore() *baseRefStore { return &baseRefStore{requests: map[string]domain.Request{}} }

func (s *baseRefStore) put(r domain.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[r.ID] = r
}

func (s *baseRefStore) read(id string) (domain.Request, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	return r, ok
}

func (s *baseRefStore) InTx(ctx context.Context, fn func(context.Context) error) error {
	if ctx.Value(baseRefTxKey{}) != nil {
		return fn(ctx)
	}
	s.txMu.Lock()
	defer s.txMu.Unlock()
	s.mu.Lock()
	s.txOpened++
	snapshot := make(map[string]domain.Request, len(s.requests))
	for k, v := range s.requests {
		snapshot[k] = v
	}
	nEvents := len(s.events)
	s.mu.Unlock()
	if err := fn(context.WithValue(ctx, baseRefTxKey{}, true)); err != nil {
		s.mu.Lock()
		s.requests, s.events = snapshot, s.events[:nEvents]
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *baseRefStore) Get(_ context.Context, id string) (domain.Request, error) {
	if r, ok := s.read(id); ok {
		return r, nil
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

func (s *baseRefStore) LockRequest(ctx context.Context, id string) (domain.Request, error) {
	if ctx.Value(baseRefTxKey{}) == nil {
		return domain.Request{}, errors.New("LockRequest outside a transaction")
	}
	return s.Get(ctx, id)
}

func (s *baseRefStore) SetBaseRef(_ context.Context, id, ref string, expected int64) (domain.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setCalls++
	if s.failSet != nil {
		return domain.Request{}, s.failSet
	}
	r, ok := s.requests[id]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	if r.Version != expected {
		return domain.Request{}, domain.ErrRequestVersionConflict(id, expected)
	}
	r.BaseRef, r.Version = ref, r.Version+1
	s.requests[id] = r
	return r, nil
}

func (s *baseRefStore) InsertOutboxEvent(_ context.Context, ev domain.OutboxEvent) error {
	if s.failEv != nil {
		return s.failEv
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	return nil
}

// The RequestRepository methods the base ref use cases never call.
func (s *baseRefStore) Create(context.Context, domain.Request) error { return errors.New("unused") }
func (s *baseRefStore) GetByNumber(context.Context, int64) (domain.Request, error) {
	return domain.Request{}, errors.New("unused")
}
func (s *baseRefStore) List(context.Context, ListFilter) (ListResult, error) {
	return ListResult{}, errors.New("unused")
}
func (s *baseRefStore) Update(context.Context, domain.Request, int64) (domain.Request, error) {
	return domain.Request{}, errors.New("unused")
}
func (s *baseRefStore) UpdateSolutionEngine(context.Context, string, domain.EngineName, int64) error {
	return errors.New("unused")
}
func (s *baseRefStore) PinSolutionEngineIfUnset(context.Context, string, domain.EngineName) (domain.EngineName, error) {
	return "", errors.New("unused")
}
func (s *baseRefStore) NextNumber(context.Context) (int64, error) { return 0, errors.New("unused") }

// fakePresence answers from a map and counts calls, so tests can assert one lookup per page.
type fakePresence struct {
	mu        sync.Mutex
	locks     map[string]domain.BaseRefLockReason
	err       error
	hasCalls  int
	manyCalls int
	manyIDs   [][]string
}

func newFakePresence() *fakePresence {
	return &fakePresence{locks: map[string]domain.BaseRefLockReason{}}
}

func (p *fakePresence) set(id string, reason domain.BaseRefLockReason) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.locks[id] = reason
}

func (p *fakePresence) Has(_ context.Context, id string) (bool, domain.BaseRefLockReason, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hasCalls++
	if p.err != nil {
		return false, domain.BaseRefUnlocked, p.err
	}
	reason := p.locks[id]
	return reason != domain.BaseRefUnlocked, reason, nil
}

func (p *fakePresence) HasMany(_ context.Context, ids []string) (map[string]domain.BaseRefLockReason, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.manyCalls++
	p.manyIDs = append(p.manyIDs, append([]string(nil), ids...))
	if p.err != nil {
		return nil, p.err
	}
	out := map[string]domain.BaseRefLockReason{}
	for _, id := range ids {
		if reason := p.locks[id]; reason != domain.BaseRefUnlocked {
			out[id] = reason
		}
	}
	return out, nil
}

// fakeRefChecker returns a canned answer and records what it was asked.
type fakeRefChecker struct {
	mu      sync.Mutex
	check   BaseRefCheck
	err     error
	calls   int
	project string
	ref     string
}

func (c *fakeRefChecker) Check(_ context.Context, projectID, ref string) (BaseRefCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.project, c.ref = projectID, ref
	if c.check.Status == "" && c.err == nil {
		return BaseRefCheck{Status: BaseRefFound}, nil
	}
	return c.check, c.err
}
