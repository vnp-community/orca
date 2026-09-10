package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// RecordHeartbeat mirrors §8's "no cross-service calls, p99 < 30ms"
// budget: no serializer, no transaction, single UPDATE.
type RecordHeartbeat struct {
	repo DispatchContextRepository
}

func NewRecordHeartbeat(repo DispatchContextRepository) *RecordHeartbeat {
	return &RecordHeartbeat{repo: repo}
}

func (uc *RecordHeartbeat) Execute(ctx context.Context, dispatchContextID string) (domain.DispatchContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if dispatchContextID == "" {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_DISPATCH_CONTEXT_ID", "dispatch_context_id is required", nil)
	}
	dc, err := uc.repo.RecordHeartbeat(ctx, tenantID, dispatchContextID)
	if err != nil {
		if errors.Is(err, ErrDispatchContextNotFound) {
			return domain.DispatchContext{}, apperrors.New(apperrors.KindNotFound, "ORCH_DISPATCH_CONTEXT_NOT_FOUND", "dispatch context not found", err)
		}
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInternal, "ORCH_RECORD_HEARTBEAT_FAILED", "failed to record heartbeat", err)
	}
	return dc, nil
}
