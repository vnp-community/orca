package usecase

import (
	"context"
	"testing"
)

func TestDeleteBrowserProfile_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteBrowserProfile(&fakeBrowserProfileRepository{})
	err := uc.Execute(context.Background(), "bp-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteBrowserProfile_EmptyID_FailsValidation(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewDeleteBrowserProfile(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected an error for a missing id")
	}
}

func TestDeleteBrowserProfile_PassesTenantIDAndIDThrough(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewDeleteBrowserProfile(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "bp-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.deletedTenantID != "tenant-1" {
		t.Errorf("deletedTenantID = %q, want tenant-1", repo.deletedTenantID)
	}
	if repo.deletedID != "bp-1" {
		t.Errorf("deletedID = %q, want bp-1", repo.deletedID)
	}
}
