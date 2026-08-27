package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// fakeIssueTrackerProvider is an in-memory IssueTrackerProvider implementing
// every method of the widened port (SOL-015/SOL-016) — most methods are
// simple configurable stubs; ListIssues/CreateIssue carry the richer
// input/output the pre-existing tests already exercise.
type fakeIssueTrackerProvider struct {
	issues            []domain.Issue
	listErr           error
	createErr         error
	createInput       domain.NewIssueInput
	createInputCalled bool
	createReturn      domain.Issue

	whoamiViewer domain.Viewer
	whoamiErr    error

	searchIssues []domain.Issue
	searchErr    error

	getIssueReturn domain.Issue
	getIssueErr    error

	updateIssueReturn domain.Issue
	updateIssueErr    error

	addCommentReturn domain.IssueComment
	addCommentErr    error

	listCommentsReturn []domain.IssueComment
	listCommentsErr    error

	listProjectsReturn []domain.ProjectRef
	listProjectsErr    error

	listIssueTypesReturn []domain.IssueTypeRef
	listIssueTypesErr    error

	listCreateFieldsReturn []domain.CreateField
	listCreateFieldsErr    error

	listAssignableUsersReturn []domain.UserRef
	listAssignableUsersErr    error

	listPrioritiesReturn []domain.PriorityRef
	listPrioritiesErr    error

	listTransitionsReturn []domain.Transition
	listTransitionsErr    error

	getProjectStatusOrderReturn domain.ProjectStatusOrder
	getProjectStatusOrderErr    error

	createProjectReturn domain.ProjectRef
	createProjectErr    error

	getProjectReturn domain.ProjectRef
	getProjectErr    error

	listTeamsReturn []domain.Team
	listTeamsErr    error

	listTeamLabelsReturn []domain.TeamLabel
	listTeamLabelsErr    error

	listTeamMembersReturn []domain.TeamMember
	listTeamMembersErr    error

	getCustomViewReturn domain.CustomView
	getCustomViewErr    error

	listWorkflowStatesReturn []domain.WorkflowState
	listWorkflowStatesErr    error

	createIssueCalled    bool
	lastCreateIssueInput domain.NewIssueInput
}

func (f *fakeIssueTrackerProvider) Whoami(ctx context.Context, cred Credential) (domain.Viewer, error) {
	if f.whoamiErr != nil {
		return domain.Viewer{}, f.whoamiErr
	}
	return f.whoamiViewer, nil
}

func (f *fakeIssueTrackerProvider) SearchIssues(ctx context.Context, cred Credential, query string, limit int) ([]domain.Issue, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchIssues, nil
}

func (f *fakeIssueTrackerProvider) ListIssues(ctx context.Context, cred Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.issues, nil
}

func (f *fakeIssueTrackerProvider) GetIssue(ctx context.Context, cred Credential, issueID string) (domain.Issue, error) {
	if f.getIssueErr != nil {
		return domain.Issue{}, f.getIssueErr
	}
	return f.getIssueReturn, nil
}

func (f *fakeIssueTrackerProvider) CreateIssue(ctx context.Context, cred Credential, in domain.NewIssueInput) (domain.Issue, error) {
	f.createIssueCalled = true
	f.lastCreateIssueInput = in
	if f.createErr != nil {
		return domain.Issue{}, f.createErr
	}
	f.createInputCalled = true
	f.createInput = in
	return f.createReturn, nil
}

func (f *fakeIssueTrackerProvider) UpdateIssue(ctx context.Context, cred Credential, in domain.IssueUpdate) (domain.Issue, error) {
	if f.updateIssueErr != nil {
		return domain.Issue{}, f.updateIssueErr
	}
	return f.updateIssueReturn, nil
}

func (f *fakeIssueTrackerProvider) AddIssueComment(ctx context.Context, cred Credential, issueID, bodyMarkdown string) (domain.IssueComment, error) {
	if f.addCommentErr != nil {
		return domain.IssueComment{}, f.addCommentErr
	}
	return f.addCommentReturn, nil
}

func (f *fakeIssueTrackerProvider) ListIssueComments(ctx context.Context, cred Credential, issueID string) ([]domain.IssueComment, error) {
	if f.listCommentsErr != nil {
		return nil, f.listCommentsErr
	}
	return f.listCommentsReturn, nil
}

