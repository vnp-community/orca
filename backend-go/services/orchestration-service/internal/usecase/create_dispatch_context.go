package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// CreateDispatchContextInput mirrors the CreateDispatchContextRequest RPC
// message 1:1 — see architecture/03's note that usecase granularity mirrors
// today's RPC methods so the TS->Go mapping stays traceable.
type CreateDispatchContextInput struct {
	Handle           string
	CoordinatorRunID string
}

// CreateDispatchContext is routed through the HandleSerializer keyed by
// Handle, per orchestration-service.md §8: dispatch-context creation is the
// kind of write a synchronous domain-event handler can fire as an
// uncoordinated async DB call, and two such calls for the same handle must
// not interleave.
type CreateDispatchContext struct {
	repo       DispatchContextRepository
	serializer HandleSerializer
}

func NewCreateDispatchContext(repo DispatchContextRepository, serializer HandleSerializer) *CreateDispatchContext {
	return &CreateDispatchContext{repo: repo, serializer: serializer}
}

func (uc *CreateDispatchContext) Execute(ctx context.Context, in CreateDispatchContextInput) (domain.DispatchContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if in.Handle == "" {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_HANDLE", "handle is required", nil)
	}

	var result domain.DispatchContext
	err = uc.serializer.Do(ctx, in.Handle, func() error {
		created, err := uc.repo.CreateDispatchContext(ctx, tenantID, in.Handle, in.CoordinatorRunID)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return domain.DispatchContext{}, apperrors.New(apperrors.KindInternal, "ORCH_CREATE_DISPATCH_FAILED", "failed to create dispatch context", err)
	}
	return result, nil
}
