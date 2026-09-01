package usecase

import (
	"context"
	"testing"
)

func TestRevokeDevServerGroupAccess_RequiresAdmin(t *testing.T) {
	uc := NewRevokeDevServerGroupAccess(&fakeDevServerGroupGrantRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "grant1"); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestRevokeDevServerGroupAccess_Deletes(t *testing.T) {
	repo := &fakeDevServerGroupGrantRepository{}
	uc := NewRevokeDevServerGroupAccess(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "grant1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "grant1" {
		t.Errorf("expected grant1 to be deleted, got %+v", repo.deleted)
	}
}