func (f *fakeIssueTrackerProvider) ListProjects(ctx context.Context, cred Credential, workspaceID string) ([]domain.ProjectRef, error) {
	if f.listProjectsErr != nil {
		return nil, f.listProjectsErr
	}
	return f.listProjectsReturn, nil
}

func (f *fakeIssueTrackerProvider) ListIssueTypes(ctx context.Context, cred Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error) {
	if f.listIssueTypesErr != nil {
		return nil, f.listIssueTypesErr
	}
	return f.listIssueTypesReturn, nil
}

func (f *fakeIssueTrackerProvider) ListCreateFields(ctx context.Context, cred Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error) {
	if f.listCreateFieldsErr != nil {
		return nil, f.listCreateFieldsErr
	}
	return f.listCreateFieldsReturn, nil
}

func (f *fakeIssueTrackerProvider) ListAssignableUsers(ctx context.Context, cred Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error) {
	if f.listAssignableUsersErr != nil {
		return nil, f.listAssignableUsersErr
	}
	return f.listAssignableUsersReturn, nil
}

func (f *fakeIssueTrackerProvider) ListPriorities(ctx context.Context, cred Credential) ([]domain.PriorityRef, error) {
	if f.listPrioritiesErr != nil {
		return nil, f.listPrioritiesErr
	}
	return f.listPrioritiesReturn, nil
}

func (f *fakeIssueTrackerProvider) ListTransitions(ctx context.Context, cred Credential, issueID string) ([]domain.Transition, error) {
	if f.listTransitionsErr != nil {
		return nil, f.listTransitionsErr
	}
	return f.listTransitionsReturn, nil
}

func (f *fakeIssueTrackerProvider) GetProjectStatusOrder(ctx context.Context, cred Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error) {
	if f.getProjectStatusOrderErr != nil {
		return domain.ProjectStatusOrder{}, f.getProjectStatusOrderErr
	}
	return f.getProjectStatusOrderReturn, nil
}

func (f *fakeIssueTrackerProvider) CreateProject(ctx context.Context, cred Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error) {
	if f.createProjectErr != nil {
		return domain.ProjectRef{}, f.createProjectErr
	}
	return f.createProjectReturn, nil
}

func (f *fakeIssueTrackerProvider) GetProject(ctx context.Context, cred Credential, projectID, workspaceID string) (domain.ProjectRef, error) {
	if f.getProjectErr != nil {
		return domain.ProjectRef{}, f.getProjectErr
	}
	return f.getProjectReturn, nil
}

func (f *fakeIssueTrackerProvider) ListTeams(ctx context.Context, cred Credential, workspaceID string) ([]domain.Team, error) {
	if f.listTeamsErr != nil {
		return nil, f.listTeamsErr
	}
	return f.listTeamsReturn, nil
}

func (f *fakeIssueTrackerProvider) ListTeamLabels(ctx context.Context, cred Credential, teamID string) ([]domain.TeamLabel, error) {
	if f.listTeamLabelsErr != nil {
		return nil, f.listTeamLabelsErr
	}
	return f.listTeamLabelsReturn, nil
}

func (f *fakeIssueTrackerProvider) ListTeamMembers(ctx context.Context, cred Credential, teamID string) ([]domain.TeamMember, error) {
	if f.listTeamMembersErr != nil {
		return nil, f.listTeamMembersErr
	}
	return f.listTeamMembersReturn, nil
}

func (f *fakeIssueTrackerProvider) GetCustomView(ctx context.Context, cred Credential, viewID, model string) (domain.CustomView, error) {
	if f.getCustomViewErr != nil {
		return domain.CustomView{}, f.getCustomViewErr
	}
	return f.getCustomViewReturn, nil
}

func (f *fakeIssueTrackerProvider) ListWorkflowStates(ctx context.Context, cred Credential, teamID string) ([]domain.WorkflowState, error) {
	if f.listWorkflowStatesErr != nil {
		return nil, f.listWorkflowStatesErr
	}
	return f.listWorkflowStatesReturn, nil
}

// fakeProviderRegistry resolves a single fixed provider, or errors if asked
// to resolve anything else — enough for these unit tests. resolveFunc, when
// set, overrides the fixed-provider behavior entirely (used by tests that
// need to observe/vary which domain.Provider a usecase resolves).
type fakeProviderRegistry struct {
	provider    domain.Provider
	impl        IssueTrackerProvider
	resolveErr  error
	resolveArgs []domain.Provider
	resolveFunc func(domain.Provider) (IssueTrackerProvider, error)
}

