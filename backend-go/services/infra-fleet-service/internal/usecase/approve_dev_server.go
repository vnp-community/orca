package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ApproveDevServer marks a dev server approved — admin-gated, see
// requireAdmin's doc comment.
type ApproveDevServer struct {
	repo DevServerRepository
}

func NewApproveDevServer(repo DevServerRepository) *ApproveDevServer {
	return &ApproveDevServer{repo: repo}
}

func (uc *ApproveDevServer) Execute(ctx context.Context, devServerID string) (domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return domain.DevServer{}, err
	}

	ds, err := uc.repo.UpdateApprovalStatus(ctx, tenantID, devServerID, domain.DevServerStatusApproved)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_APPROVE_DEV_SERVER_FAILED", "failed to approve dev server", err)
	}
	return ds, nil
}
