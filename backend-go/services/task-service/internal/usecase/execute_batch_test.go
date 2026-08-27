package usecase

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// newExecuteBatchForTest wires an ExecuteBatch whose per-task dispatch goes
// through a real ExecuteTask (permissive fakes throughout, owner-intrinsic
// short-circuit for every seeded task) so this file exercises the real
// wave-sequencing/concurrency logic, not a mocked-out dispatch step.
//
// Both simple AND complex are wired: ExecuteTask.isComplex routes any task
// with an outgoing depends_on edge to the COMPLEX path (that's what makes
// it complex in the first place) — a diamond/chain dependency batch
// necessarily exercises both executors, never just "simple", so every test
// in this file that cares about dispatch order/failure tracks through
// whichever of the two a given task actually takes.
func newExecuteBatchForTest(t *testing.T, repo *fakeTaskRepository, edges *fakeEdgeRepository, simple SimpleExecutor, complex ComplexExecutor) *ExecuteBatch {
	t.Helper()
	resolvePermission := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	worktrees := &fakeWorktreeProvisioner{worktreeID: "wt-1"}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}
	clock := &fakeClock{now: time.Unix(1000, 0)}
	executeTask := NewExecuteTask(repo, edges, simple, complex, resolvePermission, worktrees, resolver, clock)
	return NewExecuteBatch(edges, executeTask)
}

// sharedTracker records dispatch order/calls across BOTH the simple and
// complex executor wrappers below, so a test can assert on "was taskID
// dispatched" / "in what order" without caring which path a given task
// happened to take.
type sharedTracker struct {
	mu     sync.Mutex
	order  []string
	called []string
}

func (s *sharedTracker) record(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order = append(s.order, taskID)
	s.called = append(s.called, taskID)
}

func (s *sharedTracker) calledFor(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.called {
		if id == taskID {
			return true
		}
	}
	return false
}

func (s *sharedTracker) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

// orderTrackingSimpleExecutor/orderTrackingComplexExecutor both record into
// the same sharedTracker — a diamond-graph batch's non-leaf tasks dispatch
// via the complex wrapper, leaf tasks via the simple one, and both are
// exercised together.
type orderTrackingSimpleExecutor struct{ tracker *sharedTracker }

func (f *orderTrackingSimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	f.tracker.record(taskID)
	return "ref-" + taskID, nil
}

type orderTrackingComplexExecutor struct{ tracker *sharedTracker }

func (f *orderTrackingComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	f.tracker.record(taskID)
	return "ref-" + taskID, nil
}

// failingSimpleExecutor/failingComplexExecutor share a tracker AND a
// failFor task id — whichever path failFor actually dispatches through
// fails, the other succeeds; both record every task they were called for.
type failingSimpleExecutor struct {
	tracker *sharedTracker
	failFor string
}

func (f *failingSimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	f.tracker.record(taskID)
	if taskID == f.failFor {
		return "", errors.New("boom")
	}
	return "ref-" + taskID, nil
}

type failingComplexExecutor struct {
	tracker *sharedTracker
	failFor string
}

func (f *failingComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	f.tracker.record(taskID)
	if taskID == f.failFor {
		return "", errors.New("boom")
	}
	return "ref-" + taskID, nil
}

// concurrencyTrackingExecutor records the peak number of concurrently
// in-flight Execute calls — lets TestExecuteBatch_BoundedConcurrency assert
// MaxConcurrency is actually enforced, not just "eventually all run". Used
// as this test's SimpleExecutor; every task in that test is independent
// (no depends_on edges at all), so all of them take the simple path.
type concurrencyTrackingExecutor struct {
	mu          sync.Mutex
	inFlight    int32
	peak        int32
	callCount   int32
	wantOverlap int32
}

func (f *concurrencyTrackingExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	atomic.AddInt32(&f.callCount, 1)
	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		p := atomic.LoadInt32(&f.peak)
		if cur <= p || atomic.CompareAndSwapInt32(&f.peak, p, cur) {
			break
		}
	}
	// Give other goroutines a chance to start concurrently before this one
	// finishes, so peak reflects real overlap rather than accidental
	// serialization.
	if f.wantOverlap > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	atomic.AddInt32(&f.inFlight, -1)
	return "ref-" + taskID, nil
}

