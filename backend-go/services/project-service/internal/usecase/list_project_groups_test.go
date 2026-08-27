package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListProjectGroups_ScopedToTenant(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	repo.groups["g1"] = domain.ProjectGroup{ID: "g1", TenantID: "tenant-1", Name: "group-a"}
	repo.groups["g2"] = domain.ProjectGroup{ID: "g2", TenantID: "tenant-2", Name: "group-b"}
	uc := NewListProjectGroups(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "g1" {
		t.Errorf("expected only g1, got %+v", got)
	}
}

func TestListProjectGroups_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectGroupRepository()
	uc := NewListProjectGroups(repo)

	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
