package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestListDevServersByTag_RequiresTenantContext(t *testing.T) {
	uc := NewListDevServersByTag(&fakeDevServerRepository{}, &fakeFleetHealthPort{})
	_, err := uc.Execute(context.Background(), ListDevServersByTagInput{Tag: "gpu"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListDevServersByTag_RequiresTag(t *testing.T) {
	uc := NewListDevServersByTag(&fakeDevServerRepository{}, &fakeFleetHealthPort{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ListDevServersByTagInput{})
	if err == nil {
		t.Fatal("expected an error for an empty tag")
	}
}

func TestListDevServersByTag_FiltersByExactTag(t *testing.T) {
	gpuServer, err := domain.NewDevServer("ds-gpu", "tenant-1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", []string{"gpu", "region:us-east"})
	if err != nil {
		t.Fatalf("building gpu dev server: %v", err)
	}
	plainServer, err := domain.NewDevServer("ds-plain", "tenant-1", "10.0.0.2", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building plain dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		gpuServer.ID:   gpuServer,
		plainServer.ID: plainServer,
	}}
	uc := NewListDevServersByTag(repo, &fakeFleetHealthPort{})

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersByTagInput{Tag: "gpu"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds-gpu" {
		t.Errorf("expected exactly [ds-gpu], got %+v", got)
	}
}

func TestListDevServersByTag_HealthyOnlyExcludesUnreachable(t *testing.T) {
	healthy, err := domain.NewDevServer("ds-healthy", "tenant-1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", []string{"gpu"})
	if err != nil {
		t.Fatalf("building healthy dev server: %v", err)
	}
	unhealthy, err := domain.NewDevServer("ds-unhealthy", "tenant-1", "10.0.0.2", domain.ConnectionModeRelayWebSocket, "", []string{"gpu"})
	if err != nil {
		t.Fatalf("building unhealthy dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		healthy.ID:   healthy,
		unhealthy.ID: unhealthy,
	}}
	healthySample, err := domain.NewDevServerHealth("ds-healthy", true, 10, 20, 30, 5)
	if err != nil {
		t.Fatalf("building healthy sample: %v", err)
	}
	unhealthySample, err := domain.NewDevServerHealth("ds-unhealthy", false, 10, 20, 30, 5)
	if err != nil {
		t.Fatalf("building unhealthy sample: %v", err)
	}
	health := &fakeFleetHealthPort{byTenant: map[string][]domain.DevServerHealth{
		"tenant-1": {healthySample, unhealthySample},
	}}
	uc := NewListDevServersByTag(repo, health)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersByTagInput{Tag: "gpu", HealthyOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds-healthy" {
		t.Errorf("expected exactly [ds-healthy] with healthy_only=true, got %+v", got)
	}
}

func TestListDevServersByTag_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerRepository{byTagErr: errors.New("db unavailable")}
	uc := NewListDevServersByTag(repo, &fakeFleetHealthPort{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ListDevServersByTagInput{Tag: "gpu"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
