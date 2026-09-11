//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestFolderWorkspaceRepository_CreateGetUpdateDelete_RoundTrip(t *testing.T) {
	db := setupDB(t)
	fwRepo := NewFolderWorkspaceRepository(db)
	ctx := context.Background()

	tenantID, devServerID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	fw, err := domain.NewFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/srv/folder", "folder", userID, "")
	if err != nil {
		t.Fatalf("NewFolderWorkspace: %v", err)
	}
	created, err := fwRepo.Create(ctx, fw)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := fwRepo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Path != "/srv/folder" {
		t.Fatalf("unexpected folder workspace: %+v", got)
	}

	renamed, err := fwRepo.Update(ctx, created.ID, "renamed")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if renamed.Name != "renamed" {
		t.Errorf("expected renamed folder workspace, got %+v", renamed)
	}

	list, err := fwRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 folder workspace, got %d", len(list))
	}

	if err := fwRepo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := fwRepo.Delete(ctx, created.ID); err != domain.ErrFolderWorkspaceNotFound {
		t.Errorf("expected ErrFolderWorkspaceNotFound on second delete, got %v", err)
	}
}

// TestFolderWorkspaceRepository_Create_DuplicatePathReturnsSentinel proves
// the MySQL error-code 1062 (ER_DUP_ENTRY) mapping to
// domain.ErrPathAlreadyRegistered — the UNIQUE(tenant_id, dev_server_id,
// path) violation, MySQL's counterpart to Postgres's SQLSTATE 23505.
func TestFolderWorkspaceRepository_Create_DuplicatePathReturnsSentinel(t *testing.T) {
	db := setupDB(t)
	fwRepo := NewFolderWorkspaceRepository(db)
	ctx := context.Background()

	tenantID, devServerID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	fw1, err := domain.NewFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/srv/dup", "one", userID, "")
	if err != nil {
		t.Fatalf("NewFolderWorkspace: %v", err)
	}
	if _, err := fwRepo.Create(ctx, fw1); err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	fw2, err := domain.NewFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/srv/dup", "two", userID, "")
	if err != nil {
		t.Fatalf("NewFolderWorkspace: %v", err)
	}
	if _, err := fwRepo.Create(ctx, fw2); err != domain.ErrPathAlreadyRegistered {
		t.Errorf("expected ErrPathAlreadyRegistered for a duplicate (tenant_id, dev_server_id, path), got %v", err)
	}
}

// TestFolderWorkspaceRepository_Create_InvalidProjectGroupReturnsSentinel
// proves the MySQL error-code 1452 (ER_NO_REFERENCED_ROW_2) mapping to
// domain.ErrProjectGroupNotFound — MySQL's counterpart to Postgres's
// SQLSTATE 23503.
func TestFolderWorkspaceRepository_Create_InvalidProjectGroupReturnsSentinel(t *testing.T) {
	db := setupDB(t)
	fwRepo := NewFolderWorkspaceRepository(db)
	ctx := context.Background()

	fw, err := domain.NewFolderWorkspace(uuid.NewString(), uuid.NewString(), uuid.NewString(), "/srv/orphan", "orphan", uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatalf("NewFolderWorkspace: %v", err)
	}
	if _, err := fwRepo.Create(ctx, fw); err != domain.ErrProjectGroupNotFound {
		t.Errorf("expected ErrProjectGroupNotFound for a non-existent project_group_id, got %v", err)
	}
}

// TestFolderWorkspaceRepository_RepoPathExists_JoinsThroughProjects proves
// the worktrees-joined-through-projects cross-check still works with bare
// (non-schema-qualified) MySQL table names.
func TestFolderWorkspaceRepository_RepoPathExists_JoinsThroughProjects(t *testing.T) {
	db := setupDB(t)
	fwRepo := NewFolderWorkspaceRepository(db)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	ctx := context.Background()

	tenantID, devServerID := uuid.NewString(), uuid.NewString()
	p := newTestProject(uuid.NewString(), tenantID, "proj")
	p.DevServerID = devServerID
	created, err := projRepo.Create(ctx, p)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := projRepo.UpdateDevServerID(ctx, tenantID, created.ID, devServerID); err != nil {
		t.Fatalf("bind dev server: %v", err)
	}

	r, err := domain.NewRepo(uuid.NewString(), created.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	wt, err := domain.NewWorktree(uuid.NewString(), created.ID, repo.ID, "/srv/checkout", "main", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree: %v", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), TenantID: tenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)}
	if _, err := worktreeRepo.CreateWorktreeWithEvent(ctx, wt, event); err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	exists, err := fwRepo.RepoPathExists(ctx, tenantID, devServerID, "/srv/checkout")
	if err != nil {
		t.Fatalf("RepoPathExists: %v", err)
	}
	if !exists {
		t.Error("expected RepoPathExists true for a path matching an existing worktree")
	}

	notExists, err := fwRepo.RepoPathExists(ctx, tenantID, devServerID, "/srv/nowhere")
	if err != nil {
		t.Fatalf("RepoPathExists (miss): %v", err)
	}
	if notExists {
		t.Error("expected RepoPathExists false for an unrelated path")
	}
}