func (f *fakeProviderRegistry) Resolve(provider domain.Provider) (IssueTrackerProvider, error) {
	f.resolveArgs = append(f.resolveArgs, provider)
	if f.resolveFunc != nil {
		return f.resolveFunc(provider)
	}
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	if f.provider != "" && provider != f.provider {
		return nil, errors.New("fakeProviderRegistry: unexpected provider")
	}
	if f.impl != nil {
		return f.impl, nil
	}
	return nil, errors.New("fakeProviderRegistry: no impl configured")
}

// fakeCredentialResolver returns a fixed credential, or errors. Also
// implements Write/ExistingCredentialID for Connect's usecase tests.
type fakeCredentialResolver struct {
	cred       Credential
	resolveErr error

	writeReturnsID string
	writeErr       error
	writeCalled    bool

	existingID    string
	existingFound bool
	existingErr   error
}

func (f *fakeCredentialResolver) Resolve(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (Credential, error) {
	if f.resolveErr != nil {
		return Credential{}, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeCredentialResolver) Write(ctx context.Context, tenantID, userID string, provider domain.Provider, cred Credential) (string, error) {
	f.writeCalled = true
	if f.writeErr != nil {
		return "", f.writeErr
	}
	return f.writeReturnsID, nil
}

func (f *fakeCredentialResolver) ExistingCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider) (string, bool, error) {
	if f.existingErr != nil {
		return "", false, f.existingErr
	}
	return f.existingID, f.existingFound, nil
}

// fakeConnectionRepository is an in-memory ConnectionRepository.
type fakeConnectionRepository struct {
	upsertReturns      domain.ConnectionStatus
	upsertErr          error
	upsertCalled       bool
	upsertCredentialID string

	deleteErr    error
	deleteCalled bool

	getStatusReturns domain.ConnectionStatus
	getStatusErr     error

	selectReturns domain.ConnectionStatus
	selectErr     error

	credentialID  string
	credentialErr error
}

func (f *fakeConnectionRepository) Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error) {
	f.upsertCalled = true
	f.upsertCredentialID = credentialID
	if f.upsertErr != nil {
		return domain.ConnectionStatus{}, f.upsertErr
	}
	return f.upsertReturns, nil
}

func (f *fakeConnectionRepository) Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error {
	f.deleteCalled = true
	return f.deleteErr
}

func (f *fakeConnectionRepository) GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error) {
	if f.getStatusErr != nil {
		return domain.ConnectionStatus{}, f.getStatusErr
	}
	return f.getStatusReturns, nil
}

func (f *fakeConnectionRepository) SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error) {
	if f.selectErr != nil {
		return domain.ConnectionStatus{}, f.selectErr
	}
	return f.selectReturns, nil
}

func (f *fakeConnectionRepository) GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (string, error) {
	if f.credentialErr != nil {
		return "", f.credentialErr
	}
	return f.credentialID, nil
}

func TestListIssues_RequiresTenantContext(t *testing.T) {
	uc := NewListIssues(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	_, err := uc.Execute(context.Background(), ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListIssues_RejectsInvalidProvider(t *testing.T) {
	uc := NewListIssues(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")
	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.Provider("bogus")})
	if err == nil {
		t.Fatal("expected an error for an invalid provider")
	}
}

func TestListIssues_ReturnsProviderIssues(t *testing.T) {
	want := []domain.Issue{{ID: "PROJ-1", Title: "Fix bug", State: "Todo", URL: "https://example.atlassian.net/browse/PROJ-1"}}
	provider := &fakeIssueTrackerProvider{issues: want}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{cred: Credential{BaseURL: "https://example.atlassian.net", Email: "a@b.com", Token: "tok"}}

	uc := NewListIssues(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	got, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira, ProjectKey: "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "PROJ-1" {
		t.Errorf("unexpected issues: %+v", got)
	}
}

func TestListIssues_ProviderErrorPropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{listErr: errors.New("jira unavailable")}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{}

	uc := NewListIssues(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected error to propagate from provider failure")
	}
}

func TestListIssues_CredentialResolutionFailurePropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{resolveErr: errors.New("no credential on file")}

	uc := NewListIssues(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected error to propagate from credential resolution failure")
	}
}
