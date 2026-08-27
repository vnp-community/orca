package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func TestDeleteAutomation_RemovesRow(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)

	uc := NewDeleteAutomation(repo)
	if err := uc.Execute(context.Background(), DeleteAutomationInput{TenantID: "tenant-1", ID: "auto-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.byID["auto-1"]; ok {
		t.Error("expected the automation to be removed from the repository")
	}
}

func TestDeleteAutomation_RequiresID(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewDeleteAutomation(repo)

	err := uc.Execute(context.Background(), DeleteAutomationInput{TenantID: "tenant-1", ID: ""})
	if err == nil {
		t.Fatal("expected an error when id is empty")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindInvalidArgument {
		t.Errorf("expected KindInvalidArgument, got %v", appErr.Kind)
	}
}

// fakeDeleteErrorRepository fails Delete so the error-translation path
// (apperrors.KindInternal) is exercised without a real Postgres — mirrors
// the "not found for tenant" 0-rows-affected error the real Postgres
// adapter returns when tenantID doesn't own id.
type fakeDeleteErrorRepository struct {
	fakeAutomationRepository
	deleteErr error
}

func (f *fakeDeleteErrorRepository) Delete(ctx context.Context, tenantID, id string) error {
	return f.deleteErr
}

func TestDeleteAutomation_PropagatesRepositoryErrorAsInternal(t *testing.T) {
	repo := &fakeDeleteErrorRepository{
		fakeAutomationRepository: fakeAutomationRepository{byID: map[string]domain.Automation{}},
		deleteErr:                errors.New("automation auto-1 not found for tenant tenant-2"),
	}
	uc := NewDeleteAutomation(repo)

	err := uc.Execute(context.Background(), DeleteAutomationInput{TenantID: "tenant-2", ID: "auto-1"})
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
