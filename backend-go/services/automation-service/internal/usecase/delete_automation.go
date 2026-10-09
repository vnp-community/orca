package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

type DeleteAutomationInput struct {
	TenantID string
	ID       string
}

// DeleteAutomation removes an automation definition. The repository's SQL
// relies on automation_runs' ON DELETE CASCADE FK, so no separate
// run-cleanup step is needed here.
type DeleteAutomation struct {
	repo AutomationRepository
}

func NewDeleteAutomation(repo AutomationRepository) *DeleteAutomation {
	return &DeleteAutomation{repo: repo}
}

func (uc *DeleteAutomation) Execute(ctx context.Context, in DeleteAutomationInput) error {
	if in.ID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_MISSING_ID", "id is required", nil)
	}
	if err := uc.repo.Delete(ctx, in.TenantID, in.ID); err != nil {
		if errors.Is(err, domain.ErrAutomationNotFound) {
			return apperrors.New(apperrors.KindNotFound, "AUTOMATION_NOT_FOUND", "automation not found", err)
		}
		return apperrors.New(apperrors.KindInternal, "AUTOMATION_DELETE_FAILED", "failed to delete automation", err)
	}
	return nil
}
