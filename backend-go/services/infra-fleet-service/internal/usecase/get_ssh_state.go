package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type SshStateInput struct {
	SshTargetID string
}

type SshState struct {
	Connected    bool
	Status       string // "" | "established" | "degraded" | "reconnecting" | "closed"
	ConnectionID string
	LastActivity *time.Time
}

// GetSshState is 🏠 always-local — reads whichever `connections` row (if
// any) currently binds this SSH target's dev server, never dials out.
// EstablishConnection (ssh.connect) is the only path that touches the
// network.
type GetSshState struct {
	sshTargets SshTargetRepository
	devServers DevServerRepository
	conns      ConnectionRepository
}

func NewGetSshState(sshTargets SshTargetRepository, devServers DevServerRepository, conns ConnectionRepository) *GetSshState {
	return &GetSshState{sshTargets: sshTargets, devServers: devServers, conns: conns}
}

func (uc *GetSshState) Execute(ctx context.Context, in SshStateInput) (SshState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return SshState{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	// No live dev server bound to this SSH target yet -> never connected.
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, in.SshTargetID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	conn, found, err := uc.conns.GetActiveByDevServer(ctx, tenantID, devServer.ID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	return SshState{
		Connected:    conn.Status != "reconnecting" && conn.Status != "closed",
		Status:       conn.Status,
		ConnectionID: conn.ID,
		LastActivity: conn.LastActivityAt,
	}, nil
}
