package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestDeleteAccount_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteAccount(newFakeAccountRepository())
	err := uc.Execute(context.Background(), "acc-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteAccount_RequiresAccountID(t *testing.T) {
	uc := NewDeleteAccount(newFakeAccountRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected an error for missing account_id")
	}
}

func TestDeleteAccount_ForwardsTenantIDAndAccountIDToRepository(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.accounts["acc-1"] = domain.ProviderAccount{ID: "acc-1", TenantID: "tenant-1"}
	uc := NewDeleteAccount(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, "acc-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastDeleteTenantID != "tenant-1" {
		t.Errorf("expected TenantID to come from request context, got %q", repo.lastDeleteTenantID)
	}
	if repo.lastDeleteAccountID != "acc-1" {
		t.Errorf("expected AccountID to pass through unmodified, got %q", repo.lastDeleteAccountID)
	}
}

func TestDeleteAccount_PropagatesRepositoryFailure(t *testing.T) {
	repo := newFakeAccountRepository()
	repo.deleteErr = errBoom
	uc := NewDeleteAccount(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, "acc-1"); err == nil {
		t.Fatal("expected repository failure to propagate")
	}
}
