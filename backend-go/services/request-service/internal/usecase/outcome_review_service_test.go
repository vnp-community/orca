package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memReviews is an in-memory OutcomeReviewRepository with the version and key rules of the real ones.
type memReviews struct {
	mu     sync.Mutex
	rows   map[string]StoredReview
	order  []string
	writes int
}

func newMemReviews() *memReviews { return &memReviews{rows: map[string]StoredReview{}} }

func (m *memReviews) Create(_ context.Context, r StoredReview) (StoredReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.rows {
		if x.RequestID == r.RequestID && x.Phase == r.Phase && x.HeadKey == r.HeadKey {
			return StoredReview{}, domain.ErrReviewAlreadyExists(r.RequestID)
		}
	}
	r.Version, r.CreatedAt, r.UpdatedAt = 1, time.Now().UTC(), time.Now().UTC()
	m.rows[r.ID] = r
	m.order = append(m.order, r.ID)
	m.writes++
	return r, nil
}
func (m *memReviews) Get(_ context.Context, id string) (StoredReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[id]; ok {
		return r, nil
	}
	return StoredReview{}, domain.ErrReviewNotFound(id)
}
func (m *memReviews) GetByKey(_ context.Context, req string, p domain.ReviewPhase, hk string) (StoredReview, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.rows {
		if x.RequestID == req && x.Phase == p && x.HeadKey == hk {
			return x, true, nil
		}
	}
	return StoredReview{}, false, nil
}
func (m *memReviews) Latest(_ context.Context, req string) (StoredReview, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.order) - 1; i >= 0; i-- {
		if x := m.rows[m.order[i]]; x.RequestID == req && x.State != domain.ReviewStateSuperseded {
			return x, true, nil
		}
	}
	return StoredReview{}, false, nil
}
func (m *memReviews) List(_ context.Context, req string, n int, after string) ([]StoredReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []StoredReview
	for i := len(m.order) - 1; i >= 0; i-- {
		if x := m.rows[m.order[i]]; x.RequestID == req {
			out = append(out, x)
		}
	}
	return out, nil
}
func (m *memReviews) Update(_ context.Context, r StoredReview, v int64) (StoredReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rows[r.ID]
	if !ok {
		return StoredReview{}, domain.ErrReviewNotFound(r.ID)
	}
	if cur.Version != v {
		return StoredReview{}, domain.ErrReviewStaleRevision()
	}
	r.TenantID, r.RequestID, r.Phase, r.HeadKey, r.CreatedAt, r.Version = cur.TenantID, cur.RequestID, cur.Phase, cur.HeadKey, cur.CreatedAt, v+1
	m.rows[r.ID] = r
	m.writes++
	return r, nil
}
func (m *memReviews) SetState(_ context.Context, id string, st domain.ReviewState, v int64) (StoredReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.rows[id]
	if !ok {
		return StoredReview{}, domain.ErrReviewNotFound(id)
	}
	if cur.Version != v {
		return StoredReview{}, domain.ErrReviewStaleRevision()
	}
	cur.State, cur.Version = st, v+1
	m.rows[id] = cur
	m.writes++
	return cur, nil
}
func (m *memReviews) SupersedeOthers(_ context.Context, req, except string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, x := range m.rows {
		if x.RequestID == req && id != except && x.State != domain.ReviewStateSuperseded {
			x.State, x.Version = domain.ReviewStateSuperseded, x.Version+1
			m.rows[id] = x
		}
	}
	return nil
}

type fakeRequests struct{ byID map[string]domain.Request }

