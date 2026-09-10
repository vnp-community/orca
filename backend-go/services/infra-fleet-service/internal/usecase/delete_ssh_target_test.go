package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestDeleteSshTarget_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteSshTarget(&fakeSshTargetRepository{})
	err := uc.Execute(context.Background(), "target-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteSshTarget_CallsRepositoryDeleteWithTenantFromContext(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewDeleteSshTarget(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "target-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deleted) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(repo.deleted))
	}
	if repo.deleted[0] != [2]string{"tenant-1", "target-1"} {
		t.Errorf("expected (tenant-1, target-1), got %v", repo.deleted[0])
	}
}

func TestDeleteSshTarget_RepositoryErrorWrapped(t *testing.T) {
	repo := &fakeSshTargetRepository{deleteErr: errors.New("db unavailable")}
	uc := NewDeleteSshTarget(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, "target-1")
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
