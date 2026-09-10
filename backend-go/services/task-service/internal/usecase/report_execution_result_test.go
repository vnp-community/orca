package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// seedTaskWithActiveLink creates a task with an execution_links row already
// recorded as "the" active dispatch — the state ExecuteTask leaves behind
// after a successful Engine 2/3 dispatch (SetActiveExecutionLink,
// TASK-FT-002-04's precondition).
func seedTaskWithActiveLink(tasks *fakeTaskRepository, links *fakeExecutionLinkRepository, taskID string, engine domain.ExecutionEngine, externalRefID string) domain.ExecutionLink {
	link := domain.ExecutionLink{ID: "link-1", TenantID: "tenant-1", TaskID: taskID, Engine: engine, ExternalRefID: externalRefID, StatusMirror: "in_progress"}
	links.created = append(links.created, link)
	task := tasks.tasks[taskID]
	task.ActiveExecutionLinkID = link.ID
	tasks.tasks[taskID] = task
	return link
}

func TestReportTaskExecutionResult_RequiresTenantContext(t *testing.T) {
	uc := NewReportTaskExecutionResult(newFakeTaskRepository(), &fakeExecutionLinkRepository{})
	err := uc.Execute(context.Background(), ReportTaskExecutionResultInput{TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestReportTaskExecutionResult_NoActiveLink_IsNoop(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task // no ActiveExecutionLinkID set
	links := &fakeExecutionLinkRepository{}
	uc := NewReportTaskExecutionResult(tasks, links)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration", Success: true}); err != nil {
		t.Fatalf("expected a silent no-op, got error: %v", err)
	}
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call, got %+v", tasks.completeExecutionCalls)
	}
}

func TestReportTaskExecutionResult_MismatchedExecutionRef_IsNoop(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineOrchestration, "run-current")
	uc := NewReportTaskExecutionResult(tasks, links)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// A stale/duplicate callback for a DIFFERENT (older) run.
	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-stale", Engine: "orchestration", Success: true}); err != nil {
		t.Fatalf("expected a silent no-op, got error: %v", err)
	}
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call for a stale execution_ref, got %+v", tasks.completeExecutionCalls)
	}
	if links.created[0].StatusMirror != "in_progress" {
		t.Errorf("expected the active link to be untouched by a stale callback, got %+v", links.created[0])
	}
}

func TestReportTaskExecutionResult_MismatchedEngine_IsNoop(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineWorkflow, "exec-1")
	uc := NewReportTaskExecutionResult(tasks, links)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// Same external_ref_id string, but reported by the wrong engine.
	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "exec-1", Engine: "orchestration", Success: true}); err != nil {
		t.Fatalf("expected a silent no-op, got error: %v", err)
	}
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call for a mismatched engine, got %+v", tasks.completeExecutionCalls)
	}
}

func TestReportTaskExecutionResult_MatchingSuccess_CompletesTaskAndLink(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineOrchestration, "run-1")
	uc := NewReportTaskExecutionResult(tasks, links)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration", Success: true, ActualHours: 2.5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks.completeExecutionCalls) != 1 {
		t.Fatalf("expected exactly one CompleteExecution call, got %d: %+v", len(tasks.completeExecutionCalls), tasks.completeExecutionCalls)
	}
	got := tasks.completeExecutionCalls[0]
	if got.status != string(domain.StatusReview) || got.actualHours != 2.5 {
		t.Errorf("expected StatusReview + actual_hours=2.5, got %+v", got)
	}
	if links.created[0].StatusMirror != "completed" {
		t.Errorf("expected the link to be marked completed, got %+v", links.created[0])
	}
}

func TestReportTaskExecutionResult_MatchingFailure_LeavesTaskInProgress(t *testing.T) {
	tasks := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineWorkflow, "exec-1")
	uc := NewReportTaskExecutionResult(tasks, links)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "exec-1", Engine: "workflow", Success: false, ErrorMessage: "boom"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No status/completion status enforced yet (domain has no
	// blocked/failed task status — see this usecase's doc comment):
	// CompleteExecution must NOT be called on the failure path.
	if len(tasks.completeExecutionCalls) != 0 {
		t.Errorf("expected no CompleteExecution call on a failed report, got %+v", tasks.completeExecutionCalls)
	}
	if links.created[0].StatusMirror != "failed" {
		t.Errorf("expected the link to be marked failed, got %+v", links.created[0])
	}
	if tasks.tasks["task-1"].Status != domain.StatusInProgress {
		t.Errorf("expected the task to remain in_progress, got %q", tasks.tasks["task-1"].Status)
	}
}
