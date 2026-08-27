package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListDevServers_RequiresTenantContext(t *testing.T) {
	uc := NewListDevServers(&fakeDevServerRepository{})
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListDevServers_ReturnsTenantScopedServers(t *testing.T) {
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds1": {ID: "ds1", TenantID: "tenant-1", Host: "10.0.0.1", Mode: domain.ConnectionModeRelayWebSocket},
		"ds2": {ID: "ds2", TenantID: "tenant-2", Host: "10.0.0.2", Mode: domain.ConnectionModeRelaySSH},
	}}
	uc := NewListDevServers(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "ds1" {
		t.Errorf("expected only tenant-1's dev server, got %+v", out)
	}
}

func TestListDevServers_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerRepository{listErr: errors.New("db unavailable")}
	uc := NewListDevServers(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
