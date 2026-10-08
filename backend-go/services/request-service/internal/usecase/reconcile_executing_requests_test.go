package usecase

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func reconcileOnce(t *testing.T, r *exRig, reqs ...domain.Request) int {
	t.Helper()
	r.scanner.refs = nil
	for _, q := range reqs {
		r.scanner.refs = append(r.scanner.refs, ExecutingRef{TenantID: "t1", RequestID: q.ID})
	}
	n, err := r.reconcile.Execute(lcCtx())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return n
}

func TestReconcile_ReDispatchesOpenTaskWithoutRun(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	// The started event never arrived and nothing dispatched it: only task state says the work is waiting.
	if reconcileOnce(t, r, req) != 1 {
		t.Fatal("the request should have been handled")
	}
	if r.tasks.get(task.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("reconcile must dispatch the open task: %v", r.tasks.calls)
	}
}

func TestReconcile_ReturnsBacklogAfterRetryWindow(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	boom := &domain.DispatchError{Code: "TASK_EXECUTE_WORKTREE_FAILED", Err: domain.ErrTaskDispatchTransient}
	r.tasks.executeErrs[task.ID] = []error{boom, boom, boom}

	reconcileOnce(t, r, req)
	r.clock = r.clock.Add(10 * 60 * 1e9)
	reconcileOnce(t, r, req)
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("10 minutes is inside the window, got %s", got.Status)
	}
	r.clock = r.clock.Add(10 * 60 * 1e9)
	reconcileOnce(t, r, req)
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryBlockedDependency {
		t.Fatalf("20 minutes of dispatch errors returns the request, got %+v", got)
	}
}

func TestReconcile_EmitsPhaseCompletedOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1 := r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	_, _ = r.starts.TryStart(lcCtx(), domain.PhaseStart{PhaseTaskID: p1.ID, RequestID: req.ID, StartedBy: "u"})
	// The phase finished while the consumer was off: the derived-done event was lost.
	r.tasks.mu.Lock()
	r.tasks.setStatusLocked(t1.ID, domain.TaskStatusDone)
	r.tasks.mu.Unlock()

	for i := 0; i < 3; i++ {
		reconcileOnce(t, r, req)
	}
	if n := countSubject(r, domain.SubjectPhaseCompleted); n != 1 {
		t.Fatalf("phase.completed must be emitted once, got %d", n)
	}
	pending := 0
	for _, a := range r.gates.approvals {
		if a.SubjectType == domain.SubjectPhase && a.SubjectID == p2.ID && a.Status == domain.ApprovalStatusPending {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("one approval for the next phase, got %d", pending)
	}
}

func TestReconcile_CompletesRequestWhenAllDone(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusReview)
	_ = task
	reconcileOnce(t, r, req)
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("a request whose task sits in review with auto-complete on must finish, got %s", got.Status)
	}
}

func TestReconcile_SkipsRecentActivityAndOtherStates(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusRequestBacklog)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	if reconcileOnce(t, r, req) != 1 {
		t.Fatal("handled (leased) even though nothing to do")
	}
	if len(r.tasks.calls) != 0 {
		t.Fatalf("a request that left executing is left alone: %v", r.tasks.calls)
	}
	r.scanner.refs = nil
	n, err := r.reconcile.Execute(lcCtx())
	if err != nil || n != 0 {
		t.Fatalf("an empty scan handles nothing: %d %v", n, err)
	}
}

func TestReconcile_TwoReplicasDoNotShareARequest(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.scanner.refs = []ExecutingRef{{TenantID: "t1", RequestID: req.ID}}

	a := *r.reconcile
	a.Owner = "replica-a"
	b := *r.reconcile
	b.Owner = "replica-b"
	// Replica A holds the lease while B scans.
	if ok, _ := r.leases.Claim(tenant.WithTenantID(context.Background(), "t1"), req.ID, "replica-a", 0, 0); !ok {
		t.Fatal("setup claim")
	}
	n, err := b.Execute(lcCtx())
	if err != nil || n != 0 {
		t.Fatalf("B must skip a leased request: %d %v", n, err)
	}
	if r.tasks.count("execute:") != 0 {
		t.Fatal("B dispatched under A's lease")
	}
}

func TestReconcile_ConcurrentPassesDispatchOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.scanner.refs = []ExecutingRef{{TenantID: "t1", RequestID: req.ID}}
	var wg sync.WaitGroup
	var handled atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := r.reconcile.Execute(lcCtx())
			if err != nil {
				t.Error(err)
			}
			handled.Add(int32(n))
		}()
	}
	wg.Wait()
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("Execute must run once however many passes race: %v", r.tasks.calls)
	}
	if handled.Load() < 1 {
		t.Fatal("someone must have handled the request")
	}
}

func TestRunReconcileLoop_StopsWithContext(t *testing.T) {
	r := newExRig(t)
	ctx, cancel := context.WithCancel(lcCtx())
	done := make(chan struct{})
	go func() { r.reconcile.RunReconcileLoop(ctx, 1e6); close(done) }()
	cancel()
	<-done
}
