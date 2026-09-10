// fail_coordinator_run.go — identical shape to complete_coordinator_run.go,
// repo.Fail(ctx, tenantID, in.ID, in.ErrorMessage) instead of Complete.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

type FailCoordinatorRunInput struct {
	ID           string
	ErrorMessage string
}

type FailCoordinatorRun struct {
	repo       CoordinatorRunRepository
	serializer HandleSerializer
}

func NewFailCoordinatorRun(repo CoordinatorRunRepository, serializer HandleSerializer) *FailCoordinatorRun {
	return &FailCoordinatorRun{repo: repo, serializer: serializer}
}

func (uc *FailCoordinatorRun) Execute(ctx context.Context, in FailCoordinatorRunInput) (domain.CoordinatorRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_RUN_ID", "id is required", nil)
	}
	var out domain.CoordinatorRun
	err = uc.serializer.Do(ctx, in.ID, func() error {
		failed, err := uc.repo.Fail(ctx, tenantID, in.ID, in.ErrorMessage)
		if err != nil {
			return err
		}
		out = failed
		return nil
	})
	if err != nil {
		if err == ErrRunNotFound {
			return domain.CoordinatorRun{}, apperrors.New(apperrors.KindNotFound, "ORCH_RUN_NOT_FOUND", "coordinator run not found", err)
		}
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInternal, "ORCH_FAIL_COORDINATOR_RUN_FAILED", "failed to fail coordinator run", err)
	}
	return out, nil
}
