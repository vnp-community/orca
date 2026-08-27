package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// fakeAutomationRepository is an in-memory usecase.AutomationRepository —
// lets these contract tests exercise the real request -> usecase ->
// response translation in Server without a live Postgres. Server wires
// concrete *usecase.X types (not port interfaces), so "fake the usecase" per
// SOL-033's test plan means faking the port one layer down instead.
type fakeAutomationRepository struct {
	byID map[string]domain.Automation

	updateCalls []domain.Automation
	deleteCalls []struct{ tenantID, id string }
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

func (f *fakeAutomationRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	var out []domain.Automation
	for _, a := range f.byID {
		if a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	return out, "", nil
}

func (f *fakeAutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	f.updateCalls = append(f.updateCalls, a)
	f.byID[a.ID] = a
	return nil
}

func (f *fakeAutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	f.deleteCalls = append(f.deleteCalls, struct{ tenantID, id string }{tenantID, id})
	delete(f.byID, id)
	return nil
}

func (f *fakeAutomationRepository) CountByProject(ctx context.Context, tenantID, projectID string) (int, error) {
	count := 0
	for _, a := range f.byID {
		if a.TenantID == tenantID && a.ProjectID == projectID {
			count++
		}
	}
	return count, nil
}

func (f *fakeAutomationRepository) ListByTrigger(ctx context.Context, tenantID string, eventName domain.EventName) ([]domain.Automation, error) {
	var out []domain.Automation
	for _, a := range f.byID {
		if a.TenantID == tenantID && a.TriggerType == domain.TriggerTypeEvent && a.TriggerEvent == eventName && a.Enabled {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAutomationRepository) ListEventTriggered(ctx context.Context, tenantID string) ([]domain.Automation, error) {
	var out []domain.Automation
	for _, a := range f.byID {
		if a.TenantID == tenantID && a.TriggerType == domain.TriggerTypeEvent {
			out = append(out, a)
		}
	}
	return out, nil
}

func newServerForListUpdateDelete(repo *fakeAutomationRepository) *Server {
	return New(nil, nil, nil, nil,
		usecase.NewListAutomations(repo),
		usecase.NewUpdateAutomation(repo),
		usecase.NewDeleteAutomation(repo),
		nil,
	)
}

func seedGRPCAutomation(t *testing.T, repo *fakeAutomationRepository, tenantID, id string) domain.Automation {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := domain.NewAutomation(domain.NewAutomationParams{
		ID: id, TenantID: tenantID, Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{"prompt":"summarize"}`,
		DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("building automation: %v", err)
	}
	_ = repo.Create(context.Background(), a)
	return a
}

func TestServer_ListAutomations_TranslatesRequestAndResponse(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedGRPCAutomation(t, repo, "tenant-1", "auto-1")
	seedGRPCAutomation(t, repo, "tenant-1", "auto-2")
	seedGRPCAutomation(t, repo, "tenant-2", "auto-3")

	s := newServerForListUpdateDelete(repo)
	resp, err := s.ListAutomations(context.Background(), &automationv1.ListAutomationsRequest{TenantId: "tenant-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetAutomations()) != 2 {
		t.Fatalf("expected 2 automations for tenant-1, got %d", len(resp.GetAutomations()))
	}
	for _, a := range resp.GetAutomations() {
		if a.GetTenantId() != "tenant-1" {
			t.Errorf("expected only tenant-1 automations, got tenant_id=%q", a.GetTenantId())
		}
	}
}

func TestServer_ListAutomations_MissingTenantIDReturnsInvalidArgument(t *testing.T) {
	repo := newFakeAutomationRepository()
	s := newServerForListUpdateDelete(repo)

	_, err := s.ListAutomations(context.Background(), &automationv1.ListAutomationsRequest{})
	if err == nil {
		t.Fatal("expected an error when tenant_id is missing")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_UpdateAutomation_UnsetWrapperFieldsLeaveOtherFieldsUnchanged(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedGRPCAutomation(t, repo, "tenant-1", "auto-1")

	s := newServerForListUpdateDelete(repo)

	// Only "enabled" is set on the wire request — the regression guard: a
	// caller toggling one field must not zero-value-overwrite name/rrule/etc.
	resp, err := s.UpdateAutomation(context.Background(), &automationv1.UpdateAutomationRequest{
		Id:       "auto-1",
		TenantId: "tenant-1",
		Enabled:  wrapperspb.Bool(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetAutomation().GetEnabled() {
		t.Error("expected enabled=false to be applied")
	}
	if resp.GetAutomation().GetName() != "nightly-report" {
		t.Errorf("expected name to remain unchanged, got %q", resp.GetAutomation().GetName())
	}
	if resp.GetAutomation().GetRrule() != "FREQ=DAILY;INTERVAL=1" {
		t.Errorf("expected rrule to remain unchanged, got %q", resp.GetAutomation().GetRrule())
	}

	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected exactly 1 Update call, got %d", len(repo.updateCalls))
	}
	if repo.updateCalls[0].Name != "nightly-report" {
		t.Errorf("expected the persisted automation to keep its original name, got %q", repo.updateCalls[0].Name)
	}
}

func TestServer_UpdateAutomation_AppliesEveryWrapperField(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedGRPCAutomation(t, repo, "tenant-1", "auto-1")

	s := newServerForListUpdateDelete(repo)
	resp, err := s.UpdateAutomation(context.Background(), &automationv1.UpdateAutomationRequest{
		Id:             "auto-1",
		TenantId:       "tenant-1",
		Name:           wrapperspb.String("renamed"),
		Rrule:          wrapperspb.String("FREQ=WEEKLY"),
		StepConfigJson: wrapperspb.String(`{"prompt":"new"}`),
		StepType:       workflowv1.StepType_STEP_TYPE_SHELL,
		Enabled:        wrapperspb.Bool(false),
		Timezone:       wrapperspb.String("Asia/Ho_Chi_Minh"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := resp.GetAutomation()
	if a.GetName() != "renamed" {
		t.Errorf("expected name=renamed, got %q", a.GetName())
	}
	if a.GetRrule() != "FREQ=WEEKLY" {
		t.Errorf("expected rrule=FREQ=WEEKLY, got %q", a.GetRrule())
	}
	if a.GetStepConfigJson() != `{"prompt":"new"}` {
		t.Errorf("expected step_config_json updated, got %q", a.GetStepConfigJson())
	}
	if a.GetStepType() != workflowv1.StepType_STEP_TYPE_SHELL {
		t.Errorf("expected step_type=shell, got %v", a.GetStepType())
	}
	if a.GetEnabled() {
		t.Error("expected enabled=false")
	}
	if a.GetTimezone() != "Asia/Ho_Chi_Minh" {
		t.Errorf("expected timezone updated, got %q", a.GetTimezone())
	}
}

func TestServer_UpdateAutomation_InvalidDtstartReturnsInvalidArgument(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedGRPCAutomation(t, repo, "tenant-1", "auto-1")

	s := newServerForListUpdateDelete(repo)
	_, err := s.UpdateAutomation(context.Background(), &automationv1.UpdateAutomationRequest{
		Id:       "auto-1",
		TenantId: "tenant-1",
		Dtstart:  wrapperspb.String("not-a-date"),
	})
	if err == nil {
		t.Fatal("expected an error for a malformed dtstart")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestServer_UpdateAutomation_NotFoundReturnsNotFound(t *testing.T) {
	repo := newFakeAutomationRepository()
	s := newServerForListUpdateDelete(repo)

	_, err := s.UpdateAutomation(context.Background(), &automationv1.UpdateAutomationRequest{Id: "missing", TenantId: "tenant-1"})
	if err == nil {
		t.Fatal("expected an error for a missing automation")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestServer_DeleteAutomation_CallsRepositoryWithTenantAndID(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedGRPCAutomation(t, repo, "tenant-1", "auto-1")

	s := newServerForListUpdateDelete(repo)
	_, err := s.DeleteAutomation(context.Background(), &automationv1.DeleteAutomationRequest{Id: "auto-1", TenantId: "tenant-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deleteCalls) != 1 {
		t.Fatalf("expected exactly 1 Delete call, got %d", len(repo.deleteCalls))
	}
	if repo.deleteCalls[0].tenantID != "tenant-1" || repo.deleteCalls[0].id != "auto-1" {
		t.Errorf("expected Delete(tenant-1, auto-1), got Delete(%q, %q)", repo.deleteCalls[0].tenantID, repo.deleteCalls[0].id)
	}
	if _, ok := repo.byID["auto-1"]; ok {
		t.Error("expected the automation to be removed")
	}
}