func (f fakeRequests) Get(_ context.Context, id string) (domain.Request, error) {
	if r, ok := f.byID[id]; ok {
		return r, nil
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

type fakeTasks struct {
	TaskClient
	tasks []domain.TaskView
	err   error
}

func (f *fakeTasks) ListTasks(context.Context, ListTasksQuery) ([]domain.TaskView, error) {
	return f.tasks, f.err
}

type fakeOutcomes struct {
	TaskRunOutcomeRepository
	rows []domain.TaskRunOutcome
}

func (f fakeOutcomes) ListByRequest(context.Context, string) ([]domain.TaskRunOutcome, error) {
	return f.rows, nil
}

type fakeCoverage struct {
	RequestCoverageRepository
	rows []domain.CoverageRow
}

func (f *fakeCoverage) ListByRequest(context.Context, string) ([]domain.CoverageRow, error) {
	return f.rows, nil
}

type fakeDiff struct {
	byWorktree map[string]ReviewDiff
	errFor     map[string]error
	calls      int
}

func (f *fakeDiff) BranchCompare(_ context.Context, wt, _ string) (ReviewDiff, error) {
	f.calls++
	if err := f.errFor[wt]; err != nil {
		return ReviewDiff{}, err
	}
	return f.byWorktree[wt], nil
}

type reviewEnv struct {
	svc      *OutcomeReviewService
	reviews  *memReviews
	tasks    *fakeTasks
	coverage *fakeCoverage
	diff     *fakeDiff
	req      domain.Request
	ctx      context.Context
	settings *fixedSettings
}

type fixedSettings struct {
	mode domain.OutcomeReviewMode
	err  error
}

func (f *fixedSettings) OutcomeReviewMode(context.Context, string) (domain.OutcomeReviewMode, error) {
	return f.mode, f.err
}

func newReviewEnv(t *testing.T, status domain.RequestStatus) *reviewEnv {
	t.Helper()
	tenantID := uuid.NewString()
	req := domain.Request{ID: uuid.NewString(), TenantID: tenantID, ProjectID: uuid.NewString(), Status: status, PlanTaskID: "plan-1", BaseRef: "main",
		Engine: domain.EngineNative, AcceptanceCriteriaJSON: []byte(`[{"id":"AC-1","text":"one approval","status":"active"},{"id":"AC-2","text":"activity feed","status":"active"}]`)}
	tasks := &fakeTasks{tasks: []domain.TaskView{
		{ID: "plan-1", Type: domain.TaskTypePlan, Status: domain.TaskStatusInProgress},
		{ID: "t1", ParentID: "plan-1", Type: "task", Status: domain.TaskStatusDone, WorktreeID: "wt1", Title: "one", Description: "Code: modify `backend-go/a.go`; create `backend-go/migrations/0105.sql`"},
		{ID: "t2", ParentID: "plan-1", Type: "task", Status: domain.TaskStatusInProgress, WorktreeID: "wt2", Title: "two", Description: "Code: modify `frontend/b.ts`"},
	}}
	cov := &fakeCoverage{rows: []domain.CoverageRow{{ACID: "AC-1", TaskID: "t1"}, {ACID: "AC-2", TaskID: "t2"}}}
	diff := &fakeDiff{byWorktree: map[string]ReviewDiff{
		"wt1": {Status: "ready", BaseOID: "base1", HeadOID: "h1", CompareRef: "feat/one", Entries: []domain.ChangedPath{{Path: "backend-go/a.go", Added: 5, Removed: 1}, {Path: "backend-go/migrations/0105.sql", Added: 9}}},
		"wt2": {Status: "ready", BaseOID: "base1", HeadOID: "h2", CompareRef: "feat/two", Entries: []domain.ChangedPath{{Path: "frontend/b.ts", Added: 2}, {Path: "backend-go/a.go", Added: 1}}},
	}, errFor: map[string]error{}}
	settings := &fixedSettings{mode: domain.OutcomeReviewManual}
	reviews := newMemReviews()
	svc := &OutcomeReviewService{
		Requests: fakeRequests{byID: map[string]domain.Request{req.ID: req}}, Reviews: reviews, Settings: settings,
		Collector: &ReviewFactsCollector{Tasks: tasks, Outcomes: fakeOutcomes{}, Coverage: cov, Diff: diff},
	}
	return &reviewEnv{svc: svc, reviews: reviews, tasks: tasks, coverage: cov, diff: diff, req: req, settings: settings,
		ctx: tenant.WithTenantID(context.Background(), tenantID)}
}

func (e *reviewEnv) setStatus(s domain.RequestStatus) {
	e.req.Status = s
	e.svc.Requests = fakeRequests{byID: map[string]domain.Request{e.req.ID: e.req}}
}

func finishAll(e *reviewEnv) {
	for i := range e.tasks.tasks {
		if e.tasks.tasks[i].Type == "task" {
			e.tasks.tasks[i].Status = domain.TaskStatusDone
		}
	}
}

func codeIs(err error, code string) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == code
}

func TestOutcomeReview_RefusedWhenOffOrNotReady(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	e.settings.mode = domain.OutcomeReviewOff
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); !codeIs(err, domain.CodeReviewDisabled) {
		t.Fatalf("project off: %v", err)
	}
	e.settings.mode = domain.OutcomeReviewManual
	e.svc.Config = domain.OutcomeReviewConfig{KillSwitch: true}
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); !codeIs(err, domain.CodeReviewDisabled) {
		t.Fatalf("the kill switch must win over the project: %v", err)
	}
	e.svc.Config = domain.OutcomeReviewConfig{}
	e.settings.err = errors.New("db down")
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); !codeIs(err, domain.CodeReviewDisabled) {
		t.Fatalf("an unreadable setting reads as off: %v", err)
	}
	e.settings.err = nil
	for _, st := range []domain.RequestStatus{domain.RequestStatusAnalyzing, domain.RequestStatusPlanning, domain.RequestStatusCancelled, domain.RequestStatusNew} {
		e.setStatus(st)
		if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); !codeIs(err, domain.CodeReviewNotReady) {
			t.Fatalf("status %s: %v", st, err)
		}
	}
	if e.reviews.writes != 0 || e.diff.calls != 0 {
		t.Fatalf("a refused request must write nothing and read no git: writes=%d git=%d", e.reviews.writes, e.diff.calls)
	}
	if _, err := e.svc.Generate(context.Background(), GenerateInput{RequestID: e.req.ID}); err == nil {
		t.Fatalf("a call without a tenant must fail")
	}
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: "nope"}); err == nil {
		t.Fatalf("a malformed request id must fail")
	}
}

