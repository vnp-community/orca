package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// GetAgentCapabilities answers "what are the capabilities, tools, and platform
// facts of devServerID's agent session" without dialing the remote host.
// In-memory read backed by DevServerAgentClient.LastHandshakeInfo and IsConnected.
type GetAgentCapabilities struct {
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewGetAgentCapabilities(devServers DevServerRepository, agent DevServerAgentClient) *GetAgentCapabilities {
	return &GetAgentCapabilities{devServers: devServers, agent: agent}
}

func (uc *GetAgentCapabilities) Execute(ctx context.Context, devServerID string) (domain.AgentCapabilities, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentCapabilities{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if devServerID == "" {
		return domain.AgentCapabilities{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_DEV_SERVER", "devServerId is required", nil)
	}

	devServer, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return domain.AgentCapabilities{}, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}

	connected := uc.agent.IsConnected(devServer.ID)
	if !connected {
		return domain.AgentCapabilities{Connected: false}, nil
	}

	info, ok := uc.agent.LastHandshakeInfo(devServer.ID)
	if !ok {
		// connected=true but LastHandshakeInfo missing (race) -> return connected=true with empty fields, no error
		return domain.AgentCapabilities{Connected: true}, nil
	}

	return domain.AgentCapabilities{
		Connected:    true,
		Platform:     info.Platform,
		Arch:         info.Arch,
		NodeVersion:  info.NodeVersion,
		AgentVersion: info.AgentVersion,
		Capabilities: append([]string(nil), info.Capabilities...),
		Tools:        append([]string(nil), info.Tools...),
		SessionID:    info.SessionID,
	}, nil
}
