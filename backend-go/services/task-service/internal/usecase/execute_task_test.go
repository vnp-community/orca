package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// newExecuteTaskForTest wires a real ResolvePermission (not a fake) so
// ExecuteTask's permission pre-check (TASK-TG-04-01) is genuinely
// exercised — repo is expected to already have "task-1" (or whatever
// TaskID a test uses) seeded with OwnerID "user-1" so the owner-intrinsic
// short-circuit grants the caller access by default. worktrees/resolver
// default to permissive fakes; clock defaults to a fixed instant (so
// actual_hours math is deterministic without every test needing to care).
func newExecuteTaskForTest(repo *fakeTaskRepository, edges *fakeEdgeRepository, simple SimpleExecutor, complex ComplexExecutor) *ExecuteTask {
	resolvePermission := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	worktrees := &fakeWorktreeProvisioner{worktreeID: "wt-1", path: "/srv/worktrees/wt-1"}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}
	clock := &fakeClock{now: time.Unix(1000, 0)}
	return NewExecuteTask(repo, edges, simple, complex, resolvePermission, worktrees, resolver, clock)
}

// seedOwnedTask puts a task-1 (or the given id) owned by "user-1" into repo
// — every test in this file calls Execute as "user-1" via withIdentity.
func seedOwnedTask(repo *fakeTaskRepository, id string) {
	repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
}

func TestExecuteTask_RequiresTenantContext(t *testing.T) {
	uc := newExecuteTaskForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeComplexExecutor{})
	_, err := uc.Execute(context.Background(), ExecuteTaskInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestExecuteTask_SimplePath_NoSubtasksNoDependencies is the core branching
// regression: a task with neither parent_child nor depends_on edges FROM it
// must dispatch to SimpleExecutor, never ComplexExecutor. Also covers
// TASK-TG-04-03's inline completion: the simple path writes StatusReview +
// a non-zero actual_hours in the SAME call, Async=false.
func TestExecuteTask_SimplePath_NoSubtasksNoDependencies(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeComplexExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionRef != "infra-fleet-ref-1" {
		t.Errorf("expected the simple executor's ref, got %q", result.ExecutionRef)
	}
	if result.Async {
		t.Error("expected Async=false for the simple path")
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called")
	}
	if complex.called {
		t.Error("expected ComplexExecutor NOT to be called")
	}
	if len(repo.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly 1 CompleteExecution call, got %d: %+v", len(repo.completeExecutionCalls), repo.completeExecutionCalls)
	}
	call := repo.completeExecutionCalls[0]
	if call.status != domain.StatusReview {
		t.Errorf("expected CompleteExecution status=review, got %q", call.status)
	}
	if call.actualHours < 0 {
		t.Errorf("expected a non-negative actual_hours, got %v", call.actualHours)
	}
}

func TestExecuteTask_ComplexPath_HasSubtasks(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeComplexExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionRef != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", result.ExecutionRef)
	}
	if !result.Async {
		t.Error("expected Async=true for the complex path")
	}
	if !complex.called {
		t.Error("expected ComplexExecutor to be called")
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called")
	}
	if len(repo.completeExecutionCalls) != 0 {
		t.Errorf("expected NO inline CompleteExecution call for the complex (async) path, got %+v", repo.completeExecutionCalls)
	}
	// Status stays at in_progress — no further write until
	// ReportTaskExecutionResult (TASK-TG-04-05).
	if got := repo.tasks["task-1"].Status; got != domain.StatusInProgress {
		t.Errorf("expected status to remain in_progress after complex dispatch, got %q", got)
	}
}

func TestExecuteTask_ComplexPath_HasDependencies(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "blocking-task", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeComplexExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionRef != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", result.ExecutionRef)
	}
	if !complex.called || simple.called {
		t.Errorf("expected only ComplexExecutor to be called, complex=%v simple=%v", complex.called, simple.called)
	}
}

