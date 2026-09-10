package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestUpdateFleetDefinition_RequiresTenantContext(t *testing.T) {
	uc := NewUpdateFleetDefinition(&fakeFleetDefinitionRepository{})
	_, err := uc.Execute(context.Background(), UpdateFleetDefinitionInput{ID: "def-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateFleetDefinition_IncrementsVersion(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1", Version: 1,
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	uc := NewUpdateFleetDefinition(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	newServers := []domain.FleetSpecServer{{Host: "h2", UserName: "orca", VaultSSHRole: "role"}}
	got, err := uc.Execute(ctx, UpdateFleetDefinitionInput{ID: "def-1", Servers: newServers})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("expected version bumped to 2, got %d", got.Version)
	}
	if len(got.Servers) != 1 || got.Servers[0].Host != "h2" {
		t.Errorf("expected servers replaced, got %+v", got.Servers)
	}
	if len(repo.updateCalled) != 1 || repo.updateCalled[0].Version != 2 {
		t.Errorf("expected repo.Update called once with Version=2 (the NEW version), got %+v", repo.updateCalled)
	}
}

func TestUpdateFleetDefinition_VersionConflict_ReturnsConflictError(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{
		byID: map[string]domain.FleetDefinition{
			fleetDefKey("tenant-1", "def-1"): {
				ID: "def-1", TenantID: "tenant-1", Name: "fleet-1", Version: 1,
				Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			},
		},
		updateErr: domain.ErrFleetDefinitionVersionConflict,
	}
	uc := NewUpdateFleetDefinition(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateFleetDefinitionInput{
		ID:      "def-1",
		Servers: []domain.FleetSpecServer{{Host: "h2", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err == nil {
		t.Fatal("expected a version-conflict error")
	}
}

func TestUpdateFleetDefinition_UnknownDefinition_ReturnsNotFound(t *testing.T) {
	uc := NewUpdateFleetDefinition(&fakeFleetDefinitionRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateFleetDefinitionInput{
		ID:      "unknown",
		Servers: []domain.FleetSpecServer{{Host: "h2", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestUpdateFleetDefinition_EmptyServers_ReturnsInvalidArgument(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {ID: "def-1", TenantID: "tenant-1", Name: "fleet-1", Version: 1},
	}}
	uc := NewUpdateFleetDefinition(repo)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateFleetDefinitionInput{ID: "def-1", Servers: nil})
	if err == nil {
		t.Fatal("expected an error for empty servers")
	}
}
