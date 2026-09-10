package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestListPendingDecisionGates_ReturnsOnlyPendingForCallerTenant(t *testing.T) {
	repo := newFakeGateRepository(
		domain.DecisionGate{ID: "g1", TenantID: "tenant-1", Status: domain.GateStatusPending},
		domain.DecisionGate{ID: "g2", TenantID: "tenant-1", Status: domain.GateStatusResolved},
		domain.DecisionGate{ID: "g3", TenantID: "tenant-2", Status: domain.GateStatusPending},
	)
	uc := NewListPendingDecisionGates(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	gates, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gates) != 1 || gates[0].ID != "g1" {
		t.Fatalf("expected exactly g1, got %+v", gates)
	}
}

func TestListPendingDecisionGates_RequiresTenantContext(t *testing.T) {
	uc := NewListPendingDecisionGates(newFakeGateRepository())
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