func TestExecuteTask_IgnoresEdgesToTheTaskWhenDecidingComplexity(t *testing.T) {
	// task-1 is someone else's dependency (an edge TO it, not FROM it) —
	// that must not make task-1 itself "complex".
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "other-task", ToTaskID: "task-1", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeComplexExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !simple.called || complex.called {
		t.Errorf("expected only SimpleExecutor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

func TestExecuteTask_ExecutorFailurePropagates(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{err: errors.New("infra-fleet-service unavailable")}
	uc := newExecuteTaskForTest(repo, edges, simple, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
}

func TestExecuteTask_RequiresTaskID(t *testing.T) {
	uc := newExecuteTaskForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error for an empty task_id")
	}
}

// TestExecuteTask_MarksTaskInProgressBeforeDispatching is the regression for
// this usecase's real state transition (see execute_task.go's doc comment):
// Execute must call TaskRepository.UpdateStatus(StatusInProgress) before
// handing off to either executor.
func TestExecuteTask_MarksTaskInProgressBeforeDispatching(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.updateStatusCalls) != 1 {
		t.Fatalf("expected exactly one UpdateStatus call, got %d: %+v", len(repo.updateStatusCalls), repo.updateStatusCalls)
	}
	got := repo.updateStatusCalls[0]
	if got.tenantID != "tenant-1" || got.id != "task-1" || got.status != domain.StatusInProgress {
		t.Errorf("unexpected UpdateStatus call: %+v", got)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called after the status update")
	}
}

// TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch: since the
// status transition is the entire point of this usecase's Epic C addition,
// a failure to persist it must fail Execute outright rather than silently
// dispatching with no recorded state.
func TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	repo.updateStatusErr = errors.New("db unavailable")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeComplexExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error when the status update fails")
	}
	if simple.called || complex.called {
		t.Errorf("expected neither executor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

// TestExecuteTask_DispatchFailure_RevertsStatusToPrevious is TASK-TG-04-01's
// core regression: a dispatch failure must revert the in_progress write to
// the task's PREVIOUS status, not strand it in_progress forever.
func TestExecuteTask_DispatchFailure_RevertsStatusToPrevious(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["task-1"] = domain.Task{ID: "task-1", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusReview}
	simple := &fakeExecutor{err: errors.New("dev server offline")}
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error when dispatch fails")
	}

	if len(repo.updateStatusCalls) != 2 {
		t.Fatalf("expected exactly 2 UpdateStatus calls (in_progress then revert), got %d: %+v", len(repo.updateStatusCalls), repo.updateStatusCalls)
	}
	if repo.updateStatusCalls[0].status != domain.StatusInProgress {
		t.Errorf("expected first call to set in_progress, got %+v", repo.updateStatusCalls[0])
	}
	if repo.updateStatusCalls[1].status != domain.StatusReview {
		t.Errorf("expected second call to revert to the previous status (review), got %+v", repo.updateStatusCalls[1])
	}
	// The fake mutates its map on every UpdateStatus call, so the final
	// persisted status must be the reverted one, not in_progress.
	if got := repo.tasks["task-1"].Status; got != domain.StatusReview {
		t.Errorf("expected persisted status to be reverted to review, got %q", got)
	}
}

// TestExecuteTask_PermissionDenied_NeverWritesStatus is the other
// TASK-TG-04-01 regression guard: a ResolvePermission denial must
// short-circuit BEFORE any UpdateStatus call at all — the false-in_progress
// bug this fix closes.
func TestExecuteTask_PermissionDenied_NeverWritesStatus(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["task-1"] = domain.Task{ID: "task-1", TenantID: "tenant-1", OwnerID: "someone-else", Status: domain.StatusOpen}
	resolvePermission := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	worktrees := &fakeWorktreeProvisioner{worktreeID: "wt-1"}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}
	clock := &fakeClock{now: time.Unix(1000, 0)}
	uc := NewExecuteTask(repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{}, resolvePermission, worktrees, resolver, clock)
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	_, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected PermissionDenied for a caller with no execute access")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
	if len(repo.updateStatusCalls) != 0 {
		t.Errorf("expected NO UpdateStatus calls for a denied caller, got %+v", repo.updateStatusCalls)
	}
	if simple.called {
		t.Error("expected the executor to never be called for a denied caller")
	}
	if worktrees.called {
		t.Error("expected EnsureWorktree to never be called for a denied caller")
	}
}

// TestExecuteTask_NoConnection_ReturnsFailedPrecondition: a disconnected
// project must fail BEFORE the in_progress write, per TASK-TG-04-03's
// pre-check ordering.
func TestExecuteTask_NoConnection_ReturnsFailedPrecondition(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	resolvePermission := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	worktrees := &fakeWorktreeProvisioner{worktreeID: "wt-1"}
	resolver := &fakeProjectExecutionResolver{connected: false}
	clock := &fakeClock{now: time.Unix(1000, 0)}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc := NewExecuteTask(repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{}, resolvePermission, worktrees, resolver, clock)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected an error when the project has no connected dev server")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
	if len(repo.updateStatusCalls) != 0 {
		t.Errorf("expected NO UpdateStatus calls when there's no connection, got %+v", repo.updateStatusCalls)
	}
	if worktrees.called {
		t.Error("expected EnsureWorktree to never be called when there's no connection")
	}
	if simple.called {
		t.Error("expected the executor to never be called when there's no connection")
	}
}

// TestExecuteTask_ExistingWorktreeID_NeverCallsCreateBranch: a task with an
// existing WorktreeID never triggers EnsureWorktree's create branch — the
// fake provisioner reuses task.WorktreeID directly and Execute must not
// re-persist an unchanged worktree id.
func TestExecuteTask_ExistingWorktreeID_NeverCallsCreateBranch(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["task-1"] = domain.Task{ID: "task-1", TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen, WorktreeID: "existing-wt"}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, &fakeComplexExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// UpdateWorktreeID must NOT have been called — the worktree id didn't
	// change (reuse case).
	if got := repo.tasks["task-1"].WorktreeID; got != "existing-wt" {
		t.Errorf("expected the existing worktree id to remain unchanged, got %q", got)
	}
}
