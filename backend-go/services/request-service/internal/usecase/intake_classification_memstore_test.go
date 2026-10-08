package usecase

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memStore is an in-memory stand-in for every port the intake and classification use cases touch.
// InTx holds one lock and restores a snapshot on error, which is what the real adapters guarantee.
type memStore struct {
	mu          sync.Mutex
	requests    map[string]domain.Request
	counter     int64
	idem        map[domain.IdempotencyKey]string
	history     []domain.RequestTypeChange
	events      []domain.OutboxEvent
	processed   map[string]bool
	runs        map[string]domain.ClassificationRun
	failOutbox  func(subject string) error
	failCreate  error
	updateCalls int
}

type memTxKey struct{}

func newMemStore() *memStore {
	return &memStore{requests: map[string]domain.Request{}, idem: map[domain.IdempotencyKey]string{}, processed: map[string]bool{}, runs: map[string]domain.ClassificationRun{}}
}

func (s *memStore) lock(ctx context.Context) func() {
	if ctx.Value(memTxKey{}) != nil {
		return func() {}
	}
	s.mu.Lock()
	return s.mu.Unlock
}

func (s *memStore) InTx(ctx context.Context, fn func(context.Context) error) error {
	if ctx.Value(memTxKey{}) != nil {
		return fn(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	reqs := map[string]domain.Request{}
	for k, v := range s.requests {
		reqs[k] = v
	}
	idem := map[domain.IdempotencyKey]string{}
	for k, v := range s.idem {
		idem[k] = v
	}
	proc := map[string]bool{}
	for k, v := range s.processed {
		proc[k] = v
	}
	runs := map[string]domain.ClassificationRun{}
	for k, v := range s.runs {
		runs[k] = v
	}
	counter, hist, events := s.counter, append([]domain.RequestTypeChange(nil), s.history...), append([]domain.OutboxEvent(nil), s.events...)
	if err := fn(context.WithValue(ctx, memTxKey{}, true)); err != nil {
		s.requests, s.idem, s.processed, s.runs, s.counter, s.history, s.events = reqs, idem, proc, runs, counter, hist, events
		return err
	}
	return nil
}

func (s *memStore) InTransaction(ctx context.Context) bool { return ctx.Value(memTxKey{}) != nil }

func (s *memStore) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	defer s.lock(ctx)()
	if s.failOutbox != nil {
		if err := s.failOutbox(ev.Subject); err != nil {
			return err
		}
	}
	s.events = append(s.events, ev)
	return nil
}

func (s *memStore) subjects() []string {
	var out []string
	for _, e := range s.events {
		out = append(out, e.Subject)
	}
	return out
}

func (s *memStore) Create(ctx context.Context, r domain.Request) error {
	defer s.lock(ctx)()
	if s.failCreate != nil {
		return s.failCreate
	}
	s.requests[r.ID] = r
	return nil
}

func (s *memStore) Get(ctx context.Context, id string) (domain.Request, error) {
	defer s.lock(ctx)()
	r, ok := s.requests[id]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return r, nil
}

func (s *memStore) GetByNumber(ctx context.Context, n int64) (domain.Request, error) {
	defer s.lock(ctx)()
	for _, r := range s.requests {
		if r.Number == n {
			return r, nil
		}
	}
	return domain.Request{}, domain.ErrRequestNotFound("n")
}

func (s *memStore) List(context.Context, ListFilter) (ListResult, error) { return ListResult{}, nil }

func (s *memStore) Update(ctx context.Context, r domain.Request, expected int64) (domain.Request, error) {
	defer s.lock(ctx)()
	cur, ok := s.requests[r.ID]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(r.ID)
	}
	if cur.Version != expected {
		return domain.Request{}, domain.ErrRequestVersionConflict(r.ID, expected)
	}
	r.Version = expected + 1
	s.requests[r.ID] = r
	s.updateCalls++
	return r, nil
}

func (s *memStore) UpdateSolutionEngine(context.Context, string, domain.EngineName, int64) error {
	return nil
}

func (s *memStore) NextNumber(ctx context.Context) (int64, error) {
	defer s.lock(ctx)()
	s.counter++
	return s.counter, nil
}

func (s *memStore) Claim(ctx context.Context, p, site, ref, requestID string) (string, bool, error) {
	defer s.lock(ctx)()
	k := domain.IdempotencyKey{Provider: domain.SourceProvider(p), Site: site, Ref: ref}
	if id, ok := s.idem[k]; ok {
		return id, false, nil
	}
	s.idem[k] = requestID
	return "", true, nil
}

