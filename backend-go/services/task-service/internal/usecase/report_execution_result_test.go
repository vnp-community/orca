package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestReportTaskExecutionResult_RequiresTenantContext(t *testing.T) {
	uc := NewReportTaskExecutionResult(newFakeTaskRepository())
	err := uc.Execute(context.Background(), ReportTaskExecutionResultInput{TaskID: "t1", CoordinatorRunID: "run-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestReportTaskExecutionResult_TaskNotFound(t *testing.T) {
	uc := NewReportTaskExecutionResult(newFakeTaskRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "does-not-exist", CoordinatorRunID: "run-1"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

// TestReportTaskExecutionResult_MismatchedCoordinatorRunID_IsSilentNoOp is
// the core regression: a callback whose coordinator_run_id doesn't match
// the task's current ActiveExecutionID must be a silent no-op, not an
// error — at-least-once consumer idempotence.
func TestReportTaskExecutionResult_MismatchedCoordinatorRunID_IsSilentNoOp(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusInProgress, ActiveExecutionID: "run-current"}
	uc := NewReportTaskExecutionResult(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "t1", CoordinatorRunID: "run-stale", Success: true, ActualHours: 2})
	if err != nil {
		t.Fatalf("expected a silent no-op (nil error) for a mismatched coordinator_run_id, got %v", err)
	}
	if len(repo.completeExecutionCalls) != 0 {
		t.Errorf("expected NO CompleteExecution call for a stale callback, got %+v", repo.completeExecutionCalls)
	}
	// Status must remain unchanged.
	if got := repo.tasks["t1"].Status; got != domain.StatusInProgress {
		t.Errorf("expected status to remain in_progress, got %q", got)
	}
}

func TestReportTaskExecutionResult_Success_TransitionsToReview(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusInProgress, ActiveExecutionID: "run-1"}
	uc := NewReportTaskExecutionResult(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "t1", CoordinatorRunID: "run-1", Success: true, ActualHours: 3.5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly 1 CompleteExecution call, got %d", len(repo.completeExecutionCalls))
	}
	call := repo.completeExecutionCalls[0]
	if call.status != domain.StatusReview {
		t.Errorf("expected status=review, got %q", call.status)
	}
	if call.actualHours != 3.5 {
		t.Errorf("expected actual_hours=3.5, got %v", call.actualHours)
	}
}

// TestReportTaskExecutionResult_Failure_TransitionsToBlocked_NeverRevertsToPreDispatch
// is the complex path's distinct failure semantics (TASK-TG-04-05): unlike
// the simple path's revert-to-previous-status (TASK-TG-04-01), a failed
// complex execution goes to Blocked — never a silent revert.
func TestReportTaskExecutionResult_Failure_TransitionsToBlocked_NeverRevertsToPreDispatch(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusInProgress, ActiveExecutionID: "run-1"}
	uc := NewReportTaskExecutionResult(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "t1", CoordinatorRunID: "run-1", Success: false, ActualHours: 1.0}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly 1 CompleteExecution call, got %d", len(repo.completeExecutionCalls))
	}
	if got := repo.completeExecutionCalls[0].status; got != domain.StatusBlocked {
		t.Errorf("expected status=blocked on failure, got %q", got)
	}
}
