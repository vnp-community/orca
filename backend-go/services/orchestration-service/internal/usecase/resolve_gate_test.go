package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func mustGate(t *testing.T, id, taskID string) domain.DecisionGate {
	t.Helper()
	g, err := domain.NewDecisionGate(id, "tenant-1", taskID, "dc-1", "proceed?", []string{"yes", "no"})
	if err != nil {
		t.Fatalf("building gate: %v", err)
	}
	return g
}

func TestResolveGate_RequiresTenantContext(t *testing.T) {
	uc := NewResolveGate(newFakeGateRepository(mustGate(t, "g1", "t1")), &synchronousSerializer{})
	_, err := uc.Execute(context.Background(), ResolveGateInput{GateID: "g1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestResolveGate_ResolvesAndReturnsAffectedTask(t *testing.T) {
	repo := newFakeGateRepository(mustGate(t, "g1", "t1"))
	ser := &synchronousSerializer{}
	uc := NewResolveGate(repo, ser)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveGateInput{GateID: "g1", OutcomeJSON: `{"choice":"yes"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Gate.Status != domain.GateStatusResolved {
		t.Errorf("expected resolved gate, got %s", out.Gate.Status)
	}
	if len(out.AffectedTaskIDs) != 1 || out.AffectedTaskIDs[0] != "t1" {
		t.Errorf("expected affected task t1, got %v", out.AffectedTaskIDs)
	}
	if keys := ser.calledKeys(); len(keys) != 1 || keys[0] != "g1" {
		t.Errorf("expected serializer keyed by gate id g1, got %v", keys)
	}
}

// TestResolveGate_CannotResolveTwice is the usecase-level counterpart to
// the domain invariant test: calling Execute a second time for the same
// gate must fail, not silently succeed.
func TestResolveGate_CannotResolveTwice(t *testing.T) {
	repo := newFakeGateRepository(mustGate(t, "g1", "t1"))
	uc := NewResolveGate(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, ResolveGateInput{GateID: "g1", OutcomeJSON: "yes"}); err != nil {
		t.Fatalf("unexpected error on first resolution: %v", err)
	}

	_, err := uc.Execute(ctx, ResolveGateInput{GateID: "g1", OutcomeJSON: "no"})
	if err == nil {
		t.Fatal("expected an error resolving an already-resolved gate")
	}
}

func TestResolveGate_NotFoundPropagates(t *testing.T) {
	uc := NewResolveGate(newFakeGateRepository(), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResolveGateInput{GateID: "missing"})
	if !errors.Is(err, ErrGateNotFound) {
		t.Fatalf("expected wrapped ErrGateNotFound, got %v", err)
	}
}