func TestOutcomeReview_DesignPhaseWhileExecuting_ReusedOnSecondCall(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	first, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID, CreatedBy: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	r := first.View.Review
	if !first.Created || r.Phase != domain.ReviewPhaseDesign || r.Verdict.Requirements.Planned != 2 {
		t.Fatalf("first %+v", first)
	}
	if e.diff.calls != 0 {
		t.Fatalf("a design review reads no git, got %d calls", e.diff.calls)
	}
	var plan int
	for _, it := range r.Items {
		if it.Section == domain.SectionConfig {
			plan++
		}
	}
	if plan != 2 { // the plan names files under backend-go (two) and frontend (one): one row per area
		t.Fatalf("plan-target items = %d, want 2", plan)
	}
	second, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.Created || second.View.Stored.ID != first.View.Stored.ID || e.reviews.writes != 1 {
		t.Fatalf("second call must reuse: %+v writes=%d", second, e.reviews.writes)
	}
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID, Phase: domain.ReviewPhaseDelivered}); !codeIs(err, domain.CodeReviewNotReady) {
		t.Fatalf("an after-code review before execution ends: %v", err)
	}
}

func TestOutcomeReview_DeliveredMergesWorktreesAndSupersedesDesign(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	design, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil || design.View.Review.Phase != domain.ReviewPhaseDesign {
		t.Fatalf("design: %+v err=%v", design, err)
	}
	finishAll(e)
	e.setStatus(domain.RequestStatusCompleted)
	got, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	r := got.View.Review
	if r.Phase != domain.ReviewPhaseDelivered || !got.Created {
		t.Fatalf("delivered: %+v", got)
	}
	if r.Verdict.Requirements.Delivered != 2 {
		t.Fatalf("both criteria have all their tasks done: %+v", r.Verdict.Requirements)
	}
	if len(r.Subject.Heads) != 2 || r.Subject.BaseCommit != "base1" || r.Subject.HeadCommit == "" {
		t.Fatalf("subject %+v", r.Subject)
	}
	if len(r.Verdict.Release.Flags) != 1 || r.Verdict.Release.Flags[0] != domain.FlagMigration {
		t.Fatalf("flags %v", r.Verdict.Release.Flags)
	}
	if r.Verdict.Release.Risk != "unknown" || r.Verdict.Quality.Gate != "absent" {
		t.Fatalf("no assessment, no gate: %+v", r.Verdict)
	}
	old, _ := e.reviews.Get(e.ctx, design.View.Stored.ID)
	if old.State != domain.ReviewStateSuperseded {
		t.Fatalf("the design review must be superseded, is %s", old.State)
	}
	if e.diff.calls != 2 {
		t.Fatalf("one git call per worktree, got %d", e.diff.calls)
	}
	again, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil || !again.Reused {
		t.Fatalf("asking again changes nothing: %+v err=%v", again, err)
	}
	latest, ok, err := e.svc.Latest(e.ctx, e.req.ID)
	if err != nil || !ok || latest.Stored.ID != got.View.Stored.ID {
		t.Fatalf("latest %+v ok=%v err=%v", latest, ok, err)
	}
}

