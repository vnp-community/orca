package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ListGrants is task-service's public grant-listing usecase — every grant
// recorded DIRECTLY against one task, for the frontend's Access tab
// (FE-SOL-001). Distinct from ResolvePermission's internal
// ListGrantsForAncestors (whole ancestor chain, map-keyed) — this is a
// single task's own grant rows only.
type ListGrants struct {
	grants GrantRepository
}

func NewListGrants(grants GrantRepository) *ListGrants {
	return &ListGrants{grants: grants}
}

func (uc *ListGrants) Execute(ctx context.Context, taskID string) ([]domain.Grant, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	grants, err := uc.grants.ListByTask(ctx, tenantID, taskID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_LIST_GRANTS_FAILED", "failed to list grants for task", err)
	}
	return grants, nil
}
