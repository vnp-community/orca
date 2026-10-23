package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ---------------------------------------------------------------- fakes of the test-script lane

type tscRuns struct {
	mu      sync.Mutex
	rows    []domain.TestScriptRun
	missing []TestScriptRef
}

func (f *tscRuns) Current(_ context.Context, requestID string) (domain.TestScriptRun, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].RequestID == requestID && f.rows[i].SupersededAt == nil {
			return f.rows[i], true, nil
		}
	}
	return domain.TestScriptRun{}, false, nil
}

func (f *tscRuns) List(_ context.Context, requestID string) ([]domain.TestScriptRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.TestScriptRun
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].RequestID == requestID {
			out = append(out, f.rows[i])
		}
	}
	return out, nil
}

func (f *tscRuns) Create(_ context.Context, run domain.TestScriptRun) (bool, error) {
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

func (f *tscRuns) Replace(_ context.Context, run domain.TestScriptRun) error {
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

func (f *tscRuns) Update(_ context.Context, run domain.TestScriptRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].ID == run.ID {
			run.SupersededAt = f.rows[i].SupersededAt
			f.rows[i] = run
			return nil
		}
	}
	return errors.New("no such row")
}

func (f *tscRuns) Claim(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *tscRuns) Release(context.Context, string, string) error { return nil }

func (f *tscRuns) ListDue(_ context.Context, _ time.Duration, _ int) ([]TestScriptRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []TestScriptRef
	for _, r := range f.rows {
		if r.SupersededAt == nil && r.State.Active() {
			out = append(out, TestScriptRef{TenantID: r.TenantID, RequestID: r.RequestID, RunID: r.ID})
		}
	}
	return out, nil
}

func (f *tscRuns) ListReadyWithoutRun(context.Context, time.Duration, int) ([]TestScriptRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]TestScriptRef(nil), f.missing...), nil
}

func (f *tscRuns) current(t *testing.T) domain.TestScriptRun {
	t.Helper()
	r, ok, _ := f.Current(context.Background(), tsRequest)
	if !ok {
		t.Fatal("no current test-script row")
	}
	return r
}

type tscAgent struct {
	mu       sync.Mutex
	starts   []TestScriptStartInput
	startErr error
	job      TestScriptJob
	jobErr   error
	cancels  []string
}

func (a *tscAgent) Start(ctx context.Context, _ AnalysisConnection, in TestScriptStartInput) (TestScriptStarted, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u, _ := tenant.UserID(ctx); u == "" {
		return TestScriptStarted{}, errors.New("no user on the call")
	}
	a.starts = append(a.starts, in)
	if a.startErr != nil {
		return TestScriptStarted{}, a.startErr
	}
	return TestScriptStarted{JobID: fmt.Sprintf("tsa-%d", len(a.starts)), State: TSAQueued}, nil
}

func (a *tscAgent) Job(context.Context, AnalysisConnection, string) (TestScriptJob, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.job, a.jobErr
}

func (a *tscAgent) Report(_ context.Context, _ AnalysisConnection, jobID string, offset int) (TestScriptText, error) {
	return TestScriptText{Path: "tests/tsa/x/tsa-report.md", Total: 10, Text: jobID + fmt.Sprint(offset)}, nil
}

func (a *tscAgent) File(_ context.Context, _ AnalysisConnection, jobID, path string, offset int) (TestScriptText, error) {
	return TestScriptText{Path: path, Total: 3, Text: jobID + ":" + path + fmt.Sprint(offset)}, nil
}

func (a *tscAgent) Cancel(_ context.Context, _ AnalysisConnection, jobID, _ string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancels = append(a.cancels, jobID)
	return nil
}

type tscLane struct {
	reqs     *tsRequests
	specs    *tsRuns
	runs     *tscRuns
	agent    *tscAgent
	resolver *tsResolver
	actors   *tsActors
	enabled  bool
	now      time.Time
	ids      int
}

func newScriptLane() *tscLane {
	l := &tscLane{
		reqs: &tsRequests{reqs: map[string]domain.Request{tsRequest: {
			ID: tsRequest, ProjectID: tsProject, Number: 12, Status: domain.RequestStatusExecuting, BaseRef: "main",
		}}},
		specs:    &tsRuns{},
		runs:     &tscRuns{},
		agent:    &tscAgent{},
		resolver: &tsResolver{conn: AnalysisConnection{DevServerID: "ds-1", RepoPath: "/home/dev/notes"}},
		actors:   &tsActors{actor: "user-9"},
		enabled:  true,
		now:      time.Date(2026, 10, 23, 4, 0, 0, 0, time.UTC),
	}
	return l
}

