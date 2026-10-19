package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const syncChangeID = "req-42-add-ai"

var syncTasksPath = "openspec/changes/" + syncChangeID + "/tasks.md"

const syncTasksMd = `Prose.

- [ ] T9.9 a look-alike line outside the region

<!-- orca:begin plan request=REQ-42 schema=1 -->
## PH-1 first
- [ ] T1.1 one [type=feature] [h=1]
  Done when: one
- [ ] T1.2 two [type=feature] [h=1] [depends=T1.1]
  Done when: two
<!-- orca:end plan -->
`

type syncTasks struct {
	TaskClient
	tasks []domain.TaskView
}

func (f syncTasks) ListTasks(context.Context, ListTasksQuery) ([]domain.TaskView, error) {
	return f.tasks, nil
}

type syncOutcomes struct {
	TaskRunOutcomeRepository
	succeeded map[string]bool
}

func (f syncOutcomes) Exists(_ context.Context, id string, o domain.Outcome) (bool, error) {
	return o == domain.OutcomeSucceeded && f.succeeded[id], nil
}

type syncRequests struct{ req domain.Request }

func (f syncRequests) Get(context.Context, string) (domain.Request, error) { return f.req, nil }

type syncRuns struct {
	AnalysisRunStore
	running bool
}

func (f syncRuns) ListRecent(context.Context, string, int) ([]domain.AnalysisRun, error) {
	if f.running {
		return []domain.AnalysisRun{{Kind: domain.RunKindPlan, Status: domain.RunStatusRunning}}, nil
	}
	return nil, nil
}

type syncTenants struct{ ids []string }

func (f syncTenants) ListTenants(context.Context) ([]string, error) { return f.ids, nil }

// archiveExec stands in for `openspec archive`: it records the call and leaves the paths a real archive would change.
type archiveExec struct {
	ws      *fakeWorkspace
	res     ExecResult
	err     error
	leaves  []ChangedPath
	seen    []ExecInput
	calls   int
	failFor int // the first N calls fail with exit 1
}

func (e *archiveExec) Exec(_ context.Context, _ AnalysisConnection, in ExecInput) (ExecResult, error) {
	e.calls++
	e.seen = append(e.seen, in)
	if e.err != nil {
		return ExecResult{}, e.err
	}
	if e.calls <= e.failFor {
		return ExecResult{ExitCode: 1, Stderr: "spec delta does not apply"}, nil
	}
	if e.res.ExitCode == 0 && !e.res.TimedOut && e.res.SpawnError == "" {
		e.ws.changed = append([]ChangedPath(nil), e.leaves...)
	}
	return e.res, nil
}

type syncRecorder struct{ drift, tasksDone, archiveDone, failed, gaveUp int }

func (r *syncRecorder) TasksDrift() { r.drift++ }
func (r *syncRecorder) SyncFinished(step, result string) {
	switch {
	case result == "gave_up":
		r.gaveUp++
	case result == "failed":
		r.failed++
	case step == "tasks":
		r.tasksDone++
	default:
		r.archiveDone++
	}
}

type syncWorld struct {
	changes *fakeChanges
	ws      *fakeWorkspace
	exec    *archiveExec
	rec     *syncRecorder
	tasks   syncTasks
	done    map[string]bool
	reqs    syncRequests
	runs    syncRuns
	now     time.Time
	s       *TasksMdSyncer
	change  domain.OpenSpecChange
}

