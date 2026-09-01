package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestRejectDevServer_RequiresAdmin(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{}}
	uc := NewRejectDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, RejectDevServerInput{DevServerID: "ds1"}); err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestRejectDevServer_SetsRejected(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds1": {ID: "ds1", TenantID: "tenant-1", Status: domain.DevServerStatusPendingApproval},
	}}
	uc := NewRejectDevServer(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RejectDevServerInput{DevServerID: "ds1", Reason: "unknown host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.DevServerStatusRejected {
		t.Errorf("want Status=rejected, got %q", got.Status)
	}
}
