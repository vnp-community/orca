//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestProjectGroupRepository_CreateAndGet_RoundTrips(t *testing.T) {
	pool := setupPool(t)
	groupRepo := NewProjectGroupRepository(pool)
	ctx := context.Background()

	tenantID := uuid.NewString()
	g, err := domain.NewProjectGroup(uuid.NewString(), tenantID, "group-a", "")
	if err != nil {
		t.Fatalf("building group: %v", err)
	}

	created, err := groupRepo.CreateProjectGroup(ctx, g)
	if err != nil {
		t.Fatalf("create project group: %v", err)
	}

	got, err := groupRepo.GetProjectGroup(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatalf("get project group: %v", err)
	}
	if got.Name != "group-a" {
		t.Errorf("expected Name=group-a, got %q", got.Name)
	}
}

func TestProjectGroupRepository_UpdateProjectGroup_RenamesOnly(t *testing.T) {
	pool := setupPool(t)
	groupRepo := NewProjectGroupRepository(pool)
	ctx := context.Background()

	tenantID := uuid.NewString()
	g, _ := domain.NewProjectGroup(uuid.NewString(), tenantID, "old-name", "")
	created, err := groupRepo.CreateProjectGroup(ctx, g)
	if err != nil {
		t.Fatalf("create project group: %v", err)
	}

	updated, err := groupRepo.UpdateProjectGroup(ctx, tenantID, created.ID, "new-name")
	if err != nil {
		t.Fatalf("update project group: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("expected Name=new-name, got %q", updated.Name)
	}
}

func TestProjectGroupRepository_DeleteProjectGroup_CascadesToChildren(t *testing.T) {
	pool := setupPool(t)
	groupRepo := NewProjectGroupRepository(pool)
	ctx := context.Background()

	tenantID := uuid.NewString()
	parent, _ := domain.NewProjectGroup(uuid.NewString(), tenantID, "parent", "")
	createdParent, err := groupRepo.CreateProjectGroup(ctx, parent)
	if err != nil {
		t.Fatalf("create parent group: %v", err)
	}

	child, _ := domain.NewProjectGroup(uuid.NewString(), tenantID, "child", createdParent.ID)
	createdChild, err := groupRepo.CreateProjectGroup(ctx, child)
	if err != nil {
		t.Fatalf("create child group: %v", err)
	}

	if err := groupRepo.DeleteProjectGroup(ctx, tenantID, createdParent.ID); err != nil {
		t.Fatalf("delete parent group: %v", err)
	}

	if _, err := groupRepo.GetProjectGroup(ctx, tenantID, createdChild.ID); err != domain.ErrProjectGroupNotFound {
		t.Errorf("expected child group to cascade-delete with parent, got %v", err)
	}
}

func TestProjectGroupRepository_ListProjectGroups_ScopedToTenant(t *testing.T) {
	pool := setupPool(t)
	groupRepo := NewProjectGroupRepository(pool)
	ctx := context.Background()

	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	ga, _ := domain.NewProjectGroup(uuid.NewString(), tenantA, "group-a", "")
	gb, _ := domain.NewProjectGroup(uuid.NewString(), tenantB, "group-b", "")
	_, _ = groupRepo.CreateProjectGroup(ctx, ga)
	_, _ = groupRepo.CreateProjectGroup(ctx, gb)

	groups, err := groupRepo.ListProjectGroups(ctx, tenantA)
	if err != nil {
		t.Fatalf("list project groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "group-a" {
		t.Errorf("expected only tenant A's group, got %+v", groups)
	}
}