func (l *tscLane) ctx() context.Context { return tenant.WithTenantID(context.Background(), tsTenant) }

func (l *tscLane) specDone(id string, version int) {
	l.specs.rows = append(l.specs.rows, domain.TestSpecRun{
		ID: id, TenantID: tsTenant, RequestID: tsRequest, State: domain.TestSpecCompleted, Feature: "TDD-NOTES",
		Summary: json.RawMessage(fmt.Sprintf(`{"feature":"TDD-NOTES","version":%d}`, version)),
	})
}

func (l *tscLane) start() *StartTestScripts {
	return &StartTestScripts{
		Requests: l.reqs, Runs: l.runs, SpecRuns: l.specs, Agent: l.agent, Resolver: l.resolver, Actors: l.actors,
		Settings: TestSpecSettingsFunc(func(context.Context, string) bool { return l.enabled }),
		Options:  TestScriptOptions{TestsPaths: []string{"tests"}, Generator: "claude"},
		Now:      func() time.Time { return l.now },
		NewID:    func() string { l.ids++; return fmt.Sprintf("tsc-%d", l.ids) },
	}
}

func (l *tscLane) reconcile() *ReconcileTestScripts {
	return &ReconcileTestScripts{Runs: l.runs, Requests: l.reqs, Start: l.start(), Agent: l.agent, Resolver: l.resolver,
		Now: func() time.Time { return l.now }}
}

// ---------------------------------------------------------------- start

func TestTestScriptsStartWhenExecutingAndTheTestSpecsCompleted(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 3)
	started, err := l.start().Execute(l.ctx(), tsRequest)
	if err != nil || !started {
		t.Fatalf("started %v err %v", started, err)
	}
	r := l.runs.current(t)
	if r.State != domain.TestScriptRunning || r.TSAJobID != "tsa-1" || r.Branch != "tsa/REQ-12-22222222-TDD-NOTES-v3" || r.TestSpecRunID != "spec-1" ||
		r.SpecVersion != 3 || r.StartedBy != "user-9" || r.BaseRef != "main" {
		t.Fatalf("row %+v", r)
	}
	in := l.agent.starts[0]
	if in.Feature != "TDD-NOTES" || in.SpecVersion != 3 || in.Branch != r.Branch || in.BaseRef != "main" || in.ExecutionID != tsRequest ||
		in.StepID != "testscript" || len(in.TestsPaths) != 1 || in.Generator != "claude" {
		t.Fatalf("start input %+v", in)
	}
	// The same event again, or the other start path: nothing new.
	again, err := l.start().Execute(l.ctx(), tsRequest)
	if err != nil || again || len(l.agent.starts) != 1 || len(l.runs.rows) != 1 {
		t.Fatalf("second start: %v %v, %d starts, %d rows", again, err, len(l.agent.starts), len(l.runs.rows))
	}
}

func TestTestScriptsWaitForTestSpecsThatAreStillRunning(t *testing.T) {
	l := newScriptLane()
	l.specs.rows = append(l.specs.rows, domain.TestSpecRun{ID: "spec-1", TenantID: tsTenant, RequestID: tsRequest, State: domain.TestSpecRunning})
	if started, err := l.start().Execute(l.ctx(), tsRequest); err != nil || started || len(l.runs.rows) != 0 {
		t.Fatalf("started %v err %v rows %d", started, err, len(l.runs.rows))
	}
	// The test-spec lane finishes and calls the hook: now it starts.
	l.specs.rows[0].State, l.specs.rows[0].Feature = domain.TestSpecCompleted, "TDD-NOTES"
	if started, err := l.start().Execute(l.ctx(), tsRequest); err != nil || !started {
		t.Fatalf("after completion: %v %v", started, err)
	}
}

func TestTestScriptsDoNotStartBeforeExecutionOrWhenOff(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	l.reqs.reqs[tsRequest] = domain.Request{ID: tsRequest, ProjectID: tsProject, Number: 12, Status: domain.RequestStatusAwaitingPlanApproval}
	if started, _ := l.start().Execute(l.ctx(), tsRequest); started {
		t.Fatal("started before the Plan was approved")
	}
	l.reqs.reqs[tsRequest] = domain.Request{ID: tsRequest, ProjectID: tsProject, Number: 12, Status: domain.RequestStatusExecuting}
	l.enabled = false
	if started, _ := l.start().Execute(l.ctx(), tsRequest); started || len(l.runs.rows) != 0 {
		t.Fatal("started while the lane is off")
	}
}

