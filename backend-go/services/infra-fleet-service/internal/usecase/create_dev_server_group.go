package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateDevServerGroupInput mirrors RegisterDevServerInput's convention —
// TenantID is pulled from context, never trusted from the request body.
type CreateDevServerGroupInput struct {
	Name          string
	ParentGroupID string
}

// CreateDevServerGroup adds a new dev-server organizational group — see
// docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md §3.2.
// Admin-gated as of Phase 2 (BE-SOL-002) — Phase 1 shipped this usecase
// before actor-role propagation existed at all (see requireAdmin's doc
// comment), so it was necessarily ungated at first.
type CreateDevServerGroup struct {
	repo DevServerGroupRepository
}

func NewCreateDevServerGroup(repo DevServerGroupRepository) *CreateDevServerGroup {
	return &CreateDevServerGroup{repo: repo}
}

func (uc *CreateDevServerGroup) Execute(ctx context.Context, in CreateDevServerGroupInput) (domain.DevServerGroup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServerGroup{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return domain.DevServerGroup{}, err
	}

	group, err := domain.NewDevServerGroup(uuid.NewString(), tenantID, in.Name, in.ParentGroupID)
	if err != nil {
		return domain.DevServerGroup{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_DEV_SERVER_GROUP", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, group)
	if err != nil {
		return domain.DevServerGroup{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_DEV_SERVER_GROUP_FAILED", "failed to create dev server group", err)
	}
	return saved, nil
}
