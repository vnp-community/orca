package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListPendingAccessRequests admin-gated — backs the admin console's review
// queue.
type ListPendingAccessRequests struct {
	repo DevServerAccessRequestRepository
}

func NewListPendingAccessRequests(repo DevServerAccessRequestRepository) *ListPendingAccessRequests {
	return &ListPendingAccessRequests{repo: repo}
}

func (uc *ListPendingAccessRequests) Execute(ctx context.Context) ([]domain.DevServerAccessRequest, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	reqs, err := uc.repo.ListPending(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_ACCESS_REQUESTS_FAILED", "failed to list access requests", err)
	}
	return reqs, nil
}