func newSyncWorld(t *testing.T) *syncWorld {
	t.Helper()
	w := &syncWorld{changes: newFakeChanges(), ws: newFakeWorkspace(), rec: &syncRecorder{}, done: map[string]bool{}, now: time.Now().UTC()}
	w.exec = &archiveExec{ws: w.ws, leaves: []ChangedPath{
		{Path: "openspec/changes/" + syncChangeID + "/tasks.md", State: "deleted"},
		{Path: "openspec/changes/archive/2026-10-14-" + syncChangeID + "/tasks.md", State: "untracked"},
		{Path: "openspec/specs/ai/spec.md", State: "untracked"},
	}}
	w.reqs = syncRequests{req: domain.Request{ID: "req-1", TenantID: "t", ProjectID: "p", Number: 42, Status: domain.RequestStatusExecuting}}
	w.tasks = syncTasks{tasks: []domain.TaskView{
		{ID: "task-1", Type: "task", Labels: []string{"spec:t1.1"}},
		{ID: "task-2", Type: "task", Labels: []string{"spec:t1.2"}},
		{ID: "phase", Type: domain.TaskTypePhase},
	}}
	w.ws.files[syncTasksPath] = []byte(syncTasksMd)
	w.change = domain.OpenSpecChange{ID: "chg-1", RequestID: w.reqs.req.ID, ChangeID: syncChangeID, Branch: "request/42-proposal", Status: domain.ChangeStatusReady,
		SyncState: domain.SyncStatePending, Bundle: map[string]string{"tasks.md": syncTasksMd}, WorkspaceID: "wt-1"}
	if _, err := w.changes.Upsert(context.Background(), w.change); err != nil {
		t.Fatal(err)
	}
	w.build()
	return w
}

func (w *syncWorld) build() {
	w.s = &TasksMdSyncer{
		Changes: w.changes, Requests: w.reqs, Tasks: w.tasks, Outcomes: syncOutcomes{succeeded: w.done}, Runs: w.runs, Workspace: w.ws,
		Exec: w.exec, Conns: fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}}, Tenants: syncTenants{ids: []string{"t"}},
		Observer: w.rec, Now: func() time.Time { return w.now },
	}
}

func (w *syncWorld) tasksMd() string { return string(w.ws.files[syncTasksPath]) }

func (w *syncWorld) stored() domain.OpenSpecChange {
	c, _, _ := w.changes.GetByRequest(context.Background(), w.reqs.req.ID)
	return c
}

// userSeeingWorkspace records who the context says is acting when the worktree is ensured.
type userSeeingWorkspace struct {
	*fakeWorkspace
	user string
}

func (w *userSeeingWorkspace) Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error) {
	w.user, _ = tenant.UserID(ctx)
	return w.fakeWorkspace.Ensure(ctx, in)
}

// The sweep runs with no user, and project-service refuses to list a repo without one (REQUEST_REPORTER_REQUIRED in the pilot).
func TestTasksMdSyncer_ActsForTheReporterWhenItPreparesTheWorktree(t *testing.T) {
	w := newSyncWorld(t)
	w.reqs.req.ReporterID = "user-reporter"
	spy := &userSeeingWorkspace{fakeWorkspace: w.ws}
	w.done["task-1"] = true
	w.build()
	w.s.Workspace = spy
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	if spy.user != "user-reporter" {
		t.Fatalf("the worktree was prepared as %q", spy.user)
	}
}

func TestTasksMdSyncer_TicksOnlyDoneTasksInsideThePlanRegion(t *testing.T) {
	w := newSyncWorld(t)
	w.done["task-1"] = true
	res, err := w.s.Flush(context.Background(), "req-1")
	if err != nil || res.Ticked != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	md := w.tasksMd()
	if !strings.Contains(md, "- [x] T1.1 one") || !strings.Contains(md, "- [ ] T1.2 two") || !strings.Contains(md, "- [ ] T9.9 a look-alike") {
		t.Fatalf("only T1.1 inside the region may be ticked:\n%s", md)
	}
	if len(w.ws.committed) != 1 || len(w.ws.committed[0]) != 1 || w.ws.committed[0][0] != syncTasksPath {
		t.Fatalf("only tasks.md may be committed: %v", w.ws.committed)
	}
	c := w.stored()
	region, _ := domain.PlanRegionText(md)
	if c.SyncState != domain.SyncStateInSync || c.SyncDigest != domain.PlanRegionDigest(region) || c.SyncAttempts != 0 {
		t.Fatalf("%+v", c)
	}
	if c.BundleDigest() != w.change.BundleDigest() || c.TasksMDDigest != w.change.TasksMDDigest {
		t.Fatal("the stored bundle and its digest are what the plan gate approved; the sync must not touch them")
	}
}

