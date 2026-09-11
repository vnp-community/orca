//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestProjectGroupRepository_CreateGetUpdateDelete_RoundTrip(t *testing.T) {
	db := setupDB(t)
	groupRepo := NewProjectGroupRepository(db)
	ctx := context.Background()
	tenantID := uuid.NewString()

	g, err := domain.NewProjectGroup(uuid.NewString(), tenantID, "root", "")
	if err != nil {
		t.Fatalf("NewProjectGroup: %v", err)
	}
	created, err := groupRepo.CreateProjectGroup(ctx, g)
	if err != nil {
		t.Fatalf("CreateProjectGroup: %v", err)
	}

	child, err := domain.NewProjectGroup(uuid.NewString(), tenantID, "child", created.ID)
	if err != nil {
		t.Fatalf("NewProjectGroup child: %v", err)
	}
	if _, err := groupRepo.CreateProjectGroup(ctx, child); err != nil {
		t.Fatalf("CreateProjectGroup child: %v", err)
	}

	got, err := groupRepo.GetProjectGroup(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatalf("GetProjectGroup: %v", err)
	}
	if got.Name != "root" {
		t.Errorf("unexpected group: %+v", got)
	}

	renamed, err := groupRepo.UpdateProjectGroup(ctx, tenantID, created.ID, "root-renamed")
	if err != nil {
		t.Fatalf("UpdateProjectGroup: %v", err)
	}
	if renamed.Name != "root-renamed" {
		t.Errorf("expected renamed group, got %+v", renamed)
	}
	// No-op rename (same name the row already has) must also succeed — see
	// repository.go's UpdateDevServerID doc comment for the RowsAffected
	// pitfall this guards against.
	if _, err := groupRepo.UpdateProjectGroup(ctx, tenantID, created.ID, "root-renamed"); err != nil {
		t.Fatalf("no-op UpdateProjectGroup should still succeed, got: %v", err)
	}

	list, err := groupRepo.ListProjectGroups(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListProjectGroups: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 groups (root+child), got %d", len(list))
	}

	// DeleteProjectGroup cascades to descendants (ON DELETE CASCADE on
	// parent_group_id, migrations/mysql/0005).
	if err := groupRepo.DeleteProjectGroup(ctx, tenantID, created.ID); err != nil {
		t.Fatalf("DeleteProjectGroup: %v", err)
	}
	if _, err := groupRepo.GetProjectGroup(ctx, tenantID, child.ID); err != domain.ErrProjectGroupNotFound {
		t.Errorf("expected child group cascade-deleted, got %v", err)
	}
}

// TestProjectGroupRepository_ListProjectGroups_DoesNotLeakAcrossTenants is
// the TASK-BE-DB-003 pattern for a table with a direct tenant_id column and
// an RLS policy on the Postgres side (migrations/postgres/0005) that has no
// MySQL equivalent at all.
func TestProjectGroupRepository_ListProjectGroups_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	groupRepo := NewProjectGroupRepository(db)
	ctx := context.Background()

	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	ga, err := domain.NewProjectGroup(uuid.NewString(), tenantA, "a", "")
	if err != nil {
		t.Fatalf("NewProjectGroup A: %v", err)
	}
	if _, err := groupRepo.CreateProjectGroup(ctx, ga); err != nil {
		t.Fatalf("create A: %v", err)
	}
	gb, err := domain.NewProjectGroup(uuid.NewString(), tenantB, "b", "")
	if err != nil {
		t.Fatalf("NewProjectGroup B: %v", err)
	}
	if _, err := groupRepo.CreateProjectGroup(ctx, gb); err != nil {
		t.Fatalf("create B: %v", err)
	}

	listA, err := groupRepo.ListProjectGroups(ctx, tenantA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != ga.ID {
		t.Fatalf("tenant A should see only its own group, got %+v", listA)
	}
}

