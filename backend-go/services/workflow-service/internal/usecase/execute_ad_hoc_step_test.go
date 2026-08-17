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

func withTenantContext(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestExecuteAdHocStep_ResolvesAndCallsExecutor(t *testing.T) {
	registry := newFakeRegistry()
	fake := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`}}
	registry.executors[domain.StepTypeCondition] = fake

	uc := NewExecuteAdHocStep(registry)
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
}

func TestExecuteAdHocStep_RequiresTenantContext(t *testing.T) {
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeCondition] = &fakeStepExecutor{}
	uc := NewExecuteAdHocStep(registry)

	_, err := uc.Execute(context.Background(), ExecuteAdHocStepInput{StepType: domain.StepTypeCondition})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestExecuteAdHocStep_RejectsInvalidStepType(t *testing.T) {
	registry := newFakeRegistry()
	uc := NewExecuteAdHocStep(registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepType("bogus")})
	if err == nil {
		t.Fatal("expected an error for an invalid step type")
	}
}

func TestExecuteAdHocStep_UnregisteredStepTypePropagatesFailedPrecondition(t *testing.T) {
	registry := newFakeRegistry() // nothing registered
	uc := NewExecuteAdHocStep(registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepTypeShell})
	if err == nil {
		t.Fatal("expected an error when no executor is registered for the step type")
	}
}

func TestExecuteAdHocStep_ExecutorErrorPropagates(t *testing.T) {
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeAgent] = &fakeStepExecutor{err: errors.New("not implemented")}
	uc := NewExecuteAdHocStep(registry)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ExecuteAdHocStepInput{StepType: domain.StepTypeAgent})
	if err == nil {
		t.Fatal("expected the executor's error to propagate")
	}
}
