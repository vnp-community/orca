package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// RateTemplate records (or updates) the caller's 1-5 star rating for a
// template — one rating per (user, template), enforced by
// workflow.ratings' (template_id, user_id) UNIQUE constraint
// (TASK-WF-03-01). rating_sum/rating_count on templates are a
// materialized aggregate, updated in the SAME transaction as the
// ratings-table write (TemplateRepositoryTx.UpsertRating) — never a
// separate, potentially-inconsistent follow-up write.
type RateTemplate struct {
	templates TemplateRepository
}

func NewRateTemplate(templates TemplateRepository) *RateTemplate {
	return &RateTemplate{templates: templates}
}

func (uc *RateTemplate) Execute(ctx context.Context, templateID string, stars int32) (RateTemplateResult, error) {
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return RateTemplateResult{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}
	if templateID == "" {
		return RateTemplateResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_ID_REQUIRED", "template_id is required", nil)
	}
	if stars < 1 || stars > 5 {
		return RateTemplateResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_RATING", "stars must be between 1 and 5", nil)
	}

	var result RateTemplateResult
	err := uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
		var terr error
		result, terr = tx.UpsertRating(ctx, templateID, userID, stars)
		return terr
	})
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return RateTemplateResult{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return RateTemplateResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_RATE_TEMPLATE_FAILED", "failed to rate template", err)
	}
	return result, nil
}
