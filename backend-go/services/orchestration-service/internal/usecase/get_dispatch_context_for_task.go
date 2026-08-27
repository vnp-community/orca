package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// GetDispatchContextForTask is orchestration-service's first read RPC —
// backs orchestration.dispatchShow: "which terminal was this task
// dispatched to."
type GetDispatchContextForTask struct {
	repo DispatchContextRepository
}

func NewGetDispatchContextForTask(repo DispatchContextRepository) *GetDispatchContextForTask {
	return &GetDispatchContextForTask{repo: repo}
}

// Execute returns (context, found, err). found=false with err=nil means
// "no dispatch context exists yet for this task" — a legitimate read
// result the gRPC adapter maps to an unset response field, not a gRPC
// error, since ErrDispatchContextNotFound already exists as a sentinel
// (ports.go) for exactly this case.
func (uc *GetDispatchContextForTask) Execute(ctx context.Context, taskID string) (domain.DispatchContext, bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if taskID == "" {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_TASK_ID", "orchestration_task_id is required", nil)
	}
	dc, err := uc.repo.GetLatestForTask(ctx, tenantID, taskID)
	if errors.Is(err, ErrDispatchContextNotFound) {
		return domain.DispatchContext{}, false, nil
	}
	if err != nil {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInternal, "ORCH_GET_DISPATCH_FAILED", "failed to look up dispatch context", err)
	}
	return dc, true, nil
}
