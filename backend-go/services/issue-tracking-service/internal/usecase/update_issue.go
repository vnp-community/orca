package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type UpdateIssueInput struct {
	Provider         domain.Provider
	IssueID          string
	Title            string
	Description      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	WorkflowStateID  string
	CustomFieldsJSON string
	WorkspaceID      string
}

type UpdateIssue struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewUpdateIssue(registry ProviderRegistry, credentials CredentialResolver) *UpdateIssue {
	return &UpdateIssue{registry: registry, credentials: credentials}
}

func (uc *UpdateIssue) Execute(ctx context.Context, in UpdateIssueInput) (domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if in.IssueID == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_ISSUE_ID", "issue_id is required", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	issue, err := provider.UpdateIssue(ctx, cred, domain.IssueUpdate{
		IssueID: in.IssueID, Title: in.Title, Description: in.Description,
		AssigneeID: in.AssigneeID, PriorityID: in.PriorityID, LabelIDs: in.LabelIDs,
		WorkflowStateID: in.WorkflowStateID, CustomFieldsJSON: in.CustomFieldsJSON,
	})
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_UPDATE_FAILED", "failed to update issue with provider", err)
	}
	return issue, nil
}
