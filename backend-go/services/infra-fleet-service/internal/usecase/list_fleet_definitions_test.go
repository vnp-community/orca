package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListFleetDefinitions_RequiresTenantContext(t *testing.T) {
	uc := NewListFleetDefinitions(&fakeFleetDefinitionRepository{})
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListFleetDefinitions_ReturnsOnlyCallerTenant(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byTenant: map[string][]domain.FleetDefinition{
		"tenant-A": {{ID: "def-1", TenantID: "tenant-A", Name: "fleet-a"}},
		"tenant-B": {{ID: "def-2", TenantID: "tenant-B", Name: "fleet-b"}},
	}}
	uc := NewListFleetDefinitions(repo)

	ctxA := withTenant(context.Background(), "tenant-A")
	got, err := uc.Execute(ctxA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "fleet-a" {
		t.Errorf("expected only tenant-A's fleet, got %+v", got)
	}
}
