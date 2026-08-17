package stepexecutors

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestConditionExecutor_EvaluatesTrue(t *testing.T) {
	exec := NewConditionExecutor()
	cfg := `{"expression":"status == \"active\"","context":{"status":"active"}}`

	result, err := exec.Execute(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if result.OutputJSON != `{"result":true}` {
		t.Errorf("unexpected output JSON: %s", result.OutputJSON)
	}
}

func TestConditionExecutor_UnparseableExpressionIsFailedNotError(t *testing.T) {
	exec := NewConditionExecutor()
	cfg := `{"expression":"status ==","context":{}}`

	result, err := exec.Execute(context.Background(), cfg)
	// Fail-safe: an unparseable condition is a business-level "failed" step
	// outcome, not an executor error — matches domain.EvaluateCondition's
	// "fail-safe-false on unparseable input" doc.
	if err != nil {
		t.Fatalf("unexpected error (should fail safe, not error): %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestConditionExecutor_RejectsInvalidConfigJSON(t *testing.T) {
	exec := NewConditionExecutor()
	_, err := exec.Execute(context.Background(), "{not json")
	if err == nil {
		t.Fatal("expected an error for invalid config JSON")
	}
}

func TestStubExecutors_ReturnNotImplemented(t *testing.T) {
	for name, exec := range map[string]domain.StepExecutor{
		"agent":        NewAgentStub(),
		"shell":        NewShellStub(),
		"notification": NewNotificationStub(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := exec.Execute(context.Background(), "{}")
			if err == nil {
				t.Fatalf("expected %s stub to return an error", name)
			}
		})
	}
}
