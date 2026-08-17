package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// fakeAutomationRepository is an in-memory AutomationRepository — the "test
// against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeAutomationRepository struct {
	byID map[string]domain.Automation
}

func newFakeAutomationRepository() *fakeAutomationRepository {
	return &fakeAutomationRepository{byID: map[string]domain.Automation{}}
}

func (f *fakeAutomationRepository) Create(ctx context.Context, a domain.Automation) error {
	f.byID[a.ID] = a
	return nil
}

func (f *fakeAutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	a, ok := f.byID[id]
	if !ok || a.TenantID != tenantID {
		return domain.Automation{}, errors.New("not found")
	}
	return a, nil
}

// fakeAutomationRunRepository is an in-memory AutomationRunRepository.
type fakeAutomationRunRepository struct {
	byID    map[string]domain.AutomationRun
	created int
}

func newFakeAutomationRunRepository() *fakeAutomationRunRepository {
	return &fakeAutomationRunRepository{byID: map[string]domain.AutomationRun{}}
}

func (f *fakeAutomationRunRepository) Create(ctx context.Context, run domain.AutomationRun) error {
	f.byID[run.ID] = run
	f.created++
	return nil
}

func (f *fakeAutomationRunRepository) FindByRequestID(ctx context.Context, tenantID, automationID, requestID string) (domain.AutomationRun, bool, error) {
	for _, r := range f.byID {
		if r.TenantID == tenantID && r.AutomationID == automationID && r.RequestID == requestID {
			return r, true, nil
		}
	}
	return domain.AutomationRun{}, false, nil
}

func (f *fakeAutomationRunRepository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	f.byID[run.ID] = run
	return nil
}

func (f *fakeAutomationRunRepository) ListByAutomation(ctx context.Context, tenantID, automationID, pageToken string, pageSize int32) ([]domain.AutomationRun, string, error) {
	var out []domain.AutomationRun
	for _, r := range f.byID {
		if r.TenantID == tenantID && r.AutomationID == automationID {
			out = append(out, r)
		}
	}
	return out, "", nil
}

// fakeWorkflowStepExecutor is a fake WorkflowStepExecutor — RunNow's tests
// verify it's called with the right step_config_json and that idempotent
// retries (same request_id) don't call it a second time.
type fakeWorkflowStepExecutor struct {
	calls  []ExecuteAdHocStepInput
	result ExecuteAdHocStepOutput
	err    error
}

func (f *fakeWorkflowStepExecutor) ExecuteAdHocStep(ctx context.Context, in ExecuteAdHocStepInput) (ExecuteAdHocStepOutput, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return ExecuteAdHocStepOutput{}, f.err
	}
	return f.result, nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func seedAutomation(t *testing.T, repo *fakeAutomationRepository, tenantID, id, stepConfigJSON string) domain.Automation {
	t.Helper()
	now := time.Now().UTC()
	a, err := domain.NewAutomation(id, tenantID, "nightly-report", "FREQ=DAILY;INTERVAL=1", stepConfigJSON, now, now)
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
	_ = repo.Create(context.Background(), a)
	return a
}

func TestRunNow_CallsWorkflowStepExecutorWithStepConfig(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed", OutputJSON: `{"ok":true}`}}
	uc := NewRunNow(automations, runs, executor)

	stepConfig := `{"step_type":"agent","prompt":"summarize the day"}`
	seedAutomation(t, automations, "tenant-1", "auto-1", stepConfig)

	ctx := withTenant(context.Background(), "tenant-1")
	run, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != domain.RunStatusSucceeded {
		t.Errorf("expected run to succeed, got status=%v", run.Status)
	}

	if len(executor.calls) != 1 {
		t.Fatalf("expected exactly 1 call to WorkflowStepExecutor, got %d", len(executor.calls))
	}
	call := executor.calls[0]
	if call.StepConfigJSON != stepConfig {
		t.Errorf("expected step_config_json %q to be passed through verbatim, got %q", stepConfig, call.StepConfigJSON)
	}
	if call.StepType != domain.StepTypeAgent {
		t.Errorf("expected step_type=agent parsed from step_config_json, got %v", call.StepType)
	}
	if call.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id=tenant-1, got %v", call.TenantID)
	}
	if call.RequestID != "req-1" {
		t.Errorf("expected request_id=req-1, got %v", call.RequestID)
	}
}

func TestRunNow_IdempotentRetrySameRequestIDDoesNotDuplicateOrRecall(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed", OutputJSON: `{}`}}
	uc := NewRunNow(automations, runs, executor)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)

	ctx := withTenant(context.Background(), "tenant-1")
	first, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-dup"})
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	second, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-dup"})
	if err != nil {
		t.Fatalf("unexpected error on retried call: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected the retried call to return the same run, got %q and %q", first.ID, second.ID)
	}
	if runs.created != 1 {
		t.Errorf("expected exactly 1 run to be created despite 2 RunNow calls, got %d", runs.created)
	}
	if len(executor.calls) != 1 {
		t.Errorf("expected exactly 1 call to WorkflowStepExecutor despite 2 RunNow calls, got %d", len(executor.calls))
	}
}

func TestRunNow_DifferentRequestIDsCreateDistinctRuns(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed", OutputJSON: `{}`}}
	uc := NewRunNow(automations, runs, executor)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"shell"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runs.created != 2 {
		t.Errorf("expected 2 distinct runs for 2 distinct request_ids, got %d", runs.created)
	}
	if len(executor.calls) != 2 {
		t.Errorf("expected 2 calls to WorkflowStepExecutor, got %d", len(executor.calls))
	}
}

func TestRunNow_WorkflowServiceFailurePropagatesAndRunRecordedFailed(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{err: errors.New("workflow-service unreachable")}
	uc := NewRunNow(automations, runs, executor)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected RunNow to fail closed when workflow-service is unreachable")
	}

	// The run row must still exist, recorded Failed — never silently
	// swallowed the way TS's skipped_unavailable was, per
	// automation-service.md §8/§10.
	found := false
	for _, r := range runs.byID {
		if r.AutomationID == "auto-1" && r.RequestID == "req-1" {
			found = true
			if r.Status != domain.RunStatusFailed {
				t.Errorf("expected the run to be recorded Failed, got %v", r.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected a run row to exist even though the workflow-service call failed")
	}
}

func TestRunNow_WorkflowServiceReportedFailureRecordsFailedRunWithoutError(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "failed", OutputJSON: `{"error":"step failed"}`}}
	uc := NewRunNow(automations, runs, executor)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	run, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error — a business-level step failure is not a RunNow error: %v", err)
	}
	if run.Status != domain.RunStatusFailed {
		t.Errorf("expected run status Failed, got %v", run.Status)
	}
}

func TestRunNow_RequiresTenantContext(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{}
	uc := NewRunNow(automations, runs, executor)

	_, err := uc.Execute(context.Background(), RunNowInput{AutomationID: "auto-1", RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRunNow_RequiresRequestID(t *testing.T) {
	automations := newFakeAutomationRepository()
	runs := newFakeAutomationRunRepository()
	executor := &fakeWorkflowStepExecutor{}
	uc := NewRunNow(automations, runs, executor)

	seedAutomation(t, automations, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, RunNowInput{AutomationID: "auto-1", RequestID: ""})
	if err == nil {
		t.Fatal("expected an error when request_id is empty")
	}
	if len(executor.calls) != 0 {
		t.Error("expected WorkflowStepExecutor to never be called without a request_id")
	}
}
