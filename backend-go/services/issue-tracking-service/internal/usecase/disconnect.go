package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type DisconnectInput struct {
	Provider    domain.Provider
	WorkspaceID string // "" = disconnect every workspace for this provider
}

type Disconnect struct{ connections ConnectionRepository }

func NewDisconnect(connections ConnectionRepository) *Disconnect {
	return &Disconnect{connections: connections}
}

func (uc *Disconnect) Execute(ctx context.Context, in DisconnectInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if err := uc.connections.Delete(ctx, tenantID, userID, in.Provider, in.WorkspaceID); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_DISCONNECT_FAILED", "failed to disconnect", err)
	}
	return nil
}
