package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TeardownConnectionInput struct {
	ConnectionID string
}

// TeardownConnection is BR-SSH-13's "Cancel" action: marks the connection
// row closed and stops any in-flight relaySSHReconnect backoff loop for its
// dev server — idempotent on an already-closed connection.
type TeardownConnection struct {
	conns ConnectionRepository
	agent DevServerAgentClient
}

func NewTeardownConnection(conns ConnectionRepository, agent DevServerAgentClient) *TeardownConnection {
	return &TeardownConnection{conns: conns, agent: agent}
}

func (uc *TeardownConnection) Execute(ctx context.Context, in TeardownConnectionInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	devServer, found, err := uc.conns.GetDevServerByConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TEARDOWN_LOOKUP_FAILED", "failed to resolve connection", err)
	}
	if err := uc.conns.UpdateStatus(ctx, tenantID, in.ConnectionID, "closed"); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TEARDOWN_FAILED", "failed to mark connection closed", err)
	}
	if found {
		uc.agent.CancelReconnect(devServer.ID) // no-op if no reconnect loop is running — see Client.CancelReconnect
	}
	return nil
}
