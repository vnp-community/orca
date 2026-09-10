package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SubmitReviewParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	PRNumber   int32
	ReviewType domain.ReviewType
	Summary    string
	Comments   []domain.ReviewComment
}

// SubmitReview implements BUG-PI-04/SOL-PI-04. BR-PI-10/BR-PI-11 are
// re-validated here (belt-and-braces) even though api-gateway's own
// composition step validates BR-PI-10 first too, so a future second caller
// of SubmitReview can't bypass the rule by skipping the gateway path.
type SubmitReview struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewSubmitReview(credentials CredentialResolver, providers ProviderRegistry) *SubmitReview {
	return &SubmitReview{credentials: credentials, providers: providers}
}

func (uc *SubmitReview) Execute(ctx context.Context, in SubmitReviewParams) (domain.Review, error) {
	if len(in.Comments) == 0 { // BR-PI-10
		return domain.Review{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_REVIEW_EMPTY_COMMENTS", "at least one comment is required to submit a review", nil)
	}
	reviewType := in.ReviewType
	if reviewType == domain.ReviewTypeUnspecified {
		reviewType = domain.ReviewTypeRequestChanges // BR-PI-11
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	review, err := provider.SubmitReview(ctx, cred, in.Repo, in.PRNumber, domain.ReviewInput{
		Type: reviewType, Summary: in.Summary, Comments: in.Comments,
	})
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInternal, "SCM_SUBMIT_REVIEW_FAILED", "failed to submit review", err)
	}
	return review, nil
}