// A Request planned again keeps the tasks of its earlier Plan, with the same spec labels; their progress is not this Plan's.
func TestTasksMdSyncer_IgnoresTheTasksOfAnEarlierPlanOfTheSameRequest(t *testing.T) {
	w := newSyncWorld(t)
	w.reqs.req.PlanTaskID = "plan-new"
	w.tasks.tasks = []domain.TaskView{
		{ID: "plan-old", Type: domain.TaskTypePlan}, {ID: "old-1", Type: "task", ParentID: "plan-old", Labels: []string{"spec:t1.1"}},
		{ID: "plan-new", Type: domain.TaskTypePlan}, {ID: "new-1", Type: "task", ParentID: "plan-new", Labels: []string{"spec:t1.1"}},
		{ID: "new-2", Type: "task", ParentID: "plan-new", Labels: []string{"spec:t1.2"}},
	}
	w.done["old-1"], w.done["new-2"] = true, true // the old T1.1 is done; in the current plan only T1.2 is
	w.build()
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	md := w.tasksMd()
	if !strings.Contains(md, "- [ ] T1.1 one") || !strings.Contains(md, "- [x] T1.2 two") {
		t.Fatalf("only the current plan's progress is ticked:\n%s", md)
	}
}

func TestTasksMdSyncer_AUserDoneTaskAndASecondPassTickWithoutDuplicates(t *testing.T) {
	w := newSyncWorld(t)
	w.tasks.tasks[1].Status = domain.TaskStatusDone // marked done by hand: no run outcome
	w.build()
	if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || res.Ticked != 1 || !strings.Contains(w.tasksMd(), "- [x] T1.2 two") {
		t.Fatalf("%+v %v\n%s", res, err, w.tasksMd())
	}
	if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || res.Ticked != 0 || len(w.ws.committed) != 1 {
		t.Fatalf("a second pass must write nothing: %+v %v commits=%d", res, err, len(w.ws.committed))
	}
}

func TestTasksMdSyncer_NothingDoneWritesNothingAndIsInSync(t *testing.T) {
	w := newSyncWorld(t)
	if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || res.Ticked != 0 || len(w.ws.committed) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if c := w.stored(); c.SyncState != domain.SyncStateInSync || c.SyncDigest == "" {
		t.Fatalf("%+v", c)
	}
}

func TestTasksMdSyncer_RestoresARegionEditedByHand(t *testing.T) {
	w := newSyncWorld(t)
	w.ws.files[syncTasksPath] = []byte(strings.Replace(syncTasksMd, "- [ ] T1.2 two [type=feature] [h=1] [depends=T1.1]", "- [x] T1.2 two [type=feature] [h=9]", 1))
	w.done["task-1"] = true
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	md := w.tasksMd()
	if !strings.Contains(md, "- [ ] T1.2 two [type=feature] [h=1] [depends=T1.1]") || !strings.Contains(md, "- [x] T1.1 one") {
		t.Fatalf("the authored region must be back, with only the done task ticked:\n%s", md)
	}
	if w.rec.drift != 1 {
		t.Fatalf("drift must be counted once: %d", w.rec.drift)
	}
	if !strings.HasPrefix(md, "Prose.") {
		t.Fatal("the text around the region must stay byte for byte")
	}
}

