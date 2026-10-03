package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	defaultAgentSessionOriginLimit = 100
	maxAgentSessionOriginLimit     = 500
)

// ListAgentSessionsByOrigin lets api-gateway find the agents an MCP session
// started even after the replica that spawned them died. It is deliberately not
// a general listing: an origin filter is mandatory.
type ListAgentSessionsByOrigin struct{ sessions AgentSessionOriginLister }

func NewListAgentSessionsByOrigin(sessions AgentSessionOriginLister) *ListAgentSessionsByOrigin {
	return &ListAgentSessionsByOrigin{sessions: sessions}
}

func (uc *ListAgentSessionsByOrigin) Execute(ctx context.Context, f AgentSessionOriginFilter) ([]domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if f.OriginType == "" && f.OriginSessionID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_ORIGIN_FILTER_REQUIRED", "origin_type or origin_session_id is required", nil)
	}
	if f.Limit <= 0 {
		f.Limit = defaultAgentSessionOriginLimit
	}
	if f.Limit > maxAgentSessionOriginLimit {
		f.Limit = maxAgentSessionOriginLimit
	}
	out, err := uc.sessions.ListByOrigin(ctx, tenantID, f)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SESSION_LIST_FAILED", "failed to list agent sessions", err)
	}
	return out, nil
}
