//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func newTestFolderWorkspace(id, tenantID, devServerID, path, name, addedBy string) domain.FolderWorkspace {
	return domain.FolderWorkspace{
		ID: id, TenantID: tenantID, DevServerID: devServerID, Path: path, Name: name, AddedBy: addedBy,
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
