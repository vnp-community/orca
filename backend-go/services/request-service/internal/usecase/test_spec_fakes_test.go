package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// In-memory fakes for the test-spec lane. They count calls so a test can say "the code lane was not touched" or "started once".

type tsRequests struct {
	mu   sync.Mutex
	reqs map[string]domain.Request
}

func (f *tsRequests) Get(_ context.Context, id string) (domain.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reqs[id]
	if !ok {
		return domain.Request{}, domain.ErrRequestNotFound(id)
	}
	return r, nil
}

type tsRuns struct {
	mu   sync.Mutex
	rows []domain.TestSpecRun
	// missing lists executing Requests the scan reports (a lost event).
	missing []TestSpecRef
	claimed map[string]bool
}

func (f *tsRuns) Current(_ context.Context, requestID string) (domain.TestSpecRun, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].RequestID == requestID && f.rows[i].SupersededAt == nil {
			return f.rows[i], true, nil
		}
	}
	return domain.TestSpecRun{}, false, nil
}

func (f *tsRuns) List(_ context.Context, requestID string) ([]domain.TestSpecRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.TestSpecRun
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].RequestID == requestID {
			out = append(out, f.rows[i])
		}
	}
	return out, nil
}

func (f *tsRuns) Create(_ context.Context, run domain.TestSpecRun) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.RequestID == run.RequestID && r.SupersededAt == nil {
			return false, nil
		}
	}
	f.rows = append(f.rows, run)
	return true, nil
}

func (f *tsRuns) Replace(_ context.Context, run domain.TestSpecRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for i := range f.rows {
		if f.rows[i].RequestID == run.RequestID && f.rows[i].SupersededAt == nil {
			f.rows[i].SupersededAt = &now
		}
	}
	f.rows = append(f.rows, run)
	return nil
}

func (f *tsRuns) Update(_ context.Context, run domain.TestSpecRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].ID == run.ID {
			f.rows[i] = run
			return nil
		}
	}
	return errors.New("no such row")
}

func (f *tsRuns) Claim(_ context.Context, id, _ string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	if f.claimed[id] {
		return false, nil
	}
	f.claimed[id] = true
	return true, nil
}

func (f *tsRuns) Release(_ context.Context, id, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claimed, id)
	return nil
}

func (f *tsRuns) ListDue(_ context.Context, _ time.Duration, _ int) ([]TestSpecRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []TestSpecRef
	for _, r := range f.rows {
		if r.SupersededAt == nil && r.State.Active() {
			out = append(out, TestSpecRef{TenantID: r.TenantID, RequestID: r.RequestID, RunID: r.ID})
		}
	}
	return out, nil
}

func (f *tsRuns) ListExecutingWithoutRun(_ context.Context, _ time.Duration, _ int) ([]TestSpecRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]TestSpecRef(nil), f.missing...), nil
}

func (f *tsRuns) status(requestID string) domain.TestSpecRun {
	r, _, _ := f.Current(context.Background(), requestID)
	return r
}

type tsAgent struct {
	mu       sync.Mutex
	cfg      TestSpecConfig
	cfgErr   error
	startErr error
	runState TestSpecRunState
	runErr   error
	features []string
	summary  json.RawMessage
	// calls counts what the lane asked of the dev server.
	calls []string
	// lastStart is the document of the last testspec_start.
	lastStart TestSpecStartInput
}

func (a *tsAgent) note(s string) {
	a.mu.Lock()
	a.calls = append(a.calls, s)
	a.mu.Unlock()
}

func (a *tsAgent) count(s string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, c := range a.calls {
		if c == s {
			n++
		}
	}
	return n
}

func (a *tsAgent) Config(context.Context, AnalysisConnection) (TestSpecConfig, error) {
	a.note("config")
	return a.cfg, a.cfgErr
}

func (a *tsAgent) Start(_ context.Context, _ AnalysisConnection, in TestSpecStartInput) (TestSpecStarted, error) {
	a.note("start")
	a.mu.Lock()
	a.lastStart = in
	a.mu.Unlock()
	if a.startErr != nil {
		return TestSpecStarted{}, a.startErr
	}
	return TestSpecStarted{RunID: "run-abc", Mode: in.Kind, Ref: in.Ref, SHA256: "sha-1"}, nil
}

func (a *tsAgent) Dispatch(context.Context, AnalysisConnection, string) error {
	a.note("dispatch")
	return nil
}

func (a *tsAgent) Run(context.Context, AnalysisConnection, string) (TestSpecRunState, error) {
	a.note("run")
	return a.runState, a.runErr
}

func (a *tsAgent) Features(context.Context, AnalysisConnection) ([]string, error) {
	a.note("features")
	return a.features, nil
}

func (a *tsAgent) Summary(context.Context, AnalysisConnection, string) (json.RawMessage, error) {
	a.note("summary")
	return a.summary, nil
}

func (a *tsAgent) Document(_ context.Context, _ AnalysisConnection, feature, document string, offset int) (TestSpecText, error) {
	a.note("document")
	return TestSpecText{Document: document, Text: feature + ":" + document, Length: 3 + offset}, nil
}

func (a *tsAgent) Cancel(context.Context, AnalysisConnection, string, string) error {
	a.note("cancel")
	return nil
}

type tsResolver struct {
	conn AnalysisConnection
	err  error
}

// Like project-service, it refuses a lookup that carries no signed-in user.
func (r tsResolver) ResolveForProject(ctx context.Context, _ string) (AnalysisConnection, error) {
	if u, _ := tenant.UserID(ctx); u == "" {
		return AnalysisConnection{}, domain.ErrRequestReporterRequired()
	}
	return r.conn, r.err
}

type tsActors struct {
	actor string
	err   error
}

func (a tsActors) ActorFor(context.Context, domain.Request, string) (string, error) {
	return a.actor, a.err
}

type tsSolutions struct{ list []domain.Solution }

func (s tsSolutions) ListByRequest(context.Context, SolutionListFilter) ([]domain.Solution, error) {
	return s.list, nil
}

type tsChanges struct {
	change domain.OpenSpecChange
	found  bool
}

func (c tsChanges) GetByRequest(context.Context, string) (domain.OpenSpecChange, bool, error) {
	return c.change, c.found, nil
}

var errUnreachable = errors.Join(tddagent.ErrTransport, errors.New("relay down"))
