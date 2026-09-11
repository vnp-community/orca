//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestMigration0017_UniqueConstraintRejectsSameTenantHostDuplicate confirms
// migration 0017's (tenant_id, host) unique constraint — the DB-level guard
// BulkProvisionFleet (TASK-BE-FLEET-001) relies on to treat a re-run of the
// same FleetSpec as idempotent rather than a hard failure.
func TestMigration0017_UniqueConstraintRejectsSameTenantHostDuplicate(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	first, err := domain.NewSshTarget(uuid.NewString(), tenantID, "10.0.0.5", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, first); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	// Same tenant, same host, different id — must violate the unique
	// constraint added by 0017.
	second, err := domain.NewSshTarget(uuid.NewString(), tenantID, "10.0.0.5", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, second); err == nil {
		t.Fatal("expected a unique_violation creating a duplicate (tenant_id, host) row")
	}
}

// TestMigration0017_AllowsSameHostDifferentTenant confirms the constraint is
// scoped by tenant_id — two different tenants may legitimately register the
// same host string (e.g. both point their own "10.0.0.5" fleet member).
func TestMigration0017_AllowsSameHostDifferentTenant(t *testing.T) {
	_, store := setupSshTargetStore(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()

	a, err := domain.NewSshTarget(uuid.NewString(), tenantA, "10.0.0.6", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, a); err != nil {
		t.Fatalf("Create tenantA: %v", err)
	}

	b, err := domain.NewSshTarget(uuid.NewString(), tenantB, "10.0.0.6", 22, "orca", "ssh-role-dev", "", "", "", nil)
	if err != nil {
		t.Fatalf("NewSshTarget: %v", err)
	}
	if _, err := store.Create(ctx, b); err != nil {
		t.Fatalf("expected no error creating same host under a different tenant, got: %v", err)
	}
}