func TestTasksMdSyncer_ProseEditsOutsideTheRegionAreNotDrift(t *testing.T) {
	w := newSyncWorld(t)
	w.ws.files[syncTasksPath] = []byte(strings.Replace(syncTasksMd, "Prose.", "A reviewer's note.", 1))
	w.done["task-1"] = true
	if _, err := w.s.Flush(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	if w.rec.drift != 0 || !strings.HasPrefix(w.tasksMd(), "A reviewer's note.") {
		t.Fatalf("drift=%d\n%s", w.rec.drift, w.tasksMd())
	}
}

func TestTasksMdSyncer_SkipsWhileAPlanRunWrites(t *testing.T) {
	w := newSyncWorld(t)
	w.runs = syncRuns{running: true}
	w.done["task-1"] = true
	w.build()
	if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || res.Ticked != 0 || w.ws.ensured != 0 {
		t.Fatalf("a running plan run owns the worktree: %+v %v ensured=%d", res, err, w.ws.ensured)
	}
}

func TestTasksMdSyncer_OnlyReadyChangesAreTouched(t *testing.T) {
	for _, st := range []domain.ChangeStatus{domain.ChangeStatusPreparing, domain.ChangeStatusArchived, domain.ChangeStatusAbandoned} {
		w := newSyncWorld(t)
		c := w.stored()
		c.Status = st
		_, _ = w.changes.Upsert(context.Background(), c)
		w.done["task-1"] = true
		if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || res.Ticked != 0 || w.ws.ensured != 0 {
			t.Fatalf("%s: %+v %v", st, res, err)
		}
	}
	w := newSyncWorld(t)
	if res, err := w.s.Flush(context.Background(), "unknown"); err != nil || res.Ticked != 0 {
		t.Fatalf("a Request without a change: %+v %v", res, err)
	}
}

func TestTasksMdSyncer_OnTaskSucceededMarksPendingOnceAndNeverReopensAGivenUpChange(t *testing.T) {
	w := newSyncWorld(t)
	c := w.stored()
	_ = w.changes.UpdateSync(context.Background(), c.ID, domain.SyncUpdate{State: domain.SyncStateInSync, Digest: "d"}, c.Version)
	if err := w.s.OnTaskSucceeded(context.Background(), "req-1"); err != nil {
		t.Fatal(err)
	}
	if got := w.stored(); got.SyncState != domain.SyncStatePending || got.SyncDigest != "d" {
		t.Fatalf("%+v", got)
	}
	v := w.stored().Version
	if err := w.s.OnTaskSucceeded(context.Background(), "req-1"); err != nil || w.stored().Version != v {
		t.Fatal("an already pending change must not be rewritten")
	}
	c = w.stored()
	_ = w.changes.UpdateSync(context.Background(), c.ID, domain.SyncUpdate{State: domain.SyncStateFailed, Attempts: domain.MaxSyncAttempts}, c.Version)
	if err := w.s.OnTaskSucceeded(context.Background(), "req-1"); err != nil || w.stored().SyncState != domain.SyncStateFailed {
		t.Fatalf("%+v %v", w.stored(), err)
	}
}

func completeRequest(w *syncWorld) {
	w.reqs.req.Status = domain.RequestStatusCompleted
	w.changes.status[w.reqs.req.ID] = domain.RequestStatusCompleted
	w.done["task-1"], w.done["task-2"] = true, true
	w.build()
}

func TestTasksMdSyncer_ArchivesACompletedRequestAfterTickingEverything(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	res, err := w.s.Flush(context.Background(), "req-1")
	if err != nil || !res.Archived || res.Ticked != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(w.exec.seen) != 1 || w.exec.seen[0].Binary != "openspec" || strings.Join(w.exec.seen[0].Args, " ") != "archive "+syncChangeID+" --yes" || w.exec.seen[0].Cwd != "/srv/wt-1" {
		t.Fatalf("the archive must run in the proposal worktree, not the repo: %+v", w.exec.seen)
	}
	if len(w.ws.committed) != 2 || len(w.ws.committed[1]) != 3 {
		t.Fatalf("one commit for the ticks and one for the archive: %v", w.ws.committed)
	}
	c := w.stored()
	if c.Status != domain.ChangeStatusArchived || c.Commit != "sha-1" || c.SyncState != domain.SyncStateInSync || c.SyncAttempts != 0 {
		t.Fatalf("%+v", c)
	}
	if got, _ := w.changes.ListSyncWork(context.Background(), 10); len(got) != 0 {
		t.Fatalf("an archived change is no work: %+v", got)
	}
}

func TestTasksMdSyncer_ArchiveThatTouchesCodeIsDiscardedAndNeverCommitted(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	w.exec.leaves = append(w.exec.leaves, ChangedPath{Path: "src/main.go", State: "modified"})
	_, err := w.s.Flush(context.Background(), "req-1")
	if err == nil || !strings.Contains(err.Error(), "outside openspec/") || !strings.Contains(err.Error(), "src/main.go") {
		t.Fatalf("%v", err)
	}
	if w.ws.discarded == 0 || len(w.ws.committed) != 1 {
		t.Fatalf("the archive must be discarded and not committed: discarded=%d commits=%v", w.ws.discarded, w.ws.committed)
	}
	if c := w.stored(); c.Status != domain.ChangeStatusReady || c.SyncState != domain.SyncStatePending || c.SyncAttempts != 1 || c.SyncNextRetryAt == nil || c.SyncError == "" {
		t.Fatalf("%+v", c)
	}
	if w.rec.failed != 1 {
		t.Fatalf("%+v", w.rec)
	}
}

func TestTasksMdSyncer_ArchiveFailuresBackOffThenGiveUpWithoutChangingTheRequest(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	w.exec.failFor = 100
	ctx := context.Background()
	for attempt := 1; attempt <= domain.MaxSyncAttempts; attempt++ {
		if _, err := w.s.Flush(ctx, "req-1"); err == nil {
			t.Fatalf("attempt %d must fail", attempt)
		}
		c := w.stored()
		if attempt < domain.MaxSyncAttempts {
			if c.SyncState != domain.SyncStatePending || c.SyncAttempts != attempt || c.SyncNextRetryAt == nil ||
				!c.SyncNextRetryAt.Equal(w.now.Add(domain.SyncBackoff(attempt))) {
				t.Fatalf("attempt %d: %+v", attempt, c)
			}
			// Not due yet: the pass leaves it alone.
			calls := w.exec.calls
			if _, err := w.s.Flush(ctx, "req-1"); err != nil || w.exec.calls != calls {
				t.Fatal("a change that is not due must not run the archive again")
			}
			w.now = w.now.Add(domain.SyncBackoff(attempt) + time.Second)
		}
	}
	c := w.stored()
	if c.SyncState != domain.SyncStateFailed || c.Status != domain.ChangeStatusReady || w.reqs.req.Status != domain.RequestStatusCompleted || w.rec.gaveUp != 1 {
		t.Fatalf("%+v %+v", c, w.rec)
	}
	calls := w.exec.calls
	if _, err := w.s.Flush(ctx, "req-1"); err != nil || w.exec.calls != calls {
		t.Fatal("a change that was given up on is not retried")
	}
}

func TestTasksMdSyncer_ArchiveRecoversAfterAFailure(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	w.exec.failFor = 1
	if _, err := w.s.Flush(context.Background(), "req-1"); err == nil {
		t.Fatal("the first archive must fail")
	}
	w.now = w.now.Add(time.Minute)
	if res, err := w.s.Flush(context.Background(), "req-1"); err != nil || !res.Archived {
		t.Fatalf("%+v %v", res, err)
	}
	if c := w.stored(); c.Status != domain.ChangeStatusArchived || c.SyncAttempts != 0 || c.SyncError != "" {
		t.Fatalf("%+v", c)
	}
}

func TestTasksMdSyncer_ArchiveExecProblemsAreFailuresNotPanics(t *testing.T) {
	cases := map[string]ExecResult{
		"not installed": {SpawnError: "ENOENT"},
		"timed out":     {TimedOut: true},
	}
	for name, res := range cases {
		w := newSyncWorld(t)
		completeRequest(w)
		w.exec.res = res
		if _, err := w.s.Flush(context.Background(), "req-1"); err == nil || w.stored().Status != domain.ChangeStatusReady {
			t.Fatalf("%s: %v %+v", name, err, w.stored())
		}
	}
	w := newSyncWorld(t)
	completeRequest(w)
	w.exec.err = errors.New("dev server offline")
	if _, err := w.s.Flush(context.Background(), "req-1"); err == nil {
		t.Fatal("a transport error must be a failure")
	}
}

func TestTasksMdSyncer_AnArchiveThatChangesNothingIsAFailure(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	w.exec.leaves = nil
	if _, err := w.s.Flush(context.Background(), "req-1"); err == nil || w.stored().Status != domain.ChangeStatusReady {
		t.Fatalf("%v", err)
	}
}

func TestTasksMdSyncer_SweepFindsPendingAndCompletedWorkAcrossTenants(t *testing.T) {
	w := newSyncWorld(t)
	completeRequest(w)
	sum, err := w.s.Sweep(syncTenantCtx())
	if err != nil || sum.Looked != 1 || sum.Archived != 1 || sum.Ticked != 2 || sum.Failed != 0 {
		t.Fatalf("%+v %v", sum, err)
	}
	if sum, _ := w.s.Sweep(syncTenantCtx()); sum.Looked != 0 {
		t.Fatalf("nothing is left to do: %+v", sum)
	}
}

func syncTenantCtx() context.Context { return tenant.WithTenantID(context.Background(), "t") }
