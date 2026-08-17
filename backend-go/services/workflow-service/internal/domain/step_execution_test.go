package domain

import (
	"errors"
	"testing"
)

func TestNewStepExecution_Pending(t *testing.T) {
	se, err := NewStepExecution("se-1", "exec-1", "step-1", "token-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if se.Status != StepExecutionStatusPending {
		t.Errorf("expected status=pending, got %v", se.Status)
	}
	if se.Terminal() {
		t.Error("expected a freshly-constructed step execution to not be terminal")
	}
}

func TestNewStepExecution_RequiresExecutionID(t *testing.T) {
	_, err := NewStepExecution("se-1", "", "step-1", "token-1", 0)
	if !errors.Is(err, ErrStepExecutionEmptyExecutionID) {
		t.Fatalf("expected ErrStepExecutionEmptyExecutionID, got %v", err)
	}
}

func TestNewStepExecution_RequiresStepID(t *testing.T) {
	_, err := NewStepExecution("se-1", "exec-1", "", "token-1", 0)
	if !errors.Is(err, ErrStepExecutionEmptyStepID) {
		t.Fatalf("expected ErrStepExecutionEmptyStepID, got %v", err)
	}
}

func TestStepExecution_MarkRunning(t *testing.T) {
	se, _ := NewStepExecution("se-1", "exec-1", "step-1", "token-1", 0)
	se.MarkRunning()
	if se.Status != StepExecutionStatusRunning {
		t.Errorf("expected status=running, got %v", se.Status)
	}
	if se.Terminal() {
		t.Error("expected running to not be terminal")
	}
}

func TestStepExecution_FromResult_Completed(t *testing.T) {
	se, _ := NewStepExecution("se-1", "exec-1", "step-1", "token-1", 0)
	se.MarkRunning()
	se.FromResult(StepResult{Status: ResultStatusCompleted, OutputJSON: `{"ok":true}`})
	if se.Status != StepExecutionStatusCompleted {
		t.Errorf("expected status=completed, got %v", se.Status)
	}
	if se.OutputJSON != `{"ok":true}` {
		t.Errorf("expected output to be recorded verbatim, got %q", se.OutputJSON)
	}
	if !se.Terminal() {
		t.Error("expected completed to be terminal")
	}
}

func TestStepExecution_FromResult_Failed(t *testing.T) {
	se, _ := NewStepExecution("se-1", "exec-1", "step-1", "token-1", 0)
	se.MarkRunning()
	se.FromResult(StepResult{Status: ResultStatusFailed, OutputJSON: `{"error":"bad"}`})
	if se.Status != StepExecutionStatusFailed {
		t.Errorf("expected status=failed, got %v", se.Status)
	}
	if !se.Terminal() {
		t.Error("expected failed to be terminal")
	}
}

func TestStepExecution_Fail(t *testing.T) {
	se, _ := NewStepExecution("se-1", "exec-1", "step-1", "token-1", 0)
	se.MarkRunning()
	se.Fail("executor unreachable")
	if se.Status != StepExecutionStatusFailed {
		t.Errorf("expected status=failed, got %v", se.Status)
	}
	if se.Error != "executor unreachable" {
		t.Errorf("expected error message to be recorded, got %q", se.Error)
	}
	if !se.Terminal() {
		t.Error("expected failed to be terminal")
	}
}
