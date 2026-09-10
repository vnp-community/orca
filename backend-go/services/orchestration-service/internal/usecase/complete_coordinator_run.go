package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

type CompleteCoordinatorRunInput struct {
	ID         string
	ResultJSON json.RawMessage
}

// CompleteCoordinatorRun is exposed as a caller-invokable usecase (not
// only an internal transition) so an operator/admin path can force-finalize
// a run — orchestration-service.md §3 lists it as part of the RPC surface.
// It is ALSO called internally by UpdateTaskStatusAndPromote's
// run-completion tail (TASK-TASKV1-005-07) via the same repo method, not
// by re-invoking this usecase (that path already holds tenantID and needs
// no re-authentication).
type CompleteCoordinatorRun struct {
	repo       CoordinatorRunRepository
	serializer HandleSerializer
}

func NewCompleteCoordinatorRun(repo CoordinatorRunRepository, serializer HandleSerializer) *CompleteCoordinatorRun {
	return &CompleteCoordinatorRun{repo: repo, serializer: serializer}
}

func (uc *CompleteCoordinatorRun) Execute(ctx context.Context, in CompleteCoordinatorRunInput) (domain.CoordinatorRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_RUN_ID", "id is required", nil)
	}
	var out domain.CoordinatorRun
	err = uc.serializer.Do(ctx, in.ID, func() error {
		completed, err := uc.repo.Complete(ctx, tenantID, in.ID, in.ResultJSON)
		if err != nil {
			return err
		}
		out = completed
		return nil
	})
	if err != nil {
		if err == ErrRunNotFound {
			return domain.CoordinatorRun{}, apperrors.New(apperrors.KindNotFound, "ORCH_RUN_NOT_FOUND", "coordinator run not found", err)
		}
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInternal, "ORCH_COMPLETE_COORDINATOR_RUN_FAILED", "failed to complete coordinator run", err)
	}
	return out, nil
}
