package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func TestHandleExternalTrigger_DispatchesAndRecordsExternalTrigger(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed", OutputJSON: `{}`}}
	runNow := NewRunNow(automations, runs, executor)
	uc := NewHandleExternalTrigger(runNow)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	run, err := uc.Execute(ctx, HandleExternalTriggerInput{
		AutomationID: "auto-1",
		RequestID:    "external-req-1",
		PayloadJSON:  `{"source":"github"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Trigger != domain.RunTriggerExternal {
		t.Errorf("expected Trigger=external, got %v", run.Trigger)
	}
	if run.RequestID != "external-req-1" {
		t.Errorf("expected the external source's request_id to be used verbatim, got %q", run.RequestID)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("expected exactly 1 dispatch, got %d", len(executor.calls))
	}
}

func TestHandleExternalTrigger_IdempotentOnRequestID(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed", OutputJSON: `{}`}}
	runNow := NewRunNow(automations, runs, executor)
	uc := NewHandleExternalTrigger(runNow)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	in := HandleExternalTriggerInput{AutomationID: "auto-1", RequestID: "external-req-1"}
	first, err := uc.Execute(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	second, err := uc.Execute(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error on retried call: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected the retried external trigger to return the same run, got %q and %q", first.ID, second.ID)
	}
	if len(executor.calls) != 1 {
		t.Errorf("expected exactly 1 dispatch despite 2 calls with the same request_id, got %d", len(executor.calls))
	}
}
