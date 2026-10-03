package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeLeaseRepository struct {
	mu        sync.Mutex
	tasks     *fakeTaskRepository // ReleaseExecution mutates it with real compare-and-set semantics
	started   []leaseStart
	prevSet   map[string]string // linkID -> status recorded via SetPreviousStatus
	renewals  int
	startErr  error
	renewHeld bool
	renewErr  error

	expired     []domain.ExpiredRun
	legacy      []domain.ExpiredRun
	orphaned    []domain.StuckTask
	claimErr    error
	legacyErr   error
	orphanErr   error
	claimCalls  int
	legacyAge   time.Duration
	orphanGrace time.Duration
}

type leaseStart struct {
	linkID, owner, previousStatus string
	ttl                           time.Duration
}

func (f *fakeLeaseRepository) StartLease(_ context.Context, _, linkID, owner, previousStatus string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return f.startErr
	}
	f.started = append(f.started, leaseStart{linkID, owner, previousStatus, ttl})
	return nil
}

func (f *fakeLeaseRepository) SetPreviousStatus(_ context.Context, _, linkID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prevSet == nil {
		f.prevSet = map[string]string{}
	}
	f.prevSet[linkID] = status
	return nil
}

func (f *fakeLeaseRepository) RenewLease(_ context.Context, _, _, _ string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renewals++
	return f.renewHeld, f.renewErr
}

func (f *fakeLeaseRepository) ClaimExpired(_ context.Context, _ int) ([]domain.ExpiredRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls++
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	out := f.expired
	f.expired = nil // a claimed row is never handed out twice
	return out, nil
}

func (f *fakeLeaseRepository) ClaimLegacyStuck(_ context.Context, olderThan time.Duration, _ int) ([]domain.ExpiredRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.legacyAge = olderThan
	if f.legacyErr != nil {
		return nil, f.legacyErr
	}
	out := f.legacy
	f.legacy = nil
	return out, nil
}

func (f *fakeLeaseRepository) ListOrphanedRuns(_ context.Context, grace time.Duration, _ int) ([]domain.StuckTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orphanGrace = grace
	if f.orphanErr != nil {
		return nil, f.orphanErr
	}
	return f.orphaned, nil // not claimed: ReleaseExecution's compare-and-set makes repeats harmless
}

func (f *fakeLeaseRepository) ReleaseExecution(_ context.Context, tenantID, taskID, linkID string, to domain.Status) (bool, error) {
	f.tasks.mu.Lock()
	defer f.tasks.mu.Unlock()
	t, ok := f.tasks.tasks[taskID]
	if !ok || t.TenantID != tenantID || t.Status != domain.StatusInProgress || t.ActiveExecutionLinkID != linkID {
		return false, nil
	}
	t.Status = to
	f.tasks.tasks[taskID] = t
	return true, nil
}

func (f *fakeLeaseRepository) renewalCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.renewals
}

func newLeases(tasks *fakeTaskRepository) *fakeLeaseRepository {
	return &fakeLeaseRepository{tasks: tasks}
}

func inProgressTask(t *testing.T, tasks *fakeTaskRepository, id, activeLink string) {
	t.Helper()
	task, err := domain.NewTask(id, "tenant-1", "T "+id, domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	task.ActiveExecutionLinkID = activeLink
	tasks.tasks[id] = task
}

func TestRecoverInterruptedExecutions_ExpiredLeaseRestoresPreviousStatus(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "t1", "link-1")
	leases := newLeases(tasks)
	leases.expired = []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1", PreviousStatus: string(domain.StatusReview)}}

	n, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("Execute = %d, %v; want 1, nil", n, err)
	}
	if got := tasks.tasks["t1"].Status; got != domain.StatusReview {
		t.Errorf("want task restored to review, got %s", got)
	}
}

func TestRecoverInterruptedExecutions_UnknownPreviousFallsBackToOpen(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "t1", "link-1")
	leases := newLeases(tasks)
	leases.expired = []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1"}}

	if _, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := tasks.tasks["t1"].Status; got != domain.StatusOpen {
		t.Errorf("want open when previous status unknown, got %s", got)
	}
}

