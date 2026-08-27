package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListTerminalSessionsInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type ListTerminalSessionsInput struct {
	ConnectionID string // empty = every open session for the caller's tenant
}

// ListTerminalSessions is a pure read over TerminalSessionRepository — no
// agent call, unlike GetTerminalAgentStatus/InspectTerminalProcess, since
// the persisted row is authoritative for "what sessions exist", not the
// agent's live process state.
type ListTerminalSessions struct {
	sessions TerminalSessionRepository
}

func NewListTerminalSessions(sessions TerminalSessionRepository) *ListTerminalSessions {
	return &ListTerminalSessions{sessions: sessions}
}

func (uc *ListTerminalSessions) Execute(ctx context.Context, in ListTerminalSessionsInput) ([]domain.TerminalSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	sessions, err := uc.sessions.List(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_TERMINAL_SESSIONS_FAILED", "failed to list terminal sessions", err)
	}
	return sessions, nil
}