func (s *memStore) Find(ctx context.Context, p, site, ref string) (string, error) {
	defer s.lock(ctx)()
	return s.idem[domain.IdempotencyKey{Provider: domain.SourceProvider(p), Site: site, Ref: ref}], nil
}

func (s *memStore) Append(ctx context.Context, h domain.RequestTypeChange) error {
	defer s.lock(ctx)()
	if h.At.IsZero() {
		h.At = time.Now().UTC().Add(time.Duration(len(s.history)) * time.Microsecond)
	}
	s.history = append(s.history, h)
	return nil
}

func (s *memStore) List2(ctx context.Context, requestID string) []domain.RequestTypeChange {
	defer s.lock(ctx)()
	var out []domain.RequestTypeChange
	for _, h := range s.history {
		if h.RequestID == requestID {
			out = append(out, h)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// historyView adapts memStore to RequestTypeHistoryRepository (List clashes with RequestRepository.List).
type historyView struct{ s *memStore }

func (h historyView) Append(ctx context.Context, c domain.RequestTypeChange) error {
	return h.s.Append(ctx, c)
}
func (h historyView) List(ctx context.Context, id string) ([]domain.RequestTypeChange, error) {
	return h.s.List2(ctx, id), nil
}

func (s *memStore) MarkProcessed(ctx context.Context, eventID, _ string) (bool, error) {
	defer s.lock(ctx)()
	if s.processed[eventID] {
		return true, nil
	}
	s.processed[eventID] = true
	return false, nil
}

func (s *memStore) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

func (s *memStore) Start(ctx context.Context, run domain.ClassificationRun) (domain.ClassificationRun, bool, error) {
	defer s.lock(ctx)()
	for _, r := range s.runs {
		if (run.SourceEventID != "" && r.SourceEventID == run.SourceEventID) || (r.RequestID == run.RequestID && r.Status == domain.ClassificationRunRunning) {
			return r, false, nil
		}
	}
	s.runs[run.ID] = run
	return run, true, nil
}

func (s *memStore) GetRun(ctx context.Context, id string) (domain.ClassificationRun, error) {
	defer s.lock(ctx)()
	return s.runs[id], nil
}

func (s *memStore) Renew(ctx context.Context, id, owner string, until time.Time) (bool, error) {
	defer s.lock(ctx)()
	r, ok := s.runs[id]
	if !ok || r.Status != domain.ClassificationRunRunning || r.LeaseOwner != owner {
		return false, nil
	}
	r.LeaseExpiresAt = until
	s.runs[id] = r
	return true, nil
}

func (s *memStore) Finish(ctx context.Context, id string, st domain.ClassificationRunStatus, code string) error {
	defer s.lock(ctx)()
	r, ok := s.runs[id]
	if !ok || r.Status != domain.ClassificationRunRunning {
		return nil
	}
	r.Status, r.ErrorCode, r.LeaseOwner = st, code, ""
	s.runs[id] = r
	return nil
}

func (s *memStore) ClaimExpired(ctx context.Context, owner string, until time.Time, batch int) ([]domain.ClassificationRun, error) {
	defer s.lock(ctx)()
	var out []domain.ClassificationRun
	for id, r := range s.runs {
		if r.Status != domain.ClassificationRunRunning || !r.LeaseExpiresAt.Before(time.Now()) {
			continue
		}
		if r.Claims >= domain.MaxClassificationRunClaims {
			r.Status, r.ErrorCode = domain.ClassificationRunFailed, "LEASE_EXPIRED"
			s.runs[id] = r
			continue
		}
		r.Claims++
		r.LeaseOwner, r.LeaseExpiresAt = owner, until
		s.runs[id] = r
		out = append(out, r)
		if len(out) == batch {
			break
		}
	}
	return out, nil
}

// runView adapts memStore to ClassificationRunRepository (Get clashes with RequestRepository.Get).
type runView struct{ s *memStore }

func (v runView) Start(ctx context.Context, r domain.ClassificationRun) (domain.ClassificationRun, bool, error) {
	return v.s.Start(ctx, r)
}
func (v runView) Get(ctx context.Context, id string) (domain.ClassificationRun, error) {
	return v.s.GetRun(ctx, id)
}
func (v runView) Renew(ctx context.Context, id, o string, u time.Time) (bool, error) {
	return v.s.Renew(ctx, id, o, u)
}
func (v runView) Finish(ctx context.Context, id string, st domain.ClassificationRunStatus, c string) error {
	return v.s.Finish(ctx, id, st, c)
}
func (v runView) ClaimExpired(ctx context.Context, o string, u time.Time, b int) ([]domain.ClassificationRun, error) {
	return v.s.ClaimExpired(ctx, o, u, b)
}

// memTransitioner applies the transitions this feature triggers, with the semantics the real state machine specifies.
type memTransitioner struct {
	s    *memStore
	fail error
}

func (t *memTransitioner) Execute(ctx context.Context, in TransitionInput) (TransitionResult, error) {
	if t.fail != nil {
		return TransitionResult{}, t.fail
	}
	r, err := t.s.Get(ctx, in.RequestID)
	if err != nil {
		return TransitionResult{}, err
	}
	var to domain.RequestStatus
	switch in.Trigger {
	case domain.TriggerStartClassification:
		to = domain.RequestStatusClassifying
	case domain.TriggerProposalReady, domain.TriggerTypeChange:
		to = domain.RequestStatusAwaitingTypeConfirmation
	case domain.TriggerTypeConfirmed:
		to = domain.RequestStatusAnalyzing
		switch r.Type {
		case domain.RequestTypeTask, domain.RequestTypeDocs, domain.RequestTypeOpsRequest:
			to = domain.RequestStatusPlanning
		}
	}
	if in.ExpectedFrom != nil && r.Status != *in.ExpectedFrom {
		return TransitionResult{}, errors.New("state stale")
	}
	from := r.Status
	r.Status = to
	r, err = t.s.Update(ctx, r, r.Version)
	if err != nil {
		return TransitionResult{}, err
	}
	ev, _ := NewOutboxEvent(ctx, domain.SubjectRequestStatusChanged, map[string]any{"request_id": r.ID, "from": from, "to": to, "trigger": in.Trigger})
	if err := t.s.InsertOutboxEvent(ctx, ev); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Request: r, Applied: true}, nil
}

type fakeClassifier struct {
	mu    sync.Mutex
	calls int
	out   func(call int) (domain.ClassificationProposal, error)
}

func (f *fakeClassifier) Classify(context.Context, ClassificationInput) (domain.ClassificationProposal, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()
	return f.out(n)
}

func okProposal(t domain.RequestType) func(int) (domain.ClassificationProposal, error) {
	return func(int) (domain.ClassificationProposal, error) {
		return domain.ClassificationProposal{Type: t, Size: domain.RequestSizeM, Urgency: domain.UrgencyNormal, Confidence: 0.9, Reason: "because"}, nil
	}
}

func failProposal(err error) func(int) (domain.ClassificationProposal, error) {
	return func(int) (domain.ClassificationProposal, error) { return domain.ClassificationProposal{}, err }
}

type fakeIssues struct {
	mu    sync.Mutex // concurrent creates share one fake
	calls int
	snap  IssueSnapshot
	err   error
}

func (f *fakeIssues) GetIssue(context.Context, domain.SourceProvider, string, string) (IssueSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.snap, f.err
}

type recordingApprovals struct {
	opened, approved int
	cancelled        []string
	active           bool
}

func (a *recordingApprovals) RequestTypeApproval(context.Context, string) error {
	a.opened++
	return nil
}
func (a *recordingApprovals) Approve(context.Context, string, string) error { a.approved++; return nil }
func (a *recordingApprovals) CancelPending(_ context.Context, _, why string) error {
	a.cancelled = append(a.cancelled, why)
	return nil
}
func (a *recordingApprovals) HasActiveExecution(context.Context, string) (bool, error) {
	return a.active, nil
}

var (
	testTenant   = uuid.NewString()
	testReporter = uuid.NewString()
	testProject  = uuid.NewString()
)

func tctx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), testTenant), testReporter)
}

func mustCode(t *testing.T, err error, want string) {
	t.Helper()
	if codeOf(err) != want {
		t.Fatalf("want error code %s, got %v", want, err)
	}
}

// seedRequest stores a request directly in the given status with a fresh number.
func (s *memStore) seed(t *testing.T, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	r, err := domain.NewRequest(domain.NewRequestInput{TenantID: testTenant, ProjectID: testProject, Title: "t", SourceProvider: "manual", ReporterID: testReporter})
	if err != nil {
		t.Fatal(err)
	}
	s.counter++
	r.Number = s.counter
	if mod != nil {
		mod(&r)
	}
	s.requests[r.ID] = r
	return r
}
