package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestFailCoordinatorRun_HappyPath(t *testing.T) {
	repo := newFakeCoordinatorRunRepository(mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning))
	uc := NewFailCoordinatorRun(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	run, err := uc.Execute(ctx, FailCoordinatorRunInput{ID: "run-1", ErrorMessage: "boom"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != domain.RunStatusFailed {
		t.Errorf("expected RunStatusFailed, got %s", run.Status)
	}
	if run.ErrorMessage != "boom" {
		t.Errorf("expected error message to round-trip, got %q", run.ErrorMessage)
	}
}

func TestFailCoordinatorRun_NotFound(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewFailCoordinatorRun(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, FailCoordinatorRunInput{ID: "missing"})
	if err == nil {
		t.Fatal("expected an error for a missing run")
	}
}

func TestFailCoordinatorRun_RequiresTenantAndID(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewFailCoordinatorRun(repo, &synchronousSerializer{})

	if _, err := uc.Execute(context.Background(), FailCoordinatorRunInput{ID: "run-1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, FailCoordinatorRunInput{ID: ""}); err == nil {
		t.Fatal("expected an error for empty id")
	}
}
