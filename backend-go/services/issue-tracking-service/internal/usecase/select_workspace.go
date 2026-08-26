package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type SelectWorkspaceInput struct {
	Provider    domain.Provider
	WorkspaceID string // "" | "all" | a specific workspace id
}

type SelectWorkspace struct{ connections ConnectionRepository }

func NewSelectWorkspace(connections ConnectionRepository) *SelectWorkspace {
	return &SelectWorkspace{connections: connections}
}

func (uc *SelectWorkspace) Execute(ctx context.Context, in SelectWorkspaceInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	status, err := uc.connections.SelectWorkspace(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_SELECT_WORKSPACE_FAILED", "failed to select workspace", err)
	}
	return status, nil
}
