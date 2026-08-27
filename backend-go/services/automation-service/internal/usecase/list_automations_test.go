package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func TestListAutomations_ReturnsOnlyTenantScopedRows(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	seedAutomation(t, repo, "tenant-1", "auto-2", `{"step_type":"agent"}`)
	seedAutomation(t, repo, "tenant-2", "auto-3", `{"step_type":"agent"}`)

	uc := NewListAutomations(repo)
	result, err := uc.Execute(context.Background(), ListAutomationsInput{TenantID: "tenant-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Automations) != 2 {
		t.Fatalf("expected 2 automations for tenant-1, got %d", len(result.Automations))
	}
	for _, a := range result.Automations {
		if a.TenantID != "tenant-1" {
			t.Errorf("expected only tenant-1 rows, got tenant_id=%q", a.TenantID)
		}
	}
}

func TestListAutomations_RequiresTenantID(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewListAutomations(repo)

	_, err := uc.Execute(context.Background(), ListAutomationsInput{})
	if err == nil {
		t.Fatal("expected an error when tenant_id is empty")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindInvalidArgument {
		t.Errorf("expected KindInvalidArgument, got %v", appErr.Kind)
	}
}

// fakeListErrorRepository fails List so ListAutomations' error-translation
// path (apperrors.KindInternal) is exercised without a real Postgres.
type fakeListErrorRepository struct {
	fakeAutomationRepository
	listErr error
}

func (f *fakeListErrorRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Automation, string, error) {
	return nil, "", f.listErr
}

func TestListAutomations_PropagatesRepositoryErrorAsInternal(t *testing.T) {
	repo := &fakeListErrorRepository{
		fakeAutomationRepository: fakeAutomationRepository{byID: map[string]domain.Automation{}},
		listErr:                  errors.New("boom"),
	}
	uc := NewListAutomations(repo)

	_, err := uc.Execute(context.Background(), ListAutomationsInput{TenantID: "tenant-1"})
	if err == nil {
		t.Fatal("expected an error when the repository fails")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindInternal {
		t.Errorf("expected KindInternal, got %v", appErr.Kind)
	}
}
