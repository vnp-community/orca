package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeFleetHealthPort is an in-memory FleetHealthPort.
type fakeFleetHealthPort struct {
	byTenant map[string][]domain.DevServerHealth
	err      error
}

func (f *fakeFleetHealthPort) GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byTenant[tenantID], nil
}

func TestGetFleetHealth_RequiresTenantContext(t *testing.T) {
	uc := NewGetFleetHealth(&fakeFleetHealthPort{})
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetFleetHealth_ReturnsTenantScopedStatuses(t *testing.T) {
	h, err := domain.NewDevServerHealth("ds1", true, 10, 20, 30, 5)
	if err != nil {
		t.Fatalf("building health sample: %v", err)
	}
	port := &fakeFleetHealthPort{byTenant: map[string][]domain.DevServerHealth{"tenant-1": {h}}}
	uc := NewGetFleetHealth(port)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].DevServerID != "ds1" {
		t.Errorf("expected [ds1], got %+v", got)
	}
}

func TestGetFleetHealth_PortFailurePropagates(t *testing.T) {
	port := &fakeFleetHealthPort{err: errors.New("db unavailable")}
	uc := NewGetFleetHealth(port)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("expected error to propagate from port failure")
	}
}
