package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// seedExecutableTask creates a task ("tenant-1"/"proj-1"-shaped, unless
// projectID overrides) with an owner grant directly on it for "user-1" —
// the minimum ResolvePermission needs to authorize the "execute" action
// (see ResolvePermission's doc comment: chain[0] must be the task itself,
// and a grant directly on the target task counts regardless of ApplyTree).
func seedExecutableTask(t *testing.T, tasks *fakeTaskRepository, grants *fakeGrantRepository, id, projectID string) {
	t.Helper()
	task, err := domain.NewTask(id, "tenant-1", "Task "+id, domain.StatusOpen, "", projectID)
	if err != nil {
		t.Fatalf("building task %s: %v", id, err)
	}
	tasks.tasks[id] = task
	grants.grants = append(grants.grants, domain.Grant{TaskID: id, SubjectID: "user-1", Level: domain.GrantLevelOwner, ApplyTree: false})
}

// newExecutableExecuteTask wires an ExecuteTask usecase whose permission
// check, connection resolution, and worktree provisioning all succeed by
// default (a project with a connected dev server and a task-service that
// successfully reuses/creates a worktree) — individual tests override
// whichever of the returned fakes they're exercising.
func newExecutableExecuteTask(tasks *fakeTaskRepository, edges *fakeEdgeRepository, simple SimpleExecutor, complex ComplexExecutor, grants *fakeGrantRepository) (uc *ExecuteTask, worktrees *fakeWorktreeProvisioner, resolver *fakeProjectExecutionResolver, clock *fakeClock, links *fakeExecutionLinkRepository) {
	return newExecutableExecuteTaskWithWorkflow(tasks, edges, simple, complex, &fakeWorkflowExecutor{ref: "workflow-ref-1"}, grants)
}

// newExecutableExecuteTaskWithWorkflow is newExecutableExecuteTask's full
// form — used by the EngineWorkflow-dispatch tests (TASK-FT-002-03) that
// need to control/assert against the WorkflowExecutor fake directly.
func newExecutableExecuteTaskWithWorkflow(tasks *fakeTaskRepository, edges *fakeEdgeRepository, simple SimpleExecutor, complex ComplexExecutor, workflow WorkflowExecutor, grants *fakeGrantRepository) (uc *ExecuteTask, worktrees *fakeWorktreeProvisioner, resolver *fakeProjectExecutionResolver, clock *fakeClock, links *fakeExecutionLinkRepository) {
	resolvePermissionUC := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil)
	worktrees = &fakeWorktreeProvisioner{worktreeID: "wt-1", path: "/srv/worktrees/wt-1"}
	resolver = &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/wt-1", connected: true}
	clock = newFakeClock(time.Unix(1000, 0), time.Hour)
	links = &fakeExecutionLinkRepository{}
	uc = NewExecuteTask(tasks, edges, simple, complex, workflow, resolvePermissionUC, worktrees, resolver, clock, links)
	return uc, worktrees, resolver, clock, links
}

func TestExecuteTask_RequiresTenantContext(t *testing.T) {
	tasks := newFakeTaskRepository()
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{}, &fakeGrantRepository{})
	_, err := uc.Execute(context.Background(), ExecuteTaskInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestExecuteTask_RequiresTaskID(t *testing.T) {
	uc, _, _, _, _ := newExecutableExecuteTask(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{}, &fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error for an empty task_id")
	}
}

// TestExecuteTask_PermissionDenied_NeverWritesStatus is the regression
// guard for TASK-TG-04-01's permission pre-check: a caller with no grant on
// the task must be denied BEFORE any UpdateStatus/worktree/dispatch call —
// closing the gap where ExecuteTask dispatched real work without ever
// calling ResolvePermission.
func TestExecuteTask_PermissionDenied_NeverWritesStatus(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusOpen, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	grants := &fakeGrantRepository{} // no grants recorded anywhere — every action is denied
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err = uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected a permission-denied error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
	if len(tasks.updateStatusCalls) != 0 {
		t.Errorf("expected NO UpdateStatus call when permission is denied, got %+v", tasks.updateStatusCalls)
	}
	if worktrees.called {
		t.Error("expected WorktreeProvisioner NOT to be called when permission is denied")
	}
	if simple.called || complex.called {
		t.Error("expected neither executor to be called when permission is denied")
	}
}

// TestExecuteTask_SimplePath_NoSubtasksNoDependencies is the core branching
// regression: a task with neither parent_child nor depends_on edges FROM it
// must dispatch to SimpleExecutor, never ComplexExecutor.
func TestExecuteTask_SimplePath_NoSubtasksNoDependencies(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExecutionRef != "infra-fleet-ref-1" {
		t.Errorf("expected the simple executor's ref, got %q", result.ExecutionRef)
	}
	if result.Async {
		t.Error("expected the simple path to report Async: false")
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called")
	}
	if complex.called {
		t.Error("expected ComplexExecutor NOT to be called")
	}
	if len(tasks.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly 1 CompleteExecution call, got %d: %+v", len(tasks.completeExecutionCalls), tasks.completeExecutionCalls)
	}
	call := tasks.completeExecutionCalls[0]
	if call.status != string(domain.StatusReview) {
		t.Errorf("expected CompleteExecution status=review, got %q", call.status)
	}
	if call.actualHours < 0 {
		t.Errorf("expected a non-negative actual_hours, got %v", call.actualHours)
	}
}

func TestExecuteTask_ComplexPath_HasSubtasks(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, complex, grants)
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
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected NO inline CompleteExecution call for the complex (async) path, got %+v", tasks.completeExecutionCalls)
	}
	// Status stays at in_progress — no further write until
	// ReportTaskExecutionResult (TASK-TG-04-05).
	if got := tasks.tasks["task-1"].Status; got != domain.StatusInProgress {
		t.Errorf("expected status to remain in_progress after complex dispatch, got %q", got)
	}
}

