package stepexecutors

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestActionExecutor_DispatchesToRegisteredHandler(t *testing.T) {
	var gotParams map[string]any
	handlers := map[string]ActionHandler{
		"git.createBranch": func(ctx context.Context, params map[string]any) (domain.StepResult, error) {
			gotParams = params
			return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"branch":"feature/x"}`}, nil
		},
	}
	exec := NewActionExecutor(handlers)

	cfg, _ := json.Marshal(domain.ActionStepConfig{Action: "git.createBranch", Params: map[string]any{"name": "feature/x"}})
	result, err := exec.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if gotParams["name"] != "feature/x" {
		t.Errorf("expected params to reach the handler, got %v", gotParams)
	}
}

// TestActionExecutor_UnregisteredActionFailsClearlyNotPanic covers the
// task's own expectation: an unregistered action name fails clearly
// rather than panicking.
func TestActionExecutor_UnregisteredActionFailsClearlyNotPanic(t *testing.T) {
	exec := NewActionExecutor(map[string]ActionHandler{})

	cfg, _ := json.Marshal(domain.ActionStepConfig{Action: "does.not.exist"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an error for an unregistered action")
	}
}

func TestActionExecutor_HandlerErrorPropagates(t *testing.T) {
	handlers := map[string]ActionHandler{
		"git.createBranch": func(ctx context.Context, params map[string]any) (domain.StepResult, error) {
			return domain.StepResult{}, context.DeadlineExceeded
		},
	}
	exec := NewActionExecutor(handlers)

	cfg, _ := json.Marshal(domain.ActionStepConfig{Action: "git.createBranch"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected the handler's error to propagate")
	}
}

func TestActionExecutor_InvalidConfigJSONErrors(t *testing.T) {
	exec := NewActionExecutor(map[string]ActionHandler{})
	_, err := exec.Execute(context.Background(), "not json")
	if err == nil {
		t.Fatal("expected an error for invalid step config JSON")
	}
}