func TestRecoverInterruptedExecutions_LeavesTaskThatMovedOn(t *testing.T) {
	tasks := newFakeTaskRepository()
	done, _ := domain.NewTask("finished", "tenant-1", "f", domain.StatusDone, "", "proj-1")
	done.ActiveExecutionLinkID = "link-1"
	tasks.tasks["finished"] = done
	inProgressTask(t, tasks, "redispatched", "link-NEW") // a newer dispatch owns it now
	leases := newLeases(tasks)
	leases.expired = []domain.ExpiredRun{
		{TenantID: "tenant-1", LinkID: "link-1", TaskID: "finished", PreviousStatus: "open"},
		{TenantID: "tenant-1", LinkID: "link-OLD", TaskID: "redispatched", PreviousStatus: "open"},
	}

	n, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("Execute = %d, %v; want 0, nil", n, err)
	}
	if tasks.tasks["finished"].Status != domain.StatusDone || tasks.tasks["redispatched"].Status != domain.StatusInProgress {
		t.Error("recovery must not touch a task that finished or was re-dispatched")
	}
}

func TestRecoverInterruptedExecutions_LegacyStuckRunIsReleasedWithRunCapMargin(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "old", "link-old")
	leases := newLeases(tasks)
	leases.legacy = []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-old", TaskID: "old"}}

	n, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("Execute = %d, %v; want 1, nil", n, err)
	}
	if tasks.tasks["old"].Status != domain.StatusOpen {
		t.Errorf("want open, got %s", tasks.tasks["old"].Status)
	}
	if leases.legacyAge < 15*time.Minute {
		t.Errorf("legacy threshold %s must exceed the 15-minute run cap or a live run could be swept", leases.legacyAge)
	}
}

func TestRecoverInterruptedExecutions_OrphanedTasks(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "write-lost", "link-a")     // run finished, completion write lost
	inProgressTask(t, tasks, "failed-engine2", "link-b") // coordinator run failed earlier
	inProgressTask(t, tasks, "failed-no-prev", "link-c")
	leases := newLeases(tasks)
	leases.orphaned = []domain.StuckTask{
		{TenantID: "tenant-1", TaskID: "write-lost", LinkID: "link-a", LinkStatus: "completed"},
		{TenantID: "tenant-1", TaskID: "failed-engine2", LinkID: "link-b", LinkStatus: "failed", PreviousStatus: "review"},
		{TenantID: "tenant-1", TaskID: "failed-no-prev", LinkID: "link-c", LinkStatus: "failed"},
	}

	n, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("Execute = %d, %v; want 3, nil", n, err)
	}
	want := map[string]domain.Status{"write-lost": domain.StatusReview, "failed-engine2": domain.StatusReview, "failed-no-prev": domain.StatusOpen}
	for id, st := range want {
		if got := tasks.tasks[id].Status; got != st {
			t.Errorf("%s: want %s, got %s", id, st, got)
		}
	}
	if leases.orphanGrace < time.Minute {
		t.Errorf("orphan grace %s is too short — normal completion writes need time to land", leases.orphanGrace)
	}
	// A second sweep sees the same list but the compare-and-set makes it a no-op.
	if again, _ := NewRecoverInterruptedExecutions(leases).Execute(context.Background()); again != 0 {
		t.Errorf("second sweep must release nothing, released %d", again)
	}
}

func TestRecoverInterruptedExecutions_LaterSweepFailuresDoNotHideEarlierWork(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "t1", "link-1")
	leases := newLeases(tasks)
	leases.expired = []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1"}}
	leases.legacyErr = errors.New("legacy query failed")
	leases.orphanErr = errors.New("orphan query failed")

	n, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("Execute = %d, %v; want 1, nil", n, err)
	}
}

