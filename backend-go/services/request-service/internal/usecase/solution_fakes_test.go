package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// solWorld is the in-memory universe of the solution tests: the lifecycle store (requests, outbox, transactions)
// plus solutions, runs and approvals that roll back together with it.
type solWorld struct {
	*lcStore
	smu       sync.Mutex
	solutions map[string]domain.Solution
	runs      map[string]domain.AnalysisRun
	opened    []OpenSolutionApprovalInput
	cancelled []string
	digests   map[string]string
	failOpen  error
	failCanc  error
	now       time.Time
}

func newSolWorld() *solWorld {
	return &solWorld{lcStore: newLcStore(), solutions: map[string]domain.Solution{}, runs: map[string]domain.AnalysisRun{}, digests: map[string]string{}, now: time.Now().UTC()}
}

type solSnapshot struct {
	solutions map[string]domain.Solution
	runs      map[string]domain.AnalysisRun
	opened    int
	cancelled int
}

func (w *solWorld) snap() solSnapshot {
	s := solSnapshot{solutions: map[string]domain.Solution{}, runs: map[string]domain.AnalysisRun{}, opened: len(w.opened), cancelled: len(w.cancelled)}
	for k, v := range w.solutions {
		s.solutions[k] = v
	}
	for k, v := range w.runs {
		s.runs[k] = v
	}
	return s
}

func (w *solWorld) restore(s solSnapshot) {
	w.solutions, w.runs, w.opened, w.cancelled = s.solutions, s.runs, w.opened[:s.opened], w.cancelled[:s.cancelled]
}

func (w *solWorld) InTx(ctx context.Context, fn func(context.Context) error) error {
	if w.InTransaction(ctx) {
		return fn(ctx)
	}
	return w.lcStore.InTx(ctx, func(ctx context.Context) error {
		before := w.snap()
		err := fn(ctx)
		if err != nil {
			w.restore(before)
		}
		return err
	})
}

// --- SolutionStore ---

type fakeSolutions struct{ w *solWorld }

func (f fakeSolutions) Insert(_ context.Context, s domain.Solution) error {
	f.w.solutions[s.ID] = s
	return nil
}

func (f fakeSolutions) Get(ctx context.Context, id string) (domain.Solution, error) {
	tid, _ := tenant.TenantID(ctx)
	s, ok := f.w.solutions[id]
	if !ok || s.TenantID != tid {
		return domain.Solution{}, domain.ErrSolutionNotFound(id)
	}
	return s, nil
}

func (f fakeSolutions) ListByRequestID(ctx context.Context, requestID string) ([]domain.Solution, error) {
	return f.ListByRequest(ctx, SolutionListFilter{RequestID: requestID})
}

