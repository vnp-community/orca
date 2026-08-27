package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type AddIssueCommentInput struct {
	Provider     domain.Provider
	IssueID      string
	BodyMarkdown string
	WorkspaceID  string
}

type AddIssueComment struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewAddIssueComment(registry ProviderRegistry, credentials CredentialResolver) *AddIssueComment {
	return &AddIssueComment{registry: registry, credentials: credentials}
}

func (uc *AddIssueComment) Execute(ctx context.Context, in AddIssueCommentInput) (domain.IssueComment, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.IssueComment{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.IssueComment{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.IssueComment{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.IssueComment{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	comment, err := provider.AddIssueComment(ctx, cred, in.IssueID, in.BodyMarkdown)
	if err != nil {
		return domain.IssueComment{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_ADD_COMMENT_FAILED", "failed to add comment", err)
	}
	return comment, nil
}
