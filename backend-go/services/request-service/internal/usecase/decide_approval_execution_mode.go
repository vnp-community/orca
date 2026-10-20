package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// pinRequestedMode runs inside the decision's transaction, so the pin and the decision commit or roll back together.
// A Request already pinned to another mode refuses the whole decision: the approver confirmed a run in the requested mode.
func pinRequestedMode(ctx context.Context, pinner ExecutionModePinner, req domain.Request, mode domain.ExecutionMode) (domain.ExecutionMode, error) {
	if mode == "" {
		return req.ExecutionMode, nil
	}
	if pinner == nil {
		return "", apperrors.New(apperrors.KindInternal, "REQUEST_EXECUTION_MODE_PINNER_MISSING", "the execution mode cannot be recorded by this deployment", nil)
	}
	got, err := pinner.PinExecutionModeIfUnset(ctx, req.ID, mode)
	if err != nil {
		return "", err
	}
	if got != mode {
		return "", domain.ErrExecutionModeAlreadyPinned(got)
	}
	return got, nil
}
