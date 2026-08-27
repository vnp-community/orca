package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

type ListRunsInput struct {
	AutomationID string
	PageToken    string
	PageSize     int32
}

type ListRunsOutput struct {
	Runs          []domain.AutomationRun
	NextPageToken string
}

type ListRuns struct {
	runs AutomationRunRepository
}

func NewListRuns(runs AutomationRunRepository) *ListRuns {
	return &ListRuns{runs: runs}
}

func (uc *ListRuns) Execute(ctx context.Context, in ListRunsInput) (ListRunsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListRunsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTOMATION_NO_TENANT", "no tenant in request context", err)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	runs, next, err := uc.runs.ListByAutomation(ctx, tenantID, in.AutomationID, in.PageToken, pageSize)
	if err != nil {
		return ListRunsOutput{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_LIST_RUNS_FAILED", "failed to list automation runs", err)
	}
	return ListRunsOutput{Runs: runs, NextPageToken: next}, nil
}
