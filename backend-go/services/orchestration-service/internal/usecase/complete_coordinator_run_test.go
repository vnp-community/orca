package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestCompleteCoordinatorRun_HappyPath(t *testing.T) {
	repo := newFakeCoordinatorRunRepository(mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning))
	uc := NewCompleteCoordinatorRun(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	run, err := uc.Execute(ctx, CompleteCoordinatorRunInput{ID: "run-1", ResultJSON: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != domain.RunStatusCompleted {
		t.Errorf("expected RunStatusCompleted, got %s", run.Status)
	}
}

func TestCompleteCoordinatorRun_NotFound(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewCompleteCoordinatorRun(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CompleteCoordinatorRunInput{ID: "missing"})
	if err == nil {
		t.Fatal("expected an error for a missing run")
	}
}

func TestCompleteCoordinatorRun_RequiresTenantAndID(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewCompleteCoordinatorRun(repo, &synchronousSerializer{})

	if _, err := uc.Execute(context.Background(), CompleteCoordinatorRunInput{ID: "run-1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, CompleteCoordinatorRunInput{ID: ""}); err == nil {
		t.Fatal("expected an error for empty id")
	}
}
