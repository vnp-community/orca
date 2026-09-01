package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// IsDevServerConnected answers "does devServerID have a live agent session
// right now" — the real question the Dev Server list's Status field always
// answered "disconnected" to (a hardcoded placeholder, see wscompat's
// toDevServerView doc comment) regardless of the agent's actual state.
// Cheap and side-effect-free: DevServerAgentClient.IsConnected never dials,
// see its own doc comment.
type IsDevServerConnected struct {
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewIsDevServerConnected(devServers DevServerRepository, agent DevServerAgentClient) *IsDevServerConnected {
	return &IsDevServerConnected{devServers: devServers, agent: agent}
}

func (uc *IsDevServerConnected) Execute(ctx context.Context, devServerID string) (bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if devServerID == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_DEV_SERVER", "devServerId is required", nil)
	}

	devServer, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return false, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}

	return uc.agent.IsConnected(devServer.ID), nil
}
