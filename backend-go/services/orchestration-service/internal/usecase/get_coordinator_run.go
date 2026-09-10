package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// GetCoordinatorRun is a plain point-in-time read — mirrors
// GetDispatchContextForTask's shape (no serializer needed for a read).
type GetCoordinatorRun struct {
	repo CoordinatorRunRepository
}

func NewGetCoordinatorRun(repo CoordinatorRunRepository) *GetCoordinatorRun {
	return &GetCoordinatorRun{repo: repo}
}

func (uc *GetCoordinatorRun) Execute(ctx context.Context, id string) (domain.CoordinatorRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if id == "" {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_RUN_ID", "id is required", nil)
	}
	run, err := uc.repo.GetRun(ctx, tenantID, id)
	if err != nil {
		if err == ErrRunNotFound {
			return domain.CoordinatorRun{}, apperrors.New(apperrors.KindNotFound, "ORCH_RUN_NOT_FOUND", "coordinator run not found", err)
		}
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInternal, "ORCH_GET_COORDINATOR_RUN_FAILED", "failed to get coordinator run", err)
	}
	return run, nil
}
