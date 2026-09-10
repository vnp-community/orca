package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// TestRecoverExecutions_ResumesAtFirstIncompleteWave_NotWaveZero proves the
// "first non-terminal-success wave" algorithm: wave 0 already has a
// persisted, completed step_executions row (dispatched and finished before
// the crash) and must NOT be re-dispatched; wave 1 has no row at all
// (never reached) and must be dispatched fresh.
func TestRecoverExecutions_ResumesAtFirstIncompleteWave_NotWaveZero(t *testing.T) {
	templates := newFakeTemplateRepository()
	dagJSON := `{"steps":[
		{"id":"a","type":"shell"},
		{"id":"b","type":"webhook","dependsOn":["a"]}
	]}`
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", dagJSON, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	stepExecutions := newFakeStepExecutionRepository()
	seA, _ := domain.NewStepExecution("se-a", "exec-1", "a", "token-a", 0)
	seA.FromResult(domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`})
	_ = stepExecutions.CreateStepExecution(context.Background(), seA)

	registry := newFakeRegistry()
	shellExecutor := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	webhookExecutor := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	registry.executors[domain.StepTypeShell] = shellExecutor
	registry.executors[domain.StepTypeWebhook] = webhookExecutor

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	uc := NewRecoverExecutions(templates, executions, stepExecutions, registry)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := <-done
	if final.Status != domain.StatusCompleted {
		t.Fatalf("expected the recovered execution to finish completed, got %v", final.Status)
	}

	if shellExecutor.invocations != 0 {
		t.Errorf("expected wave 0's already-completed step to NOT be re-dispatched, got %d invocations", shellExecutor.invocations)
	}
	if webhookExecutor.invocations != 1 {
		t.Errorf("expected wave 1's never-dispatched step to be dispatched exactly once, got %d invocations", webhookExecutor.invocations)
	}

	rows := stepExecutions.byExecution("exec-1")
	if len(rows) != 2 {
		t.Fatalf("expected exactly 2 step_executions rows (no duplicate for wave 0's step), got %d: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Status != domain.StepExecutionStatusCompleted {
			t.Errorf("expected step %s to be completed, got %v", row.StepID, row.Status)
		}
	}
}

// TestRecoverExecutions_RedispatchesMidFlightStep proves the re-dispatch-
// on-uncertain-status decision: a step_executions row still status=running
// when the process "crashed" has an unknown real-world outcome, so the
// recovery scan re-dispatches it — reusing the SAME row (no duplicate
// insert, which would violate the (execution_id, step_id) UNIQUE
// constraint in Postgres) rather than assuming it succeeded or failed.
func TestRecoverExecutions_RedispatchesMidFlightStep(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	stepExecutions := newFakeStepExecutionRepository()
	seA, _ := domain.NewStepExecution("se-a", "exec-1", "a", "token-a", 0)
	seA.MarkRunning() // mid-flight when the process "crashed"
	_ = stepExecutions.CreateStepExecution(context.Background(), seA)

	registry := newFakeRegistry()
	shellExecutor := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	registry.executors[domain.StepTypeShell] = shellExecutor

	done := make(chan domain.WorkflowExecution, 1)
	executions.onUpdate = func(e domain.WorkflowExecution) {
		if e.Status == domain.StatusCompleted || e.Status == domain.StatusFailed {
			done <- e
		}
	}

	uc := NewRecoverExecutions(templates, executions, stepExecutions, registry)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := <-done
	if final.Status != domain.StatusCompleted {
		t.Fatalf("expected the recovered execution to finish completed, got %v", final.Status)
	}
	if shellExecutor.invocations != 1 {
		t.Errorf("expected the mid-flight step to be re-dispatched exactly once, got %d invocations", shellExecutor.invocations)
	}

	rows := stepExecutions.byExecution("exec-1")
	if len(rows) != 1 {
		t.Fatalf("expected the mid-flight step's row to be reused, not duplicated: got %d rows", len(rows))
	}
	if rows[0].ID != "se-a" {
		t.Errorf("expected the same step_execution id to be reused, got %q", rows[0].ID)
	}
	if rows[0].Status != domain.StepExecutionStatusCompleted {
		t.Errorf("expected the reused row to end up completed, got %v", rows[0].Status)
	}
}

// TestRecoverExecutions_NeverTouchesPausedExecution proves §8's explicit
// carve-out: a paused execution was a deliberate user/system action and
// must be left alone by the boot-time scan, not silently resumed by a
// restart.
func TestRecoverExecutions_NeverTouchesPausedExecution(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	exec.Status = domain.StatusPaused
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	stepExecutions := newFakeStepExecutionRepository()
	registry := newFakeRegistry()

	uc := NewRecoverExecutions(templates, executions, stepExecutions, registry)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := executions.snapshot("exec-1")
	if !ok {
		t.Fatal("expected the execution to still exist")
	}
	if got.Status != domain.StatusPaused {
		t.Fatalf("expected the paused execution to remain paused, got %v", got.Status)
	}
	if len(stepExecutions.byExecution("exec-1")) != 0 {
		t.Error("expected no step_executions to have been dispatched for a paused execution")
	}
}

// TestRecoverExecutions_NeverTouchesTerminalExecutions proves an execution
// already in a terminal status (completed/failed/cancelled) is never
// picked up — ListRunning only ever returns status=running rows, and this
// asserts the end-to-end effect of that filter through RecoverExecutions.
func TestRecoverExecutions_NeverTouchesTerminalExecutions(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)

	executions := newFakeExecutionRepository()
	for i, status := range []domain.Status{domain.StatusCompleted, domain.StatusFailed, domain.StatusCancelled} {
		exec, _ := domain.NewWorkflowExecution(
			[]string{"exec-completed", "exec-failed", "exec-cancelled"}[i],
			"tenant-1", "tmpl-1", "trace", "",
		)
		exec.Status = status
		_ = executions.CreateExecution(context.Background(), exec)
	}

	stepExecutions := newFakeStepExecutionRepository()
	registry := newFakeRegistry()

	uc := NewRecoverExecutions(templates, executions, stepExecutions, registry)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, id := range []string{"exec-completed", "exec-failed", "exec-cancelled"} {
		got, ok := executions.snapshot(id)
		if !ok {
			t.Fatalf("expected %s to still exist", id)
		}
		if got.Status == domain.StatusRunning {
			t.Errorf("expected %s to never be transitioned back to running, got %v", id, got.Status)
		}
	}
	if len(stepExecutions.byExecution("exec-completed")) != 0 {
		t.Error("expected no step_executions to have been dispatched for an already-terminal execution")
	}
}

// TestRecoverExecutions_SkipsAdHocExecutions proves the documented gap: an
// ad hoc execution (no backing template, step config never persisted) is
// left in status=running by the scan rather than guessed at.
func TestRecoverExecutions_SkipsAdHocExecutions(t *testing.T) {
	exec, _ := domain.NewAdHocWorkflowExecution("exec-1", "tenant-1", "trace-1")
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	uc := NewRecoverExecutions(newFakeTemplateRepository(), executions, newFakeStepExecutionRepository(), newFakeRegistry())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := executions.snapshot("exec-1")
	if !ok {
		t.Fatal("expected the ad hoc execution to still exist")
	}
	if got.Status != domain.StatusRunning {
		t.Fatalf("expected the ad hoc execution to be left as-is (status=running), got %v", got.Status)
	}
}