func TestTestScriptsAreSkippedWithoutUsableTestSpecs(t *testing.T) {
	l := newScriptLane()
	l.specs.rows = append(l.specs.rows, domain.TestSpecRun{ID: "spec-1", TenantID: tsTenant, RequestID: tsRequest, State: domain.TestSpecFailed,
		ReasonCode: "TESTSPEC_FAILED"})
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	r := l.runs.current(t)
	if r.State != domain.TestScriptSkipped || r.ReasonCode != domain.TestScriptNoTestSpecs || len(l.agent.starts) != 0 || r.FinishedAt == nil {
		t.Fatalf("row %+v", r)
	}
	// The test specs are run again and complete: the hook starts a new attempt.
	l.specDone("spec-2", 2)
	if started, err := l.start().Execute(l.ctx(), tsRequest); err != nil || !started {
		t.Fatalf("after new test specs: %v %v", started, err)
	}
	r = l.runs.current(t)
	if r.Attempt != 2 || r.State != domain.TestScriptRunning || r.Branch != "tsa/REQ-12-22222222-TDD-NOTES-v2" {
		t.Fatalf("row %+v", r)
	}
}

func TestTestScriptsMapAgentErrors(t *testing.T) {
	cases := []struct {
		err    error
		status domain.TestScriptStatus
		code   string
	}{
		{&tddagent.BridgeError{Code: "MCP_SERVER_NOT_ALLOWED", Message: "x"}, domain.TestScriptBlocked, domain.TestScriptToolMissing},
		{&tddagent.ToolError{Code: "INVALID_ARGUMENT", Message: "x"}, domain.TestScriptFailed, "INVALID_ARGUMENT"},
		{fmt.Errorf("%w: down", tddagent.ErrTransport), domain.TestScriptPending, domain.TestScriptUnreachable},
	}
	for _, c := range cases {
		l := newScriptLane()
		l.specDone("spec-1", 1)
		l.agent.startErr = c.err
		if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
			t.Fatal(err)
		}
		r := l.runs.current(t)
		if r.State != c.status || r.ReasonCode != c.code {
			t.Fatalf("%v: row %s %s", c.err, r.State, r.ReasonCode)
		}
	}
}

func TestTestScriptsBlockedWhenNobodyMayExecute(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	l.actors.actor = ""
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptBlocked || r.ReasonCode != domain.TestScriptActorNotAllowed {
		t.Fatalf("row %+v", r)
	}
}

func TestTestScriptsSkippedWithoutDevServer(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	l.resolver.err = ErrNoDevServer
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptSkipped || r.ReasonCode != domain.TestScriptNoDevServer {
		t.Fatalf("row %+v", r)
	}
}

func TestTestScriptsRestartCancelsTheRunningJob(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	cur, err := l.start().Restart(tenant.WithUserID(l.ctx(), "user-9"), tsRequest)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Attempt != 2 || cur.TSAJobID != "tsa-2" || len(l.agent.cancels) != 1 || l.agent.cancels[0] != "tsa-1" {
		t.Fatalf("current %+v cancels %v", cur, l.agent.cancels)
	}
	l.enabled = false
	if _, err := l.start().Restart(l.ctx(), tsRequest); err == nil {
		t.Fatal("restart while off")
	}
}

// ---------------------------------------------------------------- reconcile

func TestTestScriptsReconcileReadsTheJob(t *testing.T) {
	cases := []struct {
		job    TestScriptJob
		status domain.TestScriptStatus
		code   string
	}{
		{TestScriptJob{State: TSADone, AllGenerated: true, Commit: "abc", Worktree: "/w", Raw: []byte(`{"state":"done"}`)}, domain.TestScriptCompleted, ""},
		{TestScriptJob{State: TSADone, Commit: "abc", Raw: []byte(`{}`)}, domain.TestScriptNeedsReview, "REQUEST_TESTSCRIPT_NEEDS_REVIEW"},
		{TestScriptJob{State: TSAError, ErrorCode: "FRAMEWORK_UNSUPPORTED", ErrorText: "no pytest"}, domain.TestScriptBlocked, domain.TestScriptFrameworkUnsupported},
		{TestScriptJob{State: TSAError, ErrorCode: "SPEC_VERSION_CHANGED"}, domain.TestScriptFailed, domain.TestScriptSpecVersionChanged},
		{TestScriptJob{State: TSACancelled}, domain.TestScriptCancelled, ""},
		{TestScriptJob{State: TSARunning}, domain.TestScriptRunning, ""},
	}
	for _, c := range cases {
		l := newScriptLane()
		l.specDone("spec-1", 1)
		if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
			t.Fatal(err)
		}
		l.agent.job = c.job
		if _, err := l.reconcile().Execute(context.Background()); err != nil {
			t.Fatal(err)
		}
		r := l.runs.current(t)
		if r.State != c.status || r.ReasonCode != c.code {
			t.Fatalf("%s: row %s %s", c.job.State, r.State, r.ReasonCode)
		}
		if c.status == domain.TestScriptCompleted && (r.CommitSHA != "abc" || r.WorktreePath != "/w" || string(r.Summary) != `{"state":"done"}`) {
			t.Fatalf("completed row %+v", r)
		}
	}
}

