//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func newTestFolderWorkspace(id, tenantID, devServerID, path, name, addedBy string) domain.FolderWorkspace {
	return domain.FolderWorkspace{
		ID: id, TenantID: tenantID, DevServerID: devServerID, Path: path, Name: name, AddedBy: addedBy,
	}
}

// TestFolderWorkspaceRepository_CreateThenGet_ProjectGroupID covers the
// COALESCE(project_group_id::text, ”)/nullableString round trip migration
// 0012 added — both the "has a group" and "no group at all" cases.
func TestFolderWorkspaceRepository_CreateThenGet_ProjectGroupID(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)
	groupRepo := NewProjectGroupRepository(pool)
	ctx := context.Background()

	tenantID, devServerID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	group, err := groupRepo.CreateProjectGroup(ctx, domain.ProjectGroup{
		ID: uuid.NewString(), TenantID: tenantID, Name: "g1",
	})
	if err != nil {
		t.Fatalf("create project group: %v", err)
	}

	withGroup := newTestFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/home/with-group", "x", userID)
	withGroup.ProjectGroupID = group.ID
	created, err := repo.Create(ctx, withGroup)
	if err != nil {
		t.Fatalf("create with group: %v", err)
	}
	if created.ProjectGroupID != group.ID {
		t.Errorf("expected ProjectGroupID %q, got %q", group.ID, created.ProjectGroupID)
	}
	got, err := repo.Get(ctx, created.ID)
	if err != nil || got == nil || got.ProjectGroupID != group.ID {
		t.Fatalf("get with group: err=%v got=%+v", err, got)
	}

	noGroup := newTestFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/home/no-group", "y", userID)
	createdNoGroup, err := repo.Create(ctx, noGroup)
	if err != nil {
		t.Fatalf("create without group: %v", err)
	}
	if createdNoGroup.ProjectGroupID != "" {
		t.Errorf("expected empty ProjectGroupID, got %q", createdNoGroup.ProjectGroupID)
	}
}

// TestFolderWorkspaceRepository_Create_InvalidProjectGroupID confirms the
// real Postgres foreign-key constraint (not just the fake in usecase
// tests) actually rejects a nonexistent project_group_id.
func TestFolderWorkspaceRepository_Create_InvalidProjectGroupID(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)
	ctx := context.Background()

	fw := newTestFolderWorkspace(uuid.NewString(), uuid.NewString(), uuid.NewString(), "/home/bad-group", "x", uuid.NewString())
	fw.ProjectGroupID = uuid.NewString() // never created
	_, err := repo.Create(ctx, fw)
	if !errors.Is(err, domain.ErrProjectGroupNotFound) {
		t.Errorf("expected ErrProjectGroupNotFound, got %v", err)
	}
}

func TestFolderWorkspaceRepository_CreateThenGet_RoundTrip(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)
	ctx := context.Background()

	tenantID, devServerID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	fw := newTestFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/home/x", "x", userID)

	created, err := repo.Create(ctx, fw)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("expected id/created_at to be populated, got %+v", created)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Path != "/home/x" || got.TenantID != tenantID {
		t.Fatalf("unexpected get result: %+v", got)
	}
}

func TestFolderWorkspaceRepository_Create_DuplicatePath_ReturnsErrPathAlreadyRegistered(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)
	ctx := context.Background()

	tenantID, devServerID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	fw := newTestFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/home/dup", "dup", userID)

	if _, err := repo.Create(ctx, fw); err != nil {
		t.Fatalf("first create: %v", err)
	}

	fw2 := newTestFolderWorkspace(uuid.NewString(), tenantID, devServerID, "/home/dup", "dup2", userID)
	_, err := repo.Create(ctx, fw2)
	if err != domain.ErrPathAlreadyRegistered {
		t.Errorf("expected ErrPathAlreadyRegistered (asserting the pgconn.PgError unique-violation mapping fires against a real Postgres), got %v", err)
	}
}

func TestFolderWorkspaceRepository_Delete_NotFound(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)

	if err := repo.Delete(context.Background(), uuid.NewString()); err != domain.ErrFolderWorkspaceNotFound {
		t.Errorf("expected ErrFolderWorkspaceNotFound, got %v", err)
	}
}

func TestFolderWorkspaceRepository_Get_NotFound_ReturnsNilNil(t *testing.T) {
	pool := setupPool(t)
	repo := NewFolderWorkspaceRepository(pool)

	got, err := repo.Get(context.Background(), uuid.NewString())
	if err != nil || got != nil {
		t.Errorf("expected nil, nil for a missing row, got %+v %v", got, err)
	}
}
