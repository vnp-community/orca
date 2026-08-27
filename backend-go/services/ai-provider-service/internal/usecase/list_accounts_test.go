package usecase

import (
	"context"
	"testing"
)

func TestListAccounts_FiltersByDevServerID(t *testing.T) {
	repo := newFakeAccountRepository()
	uc := NewListAccounts(repo)

	ctx := withIdentity(context.Background(), "t1", "user-1")
	if _, err := uc.Execute(ctx, ListAccountsInput{DevServerID: "ds-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastListFilter.DevServerID != "ds-1" {
		t.Errorf("filter.DevServerID = %q, want ds-1", repo.lastListFilter.DevServerID)
	}
	if repo.lastListFilter.TenantID != "t1" {
		t.Errorf("filter.TenantID = %q, want t1", repo.lastListFilter.TenantID)
	}
}

func TestListAccounts_NoTenant_Errors(t *testing.T) {
	repo := newFakeAccountRepository()
	uc := NewListAccounts(repo)
	if _, err := uc.Execute(context.Background(), ListAccountsInput{}); err == nil {
		t.Fatal("expected an error with no tenant in context")
	}
}