func TestRecoverInterruptedExecutions_ClaimErrorPropagates(t *testing.T) {
	leases := newLeases(newFakeTaskRepository())
	leases.claimErr = errors.New("db down")
	if _, err := NewRecoverInterruptedExecutions(leases).Execute(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestRecoverInterruptedExecutions_LoopSweepsAndStops(t *testing.T) {
	leases := newLeases(newFakeTaskRepository())
	uc := NewRecoverInterruptedExecutions(leases)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { uc.RunRecoveryLoop(ctx, 5*time.Millisecond); close(done) }()

	deadline := time.After(2 * time.Second)
	for {
		leases.mu.Lock()
		calls := leases.claimCalls
		leases.mu.Unlock()
		if calls >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("loop never swept")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not stop on cancel")
	}
}

func TestExecuteTask_DirectAgent_LeasesWithPreviousStatus(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	leases := &fakeLeaseRepository{tasks: tasks, renewHeld: true}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "ref"}, &fakeExecutor{}, grants)
	uc.WithExecutionLeases(leases, "host:1", time.Minute, 20*time.Second)

	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if len(leases.started) != 1 {
		t.Fatalf("want one lease, got %d", len(leases.started))
	}
	got := leases.started[0]
	if got.owner != "host:1" || got.previousStatus != string(domain.StatusOpen) || got.ttl != time.Minute {
		t.Errorf("unexpected lease %+v", got)
	}
}

func TestExecuteTask_DirectAgent_LeaseFailureDoesNotBlockRun(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeSimpleExecutor{ref: "ref"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	uc.WithExecutionLeases(&fakeLeaseRepository{tasks: tasks, startErr: errors.New("no such column")}, "host:1", time.Minute, 0)

	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatalf("a lease failure (e.g. migration not applied) must not fail the run: %v", err)
	}
	if !simple.called {
		t.Error("run must still dispatch")
	}
}

// blockingSimpleExecutor holds the run open so the heartbeat can tick.
type blockingSimpleExecutor struct{ release chan struct{} }

func (b *blockingSimpleExecutor) Execute(_ context.Context, _, _, _, _, _ string) (string, error) {
	<-b.release
	return "ref", nil
}

func TestExecuteTask_DirectAgent_HeartbeatRenewsWhileRunningThenStops(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	leases := &fakeLeaseRepository{tasks: tasks, renewHeld: true}
	simple := &blockingSimpleExecutor{release: make(chan struct{})}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	uc.WithExecutionLeases(leases, "host:1", 300*time.Millisecond, 10*time.Millisecond)

	finished := make(chan struct{})
	go func() {
		_, _ = uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"})
		close(finished)
	}()

	deadline := time.After(2 * time.Second)
	for leases.renewalCount() < 3 {
		select {
		case <-deadline:
			t.Fatalf("heartbeat never renewed while running; renewals=%d", leases.renewalCount())
		case <-time.After(5 * time.Millisecond):
		}
	}
	close(simple.release)
	<-finished

	after := leases.renewalCount()
	time.Sleep(60 * time.Millisecond)
	if leases.renewalCount() != after {
		t.Error("heartbeat must stop once the run finished")
	}
}

type fakeClaimer struct {
	claimed bool
	err     error
	gotFrom []domain.Status
}

func (f *fakeClaimer) ClaimForExecution(_ context.Context, _, _ string, from domain.Status) (bool, error) {
	f.gotFrom = append(f.gotFrom, from)
	return f.claimed, f.err
}

func TestExecuteTask_AtomicClaim_LoserIsRejectedWithoutDispatch(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeSimpleExecutor{ref: "ref"}
	uc, _, _, _, links := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	claimer := &fakeClaimer{claimed: false} // another Execute won the race
	uc.WithExecutionClaim(claimer)

	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_EXECUTE_ALREADY_IN_PROGRESS" {
		t.Fatalf("want TASK_EXECUTE_ALREADY_IN_PROGRESS, got %v", err)
	}
	if simple.called || len(links.created) != 0 {
		t.Errorf("the losing call must not dispatch or write an execution link (called=%v links=%d)", simple.called, len(links.created))
	}
	if len(claimer.gotFrom) != 1 || claimer.gotFrom[0] != domain.StatusOpen {
		t.Errorf("claim must be conditional on the status that was read, got %v", claimer.gotFrom)
	}
}

