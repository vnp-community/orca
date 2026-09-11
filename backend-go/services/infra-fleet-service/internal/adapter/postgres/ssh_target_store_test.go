//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestRepository_Delete_RemovesRow covers TASK-BE-FLEET-002's compensating
// rollback path — DeleteSshTarget removing an orphaned ssh_targets row.
func TestRepository_Delete_RemovesRow(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	target, err := domain.NewSshTarget(uuid.NewString(), tenantID, "10.0.0.1", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, target); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete(ctx, tenantID, target.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(ctx, tenantID, target.ID); err == nil {
		t.Fatal("expected Get to fail after Delete removed the row")
	}
}

// TestRepository_Delete_ScopedByTenant confirms the WHERE clause always
// includes tenant_id — deleting under a different tenant must not remove
// another tenant's row, even in the (practically unreachable, since id is a
// UUID) case of a shared id.
func TestRepository_Delete_ScopedByTenant(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()

	target, err := domain.NewSshTarget(uuid.NewString(), tenantA, "10.0.0.2", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, target); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Deleting under tenantB's scope must not remove tenantA's row.
	if err := store.Delete(ctx, tenantB, target.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(ctx, tenantA, target.ID); err != nil {
		t.Fatalf("expected tenantA's row to survive a delete scoped to tenantB, got: %v", err)
	}
}