func (f fakeSolutions) ListByRequest(ctx context.Context, flt SolutionListFilter) ([]domain.Solution, error) {
	tid, _ := tenant.TenantID(ctx)
	var out []domain.Solution
	for _, s := range f.w.solutions {
		if s.TenantID != tid || s.RequestID != flt.RequestID {
			continue
		}
		if (flt.Kind != "" && s.Kind != flt.Kind) || (flt.Status != "" && s.Status != flt.Status) {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (f fakeSolutions) Update(ctx context.Context, s domain.Solution, expected int64) (domain.Solution, error) {
	cur, err := f.Get(ctx, s.ID)
	if err != nil {
		return domain.Solution{}, err
	}
	if cur.Version != expected {
		return domain.Solution{}, domain.ErrSolutionVersionConflict(s.ID, expected)
	}
	s.Version = expected + 1
	f.w.solutions[s.ID] = s
	return s, nil
}

func (f fakeSolutions) Choose(ctx context.Context, id string, idx int, expected int64) (bool, error) {
	cur, err := f.Get(ctx, id)
	if err != nil || cur.Status != domain.SolutionStatusProposed || cur.Version != expected {
		return false, err
	}
	cur.ChosenOption, cur.Version = &idx, cur.Version+1
	f.w.solutions[id] = cur
	return true, nil
}

func (f fakeSolutions) SupersedeOpen(ctx context.Context, requestID string, kind domain.SolutionKind, except string) (int, error) {
	tid, _ := tenant.TenantID(ctx)
	n := 0
	for id, s := range f.w.solutions {
		if s.TenantID == tid && s.RequestID == requestID && s.Kind == kind && id != except &&
			(s.Status == domain.SolutionStatusProposed || s.Status == domain.SolutionStatusRejected) {
			s.Status, s.Version = domain.SolutionStatusSuperseded, s.Version+1
			f.w.solutions[id] = s
			n++
		}
	}
	return n, nil
}

func (f fakeSolutions) DeleteDraft(_ context.Context, id string) error {
	if s, ok := f.w.solutions[id]; ok && s.Status == domain.SolutionStatusDraft {
		delete(f.w.solutions, id)
	}
	return nil
}

// --- AnalysisRunStore ---

type fakeRuns struct{ w *solWorld }

func (f fakeRuns) EnsureProjectGate(context.Context, string) error { return nil }

func (f fakeRuns) StartRun(ctx context.Context, run domain.AnalysisRun, draft domain.Solution, opts StartRunOptions) (StartRunResult, error) {
	for _, r := range f.w.runs {
		sameKey := run.IdempotencyKey != nil && r.IdempotencyKey != nil && *run.IdempotencyKey == *r.IdempotencyKey && r.RequestID == run.RequestID
		if sameKey || (r.RequestID == run.RequestID && r.Kind == run.Kind && r.Status == domain.RunStatusRunning) {
			return StartRunResult{Run: r}, nil
		}
	}
	if run.Mode == domain.AnalysisModeAgentReadonly && opts.MaxAgentRuns > 0 {
		n, _ := f.CountRunning(ctx, run.ProjectID, domain.AnalysisModeAgentReadonly)
		if n >= opts.MaxAgentRuns {
			return StartRunResult{}, domain.ErrAnalysisBusy()
		}
	}
	exp := f.w.now.Add(opts.LeaseTTL)
	run.LeaseExpiresAt = &exp
	f.w.runs[run.ID] = run
	f.w.solutions[draft.ID] = draft
	return StartRunResult{Run: run, Created: true}, nil
}

func (f fakeRuns) Get(_ context.Context, id string) (domain.AnalysisRun, error) {
	r, ok := f.w.runs[id]
	if !ok {
		return domain.AnalysisRun{}, domain.ErrSolutionNotFound(id)
	}
	return r, nil
}

func (f fakeRuns) RenewLease(_ context.Context, id, owner string, ttl time.Duration) (bool, error) {
	f.w.smu.Lock()
	defer f.w.smu.Unlock()
	r, ok := f.w.runs[id]
	if !ok || r.Status != domain.RunStatusRunning || r.LeaseOwner == nil || *r.LeaseOwner != owner {
		return false, nil
	}
	exp := f.w.now.Add(ttl)
	r.LeaseExpiresAt = &exp
	f.w.runs[id] = r
	return true, nil
}

func (f fakeRuns) FinishOwned(_ context.Context, run domain.AnalysisRun, owner string) (bool, error) {
	cur, ok := f.w.runs[run.ID]
	if !ok || cur.Status != domain.RunStatusRunning || cur.LeaseOwner == nil || *cur.LeaseOwner != owner {
		return false, nil
	}
	run.LeaseOwner, run.LeaseExpiresAt = nil, nil
	f.w.runs[run.ID] = run
	return true, nil
}

func (f fakeRuns) ClaimExpired(_ context.Context, owner string, ttl time.Duration, batch int) ([]domain.AnalysisRun, error) {
	var out []domain.AnalysisRun
	for id, r := range f.w.runs {
		if r.Status == domain.RunStatusRunning && r.LeaseExpiresAt != nil && r.LeaseExpiresAt.Before(f.w.now) && len(out) < batch {
			exp := f.w.now.Add(ttl)
			r.LeaseOwner, r.LeaseExpiresAt = &owner, &exp
			f.w.runs[id] = r
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeRuns) ListRecent(_ context.Context, requestID string, limit int) ([]domain.AnalysisRun, error) {
	var out []domain.AnalysisRun
	for _, r := range f.w.runs {
		if r.RequestID == requestID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f fakeRuns) CountRunning(_ context.Context, projectID string, mode domain.AnalysisMode) (int, error) {
	n := 0
	for _, r := range f.w.runs {
		if r.ProjectID == projectID && r.Mode == mode && r.Status == domain.RunStatusRunning {
			n++
		}
	}
	return n, nil
}

// --- approvals, authorisation, AI ---

type fakeOpener struct{ w *solWorld }

func (o fakeOpener) Open(_ context.Context, in OpenSolutionApprovalInput) error {
	if o.w.failOpen != nil {
		return o.w.failOpen
	}
	o.w.opened = append(o.w.opened, in)
	return nil
}

type fakeCanceller struct{ w *solWorld }

func (c fakeCanceller) CancelPending(_ context.Context, requestID, why string) error {
	if c.w.failCanc != nil {
		return c.w.failCanc
	}
	c.w.cancelled = append(c.w.cancelled, requestID+":"+why)
	return nil
}

type fakeDigests struct{ w *solWorld }

func (d fakeDigests) UpdatePendingDigest(_ context.Context, _ string, st domain.SubjectType, subjectID, digest string) (bool, error) {
	d.w.digests[string(st)+":"+subjectID] = digest
	return true, nil
}

type allowAll struct{}

func (allowAll) AuthorizeGenerate(context.Context, domain.Request) error { return nil }
func (allowAll) AuthorizeChoose(context.Context, domain.Request) error   { return nil }

type fakeConns struct {
	conn AnalysisConnection
	err  error
}

func (c fakeConns) ResolveForProject(context.Context, string) (AnalysisConnection, error) {
	return c.conn, c.err
}

// scriptedCompleter returns replies in order and records prompts; the last reply repeats.
type scriptedCompleter struct {
	mu      sync.Mutex
	replies []string
	errs    []error
	prompts []string
	block   chan struct{}
	onCall  func()
}

func (s *scriptedCompleter) Complete(ctx context.Context, _ string, prompt string) (string, error) {
	s.mu.Lock()
	n := len(s.prompts)
	s.prompts = append(s.prompts, prompt)
	s.mu.Unlock()
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if s.onCall != nil {
		s.onCall()
	}
	if n < len(s.errs) && s.errs[n] != nil {
		return "", s.errs[n]
	}
	if len(s.replies) == 0 {
		return "", errors.New("no scripted reply")
	}
	return s.replies[min(n, len(s.replies)-1)], nil
}

func (s *scriptedCompleter) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.prompts)
}

type scriptedAgent struct {
	mu     sync.Mutex
	inputs []AgentPromptInput
	conns  []AnalysisConnection
	res    func(call int, in AgentPromptInput) (AgentPromptResult, error)
}

func (a *scriptedAgent) ExecPrompt(_ context.Context, conn AnalysisConnection, in AgentPromptInput) (AgentPromptResult, error) {
	a.mu.Lock()
	n := len(a.inputs)
	a.inputs = append(a.inputs, in)
	a.conns = append(a.conns, conn)
	a.mu.Unlock()
	return a.res(n, in)
}

type scriptedProbe struct {
	mu    sync.Mutex
	snaps []RepoSnapshot
	err   error
	calls int
}

func (p *scriptedProbe) Snapshot(_ context.Context, worktreeID string) (RepoSnapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if worktreeID == "" {
		return RepoSnapshot{}, ErrProbeUnavailable
	}
	if p.err != nil {
		return RepoSnapshot{}, p.err
	}
	i := min(p.calls, len(p.snaps)-1)
	p.calls++
	return p.snaps[i], nil
}

type fakeCaps struct {
	features []string
	err      error
}

func (c fakeCaps) Get(context.Context, DevServerRef, bool) (domain.DevServerCapability, error) {
	if c.err != nil {
		return domain.DevServerCapability{}, c.err
	}
	return domain.DevServerCapability{Features: c.features}, nil
}

// capturingSpawner records the run instead of starting a goroutine.
type capturingSpawner struct {
	mu   sync.Mutex
	runs []domain.AnalysisRun
}

func (c *capturingSpawner) Spawn(run domain.AnalysisRun) {
	c.mu.Lock()
	c.runs = append(c.runs, run)
	c.mu.Unlock()
}

// failingTransition makes the last step of a multi-step write fail, to prove nothing stays behind.
type failingTransition struct{ err error }

func (f failingTransition) Execute(context.Context, TransitionInput) (TransitionResult, error) {
	return TransitionResult{}, f.err
}

// --- builders ---

const testOwner = "worker-1"

func (w *solWorld) ctx() context.Context {
	return tenant.WithUserID(lcCtx(), uuid.NewString())
}

func (w *solWorld) transitioner() RequestTransitioner { return NewTransitionRequest(w, w, w) }

func (w *solWorld) seedRequest(t domain.RequestType, status domain.RequestStatus) domain.Request {
	return w.seed(func(r *domain.Request) {
		r.Type, r.Status, r.Size = t, status, domain.RequestSizeM
		r.Title, r.Body = "Cache nội dung", "Trang tải chậm"
	})
}

func (w *solWorld) newGenerate(spawner AnalysisSpawner, conns AnalysisConnectionResolver) *GenerateSolution {
	return NewGenerateSolution(GenerateSolutionDeps{
		Requests: w, Solutions: fakeSolutions{w}, Runs: fakeRuns{w}, Conns: conns, Auth: allowAll{}, Approvals: fakeCanceller{w},
		Transition: w.transitioner(), Tx: w, Spawner: spawner, LeaseOwner: testOwner,
	})
}

func (w *solWorld) newWriter(transition RequestTransitioner) *AnalysisResultWriter {
	return NewAnalysisResultWriter(w, fakeSolutions{w}, fakeRuns{w}, fakeOpener{w}, transition, w, w, testOwner)
}

func (w *solWorld) outboxSubjects() []string {
	var out []string
	for _, e := range w.events {
		out = append(out, e.Subject)
	}
	return out
}

// must unwraps a (value, error) pair in test setup, where an error means the test itself is broken.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !errorHasCode(err, code) {
		t.Fatalf("want error code %s, got %v", code, err)
	}
}

func describe(r domain.AnalysisRun) string {
	code := ""
	if r.ErrorCode != nil {
		code = *r.ErrorCode
	}
	return fmt.Sprintf("%s/%s code=%s", r.Kind, r.Status, code)
}

func lcCtxNoTenant() context.Context { return context.Background() }
