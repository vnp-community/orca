package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type GetConnectionStatusInput struct {
	Provider domain.Provider
}

type GetConnectionStatus struct{ connections ConnectionRepository }

func NewGetConnectionStatus(connections ConnectionRepository) *GetConnectionStatus {
	return &GetConnectionStatus{connections: connections}
}

func (uc *GetConnectionStatus) Execute(ctx context.Context, in GetConnectionStatusInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	// Not-connected is a legitimate, non-error state — GetStatus returns a
	// zero-value ConnectionStatus{Connected:false} rather than an error
	// when no rows exist.
	status, err := uc.connections.GetStatus(ctx, tenantID, userID, in.Provider)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_GET_STATUS_FAILED", "failed to read connection status", err)
	}
	return status, nil
}
