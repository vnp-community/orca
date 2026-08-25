package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

type ListAutomationsInput struct {
	TenantID  string
	PageToken string
	PageSize  int32
}

type ListAutomationsResult struct {
	Automations   []domain.Automation
	NextPageToken string
}

// ListAutomations is automation-service's "all automations for a tenant"
// read path — distinct from ListRuns, which lists runs of one automation.
type ListAutomations struct {
	repo AutomationRepository
}

func NewListAutomations(repo AutomationRepository) *ListAutomations {
	return &ListAutomations{repo: repo}
}

func (uc *ListAutomations) Execute(ctx context.Context, in ListAutomationsInput) (ListAutomationsResult, error) {
	if in.TenantID == "" {
		return ListAutomationsResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_MISSING_TENANT_ID", "tenant_id is required", nil)
	}
	automations, nextToken, err := uc.repo.List(ctx, in.TenantID, in.PageToken, in.PageSize)
	if err != nil {
		return ListAutomationsResult{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_LIST_FAILED", "failed to list automations", err)
	}
	return ListAutomationsResult{Automations: automations, NextPageToken: nextToken}, nil
}
