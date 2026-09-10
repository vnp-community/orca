package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestExecute_RequiresTenantContext(t *testing.T) {
	templates := newFakeTemplateRepository()
	uc := NewExecute(templates, newFakeExecutionRepository(), newFakeStepExecutionRepository(), newFakeRegistry(), &fakeTaskClient{})

	_, err := uc.Execute(context.Background(), ExecuteInput{TemplateID: "tmpl-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestExecute_TemplateNotFound(t *testing.T) {
	templates := newFakeTemplateRepository()
	executions := newFakeExecutionRepository()
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), newFakeRegistry(), &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteInput{TemplateID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for an unknown template")
	}
	if executions.count() != 0 {
		t.Errorf("expected no execution to be persisted, got %d", executions.count())
	}
}

func TestExecute_InvalidDAGRejectedSynchronously(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell","dependsOn":["ghost"]}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)
	executions := newFakeExecutionRepository()
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), newFakeRegistry(), &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"})
	if err == nil {
		t.Fatal("expected a DAG validation error")
	}
	if executions.count() != 0 {
		t.Errorf("expected no execution to be persisted for an invalid DAG, got %d", executions.count())
	}
}

func TestExecute_CyclicDAGRejectedSynchronously(t *testing.T) {
	// a->b->c->a: passes Validate's pairwise checks (every edge resolves to
	// a real, distinct step) but must still fail Execute synchronously —
	// see execute.go's doc comment: a cyclic DAG is discovered by
	// BuildWaves before any execution row is persisted, not discovered
	// only once background dispatch starts.
	templates := newFakeTemplateRepository()
	dagJSON := `{"steps":[
		{"id":"a","type":"shell","dependsOn":["c"]},
		{"id":"b","type":"shell","dependsOn":["a"]},
		{"id":"c","type":"shell","dependsOn":["b"]}
	]}`
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", dagJSON, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)
	executions := newFakeExecutionRepository()
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), newFakeRegistry(), &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"})
	if err == nil {
		t.Fatal("expected a cyclic-dependency error")
	}
	if !errors.Is(err, domain.ErrCyclicDependency) {
		t.Fatalf("expected error wrapping ErrCyclicDependency, got %v", err)
	}
	if executions.count() != 0 {
		t.Errorf("expected no execution to be persisted for a cyclic DAG, got %d", executions.count())
	}
}

func TestExecute_ReturnsRunningImmediatelyWithoutWaitingForDispatch(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	registry := newFakeRegistry()
	// The executor blocks forever — if Execute() were synchronous over the
	// whole DAG, this call would hang the test. It returning promptly is
	// the proof that dispatch happens off the RPC path.
	block := make(chan struct{})
	registry.executors[domain.StepTypeShell] = blockingExecutor{block: block}

	done := make(chan struct{})
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			close(done)
		}
	}

	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	exec, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != domain.StatusRunning {
		t.Errorf("expected the returned execution to be status=running, got %v", exec.Status)
	}
	// Let the background goroutine finish and wait for it, so it doesn't
	// leak past the test.
	close(block)
	<-done
}

// blockingExecutor is a domain.StepExecutor that blocks until block is
// closed — used to prove Execute() returns before dispatch completes,
// without a sleep.
type blockingExecutor struct {
	block <-chan struct{}
}

func (b blockingExecutor) Execute(ctx context.Context, _ string) (domain.StepResult, error) {
	select {
	case <-b.block:
	case <-ctx.Done():
	}
	return domain.StepResult{Status: domain.ResultStatusCompleted}, nil
}

