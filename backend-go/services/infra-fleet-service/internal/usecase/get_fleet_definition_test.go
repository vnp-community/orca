package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetFleetDefinition_RequiresTenantContext(t *testing.T) {
	uc := NewGetFleetDefinition(&fakeFleetDefinitionRepository{})
	_, err := uc.Execute(context.Background(), "def-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetFleetDefinition_ScopedByTenant(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-A", "def-1"): {ID: "def-1", TenantID: "tenant-A", Name: "fleet-a"},
	}}
	uc := NewGetFleetDefinition(repo)

	ctxA := withTenant(context.Background(), "tenant-A")
	got, err := uc.Execute(ctxA, "def-1")
	if err != nil {
		t.Fatalf("unexpected error for tenant-A: %v", err)
	}
	if got.Name != "fleet-a" {
		t.Errorf("expected fleet-a, got %+v", got)
	}

	ctxB := withTenant(context.Background(), "tenant-B")
	_, err = uc.Execute(ctxB, "def-1")
	if err == nil {
		t.Fatal("expected tenant-B to NOT be able to read tenant-A's fleet definition")
	}
}

func TestGetFleetDefinition_NotFound(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{}
	uc := NewGetFleetDefinition(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "unknown")
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
