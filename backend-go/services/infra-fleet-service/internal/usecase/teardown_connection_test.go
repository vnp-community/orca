package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestTeardownConnection_MarksClosedAndCancelsReconnect(t *testing.T) {
	devServer, err := domain.NewDevServer("ds-1", "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-1", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	conns := &fakeConnectionRepository{devServerByConnectionFound: true, devServerByConnection: devServer}
	agent := &fakeDevServerAgentClient{}
	uc := NewTeardownConnection(conns, agent)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, TeardownConnectionInput{ConnectionID: "conn-1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(conns.updatedStatuses) != 1 || conns.updatedStatuses[0] != "conn-1:closed" {
		t.Errorf("expected connection conn-1 to be marked closed, got %+v", conns.updatedStatuses)
	}
	if len(agent.cancelReconnectCalls) != 1 || agent.cancelReconnectCalls[0] != "ds-1" {
		t.Errorf("expected CancelReconnect(ds-1) to be called, got %+v", agent.cancelReconnectCalls)
	}
}

func TestTeardownConnection_IdempotentWhenConnectionAlreadyGone(t *testing.T) {
	// devServerByConnectionFound left false: GetDevServerByConnection
	// returns found=false — the connection row doesn't exist (or is already
	// gone). Execute must still succeed and must skip CancelReconnect.
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{}
	uc := NewTeardownConnection(conns, agent)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, TeardownConnectionInput{ConnectionID: "conn-missing"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(agent.cancelReconnectCalls) != 0 {
		t.Errorf("expected CancelReconnect to be skipped when found=false, got %+v", agent.cancelReconnectCalls)
	}
	// UpdateStatus still runs (idempotent close on an already-closed/missing row).
	if len(conns.updatedStatuses) != 1 {
		t.Errorf("expected UpdateStatus to still run, got %+v", conns.updatedStatuses)
	}
}

func TestTeardownConnection_RequiresTenantContext(t *testing.T) {
	uc := NewTeardownConnection(&fakeConnectionRepository{}, &fakeDevServerAgentClient{})
	if err := uc.Execute(context.Background(), TeardownConnectionInput{ConnectionID: "conn-1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestTeardownConnection_LookupFailurePropagates(t *testing.T) {
	conns := &fakeConnectionRepository{devServerByConnectionErr: errors.New("db unreachable")}
	uc := NewTeardownConnection(conns, &fakeDevServerAgentClient{})

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, TeardownConnectionInput{ConnectionID: "conn-1"}); err == nil {
		t.Fatal("expected the lookup error to propagate")
	}
}

func TestTeardownConnection_UpdateStatusFailurePropagates(t *testing.T) {
	conns := &fakeConnectionRepository{updateStatusErr: errors.New("db write failed")}
	uc := NewTeardownConnection(conns, &fakeDevServerAgentClient{})

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, TeardownConnectionInput{ConnectionID: "conn-1"}); err == nil {
		t.Fatal("expected the UpdateStatus error to propagate")
	}
}