func TestExecute_DispatchesWavesAndMarksExecutionCompleted(t *testing.T) {
	templates := newFakeTemplateRepository()
	dagJSON := `{"steps":[
		{"id":"a","type":"shell"},
		{"id":"b","type":"webhook","dependsOn":["a"]}
	]}`
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", dagJSON, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	stepExecutions := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	registry.executors[domain.StepTypeWebhook] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}

	// Deterministic synchronization: the background goroutine's only
	// terminal write is its final UpdateExecution call — block on that via
	// onUpdate rather than sleeping.
	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	uc := NewExecute(templates, executions, stepExecutions, registry, &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	exec, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := <-done
	if final.Status != domain.StatusCompleted {
		t.Fatalf("expected the execution to finish completed, got %v", final.Status)
	}
	if executions.lastUpdateEvent == nil || executions.lastUpdateEvent.Subject != "orca.workflow.execution.completed" {
		t.Errorf("expected exactly one orca.workflow.execution.completed outbox event, got %+v", executions.lastUpdateEvent)
	}

	rows := stepExecutions.byExecution(exec.ID)
	if len(rows) != 2 {
		t.Fatalf("expected 2 persisted step_executions rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.Status != domain.StepExecutionStatusCompleted {
			t.Errorf("expected step %s to be completed, got %v", row.StepID, row.Status)
		}
	}
}

func TestExecute_WaveFailureMarksExecutionFailed(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed}}

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := <-done
	if final.Status != domain.StatusFailed {
		t.Fatalf("expected the execution to finish failed, got %v", final.Status)
	}
	if executions.lastUpdateEvent == nil || executions.lastUpdateEvent.Subject != "orca.workflow.execution.failed" {
		t.Errorf("expected exactly one orca.workflow.execution.failed outbox event, got %+v", executions.lastUpdateEvent)
	}
}

// TestExecute_OriginTaskIDSet_ReportsResultToTaskService is
// TASK-FT-002-05's core regression: an execution dispatched via
// task-service's Engine 3 WorkflowExecutor (OriginTaskID set) must call
// TaskClient.ReportTaskExecutionResult exactly once on completion, with
// Engine: "workflow" and Success reflecting the terminal status.
func TestExecute_OriginTaskIDSet_ReportsResultToTaskService(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	taskClient := &fakeTaskClient{}
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, taskClient)
	ctx := withTenantContext(context.Background(), "tenant-1")

	exec, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1", OriginTaskID: "task-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done

	if taskClient.callCount() != 1 {
		t.Fatalf("expected exactly one ReportTaskExecutionResult call, got %d", taskClient.callCount())
	}
	got := taskClient.lastCall()
	if got.TaskID != "task-1" || got.ExecutionRef != exec.ID || got.Engine != "workflow" || !got.Success {
		t.Errorf("unexpected callback payload: %+v", got)
	}
}

// TestExecute_OriginTaskIDSet_FailedRun_ReportsSuccessFalse covers the
// failure half of the same regression — Success must reflect
// StatusFailed, not be hardcoded true.
func TestExecute_OriginTaskIDSet_FailedRun_ReportsSuccessFalse(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed}}

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	taskClient := &fakeTaskClient{}
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, taskClient)
	ctx := withTenantContext(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1", OriginTaskID: "task-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done

	if taskClient.callCount() != 1 {
		t.Fatalf("expected exactly one ReportTaskExecutionResult call, got %d", taskClient.callCount())
	}
	if taskClient.lastCall().Success {
		t.Error("expected Success=false for a failed run")
	}
}

// TestExecute_OriginTaskIDEmpty_NeverReportsToTaskService is
// TASK-FT-002-05's explicit regression guard: a standalone workflow run
// (no OriginTaskID) must not call TaskClient at all — "does not break the
// existing behavior of Workflow Orchestration standing alone."
func TestExecute_OriginTaskIDEmpty_NeverReportsToTaskService(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	taskClient := &fakeTaskClient{}
	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, taskClient)
	ctx := withTenantContext(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"}); err != nil { // no OriginTaskID
		t.Fatalf("unexpected error: %v", err)
	}
	<-done

	if taskClient.callCount() != 0 {
		t.Errorf("expected zero ReportTaskExecutionResult calls for a standalone run, got %d: %+v", taskClient.callCount(), taskClient.calls)
	}
}

func TestExecute_ZeroStepTemplateCompletesImmediately(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) { done <- e }

	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), newFakeRegistry(), &fakeTaskClient{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, ExecuteInput{TemplateID: "tmpl-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := <-done
	if final.Status != domain.StatusCompleted {
		t.Fatalf("expected a zero-step DAG to trivially complete, got %v", final.Status)
	}
}