func TestExecuteTask_ComplexPath_HasDependencies(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "blocking-task", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, complex, grants)
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
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "other-task", ToTaskID: "task-1", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !simple.called || complex.called {
		t.Errorf("expected only SimpleExecutor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

func TestExecuteTask_ExecutorFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{err: errors.New("infra-fleet-service unavailable")}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, &fakeExecutor{}, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
}

// TestExecuteTask_DispatchFailure_RevertsStatusToPrevious is TASK-TG-04-01's
// core regression: a dispatch failure used to leave the task marked
// in_progress PERMANENTLY (no RPC ever cleared it). Execute must now write
// InProgress, then on failure revert back to the task's pre-dispatch
// status, in that order.
func TestExecuteTask_DispatchFailure_RevertsStatusToPrevious(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1") // starts StatusOpen
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{err: errors.New("infra-fleet-service unavailable")}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, simple, &fakeExecutor{}, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
	if len(tasks.updateStatusCalls) != 2 {
		t.Fatalf("expected exactly two UpdateStatus calls (in_progress then revert), got %d: %+v", len(tasks.updateStatusCalls), tasks.updateStatusCalls)
	}
	if tasks.updateStatusCalls[0].status != domain.StatusInProgress {
		t.Errorf("expected the first UpdateStatus call to be StatusInProgress, got %+v", tasks.updateStatusCalls[0])
	}
	if tasks.updateStatusCalls[1].status != domain.StatusOpen {
		t.Errorf("expected the second UpdateStatus call to revert to the pre-dispatch status (open), got %+v", tasks.updateStatusCalls[1])
	}
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call on a failed dispatch, got %+v", tasks.completeExecutionCalls)
	}
	// The fake mutates its map on every UpdateStatus call, so the final
	// persisted status must be the reverted one, not in_progress.
	if got := tasks.tasks["task-1"].Status; got != domain.StatusOpen {
		t.Errorf("expected persisted status to be reverted to open, got %q", got)
	}
}

func TestExecuteTask_MarksTaskInProgressBeforeDispatching(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks.updateStatusCalls) != 1 {
		t.Fatalf("expected exactly one UpdateStatus call, got %d: %+v", len(tasks.updateStatusCalls), tasks.updateStatusCalls)
	}
	got := tasks.updateStatusCalls[0]
	if got.tenantID != "tenant-1" || got.id != "task-1" || got.status != domain.StatusInProgress {
		t.Errorf("unexpected UpdateStatus call: %+v", got)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called after the status update")
	}
}

// TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch: a failure
// to persist the in_progress transition must fail Execute outright rather
// than silently dispatching with no recorded state.
func TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	tasks.updateStatusErr = errors.New("db unavailable")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error when the status update fails")
	}
	if simple.called || complex.called {
		t.Errorf("expected neither executor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

// TestExecuteTask_NoConnection_FailsBeforeAnyStatusWrite is TASK-TG-04-03's
// pre-check 2: a disconnected project must fail closed BEFORE the
// in_progress write and BEFORE worktree provisioning.
func TestExecuteTask_NoConnection_FailsBeforeAnyStatusWrite(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, worktrees, resolver, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	resolver.connected = false
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected an error when the project has no connected dev server")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
	if len(tasks.updateStatusCalls) != 0 {
		t.Errorf("expected NO UpdateStatus call when there's no connection, got %+v", tasks.updateStatusCalls)
	}
	if worktrees.called {
		t.Error("expected WorktreeProvisioner NOT to be called when there's no connection")
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called when there's no connection")
	}
}

// TestExecuteTask_WorktreeProvisionFailure_PropagatesBeforeStatusWrite:
// EnsureWorktree failing must surface as an error before any status
// mutation, same as the connection/permission pre-checks.
func TestExecuteTask_WorktreeProvisionFailure_PropagatesBeforeStatusWrite(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	worktrees.err = errors.New("git-gateway-service unavailable")
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error when worktree provisioning fails")
	}
	if len(tasks.updateStatusCalls) != 0 {
		t.Errorf("expected NO UpdateStatus call when worktree provisioning fails, got %+v", tasks.updateStatusCalls)
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called when worktree provisioning fails")
	}
}

// TestExecuteTask_ReusesWorktree_DoesNotRewriteUnchangedWorktreeID: a task
// with an existing worktree ID reuses it via EnsureWorktree — receiving the
// SAME id back must not trigger a redundant UpdateWorktreeID write.
func TestExecuteTask_ReusesWorktree_DoesNotRewriteUnchangedWorktreeID(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	task := tasks.tasks["task-1"]
	task.WorktreeID = "wt-existing"
	tasks.tasks["task-1"] = task

	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	worktrees.worktreeID = "wt-existing" // EnsureWorktree's real reuse branch echoes the task's existing id back
	worktrees.path = ""
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !worktrees.called {
		t.Error("expected EnsureWorktree to be called")
	}
	if worktrees.gotTask.WorktreeID != "wt-existing" {
		t.Errorf("expected EnsureWorktree to receive the task's persisted worktree id, got %q", worktrees.gotTask.WorktreeID)
	}
	if len(tasks.updateWorktreeIDCalls) != 0 {
		t.Errorf("expected NO UpdateWorktreeID call when the returned worktree id matches the task's existing one, got %+v", tasks.updateWorktreeIDCalls)
	}
}

// TestExecuteTask_CreatesWorktree_PersistsNewWorktreeID: a task with no
// existing worktree gets the newly created id persisted via
// UpdateWorktreeID before dispatch.
func TestExecuteTask_CreatesWorktree_PersistsNewWorktreeID(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1") // no WorktreeID yet
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, worktrees, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	worktrees.worktreeID = "wt-new"
	worktrees.path = "/srv/worktrees/wt-new"
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks.updateWorktreeIDCalls) != 1 {
		t.Fatalf("expected exactly one UpdateWorktreeID call, got %d: %+v", len(tasks.updateWorktreeIDCalls), tasks.updateWorktreeIDCalls)
	}
	if tasks.updateWorktreeIDCalls[0].worktreeID != "wt-new" {
		t.Errorf("expected the new worktree id to be persisted, got %q", tasks.updateWorktreeIDCalls[0].worktreeID)
	}
}

// TestExecuteTask_SimplePath_CompletesInlineWithActualHours is
// TASK-TG-04-03's core regression: SimpleExecutor.Execute already blocks
// until completion, so Execute must persist StatusReview + a non-zero
// actual_hours in the SAME call — no second RPC needed.
func TestExecuteTask_SimplePath_CompletesInlineWithActualHours(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Async {
		t.Error("expected the simple path to report Async: false")
	}
	if len(tasks.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly one CompleteExecution call, got %d: %+v", len(tasks.completeExecutionCalls), tasks.completeExecutionCalls)
	}
	got := tasks.completeExecutionCalls[0]
	if got.status != string(domain.StatusReview) {
		t.Errorf("expected StatusReview, got %q", got.status)
	}
	if got.actualHours <= 0 {
		t.Errorf("expected a non-zero actual_hours, got %v", got.actualHours)
	}
}

// TestExecuteTask_ComplexPath_ReturnsAsyncAndLeavesStatusInProgress: the
// complex path has no synchronous completion signal — it must return
// Async: true and leave the task at InProgress, with no CompleteExecution
// call (that arrives later via TASK-TG-04-05's ReportTaskExecutionResult).
func TestExecuteTask_ComplexPath_ReturnsAsyncAndLeavesStatusInProgress(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, &fakeExecutor{}, complex, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Async {
		t.Error("expected the complex path to report Async: true")
	}
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call on the complex path, got %+v", tasks.completeExecutionCalls)
	}
	if len(tasks.updateStatusCalls) != 1 || tasks.updateStatusCalls[0].status != domain.StatusInProgress {
		t.Errorf("expected status to remain in_progress after a successful complex dispatch (no revert, no completion), got %+v", tasks.updateStatusCalls)
	}
}