func TestExecuteBatch_RequiresTenantContext(t *testing.T) {
	repo := newFakeTaskRepository()
	tracker := &sharedTracker{}
	uc := newExecuteBatchForTest(t, repo, &fakeEdgeRepository{}, &orderTrackingSimpleExecutor{tracker: tracker}, &orderTrackingComplexExecutor{tracker: tracker})
	if _, err := uc.Execute(context.Background(), ExecuteBatchInput{TaskIDs: []string{"t1"}}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestExecuteBatch_DiamondDependency_RespectsWaveOrder confirms tasks
// dispatch in dependency-respecting waves — a task never dispatches before
// its depends_on target has completed. A(depends on B,C), B/C(depend on D),
// D(no deps): A and B and C all have an outgoing depends_on edge, so they
// dispatch via the COMPLEX path; D (a leaf, no outgoing depends_on edge)
// dispatches via the SIMPLE path — both wrappers share one tracker.
func TestExecuteBatch_DiamondDependency_RespectsWaveOrder(t *testing.T) {
	repo := newFakeTaskRepository()
	for _, id := range []string{"A", "B", "C", "D"} {
		repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "A", ToTaskID: "C", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "D", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "C", ToTaskID: "D", Kind: domain.EdgeKindDependsOn},
	}}
	tracker := &sharedTracker{}
	uc := newExecuteBatchForTest(t, repo, edges, &orderTrackingSimpleExecutor{tracker: tracker}, &orderTrackingComplexExecutor{tracker: tracker})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteBatchInput{TaskIDs: []string{"A", "B", "C", "D"}, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Completed) != 4 {
		t.Fatalf("expected all 4 tasks to complete, got %+v", result)
	}

	order := tracker.snapshot()
	dIdx, bIdx, cIdx, aIdx := indexOf(order, "D"), indexOf(order, "B"), indexOf(order, "C"), indexOf(order, "A")
	if dIdx >= bIdx || dIdx >= cIdx {
		t.Errorf("expected D to dispatch before B and C, got order %v", order)
	}
	if bIdx >= aIdx || cIdx >= aIdx {
		t.Errorf("expected B and C to dispatch before A, got order %v", order)
	}
}

// TestExecuteBatch_BoundedConcurrency: a wave of independent tasks must not
// exceed MaxConcurrency concurrently in-flight dispatches.
func TestExecuteBatch_BoundedConcurrency(t *testing.T) {
	repo := newFakeTaskRepository()
	ids := []string{"A", "B", "C", "D", "E", "F"}
	for _, id := range ids {
		repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	}
	simple := &concurrencyTrackingExecutor{wantOverlap: 1}
	uc := newExecuteBatchForTest(t, repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteBatchInput{TaskIDs: ids, MaxConcurrency: 2, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Completed) != len(ids) {
		t.Fatalf("expected all tasks to complete, got %+v", result)
	}
	if atomic.LoadInt32(&simple.callCount) != int32(len(ids)) {
		t.Errorf("expected %d Execute calls, got %d", len(ids), simple.callCount)
	}
	if atomic.LoadInt32(&simple.peak) > 2 {
		t.Errorf("expected peak concurrency <= 2 (MaxConcurrency), got %d", simple.peak)
	}
}

// TestExecuteBatch_StopOnFailure_HaltsBeforeNextWave: a failure within a
// wave must prevent the NEXT wave from ever dispatching when
// StopOnFailure=true. B depends on A -> wave0=[A] (simple path, A has no
// outgoing depends_on edge), wave1=[B] (complex path, B does).
func TestExecuteBatch_StopOnFailure_HaltsBeforeNextWave(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["A"] = domain.Task{ID: "A", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	repo.tasks["B"] = domain.Task{ID: "B", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "B", ToTaskID: "A", Kind: domain.EdgeKindDependsOn},
	}}
	tracker := &sharedTracker{}
	uc := newExecuteBatchForTest(t, repo, edges,
		&failingSimpleExecutor{tracker: tracker, failFor: "A"},
		&failingComplexExecutor{tracker: tracker, failFor: "A"})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteBatchInput{TaskIDs: []string{"A", "B"}, StopOnFailure: true, RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected an error when StopOnFailure=true and a wave has a failure")
	}
	if _, failed := result.Failed["A"]; !failed {
		t.Errorf("expected A to be recorded as failed, got %+v", result)
	}
	if tracker.calledFor("B") {
		t.Error("expected B (the next wave) to never dispatch after A's failure with StopOnFailure=true")
	}
}

// TestExecuteBatch_StopOnFailureFalse_ContinuesToNextWave is the mirror
// case: StopOnFailure=false lets subsequent waves still run.
func TestExecuteBatch_StopOnFailureFalse_ContinuesToNextWave(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["A"] = domain.Task{ID: "A", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	repo.tasks["B"] = domain.Task{ID: "B", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "B", ToTaskID: "A", Kind: domain.EdgeKindDependsOn},
	}}
	tracker := &sharedTracker{}
	uc := newExecuteBatchForTest(t, repo, edges,
		&failingSimpleExecutor{tracker: tracker, failFor: "A"},
		&failingComplexExecutor{tracker: tracker, failFor: "A"})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteBatchInput{TaskIDs: []string{"A", "B"}, StopOnFailure: false, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracker.calledFor("B") {
		t.Error("expected B to still dispatch in the next wave when StopOnFailure=false")
	}
	if _, failed := result.Failed["A"]; !failed {
		t.Errorf("expected A to be recorded as failed, got %+v", result)
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
