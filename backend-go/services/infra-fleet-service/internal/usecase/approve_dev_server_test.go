package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestApproveDevServer_RequiresAdmin(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{}}
	uc := NewApproveDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1") // no admin role
	if _, err := uc.Execute(ctx, "ds1"); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestApproveDevServer_SetsApproved(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds1": {ID: "ds1", TenantID: "tenant-1", Status: domain.DevServerStatusPendingApproval},
	}}
	uc := NewApproveDevServer(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, "ds1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.DevServerStatusApproved {
		t.Errorf("want Status=approved, got %q", got.Status)
	}
}