// TestExecuteTask_WorkflowTemplateID_TakesPriorityOverSubtasks is a
// regression guard against the exact ambiguity CR-FLOW-TASK-001 calls out:
// a task with BOTH a workflow_template_id AND child edges must select
// EngineWorkflow, not EngineOrchestration.
func TestExecuteTask_WorkflowTemplateID_TakesPriorityOverSubtasks(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	task := tasks.tasks["task-1"]
	task.WorkflowTemplateID = "wf-tmpl-1"
	tasks.tasks["task-1"] = task
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	workflow := &fakeWorkflowExecutor{ref: "workflow-ref-1"}
	uc, _, _, _, links := newExecutableExecuteTaskWithWorkflow(tasks, edges, simple, complex, workflow, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links.created) != 1 || links.created[0].Engine != domain.EngineWorkflow {
		t.Fatalf("expected exactly one execution_links row with engine=workflow, got %+v", links.created)
	}
	if !workflow.called || complex.called || simple.called {
		t.Errorf("expected only WorkflowExecutor to be called, workflow=%v complex=%v simple=%v", workflow.called, complex.called, simple.called)
	}
}

// TestExecuteTask_WorkflowPath_DispatchesToWorkflowExecutor is
// TASK-FT-002-03's core regression: EngineWorkflow must call
// WorkflowExecutor.Execute (with the task's workflow_template_id), never
// ComplexExecutor, and report Async: true (same shape as the complex path
// — workflow-service.Execute dispatches asynchronously too).
func TestExecuteTask_WorkflowPath_DispatchesToWorkflowExecutor(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	task := tasks.tasks["task-1"]
	task.WorkflowTemplateID = "wf-tmpl-1"
	tasks.tasks["task-1"] = task
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	workflow := &fakeWorkflowExecutor{ref: "workflow-exec-1"}
	uc, _, _, _, _ := newExecutableExecuteTaskWithWorkflow(tasks, &fakeEdgeRepository{}, simple, complex, workflow, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !workflow.called {
		t.Error("expected WorkflowExecutor to be called")
	}
	if complex.called || simple.called {
		t.Errorf("expected neither SimpleExecutor nor ComplexExecutor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
	if workflow.gotWorkflowTemplateID != "wf-tmpl-1" {
		t.Errorf("expected the task's workflow_template_id to pass through, got %q", workflow.gotWorkflowTemplateID)
	}
	if result.ExecutionRef != "workflow-exec-1" {
		t.Errorf("expected the workflow executor's ref, got %q", result.ExecutionRef)
	}
	if !result.Async {
		t.Error("expected the workflow path to report Async: true")
	}
}

// TestExecuteTask_EveryEngineBranch_EnqueuesExactlyOneExecutionLink covers
// BE-SOL-001/CR-FLOW-TASK-001's acceptance criteria: every Execute call that
// reaches dispatch — across all three engines — records exactly one
// execution_links row tagged with the engine selected.
func TestExecuteTask_EveryEngineBranch_EnqueuesExactlyOneExecutionLink(t *testing.T) {
	cases := []struct {
		name           string
		edges          []domain.TaskEdge
		workflowTmplID string
		wantEngine     domain.ExecutionEngine
	}{
		{name: "no edges, no template -> direct_agent", wantEngine: domain.EngineDirectAgent},
		{name: "parent_child edge -> orchestration", edges: []domain.TaskEdge{{FromTaskID: "task-1", ToTaskID: "sub-1", Kind: domain.EdgeKindParentChild}}, wantEngine: domain.EngineOrchestration},
		{name: "depends_on edge -> orchestration", edges: []domain.TaskEdge{{FromTaskID: "task-1", ToTaskID: "dep-1", Kind: domain.EdgeKindDependsOn}}, wantEngine: domain.EngineOrchestration},
		{name: "workflow_template_id set -> workflow", workflowTmplID: "wf-1", wantEngine: domain.EngineWorkflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := newFakeTaskRepository()
			grants := &fakeGrantRepository{}
			seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
			if tc.workflowTmplID != "" {
				task := tasks.tasks["task-1"]
				task.WorkflowTemplateID = tc.workflowTmplID
				tasks.tasks["task-1"] = task
			}
			simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
			complex := &fakeExecutor{ref: "orchestration-ref-1"}
			uc, _, _, _, links := newExecutableExecuteTask(tasks, &fakeEdgeRepository{edges: tc.edges}, simple, complex, grants)
			ctx := withIdentity(context.Background(), "tenant-1", "user-1")

			if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(links.created) != 1 {
				t.Fatalf("expected exactly one execution_links row, got %d", len(links.created))
			}
			if links.created[0].Engine != tc.wantEngine {
				t.Errorf("expected engine %q, got %q", tc.wantEngine, links.created[0].Engine)
			}
		})
	}
}