func TestOutcomeReview_DiffTrouble_IsAWarningNotAFailure(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusCompleted)
	finishAll(e)
	e.diff.errFor["wt2"] = errors.New("dev server offline")
	got, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	r := got.View.Review
	var partial bool
	for _, w := range r.Warnings {
		partial = partial || strings.HasPrefix(w, "partial_facts:diff")
	}
	if !partial || len(r.Subject.Heads) != 1 {
		t.Fatalf("one worktree unreadable: heads=%d warnings=%v", len(r.Subject.Heads), r.Warnings)
	}
	if len(r.Verdict.Release.Flags) != 0 {
		t.Fatalf("flags from a partly read diff must not be reported: %v", r.Verdict.Release.Flags)
	}
}

func TestOutcomeReview_NoWorktreeAtAll_DoesNotCollideWithDesign(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	for i := range e.tasks.tasks {
		e.tasks.tasks[i].WorktreeID = ""
	}
	finishAll(e)
	e.setStatus(domain.RequestStatusCompleted)
	got, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.View.Stored.HeadKey != domain.ReviewNoCodeHeadKey || got.View.Review.Phase != domain.ReviewPhaseDelivered {
		t.Fatalf("stored %+v", got.View.Stored)
	}
}

func TestOutcomeReview_GetMarksStaleWhenDatabaseFactsChange(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	gen, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	same, err := e.svc.Get(e.ctx, gen.View.Stored.ID)
	if err != nil || same.Review.State != domain.ReviewStateReady {
		t.Fatalf("nothing changed: %+v err=%v", same.Review.State, err)
	}
	e.req.AcceptanceCriteriaJSON = []byte(`[{"id":"AC-1","text":"one approval, reworded","status":"active"}]`)
	e.setStatus(domain.RequestStatusExecuting)
	stale, err := e.svc.Get(e.ctx, gen.View.Stored.ID)
	if err != nil || stale.Review.State != domain.ReviewStateStale {
		t.Fatalf("criteria changed, review must be stale: %+v err=%v", stale.Review.State, err)
	}
	row, _ := e.reviews.Get(e.ctx, gen.View.Stored.ID)
	if row.State != domain.ReviewStateStale {
		t.Fatalf("the stale state must be saved, row has %s", row.State)
	}
	regen, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil || regen.Reused || regen.Created || regen.View.Stored.ID != gen.View.Stored.ID || regen.View.Review.State != domain.ReviewStateReady {
		t.Fatalf("regenerating rewrites the same row: %+v err=%v", regen, err)
	}
	if regen.View.Stored.Version <= gen.View.Stored.Version {
		t.Fatalf("version must grow")
	}
}

