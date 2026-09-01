package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RevokeDevServerGroupAccess admin-gated.
type RevokeDevServerGroupAccess struct {
	repo DevServerGroupGrantRepository
}

func NewRevokeDevServerGroupAccess(repo DevServerGroupGrantRepository) *RevokeDevServerGroupAccess {
	return &RevokeDevServerGroupAccess{repo: repo}
}

func (uc *RevokeDevServerGroupAccess) Execute(ctx context.Context, grantID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return err
	}

	if err := uc.repo.Delete(ctx, tenantID, grantID); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_REVOKE_GRANT_FAILED", "failed to revoke grant", err)
	}
	return nil
}
