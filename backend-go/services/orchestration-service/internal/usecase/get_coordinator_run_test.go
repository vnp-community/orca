package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func mustRun(t *testing.T, id, tenantID string, status domain.RunStatus) domain.CoordinatorRun {
	t.Helper()
	run, err := domain.NewCoordinatorRun(id, tenantID, "origin-1", "coord-"+id, nil, 0)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	run.Status = status
	return run
}

func TestGetCoordinatorRun_HappyPath(t *testing.T) {
	repo := newFakeCoordinatorRunRepository(mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning))
	uc := NewGetCoordinatorRun(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	run, err := uc.Execute(ctx, "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "run-1" {
		t.Errorf("expected run-1, got %s", run.ID)
	}
}

func TestGetCoordinatorRun_NotFound(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewGetCoordinatorRun(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "missing")
	if err == nil {
		t.Fatal("expected an error for a missing run")
	}
}

func TestGetCoordinatorRun_RequiresTenantAndID(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewGetCoordinatorRun(repo)

	if _, err := uc.Execute(context.Background(), "run-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ""); err == nil {
		t.Fatal("expected an error for empty id")
	}
}