func TestExecuteTask_AtomicClaim_WinnerDispatches(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeSimpleExecutor{ref: "ref"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	uc.WithExecutionClaim(&fakeClaimer{claimed: true})

	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if !simple.called {
		t.Error("the claim winner must dispatch")
	}
}

func failedReportSetup(t *testing.T, previous string) (*ReportTaskExecutionResult, *fakeTaskRepository, *fakeExecutionLinkRepository) {
	t.Helper()
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineOrchestration, "run-1")
	links.created[0].PreviousStatus = previous
	uc := NewReportTaskExecutionResult(tasks, links).WithExecutionRelease(newLeases(tasks))
	return uc, tasks, links
}

func TestReportTaskExecutionResult_FailureRestoresPreviousStatus(t *testing.T) {
	uc, tasks, links := failedReportSetup(t, string(domain.StatusReview))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration", Success: false, ErrorMessage: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	if got := tasks.tasks["task-1"].Status; got != domain.StatusReview {
		t.Errorf("a failed Engine 2/3 run must not leave the task in_progress; want review, got %s", got)
	}
	if links.created[0].StatusMirror != "failed" {
		t.Errorf("link must be marked failed, got %q", links.created[0].StatusMirror)
	}
}

func TestReportTaskExecutionResult_FailureWithoutRecordedPreviousRestoresToOpen(t *testing.T) {
	uc, tasks, _ := failedReportSetup(t, "")
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration"}); err != nil {
		t.Fatal(err)
	}
	if got := tasks.tasks["task-1"].Status; got != domain.StatusOpen {
		t.Errorf("want open, got %s", got)
	}
}

func TestReportTaskExecutionResult_StaleFailureDoesNotTouchTask(t *testing.T) {
	uc, tasks, links := failedReportSetup(t, "open")
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// A callback for a different run than the task's active one.
	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "some-older-run", Engine: "orchestration"}); err != nil {
		t.Fatal(err)
	}
	if tasks.tasks["task-1"].Status != domain.StatusInProgress || links.created[0].StatusMirror != "in_progress" {
		t.Error("a stale failure callback must change nothing")
	}
}

func TestExecuteTask_ComplexPath_RecordsPreviousStatusForFailureRestore(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "task-1", ToTaskID: "sub", Kind: domain.EdgeKindParentChild}}}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, &fakeSimpleExecutor{}, &fakeExecutor{ref: "run-1"}, grants)
	leases := newLeases(tasks)
	uc.WithExecutionLeases(leases, "host:1", time.Minute, 0)

	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if got := leases.prevSet["link-1"]; got != string(domain.StatusOpen) {
		t.Errorf("complex dispatch must record the pre-dispatch status for a failure report, got %q", got)
	}
	if len(leases.started) != 0 {
		t.Error("Engines 2/3 report back; they must not get a lease that the sweeper would expire")
	}
}

func TestExecuteTask_PromptOverrideRefusedOnComplexTask(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "task-1", ToTaskID: "sub", Kind: domain.EdgeKindParentChild}}}
	complex := &fakeExecutor{ref: "run-1"}
	uc, _, _, _, links := newExecutableExecuteTask(tasks, edges, &fakeSimpleExecutor{}, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "r", Prompt: "Write a spec only"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_EXECUTE_PROMPT_UNSUPPORTED" {
		t.Fatalf("want TASK_EXECUTE_PROMPT_UNSUPPORTED, got %v", err)
	}
	if complex.called || len(links.created) != 0 || len(tasks.updateStatusCalls) != 0 {
		t.Error("a refused request must not dispatch, write a link, or touch the status")
	}

	// Without a prompt the same task still runs on the coordinator.
	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "r2"}); err != nil {
		t.Fatalf("plain run of a complex task must still work: %v", err)
	}
	if !complex.called {
		t.Error("expected the complex executor to run")
	}
}