func TestOutcomeReview_EstimateWritesNothingAndReportsStale(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	est, err := e.svc.Estimate(e.ctx, e.req.ID)
	if err != nil || est.Exists || est.Phase != domain.ReviewPhaseDesign || est.Finished {
		t.Fatalf("estimate before: %+v err=%v", est, err)
	}
	if e.reviews.writes != 0 {
		t.Fatalf("Estimate must not write")
	}
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	est, _ = e.svc.Estimate(e.ctx, e.req.ID)
	if !est.Exists || est.Stale {
		t.Fatalf("estimate after: %+v", est)
	}
	e.coverage.rows = append(e.coverage.rows, domain.CoverageRow{ACID: "AC-2", TaskID: "t1"})
	est, _ = e.svc.Estimate(e.ctx, e.req.ID)
	if !est.Stale {
		t.Fatalf("coverage changed, estimate must say stale: %+v", est)
	}
}

func TestOutcomeReview_ListNewestFirst_AndRowsDecode(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	finishAll(e)
	e.setStatus(domain.RequestStatusCompleted)
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	l, err := e.svc.List(e.ctx, e.req.ID, 10, "")
	if err != nil || len(l) != 2 {
		t.Fatalf("list %d err=%v", len(l), err)
	}
	phases := []string{string(l[0].Review.Phase), string(l[1].Review.Phase)}
	sort.Strings(phases)
	if phases[0] != "delivered" || phases[1] != "design" || l[0].Review.Phase != domain.ReviewPhaseDelivered {
		t.Fatalf("order %v", phases)
	}
}

type recordingAudit struct{ events []AuditEvent }

func (r *recordingAudit) Record(_ context.Context, e AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

func TestOutcomeReview_AuditsOnlyWhatIsWrittenAndNeverTheContent(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	aud := &recordingAudit{}
	e.svc.Audit = aud
	first, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID}); err != nil { // reused: nothing written
		t.Fatal(err)
	}
	if len(aud.events) != 1 {
		t.Fatalf("one event for one write, got %d", len(aud.events))
	}
	ev := aud.events[0]
	if ev.Action != domain.AuditReviewGenerate || ev.TargetID != e.req.ID || ev.Metadata["review_id"] != first.View.Stored.ID || ev.Metadata["created"] != true {
		t.Fatalf("event %+v", ev)
	}
	for k, v := range ev.Metadata {
		if s, ok := v.(string); ok && strings.Contains(s, "one approval") {
			t.Fatalf("metadata %s quotes review content: %q", k, s)
		}
	}
}

// A token that a person put in a criterion must not reach the stored review, whatever the redactor's coverage elsewhere.
func TestOutcomeReview_RealRedactorKeepsATokenOutOfTheStoredModel(t *testing.T) {
	e := newReviewEnv(t, domain.RequestStatusExecuting)
	token := "ghp_" + strings.Repeat("a1B2", 9)
	e.req.AcceptanceCriteriaJSON = []byte(`[{"id":"AC-1","text":"call the API with ` + token + ` in the header","status":"active"}]`)
	e.setStatus(domain.RequestStatusExecuting)
	e.svc.Redact = RedactRaw
	got, err := e.svc.Generate(e.ctx, GenerateInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got.View.Stored.Model), token) {
		t.Fatalf("the token is in the stored model")
	}
	if !strings.Contains(string(got.View.Stored.Model), "[REDACTED]") {
		t.Fatalf("the redaction must be visible in the model: %s", got.View.Stored.Model)
	}
}
