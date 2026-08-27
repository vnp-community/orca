package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ListGrants requires 'manage' on TaskID — spec: "the 'manage' permission
// includes viewing/managing existing grants". Returns only the target
// task's own grant rows, not the whole ancestor chain.
type ListGrants struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
}

func NewListGrants(grants GrantRepository, resolvePermission *ResolvePermission) *ListGrants {
	return &ListGrants{grants: grants, resolvePermission: resolvePermission}
}

func (uc *ListGrants) Execute(ctx context.Context, taskID string) ([]domain.Grant, error) {
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return nil, err
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	// Only the target task's own grants — not the ancestor chain, which
	// would leak ancestor-task grant details to a caller who may not have
	// visibility into the ancestor task itself.
	return uc.grants.ListGrantsForTask(ctx, tenantID, taskID)
}
