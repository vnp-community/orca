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
	mu         sync.Mutex
	started    []leaseStart
	renewals   int
	startErr   error
	renewHeld  bool
	renewErr   error
	expired    []domain.ExpiredRun
	claimErr   error
	claimCalls int
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

func (f *fakeLeaseRepository) renewalCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.renewals
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

func TestRecoverInterruptedExecutions_RevertsToPreviousStatus(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "t1", "link-1")
	leases := &fakeLeaseRepository{expired: []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1", PreviousStatus: string(domain.StatusReview)}}}

	n, err := NewRecoverInterruptedExecutions(leases, tasks).Execute(context.Background())
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
	leases := &fakeLeaseRepository{expired: []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1"}}}

	if _, err := NewRecoverInterruptedExecutions(leases, tasks).Execute(context.Background()); err != nil {
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
	leases := &fakeLeaseRepository{expired: []domain.ExpiredRun{
		{TenantID: "tenant-1", LinkID: "link-1", TaskID: "finished", PreviousStatus: "open"},
		{TenantID: "tenant-1", LinkID: "link-OLD", TaskID: "redispatched", PreviousStatus: "open"},
	}}

	n, err := NewRecoverInterruptedExecutions(leases, tasks).Execute(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("Execute = %d, %v; want 0, nil", n, err)
	}
	if tasks.tasks["finished"].Status != domain.StatusDone || tasks.tasks["redispatched"].Status != domain.StatusInProgress {
		t.Error("recovery must not touch a task that finished or was re-dispatched")
	}
}

func TestRecoverInterruptedExecutions_ClaimedOnceAcrossSweeps(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "t1", "link-1")
	leases := &fakeLeaseRepository{expired: []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "t1", PreviousStatus: "open"}}}
	uc := NewRecoverInterruptedExecutions(leases, tasks)

	first, _ := uc.Execute(context.Background())
	second, _ := uc.Execute(context.Background())
	if first != 1 || second != 0 {
		t.Errorf("want 1 then 0, got %d then %d", first, second)
	}
}

func TestRecoverInterruptedExecutions_ClaimErrorPropagates(t *testing.T) {
	leases := &fakeLeaseRepository{claimErr: errors.New("db down")}
	if _, err := NewRecoverInterruptedExecutions(leases, newFakeTaskRepository()).Execute(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestRecoverInterruptedExecutions_LoopSweepsAndStops(t *testing.T) {
	leases := &fakeLeaseRepository{}
	uc := NewRecoverInterruptedExecutions(leases, newFakeTaskRepository())
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
	leases := &fakeLeaseRepository{renewHeld: true}
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
	uc.WithExecutionLeases(&fakeLeaseRepository{startErr: errors.New("no such column")}, "host:1", time.Minute, 0)

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
	leases := &fakeLeaseRepository{renewHeld: true}
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
