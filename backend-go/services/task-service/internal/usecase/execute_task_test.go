package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// newExecuteTaskForTest wires a real ResolvePermission (not a fake) so
// ExecuteTask's permission pre-check (TASK-TG-04-01) is genuinely
// exercised — repo is expected to already have "task-1" (or whatever
// TaskID a test uses) seeded with OwnerID "user-1" so the owner-intrinsic
// short-circuit grants the caller access by default.
func newExecuteTaskForTest(repo *fakeTaskRepository, edges *fakeEdgeRepository, simple SimpleExecutor, complex ComplexExecutor) *ExecuteTask {
	resolvePermission := NewResolvePermission(repo, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	return NewExecuteTask(repo, edges, simple, complex, resolvePermission)
}

// seedOwnedTask puts a task-1 (or the given id) owned by "user-1" into repo
// — every test in this file calls Execute as "user-1" via withIdentity.
func seedOwnedTask(repo *fakeTaskRepository, id string) {
	repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", OwnerID: "user-1", Status: domain.StatusOpen}
}

func TestExecuteTask_RequiresTenantContext(t *testing.T) {
	uc := newExecuteTaskForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
	_, err := uc.Execute(context.Background(), ExecuteTaskInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestExecuteTask_SimplePath_NoSubtasksNoDependencies is the core branching
// regression: a task with neither parent_child nor depends_on edges FROM it
// must dispatch to SimpleExecutor, never ComplexExecutor.
func TestExecuteTask_SimplePath_NoSubtasksNoDependencies(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "infra-fleet-ref-1" {
		t.Errorf("expected the simple executor's ref, got %q", ref)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called")
	}
	if complex.called {
		t.Error("expected ComplexExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasSubtasks(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
	}
	if !complex.called {
		t.Error("expected ComplexExecutor to be called")
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasDependencies(t *testing.T) {
	repo := newFakeTaskRepository()
	seedOwnedTask(repo, "task-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "blocking-task", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := newExecuteTaskForTest(repo, edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
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
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
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
	uc := newExecuteTaskForTest(repo, edges, simple, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
}

func TestExecuteTask_RequiresTaskID(t *testing.T) {
	uc := newExecuteTaskForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
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
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, &fakeExecutor{})
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
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
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
	uc := newExecuteTaskForTest(repo, &fakeEdgeRepository{}, simple, &fakeExecutor{})
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
	uc := NewExecuteTask(repo, &fakeEdgeRepository{}, simple, &fakeExecutor{}, resolvePermission)
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
}
