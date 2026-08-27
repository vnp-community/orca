package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeStepExecutor is an in-memory domain.StepExecutor — the "test against
// fakes, not real infra" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeStepExecutor struct {
	result      domain.StepResult
	err         error
	lastConfig  string
	invocations int
}

func (f *fakeStepExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	f.invocations++
	f.lastConfig = stepConfigJSON
	if f.err != nil {
		return domain.StepResult{}, f.err
	}
	return f.result, nil
}

// fakeRegistry is an in-memory StepExecutorRegistry.
type fakeRegistry struct {
	executors map[domain.StepType]domain.StepExecutor
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{executors: make(map[domain.StepType]domain.StepExecutor)}
}

func (r *fakeRegistry) Resolve(stepType domain.StepType) (domain.StepExecutor, error) {
	e, ok := r.executors[stepType]
	if !ok {
		return nil, ErrStepExecutorNotRegistered
	}
	return e, nil
}

// withTenantContext attaches both tenant and acting-user identity — a fixed
// user id is fine here since these usecase tests only care that a user is
// present (e.g. CreateTemplate's OwnerID), not who specifically.
func withTenantContext(ctx context.Context, tenantID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(ctx, tenantID), "user-1")
}

func TestExecuteAdHocStep_ResolvesAndCallsExecutor(t *testing.T) {
	registry := newFakeRegistry()
	fake := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`}}
	registry.executors[domain.StepTypeCondition] = fake

	executions := newFakeExecutionRepository()
	stepExecutions := newFakeStepExecutionRepository()
	uc := NewExecuteAdHocStep(executions, stepExecutions, registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, ExecuteAdHocStepInput{
		StepType:       domain.StepTypeCondition,
		StepConfigJSON: `{"expression":"a == b"}`,
		RequestID:      "req-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if fake.invocations != 1 {
		t.Errorf("expected exactly 1 invocation of the executor, got %d", fake.invocations)
	}
	if fake.lastConfig != `{"expression":"a == b"}` {
		t.Errorf("expected step config to be passed through verbatim, got %q", fake.lastConfig)
	}

	// §3.1's persistence gap: a synthetic one-step execution (no backing
	// template) plus one wave-0 step_executions row should now exist,
	// both reaching a completed terminal status.
	if len(executions.executions) != 1 {
		t.Fatalf("expected exactly 1 synthetic execution to be persisted, got %d", len(executions.executions))
	}
	var exec domain.WorkflowExecution
	for _, e := range executions.executions {
		exec = e
	}
	if exec.TemplateID != "" {
		t.Errorf("expected the synthetic execution to have no backing template, got %q", exec.TemplateID)
	}
	if exec.Status != domain.StatusCompleted {
		t.Errorf("expected the synthetic execution to be completed, got %v", exec.Status)
	}

	rows := stepExecutions.byExecution(exec.ID)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 step_executions row, got %d", len(rows))
	}
	if rows[0].Wave != 0 {
		t.Errorf("expected the ad hoc step to be recorded as wave 0, got %d", rows[0].Wave)
	}
	if rows[0].Status != domain.StepExecutionStatusCompleted {
		t.Errorf("expected the step execution row to be completed, got %v", rows[0].Status)
	}
	if rows[0].OutputJSON != `{"ok":true}` {
		t.Errorf("expected the step execution's output to be recorded, got %q", rows[0].OutputJSON)
	}
}

func TestExecuteAdHocStep_RequiresTenantContext(t *testing.T) {
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeCondition] = &fakeStepExecutor{}
	uc := NewExecuteAdHocStep(newFakeExecutionRepository(), newFakeStepExecutionRepository(), registry)

	_, err := uc.Execute(context.Background(), ExecuteAdHocStepInput{StepType: domain.StepTypeCondition})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestExecuteAdHocStep_RejectsInvalidStepType(t *testing.T) {
	registry := newFakeRegistry()
	uc := NewExecuteAdHocStep(newFakeExecutionRepository(), newFakeStepExecutionRepository(), registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepType("bogus")})
	if err == nil {
		t.Fatal("expected an error for an invalid step type")
	}
}

func TestExecuteAdHocStep_UnregisteredStepTypePropagatesFailedPrecondition(t *testing.T) {
	registry := newFakeRegistry() // nothing registered
	executions := newFakeExecutionRepository()
	stepExecutions := newFakeStepExecutionRepository()
	uc := NewExecuteAdHocStep(executions, stepExecutions, registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepTypeShell})
	if err == nil {
		t.Fatal("expected an error when no executor is registered for the step type")
	}

	// The synthetic execution/step_execution rows should still have been
	// persisted (and marked failed) even though the run itself failed —
	// observability of a failed ad hoc run matters as much as a successful
	// one.
	var exec domain.WorkflowExecution
	for _, e := range executions.executions {
		exec = e
	}
	if exec.Status != domain.StatusFailed {
		t.Errorf("expected the synthetic execution to be marked failed, got %v", exec.Status)
	}
	rows := stepExecutions.byExecution(exec.ID)
	if len(rows) != 1 || rows[0].Status != domain.StepExecutionStatusFailed {
		t.Fatalf("expected the step execution row to be marked failed, got %+v", rows)
	}
}

func TestExecuteAdHocStep_ExecutorErrorPropagates(t *testing.T) {
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeAgent] = &fakeStepExecutor{err: errors.New("not implemented")}
	uc := NewExecuteAdHocStep(newFakeExecutionRepository(), newFakeStepExecutionRepository(), registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepTypeAgent})
	if err == nil {
		t.Fatal("expected the executor's error to propagate")
	}
}

func TestExecuteAdHocStep_BusinessFailedResultMarksExecutionFailed(t *testing.T) {
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeCondition] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: `{"error":"bad expression"}`}}
	executions := newFakeExecutionRepository()
	stepExecutions := newFakeStepExecutionRepository()
	uc := NewExecuteAdHocStep(executions, stepExecutions, registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepTypeCondition, StepConfigJSON: `{"expression":"bad"}`})
	if err != nil {
		t.Fatalf("a business-level failed StepResult should not be a Go error: %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected a failed result, got %v", result.Status)
	}

	var exec domain.WorkflowExecution
	for _, e := range executions.executions {
		exec = e
	}
	if exec.Status != domain.StatusFailed {
		t.Errorf("expected the synthetic execution to be marked failed for a business-level failed step, got %v", exec.Status)
	}
}