func TestTestScriptsReconcileEndsLostAndLateJobs(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	l.agent.jobErr = &tddagent.ToolError{Code: "TSA_JOB_NOT_FOUND", Message: "gone"}
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptFailed || r.ReasonCode != domain.TestScriptJobLost {
		t.Fatalf("row %+v", r)
	}

	l = newScriptLane()
	l.specDone("spec-1", 1)
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	l.now = l.now.Add(2 * time.Hour)
	l.agent.job = TestScriptJob{State: TSARunning}
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptFailed || r.ReasonCode != domain.TestScriptDeadline || len(l.agent.cancels) != 1 {
		t.Fatalf("row %+v cancels %v", r, l.agent.cancels)
	}
}

func TestTestScriptsReconcileStartsWhatTheEventsMissedAndResumesPending(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	l.runs.missing = []TestScriptRef{{TenantID: tsTenant, RequestID: tsRequest}}
	l.agent.startErr = fmt.Errorf("%w: down", tddagent.ErrTransport)
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptPending {
		t.Fatalf("row %+v", r)
	}
	l.runs.missing, l.agent.startErr = nil, nil
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := l.runs.current(t); r.State != domain.TestScriptRunning || r.TSAJobID == "" {
		t.Fatalf("row %+v", r)
	}
}

// ---------------------------------------------------------------- queries

func TestTestScriptQueries(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	get := &GetTestScripts{Requests: l.reqs, Runs: l.runs, Settings: TestSpecSettingsFunc(func(context.Context, string) bool { return true })}
	view, err := get.Execute(l.ctx(), tsRequest)
	if err != nil || view.Current != nil || !view.Enabled {
		t.Fatalf("view %+v err %v", view, err)
	}
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	report := &GetTestScriptReport{Requests: l.reqs, Runs: l.runs, Agent: l.agent, Resolver: l.resolver}
	if _, err := report.Execute(tenant.WithUserID(l.ctx(), "user-9"), tsRequest, 0); err == nil {
		t.Fatal("report of a running job")
	}
	l.agent.job = TestScriptJob{State: TSADone, AllGenerated: true}
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := l.start().Restart(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	view, _ = get.Execute(l.ctx(), tsRequest)
	if view.Current == nil || view.Current.Attempt != 2 || len(view.History) != 1 {
		t.Fatalf("view %+v", view)
	}
	l.agent.job = TestScriptJob{State: TSADone, AllGenerated: true}
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	txt, err := report.Execute(tenant.WithUserID(l.ctx(), "user-9"), tsRequest, 5)
	if err != nil || txt.Text != "tsa-25" {
		t.Fatalf("report %+v err %v", txt, err)
	}
}

func TestTestScriptFileReadsOnlyWhatTheJobWrote(t *testing.T) {
	l := newScriptLane()
	l.specDone("spec-1", 1)
	if _, err := l.start().Execute(l.ctx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	file := &GetTestScriptFile{Requests: l.reqs, Runs: l.runs, Agent: l.agent, Resolver: l.resolver}
	ctx := tenant.WithUserID(l.ctx(), "user-9")
	if _, err := file.Execute(ctx, tsRequest, "tests/tsa/x/test_a.py", 0); err == nil {
		t.Fatal("file of a running job")
	}
	l.agent.job = TestScriptJob{State: TSADone, AllGenerated: true, Raw: []byte(`{"state":"done","files":[{"path":"tests/tsa/x/test_a.py","status":"generated"}],
		"manifest":"tests/tsa/x/assertion-manifest.json","report":"tests/tsa/x/tsa-report.md"}`)}
	if _, err := l.reconcile().Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, ok := range []string{"tests/tsa/x/test_a.py", "tests/tsa/x/assertion-manifest.json", "tests/tsa/x/tsa-report.md"} {
		got, err := file.Execute(ctx, tsRequest, ok, 2)
		if err != nil || got.Text != "tsa-1:"+ok+"2" {
			t.Fatalf("%s: %+v %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "tests/conftest.py", "../etc/passwd", "/etc/passwd", "tests/tsa/x/../../app.py"} {
		if _, err := file.Execute(ctx, tsRequest, bad, 0); err == nil {
			t.Fatalf("%q was read", bad)
		}
	}
}
