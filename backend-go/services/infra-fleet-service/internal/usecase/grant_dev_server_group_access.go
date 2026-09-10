package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type GrantDevServerGroupAccessInput struct {
	DevServerGroupID string
	GranteeKind      domain.GranteeKind
	GranteeID        string
}

// GrantDevServerGroupAccess admin-gated — see
// docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md.
type GrantDevServerGroupAccess struct {
	repo DevServerGroupGrantRepository
}

func NewGrantDevServerGroupAccess(repo DevServerGroupGrantRepository) *GrantDevServerGroupAccess {
	return &GrantDevServerGroupAccess{repo: repo}
}

func (uc *GrantDevServerGroupAccess) Execute(ctx context.Context, in GrantDevServerGroupAccessInput) (domain.DevServerGroupGrant, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServerGroupGrant{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return domain.DevServerGroupGrant{}, err
	}

	grant, err := domain.NewDevServerGroupGrant(uuid.NewString(), tenantID, in.DevServerGroupID, in.GranteeKind, in.GranteeID)
	if err != nil {
		return domain.DevServerGroupGrant{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_GRANT", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, grant)
	if err != nil {
		return domain.DevServerGroupGrant{}, apperrors.New(apperrors.KindInternal, "INFRA_GRANT_FAILED", "failed to create grant", err)
	}
	return saved, nil
}
