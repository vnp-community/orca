package stepexecutors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

func TestActionExecutor_UnregisteredActionName_ReturnsTypedError(t *testing.T) {
	e := NewActionExecutor()
	cfg, _ := json.Marshal(domain.ActionStepConfig{ActionName: "does-not-exist"})

	_, err := e.Execute(context.Background(), string(cfg))
	if !errors.Is(err, usecase.ErrNoActionHandlerRegistered) {
		t.Fatalf("expected ErrNoActionHandlerRegistered, got %v", err)
	}
}

func TestActionExecutor_RegisteredHandler_Runs(t *testing.T) {
	e := NewActionExecutor()
	var gotParams string
	e.Register("greet", func(_ context.Context, paramsJSON string) (domain.StepResult, error) {
		gotParams = paramsJSON
		return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"greeted":true}`}, nil
	})

	cfg, _ := json.Marshal(domain.ActionStepConfig{ActionName: "greet", Params: json.RawMessage(`{"name":"world"}`)})
	result, err := e.Execute(context.Background(), string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed, got %v", result.Status)
	}
	if gotParams != `{"name":"world"}` {
		t.Errorf("expected params forwarded, got %q", gotParams)
	}
}

func TestActionExecutor_InvalidConfigJSON_Errors(t *testing.T) {
	e := NewActionExecutor()
	_, err := e.Execute(context.Background(), "{not json")
	if err == nil {
		t.Fatal("expected an error for malformed config JSON")
	}
}
