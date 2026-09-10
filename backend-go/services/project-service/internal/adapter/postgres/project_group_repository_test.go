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

// TestProjectGroupRepository_ImportNested_StampsDevServerIDOnRepoToo proves
// Phase 10's fix: the dev server the candidates were scanned on must land on
// the created REPO row, not just its owning project — previously only the
// project got dev_server_id, leaving the repo's own binding unset.
func TestProjectGroupRepository_ImportNested_StampsDevServerIDOnRepoToo(t *testing.T) {
	pool := setupPool(t)
	groupRepo := NewProjectGroupRepository(pool)
	repoRepo := NewRepoRepository(pool)
	ctx := context.Background()

	tenantID := uuid.NewString()
	userID := uuid.NewString()
	devServerID := uuid.NewString()

	candidates := []domain.NestedRepoCandidate{
		{Path: "/home/dev/repo-a", SuggestedName: "repo-a", IsGitRepo: true},
	}
	_, projects, err := groupRepo.ImportNested(ctx, tenantID, userID, devServerID, "", candidates)
	if err != nil {
		t.Fatalf("import nested: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected exactly 1 project, got %d: %+v", len(projects), projects)
	}
	if projects[0].DevServerID != devServerID {
		t.Errorf("expected project DevServerID=%q, got %q", devServerID, projects[0].DevServerID)
	}

	repos, err := repoRepo.ListRepos(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("list repos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected exactly 1 repo, got %d: %+v", len(repos), repos)
	}
	if repos[0].DevServerID != devServerID {
		t.Errorf("expected repo DevServerID=%q (stamped from the scan), got %q", devServerID, repos[0].DevServerID)
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
