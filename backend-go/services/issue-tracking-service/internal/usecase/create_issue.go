package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// CreateIssueInput mirrors the CreateIssue gRPC request 1:1 by design, minus
// TenantId — see ListIssuesInput's doc comment for why.
type CreateIssueInput struct {
	Provider         domain.Provider
	ProjectKey       string
	Title            string
	Description      string
	IssueTypeID      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	ParentIssueID    string
	CustomFieldsJSON string
	WorkspaceID      string
	TeamID           string // Linear: replaces ProjectKey as the primary grouping
	StateID          string // Linear: initial workflow state
}

// CreateIssue creates a new issue in Jira or Linear on the caller's behalf.
type CreateIssue struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewCreateIssue(registry ProviderRegistry, credentials CredentialResolver) *CreateIssue {
	return &CreateIssue{registry: registry, credentials: credentials}
}

func (uc *CreateIssue) Execute(ctx context.Context, in CreateIssueInput) (domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Title == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_TITLE", "title is required", domain.ErrEmptyTitle)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}

	// Mutations are not silently retried on ambiguous failure, to avoid
	// duplicate issue creation (design doc §8) — no retry wrapper here.
	projectKey := in.ProjectKey
	if in.Provider == domain.ProviderLinear && in.TeamID != "" {
		projectKey = in.TeamID // linear/client.go resolves team by key/id from ProjectKey
	}
	issue, err := provider.CreateIssue(ctx, cred, domain.NewIssueInput{
		ProjectKey: projectKey, TeamID: in.TeamID, StateID: in.StateID, Title: in.Title, Description: in.Description,
		IssueTypeID: in.IssueTypeID, AssigneeID: in.AssigneeID, PriorityID: in.PriorityID,
		LabelIDs: in.LabelIDs, ParentIssueID: in.ParentIssueID, CustomFieldsJSON: in.CustomFieldsJSON,
	})
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREATE_FAILED", "failed to create issue with provider", err)
	}
	return issue, nil
}