// TestProjectGroupRepository_UpsertLeafGroupForProject_IsIdempotent
// exercises the ON DUPLICATE KEY UPDATE translation of Postgres's
// `ON CONFLICT (project_id) WHERE project_id IS NOT NULL` — see
// migrations/mysql/0008's comment for why a plain UNIQUE index has the same
// effective scope.
func TestProjectGroupRepository_UpsertLeafGroupForProject_IsIdempotent(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	groupRepo := NewProjectGroupRepository(db)
	ctx := context.Background()

	tenantID := uuid.NewString()
	p := newTestProject(uuid.NewString(), tenantID, "leaf-owner")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	parent1, err := domain.NewProjectGroup(uuid.NewString(), tenantID, "parent-1", "")
	if err != nil {
		t.Fatalf("NewProjectGroup parent1: %v", err)
	}
	if _, err := groupRepo.CreateProjectGroup(ctx, parent1); err != nil {
		t.Fatalf("create parent1: %v", err)
	}
	parent2, err := domain.NewProjectGroup(uuid.NewString(), tenantID, "parent-2", "")
	if err != nil {
		t.Fatalf("NewProjectGroup parent2: %v", err)
	}
	if _, err := groupRepo.CreateProjectGroup(ctx, parent2); err != nil {
		t.Fatalf("create parent2: %v", err)
	}

	first, err := groupRepo.UpsertLeafGroupForProject(ctx, tenantID, p.ID, p.Name, parent1.ID)
	if err != nil {
		t.Fatalf("UpsertLeafGroupForProject (create): %v", err)
	}
	if first.ParentGroupID != parent1.ID {
		t.Fatalf("expected leaf group under parent1, got %+v", first)
	}

	second, err := groupRepo.UpsertLeafGroupForProject(ctx, tenantID, p.ID, p.Name, parent2.ID)
	if err != nil {
		t.Fatalf("UpsertLeafGroupForProject (update): %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("expected the SAME leaf group row reused (id unchanged), got first=%q second=%q", first.ID, second.ID)
	}
	if second.ParentGroupID != parent2.ID {
		t.Errorf("expected leaf group re-parented to parent2, got %+v", second)
	}

	all, err := groupRepo.ListProjectGroups(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListProjectGroups: %v", err)
	}
	leafCount := 0
	for _, g := range all {
		if g.ProjectID == p.ID {
			leafCount++
		}
	}
	if leafCount != 1 {
		t.Errorf("expected exactly one leaf group for the project (upsert, not duplicate insert), got %d", leafCount)
	}
}

// TestProjectGroupRepository_ImportNested_CreatesGroupsProjectsAndRepos
// proves the multi-table atomic transaction the Postgres source's
// ImportNested performs (see project_group_repository.go's doc comment for
// the pre-existing Postgres bug this MySQL version deliberately does NOT
// reproduce).
func TestProjectGroupRepository_ImportNested_CreatesGroupsProjectsAndRepos(t *testing.T) {
	db := setupDB(t)
	groupRepo := NewProjectGroupRepository(db)
	repoRepoAdapter := NewRepoRepository(db)
	ctx := context.Background()

	tenantID := uuid.NewString()
	devServerID := uuid.NewString()
	createdBy := uuid.NewString()

	candidates := []domain.NestedRepoCandidate{
		{Path: "/srv/repo-a", SuggestedName: "repo-a", IsGitRepo: true},
		{Path: "/srv/repo-b", SuggestedName: "", IsGitRepo: true},
	}

	groups, projects, err := groupRepo.ImportNested(ctx, tenantID, createdBy, devServerID, "", candidates)
	if err != nil {
		t.Fatalf("ImportNested: %v", err)
	}
	if len(groups) != 2 || len(projects) != 2 {
		t.Fatalf("expected 2 groups and 2 projects, got %d groups, %d projects", len(groups), len(projects))
	}
	if projects[1].Name != "/srv/repo-b" {
		t.Errorf("expected second project's name to fall back to path when SuggestedName is empty, got %q", projects[1].Name)
	}
	if projects[0].IssueStatusSyncEnabled != true {
		t.Errorf("expected IssueStatusSyncEnabled default true, got %+v", projects[0])
	}

	repos, err := repoRepoAdapter.ListRepos(ctx, projects[0].ID)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 1 || repos[0].URL != "/srv/repo-a" {
		t.Fatalf("expected the imported repo to reuse the `url` column for the on-disk path, got %+v", repos)
	}
}
