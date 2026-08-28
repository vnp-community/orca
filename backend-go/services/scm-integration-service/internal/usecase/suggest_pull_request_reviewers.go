package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SuggestPullRequestReviewersParams struct {
	TenantID     string
	Provider     domain.ScmProvider
	Repo         string
	BaseRef      string
	ChangedFiles []string
}

type SuggestedReviewers struct {
	ReviewerLogins []string
	TeamSlugs      []string
	Found          bool
}

// codeownersPaths mirrors GitHub's own documented lookup order.
var codeownersPaths = []string{"CODEOWNERS", ".github/CODEOWNERS", ".gitlab/CODEOWNERS", "docs/CODEOWNERS"}

type SuggestPullRequestReviewers struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewSuggestPullRequestReviewers(credentials CredentialResolver, providers ProviderRegistry) *SuggestPullRequestReviewers {
	return &SuggestPullRequestReviewers{credentials: credentials, providers: providers}
}

func (uc *SuggestPullRequestReviewers) Execute(ctx context.Context, in SuggestPullRequestReviewersParams) (SuggestedReviewers, error) {
	if in.TenantID == "" {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	for _, path := range codeownersPaths {
		content, found, err := provider.GetRepoFileContent(ctx, cred, in.Repo, path, in.BaseRef)
		if err != nil {
			return SuggestedReviewers{}, apperrors.New(apperrors.KindInternal, "SCM_CODEOWNERS_FETCH_FAILED", "failed to fetch CODEOWNERS", err)
		}
		if !found {
			continue
		}
		logins, teams := MatchOwners(ParseCodeowners(content), in.ChangedFiles)
		return SuggestedReviewers{ReviewerLogins: logins, TeamSlugs: teams, Found: true}, nil
	}
	return SuggestedReviewers{Found: false}, nil // no CODEOWNERS anywhere — not an error, BR-CR-18 says "if present"
}
