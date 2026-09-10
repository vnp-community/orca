package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// ListPendingDecisionGates is read-only, mirrors
// ListActiveDispatchContextsForUser's shape exactly (tenant from identity,
// no other input).
type ListPendingDecisionGates struct {
	repo GateRepository
}

func NewListPendingDecisionGates(repo GateRepository) *ListPendingDecisionGates {
	return &ListPendingDecisionGates{repo: repo}
}

func (uc *ListPendingDecisionGates) Execute(ctx context.Context) ([]domain.DecisionGate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	gates, err := uc.repo.ListPending(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ORCH_LIST_PENDING_GATES_FAILED", "failed to list pending decision gates", err)
	}
	return gates, nil
}
