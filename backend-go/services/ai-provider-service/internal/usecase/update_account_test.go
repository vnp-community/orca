package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestUpdateAccount_RequiresTenantContext(t *testing.T) {
	uc := NewUpdateAccount(newFakeAccountRepository())
	_, err := uc.Execute(context.Background(), UpdateFields{AccountID: "acc-1", Label: "new label"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateAccount_RequiresAccountID(t *testing.T) {
	uc := NewUpdateAccount(newFakeAccountRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, UpdateFields{Label: "new label"})
	if err == nil {
		t.Fatal("expected an error for missing account_id")
	}
}

func TestUpdateAccount_ForwardsTenantIDAndFieldsToRepository(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.accounts["acc-1"] = domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1"}
	uc := NewUpdateAccount(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, UpdateFields{AccountID: "acc-1", Label: "new label", ModelHint: "sonnet", BaseURL: "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastUpdateInput.TenantID != "tenant-1" {
		t.Errorf("expected TenantID to come from request context, got %q", repo.lastUpdateInput.TenantID)
	}
	if repo.lastUpdateInput.AccountID != "acc-1" || repo.lastUpdateInput.Label != "new label" ||
		repo.lastUpdateInput.ModelHint != "sonnet" || repo.lastUpdateInput.BaseURL != "https://example.com" {
		t.Errorf("expected fields to pass through unmodified, got %+v", repo.lastUpdateInput)
	}
}

func TestUpdateAccount_PropagatesRepositoryFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.updateErr = errBoom
	uc := NewUpdateAccount(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, UpdateFields{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected repository failure to propagate")
	}
}
