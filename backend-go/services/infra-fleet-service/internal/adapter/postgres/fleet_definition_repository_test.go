//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func setupFleetDefinitionStore(t *testing.T) (*Repository, *FleetDefinitionStore) {
	t.Helper()
	repo := setupRepository(t)
	return repo, NewFleetDefinitionStore(repo.pool)
}

func TestFleetDefinitionRepository_Create_RoundTrips(t *testing.T) {
	_, store := setupFleetDefinitionStore(t)
	ctx := context.Background()

	def, err := domain.NewFleetDefinition(
		uuid.NewString(), uuid.NewString(), "my-fleet",
		[]domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role", Kind: domain.AgentKindDevServer}},
		&domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra", VarsFile: "prod.tfvars"},
		uuid.NewString(),
	)
	if err != nil {
		t.Fatalf("NewFleetDefinition: %v", err)
	}

	created, err := store.Create(ctx, def)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("expected CreatedAt/UpdatedAt to be populated")
	}

	got, err := store.Get(ctx, def.TenantID, def.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Servers) != 1 || got.Servers[0].Host != "h1" || got.Servers[0].Kind != domain.AgentKindDevServer {
		t.Errorf("servers did not round-trip: %+v", got.Servers)
	}
	if got.Provision == nil || got.Provision.WorkingDir != "/infra" || got.Provision.VarsFile != "prod.tfvars" {
		t.Errorf("provision did not round-trip: %+v", got.Provision)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
}

func TestFleetDefinitionRepository_UniqueNameConstraint(t *testing.T) {
	_, store := setupFleetDefinitionStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	first, err := domain.NewFleetDefinition(uuid.NewString(), tenantID, "my-fleet",
		[]domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}, nil, uuid.NewString())
	if err != nil {
		t.Fatalf("NewFleetDefinition: %v", err)
	}
	if _, err := store.Create(ctx, first); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	second, err := domain.NewFleetDefinition(uuid.NewString(), tenantID, "my-fleet",
		[]domain.FleetSpecServer{{Host: "h2", UserName: "orca", VaultSSHRole: "role"}}, nil, uuid.NewString())
	if err != nil {
		t.Fatalf("NewFleetDefinition: %v", err)
	}
	if _, err := store.Create(ctx, second); err == nil {
		t.Fatal("expected a unique_violation for a duplicate (tenant_id, name)")
	}
}

func TestFleetDefinitionRepository_Update_OptimisticLockConflict(t *testing.T) {
	_, store := setupFleetDefinitionStore(t)
	ctx := context.Background()

	def, err := domain.NewFleetDefinition(uuid.NewString(), uuid.NewString(), "my-fleet",
		[]domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}, nil, uuid.NewString())
	if err != nil {
		t.Fatalf("NewFleetDefinition: %v", err)
	}
	created, err := store.Create(ctx, def)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// First update: version 1 -> 2, should succeed.
	toUpdate := created
	toUpdate.Version = 2
	toUpdate.Servers = []domain.FleetSpecServer{{Host: "h2", UserName: "orca", VaultSSHRole: "role"}}
	if _, err := store.Update(ctx, toUpdate); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	// Second update using the STALE version (still claims oldVersion=1,
	// i.e. Version=2) — the row is already at version 2, so this must
	// conflict.
	stale := created
	stale.Version = 2 // oldVersion computed as 1, but the row is now at 2
	stale.Servers = []domain.FleetSpecServer{{Host: "h3", UserName: "orca", VaultSSHRole: "role"}}
	if _, err := store.Update(ctx, stale); err != domain.ErrFleetDefinitionVersionConflict {
		t.Errorf("expected ErrFleetDefinitionVersionConflict, got %v", err)
	}
}
