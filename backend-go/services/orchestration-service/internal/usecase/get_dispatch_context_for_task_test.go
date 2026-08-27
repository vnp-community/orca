package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestGetDispatchContextForTask_Found(t *testing.T) {
	want := domain.DispatchContext{ID: "dc-1", Handle: "terminal-3", OrchestrationTaskID: "task-1", CreatedAt: time.Now()}
	repo := &fakeDispatchContextRepository{getLatestReturns: want}
	uc := NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	dc, found, err := uc.Execute(ctx, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("want found=true")
	}
	if dc.Handle != "terminal-3" {
		t.Errorf("want Handle=terminal-3, got %q", dc.Handle)
	}
}

func TestGetDispatchContextForTask_NotFound_ReturnsFalseNotError(t *testing.T) {
	repo := &fakeDispatchContextRepository{getLatestErr: ErrDispatchContextNotFound}
	uc := NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	dc, found, err := uc.Execute(ctx, "task-missing")
	if err != nil {
		t.Fatalf("want nil error for not-found, got %v", err)
	}
	if found {
		t.Fatal("want found=false")
	}
	if dc != (domain.DispatchContext{}) {
		t.Errorf("want zero-value DispatchContext, got %+v", dc)
	}
}

func TestGetDispatchContextForTask_EmptyTaskID_FailsBeforeRepoCall(t *testing.T) {
	repo := &fakeDispatchContextRepository{
		getLatestFunc: func(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error) {
			t.Fatal("repo must not be called for an empty task id")
			return domain.DispatchContext{}, nil
		},
	}
	uc := NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	_, _, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty task id")
	}
}

func TestGetDispatchContextForTask_RequiresTenantContext(t *testing.T) {
	repo := &fakeDispatchContextRepository{}
	uc := NewGetDispatchContextForTask(repo)

	_, _, err := uc.Execute(context.Background(), "task-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
