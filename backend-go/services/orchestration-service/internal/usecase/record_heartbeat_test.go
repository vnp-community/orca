package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestRecordHeartbeat_HappyPath(t *testing.T) {
	repo := &fakeDispatchContextRepository{
		byID: map[string]domain.DispatchContext{
			"dc-1": {ID: "dc-1", TenantID: "tenant-1"},
		},
	}
	uc := NewRecordHeartbeat(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	dc, err := uc.Execute(ctx, "dc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dc.LastHeartbeatAt.IsZero() {
		t.Error("expected LastHeartbeatAt to be updated")
	}
}

func TestRecordHeartbeat_NotFound(t *testing.T) {
	repo := &fakeDispatchContextRepository{}
	uc := NewRecordHeartbeat(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "missing")
	if err == nil {
		t.Fatal("expected an error for a missing dispatch context")
	}
}

func TestRecordHeartbeat_RequiresTenantAndID(t *testing.T) {
	repo := &fakeDispatchContextRepository{}
	uc := NewRecordHeartbeat(repo)

	if _, err := uc.Execute(context.Background(), "dc-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ""); err == nil {
		t.Fatal("expected an error for empty dispatch_context_id")
	}
}
