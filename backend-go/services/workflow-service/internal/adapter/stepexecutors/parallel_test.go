package stepexecutors

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeSubExecutor is a domain.StepExecutor recording its call count
// (mutex-guarded — sub-steps run concurrently via ParallelExecutor) and
// returning a fixed, configurable result/error.
type fakeSubExecutor struct {
	mu     sync.Mutex
	calls  int32
	result domain.StepResult
	err    error
}

func (f *fakeSubExecutor) Execute(_ context.Context, _ string) (domain.StepResult, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result, f.err
}

func (f *fakeSubExecutor) callCount() int32 {
	return atomic.LoadInt32(&f.calls)
}

func TestParallelExecutor_AllSubStepsSucceed(t *testing.T) {
	registry := NewRegistry()
	registry.Register(domain.StepTypeShell, &fakeSubExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`}})

	e := NewParallelExecutor()
	e.SetRegistry(registry)

	cfg, _ := json.Marshal(domain.ParallelStepConfig{SubSteps: []domain.Step{
		{ID: "a", Type: domain.StepTypeShell},
		{ID: "b", Type: domain.StepTypeShell},
	}})
	result, err := e.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed, got %v", result.Status)
	}
	var agg map[string]subStepOutcome
	if err := json.Unmarshal([]byte(result.OutputJSON), &agg); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(agg) != 2 || agg["a"].Status != domain.ResultStatusCompleted || agg["b"].Status != domain.ResultStatusCompleted {
		t.Errorf("expected both sub-steps completed in the aggregate, got %+v", agg)
	}
}

func TestParallelExecutor_OneFails_NoPartialFailure_AggregateFails_ButEveryStepRan(t *testing.T) {
	registry := NewRegistry()
	ok := &fakeSubExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	fail := &fakeSubExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed}}
	registry.Register(domain.StepTypeShell, ok)
	registry.Register(domain.StepTypeWebhook, fail)

	e := NewParallelExecutor()
	e.SetRegistry(registry)

	cfg, _ := json.Marshal(domain.ParallelStepConfig{
		SubSteps: []domain.Step{
			{ID: "a", Type: domain.StepTypeShell},
			{ID: "b", Type: domain.StepTypeWebhook},
			{ID: "c", Type: domain.StepTypeShell},
		},
		AllowPartialFailure: false,
	})
	result, err := e.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an aggregate error when AllowPartialFailure=false and a sub-step failed")
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected aggregate status failed, got %v", result.Status)
	}
	// Every sub-step still ran, despite the failure — allSettled semantics.
	if ok.callCount() != 2 {
		t.Errorf("expected the shell executor called twice (steps a and c), got %d", ok.callCount())
	}
	if fail.callCount() != 1 {
		t.Errorf("expected the webhook executor called once (step b), got %d", fail.callCount())
	}
}

func TestParallelExecutor_OneFails_AllowPartialFailure_AggregateCompletes(t *testing.T) {
	registry := NewRegistry()
	registry.Register(domain.StepTypeShell, &fakeSubExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}})
	registry.Register(domain.StepTypeWebhook, &fakeSubExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: `{"error":"boom"}`}})

	e := NewParallelExecutor()
	e.SetRegistry(registry)

	cfg, _ := json.Marshal(domain.ParallelStepConfig{
		SubSteps: []domain.Step{
			{ID: "a", Type: domain.StepTypeShell},
			{ID: "b", Type: domain.StepTypeWebhook},
		},
		AllowPartialFailure: true,
	})
	result, err := e.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error with AllowPartialFailure=true: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected aggregate status completed despite the partial failure, got %v", result.Status)
	}
	var agg map[string]subStepOutcome
	if err := json.Unmarshal([]byte(result.OutputJSON), &agg); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if agg["b"].Status != domain.ResultStatusFailed {
		t.Errorf("expected the failed sub-step's outcome still present in the aggregate, got %+v", agg["b"])
	}
}

func TestParallelExecutor_SubStepExecutorError_CountsAsFailure(t *testing.T) {
	registry := NewRegistry()
	registry.Register(domain.StepTypeShell, &fakeSubExecutor{err: errors.New("connection refused")})

	e := NewParallelExecutor()
	e.SetRegistry(registry)

	cfg, _ := json.Marshal(domain.ParallelStepConfig{SubSteps: []domain.Step{{ID: "a", Type: domain.StepTypeShell}}})
	_, err := e.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected a hard executor error on a sub-step to fail the aggregate")
	}
}

func TestParallelExecutor_UnresolvedSubStepType_CountsAsFailure(t *testing.T) {
	registry := NewRegistry() // nothing registered
	e := NewParallelExecutor()
	e.SetRegistry(registry)

	cfg, _ := json.Marshal(domain.ParallelStepConfig{SubSteps: []domain.Step{{ID: "a", Type: domain.StepTypeShell}}})
	_, err := e.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an error when a sub-step's type has no registered executor")
	}
}
