package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReplaceRequestCoverage swaps the whole coverage block of one plan. It opens no transaction of its own:
// CommitPlan calls it inside its CAS transaction so a later failure takes the replacement back.
type ReplaceRequestCoverage struct {
	coverage RequestCoverageRepository
}

func NewReplaceRequestCoverage(coverage RequestCoverageRepository) *ReplaceRequestCoverage {
	return &ReplaceRequestCoverage{coverage: coverage}
}

func (uc *ReplaceRequestCoverage) Execute(ctx context.Context, requestID, planTaskID string, plan domain.PlanSpecs) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	return uc.coverage.ReplaceForPlan(ctx, requestID, planTaskID, domain.BuildCoverageRows(planTaskID, plan))
}
