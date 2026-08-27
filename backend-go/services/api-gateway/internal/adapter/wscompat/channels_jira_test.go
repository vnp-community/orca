package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// fakeIssueTrackingClient is shared by channels_jira_test.go and
// channels_linear_test.go (both relay to the same IssueTrackingService) —
// embeds the generated interface so only the methods a given test actually
// exercises need a func field.
type fakeIssueTrackingClient struct {
	issuetrackingv1.IssueTrackingServiceClient

	getConnectionStatusFunc   func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error)
	createIssueFunc           func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error)
	listIssuesFunc            func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error)
	getIssueFunc              func(ctx context.Context, in *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error)
	updateIssueFunc           func(ctx context.Context, in *issuetrackingv1.UpdateIssueRequest) (*issuetrackingv1.Issue, error)
	addIssueCommentFunc       func(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error)
	listIssueCommentsFunc     func(ctx context.Context, in *issuetrackingv1.ListIssueCommentsRequest) (*issuetrackingv1.ListIssueCommentsResponse, error)
	listProjectsFunc          func(ctx context.Context, in *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error)
	listCreateFieldsFunc      func(ctx context.Context, in *issuetrackingv1.ListCreateFieldsRequest) (*issuetrackingv1.ListCreateFieldsResponse, error)
	listTeamsFunc             func(ctx context.Context, in *issuetrackingv1.ListTeamsRequest) (*issuetrackingv1.ListTeamsResponse, error)
	connectFunc               func(ctx context.Context, in *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error)
	disconnectFunc            func(ctx context.Context, in *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error)
	selectWorkspaceFunc       func(ctx context.Context, in *issuetrackingv1.SelectWorkspaceRequest) (*issuetrackingv1.ConnectionStatus, error)
	testConnectionFunc        func(ctx context.Context, in *issuetrackingv1.TestConnectionRequest) (*issuetrackingv1.TestConnectionResult, error)
	searchIssuesFunc          func(ctx context.Context, in *issuetrackingv1.SearchIssuesRequest) (*issuetrackingv1.SearchIssuesResponse, error)
	listIssueTypesFunc        func(ctx context.Context, in *issuetrackingv1.ListIssueTypesRequest) (*issuetrackingv1.ListIssueTypesResponse, error)
	listAssignableUsersFunc   func(ctx context.Context, in *issuetrackingv1.ListAssignableUsersRequest) (*issuetrackingv1.ListAssignableUsersResponse, error)
	listPrioritiesFunc        func(ctx context.Context, in *issuetrackingv1.ListPrioritiesRequest) (*issuetrackingv1.ListPrioritiesResponse, error)
	listTransitionsFunc       func(ctx context.Context, in *issuetrackingv1.ListTransitionsRequest) (*issuetrackingv1.ListTransitionsResponse, error)
	getProjectStatusOrderFunc func(ctx context.Context, in *issuetrackingv1.GetProjectStatusOrderRequest) (*issuetrackingv1.GetProjectStatusOrderResponse, error)
	createProjectFunc         func(ctx context.Context, in *issuetrackingv1.CreateProjectRequest) (*issuetrackingv1.Project, error)
	getProjectFunc            func(ctx context.Context, in *issuetrackingv1.GetProjectRequest) (*issuetrackingv1.Project, error)
	listTeamLabelsFunc        func(ctx context.Context, in *issuetrackingv1.ListTeamLabelsRequest) (*issuetrackingv1.ListTeamLabelsResponse, error)
	listTeamMembersFunc       func(ctx context.Context, in *issuetrackingv1.ListTeamMembersRequest) (*issuetrackingv1.ListTeamMembersResponse, error)
	getCustomViewFunc         func(ctx context.Context, in *issuetrackingv1.GetCustomViewRequest) (*issuetrackingv1.CustomView, error)
	listWorkflowStatesFunc    func(ctx context.Context, in *issuetrackingv1.ListWorkflowStatesRequest) (*issuetrackingv1.ListWorkflowStatesResponse, error)

	// credentials.* group (channels_credentials_test.go, TASK-042).
	setIntegrationCredentialFunc       func(ctx context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error)
	getIntegrationCredentialStatusFunc func(ctx context.Context, in *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error)
	listIntegrationCredentialsFunc     func(ctx context.Context, in *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error)
	revokeAuthFunc                     func(ctx context.Context, in *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error)
}

func (f *fakeIssueTrackingClient) GetConnectionStatus(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest, _ ...grpc.CallOption) (*issuetrackingv1.ConnectionStatus, error) {
	return f.getConnectionStatusFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) CreateIssue(ctx context.Context, in *issuetrackingv1.CreateIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.CreateIssueResponse, error) {
	return f.createIssueFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListIssues(ctx context.Context, in *issuetrackingv1.ListIssuesRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListIssuesResponse, error) {
	return f.listIssuesFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) GetIssue(ctx context.Context, in *issuetrackingv1.GetIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.Issue, error) {
	return f.getIssueFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) UpdateIssue(ctx context.Context, in *issuetrackingv1.UpdateIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.Issue, error) {
	return f.updateIssueFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) AddIssueComment(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest, _ ...grpc.CallOption) (*issuetrackingv1.IssueComment, error) {
	return f.addIssueCommentFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListIssueComments(ctx context.Context, in *issuetrackingv1.ListIssueCommentsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListIssueCommentsResponse, error) {
	return f.listIssueCommentsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListProjects(ctx context.Context, in *issuetrackingv1.ListProjectsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListProjectsResponse, error) {
	return f.listProjectsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListCreateFields(ctx context.Context, in *issuetrackingv1.ListCreateFieldsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListCreateFieldsResponse, error) {
	return f.listCreateFieldsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListTeams(ctx context.Context, in *issuetrackingv1.ListTeamsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListTeamsResponse, error) {
	return f.listTeamsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) Connect(ctx context.Context, in *issuetrackingv1.ConnectRequest, _ ...grpc.CallOption) (*issuetrackingv1.ConnectionStatus, error) {
	return f.connectFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) Disconnect(ctx context.Context, in *issuetrackingv1.DisconnectRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.disconnectFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) SelectWorkspace(ctx context.Context, in *issuetrackingv1.SelectWorkspaceRequest, _ ...grpc.CallOption) (*issuetrackingv1.ConnectionStatus, error) {
	return f.selectWorkspaceFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) TestConnection(ctx context.Context, in *issuetrackingv1.TestConnectionRequest, _ ...grpc.CallOption) (*issuetrackingv1.TestConnectionResult, error) {
	return f.testConnectionFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) SearchIssues(ctx context.Context, in *issuetrackingv1.SearchIssuesRequest, _ ...grpc.CallOption) (*issuetrackingv1.SearchIssuesResponse, error) {
	return f.searchIssuesFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListIssueTypes(ctx context.Context, in *issuetrackingv1.ListIssueTypesRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListIssueTypesResponse, error) {
	return f.listIssueTypesFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListAssignableUsers(ctx context.Context, in *issuetrackingv1.ListAssignableUsersRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListAssignableUsersResponse, error) {
	return f.listAssignableUsersFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListPriorities(ctx context.Context, in *issuetrackingv1.ListPrioritiesRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListPrioritiesResponse, error) {
	return f.listPrioritiesFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListTransitions(ctx context.Context, in *issuetrackingv1.ListTransitionsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListTransitionsResponse, error) {
	return f.listTransitionsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) GetProjectStatusOrder(ctx context.Context, in *issuetrackingv1.GetProjectStatusOrderRequest, _ ...grpc.CallOption) (*issuetrackingv1.GetProjectStatusOrderResponse, error) {
	return f.getProjectStatusOrderFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) CreateProject(ctx context.Context, in *issuetrackingv1.CreateProjectRequest, _ ...grpc.CallOption) (*issuetrackingv1.Project, error) {
	return f.createProjectFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) GetProject(ctx context.Context, in *issuetrackingv1.GetProjectRequest, _ ...grpc.CallOption) (*issuetrackingv1.Project, error) {
	return f.getProjectFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListTeamLabels(ctx context.Context, in *issuetrackingv1.ListTeamLabelsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListTeamLabelsResponse, error) {
	return f.listTeamLabelsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListTeamMembers(ctx context.Context, in *issuetrackingv1.ListTeamMembersRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListTeamMembersResponse, error) {
	return f.listTeamMembersFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) GetCustomView(ctx context.Context, in *issuetrackingv1.GetCustomViewRequest, _ ...grpc.CallOption) (*issuetrackingv1.CustomView, error) {
	return f.getCustomViewFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListWorkflowStates(ctx context.Context, in *issuetrackingv1.ListWorkflowStatesRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListWorkflowStatesResponse, error) {
	return f.listWorkflowStatesFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) SetIntegrationCredential(ctx context.Context, in *issuetrackingv1.SetIntegrationCredentialRequest, _ ...grpc.CallOption) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
	return f.setIntegrationCredentialFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) GetIntegrationCredentialStatus(ctx context.Context, in *issuetrackingv1.GetIntegrationCredentialStatusRequest, _ ...grpc.CallOption) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
	return f.getIntegrationCredentialStatusFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) ListIntegrationCredentials(ctx context.Context, in *issuetrackingv1.ListIntegrationCredentialsRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
	return f.listIntegrationCredentialsFunc(ctx, in)
}

func (f *fakeIssueTrackingClient) RevokeAuth(ctx context.Context, in *issuetrackingv1.RevokeAuthRequest, _ ...grpc.CallOption) (*issuetrackingv1.RevokeAuthResponse, error) {
	return f.revokeAuthFunc(ctx, in)
}

func TestJiraStatusChannel_Success(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getConnectionStatusFunc: func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
				t.Fatalf("want jira provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.ConnectionStatus{Connected: true, ViewerId: "acc-1"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(jiraConnectionStatusView)
	if !ok || !view.Connected {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraStatusChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	fake := &fakeIssueTrackingClient{
		getConnectionStatusFunc: func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.status", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestJiraCreateIssueChannel_MapsRequestFieldsAndViewShape(t *testing.T) {
	var gotReq *issuetrackingv1.CreateIssueRequest
	fake := &fakeIssueTrackingClient{
		createIssueFunc: func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
			gotReq = in
			return &issuetrackingv1.CreateIssueResponse{Issue: &issuetrackingv1.Issue{Id: "1", Key: "PROJ-1", Title: in.GetTitle()}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectKey": "PROJ", "title": "New bug", "issueType": "Bug", "siteId": "https://x.atlassian.net"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.createIssue", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProjectKey() != "PROJ" || gotReq.GetIssueTypeId() != "Bug" || gotReq.GetWorkspaceId() != "https://x.atlassian.net" {
		t.Fatalf("request fields not mapped correctly: %+v", gotReq)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Fatalf("want jira provider, got %v", gotReq.GetProvider())
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true {
		t.Fatalf("unexpected result shape: %+v", result)
	}
}

func TestJiraCreateIssueChannel_ProviderErrorReturnsOKFalse(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		createIssueFunc: func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
			return nil, errors.New("jira rejected the request")
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectKey": "PROJ", "title": "New bug"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.createIssue", args)
	if err != nil {
		t.Fatalf("jira.createIssue must resolve, not reject, on a provider error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestJiraListIssuesChannel_ReturnsBareArray_NotEnvelope(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssuesFunc: func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
			return &issuetrackingv1.ListIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "PROJ-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.listIssues", argsJSON(t, map[string]any{"projectKey": "PROJ"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.([]jiraIssueView); !ok {
		t.Fatalf("jira.listIssues must return a bare []jiraIssueView, not an envelope — got %T", result)
	}
}

func TestJiraGetIssueChannel_MapsFields(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getIssueFunc: func(ctx context.Context, in *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error) {
			if in.GetIssueId() != "PROJ-1" {
				t.Fatalf("want issue id PROJ-1, got %q", in.GetIssueId())
			}
			return &issuetrackingv1.Issue{Id: "1", Key: "PROJ-1", Title: "Bug"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.getIssue", argsJSON(t, map[string]any{"issueIdOrKey": "PROJ-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(jiraIssueView)
	if !ok || view.Key != "PROJ-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraUpdateIssueChannel_PropagatesError(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		updateIssueFunc: func(ctx context.Context, in *issuetrackingv1.UpdateIssueRequest) (*issuetrackingv1.Issue, error) {
			return nil, errors.New("update rejected")
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.updateIssue", argsJSON(t, map[string]any{"issueIdOrKey": "PROJ-1", "title": "New title"}))
	if err != nil {
		t.Fatalf("jira.updateIssue must resolve, not reject, on a provider error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestJiraAddIssueCommentChannel_MapsFields(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		addIssueCommentFunc: func(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error) {
			if in.GetBodyMarkdown() != "a comment" {
				t.Fatalf("want body 'a comment', got %q", in.GetBodyMarkdown())
			}
			return &issuetrackingv1.IssueComment{Id: "c1"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.addIssueComment", argsJSON(t, map[string]any{"issueIdOrKey": "PROJ-1", "body": "a comment"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true || out["id"] != "c1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraIssueCommentsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssueCommentsFunc: func(ctx context.Context, in *issuetrackingv1.ListIssueCommentsRequest) (*issuetrackingv1.ListIssueCommentsResponse, error) {
			return &issuetrackingv1.ListIssueCommentsResponse{Comments: []*issuetrackingv1.IssueComment{{Id: "c1", BodyMarkdown: "hi"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.issueComments", argsJSON(t, map[string]any{"issueIdOrKey": "PROJ-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraCommentView)
	if !ok || len(views) != 1 || views[0].Body != "hi" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraListProjectsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listProjectsFunc: func(ctx context.Context, in *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error) {
			return &issuetrackingv1.ListProjectsResponse{Projects: []*issuetrackingv1.Project{{Id: "1", Key: "PROJ", Name: "Project"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listProjects", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraProjectView)
	if !ok || len(views) != 1 || views[0].Key != "PROJ" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraListCreateFieldsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listCreateFieldsFunc: func(ctx context.Context, in *issuetrackingv1.ListCreateFieldsRequest) (*issuetrackingv1.ListCreateFieldsResponse, error) {
			return &issuetrackingv1.ListCreateFieldsResponse{Fields: []*issuetrackingv1.CreateField{{Key: "summary", Name: "Summary", Required: true}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listCreateFields", argsJSON(t, map[string]any{"projectIdOrKey": "PROJ", "issueTypeId": "10001"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraCreateFieldView)
	if !ok || len(views) != 1 || views[0].Key != "summary" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraConnectChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.ConnectRequest
	fake := &fakeIssueTrackingClient{
		connectFunc: func(ctx context.Context, in *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
			gotReq = in
			return &issuetrackingv1.ConnectionStatus{ViewerId: "acc-1", ViewerDisplayName: "Ada", ViewerEmail: "ada@x.com"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.connect",
		argsJSON(t, map[string]any{"siteUrl": "https://x.atlassian.net", "email": "a@b.com", "apiToken": "tok"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true {
		t.Fatalf("unexpected result shape: %+v", result)
	}
	viewer, ok := out["viewer"].(jiraViewerView)
	if !ok || viewer.AccountID != "acc-1" {
		t.Fatalf("unexpected viewer: %+v", out["viewer"])
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA || gotReq.GetSiteUrl() != "https://x.atlassian.net" {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
}

func TestJiraConnectChannel_ProviderErrorReturnsOKFalse(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		connectFunc: func(ctx context.Context, in *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
			return nil, errors.New("401 unauthorized")
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "jira.connect",
		argsJSON(t, map[string]any{"siteUrl": "https://x.atlassian.net", "email": "a@b.com", "apiToken": "bad"}))
	if err != nil {
		t.Fatalf("jira.connect must resolve, not reject, on an auth failure: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestJiraDisconnectChannel_SiteIDOptional(t *testing.T) {
	var gotReq *issuetrackingv1.DisconnectRequest
	fake := &fakeIssueTrackingClient{
		disconnectFunc: func(ctx context.Context, in *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.disconnect", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA || gotReq.GetWorkspaceId() != "" {
		t.Errorf("expected empty workspaceId when siteId omitted (disconnects all), got %+v", gotReq)
	}
}

func TestJiraDisconnectChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("no stored credential")
	fake := &fakeIssueTrackingClient{
		disconnectFunc: func(ctx context.Context, in *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.disconnect", argsJSON(t, map[string]any{"siteId": "https://x.atlassian.net"}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestJiraSelectSiteChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.SelectWorkspaceRequest
	fake := &fakeIssueTrackingClient{
		selectWorkspaceFunc: func(ctx context.Context, in *issuetrackingv1.SelectWorkspaceRequest) (*issuetrackingv1.ConnectionStatus, error) {
			gotReq = in
			return &issuetrackingv1.ConnectionStatus{Connected: true, SelectedWorkspaceId: in.GetWorkspaceId()}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.selectSite", argsJSON(t, map[string]any{"siteId": "https://x.atlassian.net"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(jiraConnectionStatusView)
	if !ok || view.SelectedSiteID != "https://x.atlassian.net" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Errorf("expected jira provider, got %v", gotReq.GetProvider())
	}
}

func TestJiraSelectSiteChannel_RequiresSiteID(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		selectWorkspaceFunc: func(ctx context.Context, in *issuetrackingv1.SelectWorkspaceRequest) (*issuetrackingv1.ConnectionStatus, error) {
			t.Fatal("SelectWorkspace must not be called without siteId")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	// jira.selectSite decodes siteId with decodeArg's required-field error
	// path when args are entirely absent.
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.selectSite", nil)
	if err == nil {
		t.Fatal("expected an error when args are missing")
	}
}

func TestJiraTestConnectionChannel_ReturnsOKFalseOnProviderRejection(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		testConnectionFunc: func(ctx context.Context, in *issuetrackingv1.TestConnectionRequest) (*issuetrackingv1.TestConnectionResult, error) {
			return &issuetrackingv1.TestConnectionResult{Ok: false, Error: "invalid token"}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.testConnection", argsJSON(t, map[string]any{"siteId": "https://x.atlassian.net"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false || out["error"] != "invalid token" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraTestConnectionChannel_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("issue-tracking-service unavailable")
	fake := &fakeIssueTrackingClient{
		testConnectionFunc: func(ctx context.Context, in *issuetrackingv1.TestConnectionRequest) (*issuetrackingv1.TestConnectionResult, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.testConnection", nil)
	if err != nil {
		t.Fatalf("jira.testConnection must resolve, not reject, on an RPC error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestJiraSearchIssuesChannel_MapsFields(t *testing.T) {
	var gotReq *issuetrackingv1.SearchIssuesRequest
	fake := &fakeIssueTrackingClient{
		searchIssuesFunc: func(ctx context.Context, in *issuetrackingv1.SearchIssuesRequest) (*issuetrackingv1.SearchIssuesResponse, error) {
			gotReq = in
			return &issuetrackingv1.SearchIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "PROJ-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.searchIssues", argsJSON(t, map[string]any{"jql": "project = PROJ", "limit": 10}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraIssueView)
	if !ok || len(views) != 1 || views[0].Key != "PROJ-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetQuery() != "project = PROJ" || gotReq.GetLimit() != 10 {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Errorf("expected jira provider, got %v", gotReq.GetProvider())
	}
}

func TestJiraListIssueTypesChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssueTypesFunc: func(ctx context.Context, in *issuetrackingv1.ListIssueTypesRequest) (*issuetrackingv1.ListIssueTypesResponse, error) {
			return &issuetrackingv1.ListIssueTypesResponse{IssueTypes: []*issuetrackingv1.IssueType{{Id: "10001", Name: "Bug", Subtask: false}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listIssueTypes", argsJSON(t, map[string]any{"projectIdOrKey": "PROJ"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraIssueTypeView)
	if !ok || len(views) != 1 || views[0].Name != "Bug" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraListAssignableUsersChannel_ReturnsViews(t *testing.T) {
	var gotReq *issuetrackingv1.ListAssignableUsersRequest
	fake := &fakeIssueTrackingClient{
		listAssignableUsersFunc: func(ctx context.Context, in *issuetrackingv1.ListAssignableUsersRequest) (*issuetrackingv1.ListAssignableUsersResponse, error) {
			gotReq = in
			return &issuetrackingv1.ListAssignableUsersResponse{Users: []*issuetrackingv1.UserRef{{Id: "acc-1", DisplayName: "Ada", Email: "ada@x.com"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listAssignableUsers", argsJSON(t, map[string]any{"projectIdOrKey": "PROJ", "issueIdOrKey": "PROJ-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraUserView)
	if !ok || len(views) != 1 || views[0].AccountID != "acc-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Errorf("expected jira provider, got %v", gotReq.GetProvider())
	}
}

func TestJiraListPrioritiesChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listPrioritiesFunc: func(ctx context.Context, in *issuetrackingv1.ListPrioritiesRequest) (*issuetrackingv1.ListPrioritiesResponse, error) {
			return &issuetrackingv1.ListPrioritiesResponse{Priorities: []*issuetrackingv1.Priority{{Id: "1", Name: "High"}}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listPriorities", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraPriorityView)
	if !ok || len(views) != 1 || views[0].Name != "High" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraListTransitionsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listTransitionsFunc: func(ctx context.Context, in *issuetrackingv1.ListTransitionsRequest) (*issuetrackingv1.ListTransitionsResponse, error) {
			return &issuetrackingv1.ListTransitionsResponse{Transitions: []*issuetrackingv1.Transition{
				{Id: "31", Name: "Done", To: &issuetrackingv1.WorkflowState{Id: "3", Name: "Done"}},
			}}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.listTransitions", argsJSON(t, map[string]any{"issueIdOrKey": "PROJ-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]jiraTransitionView)
	if !ok || len(views) != 1 || views[0].To.Name != "Done" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJiraGetProjectStatusOrderChannel_ReturnsColumns(t *testing.T) {
	var gotReq *issuetrackingv1.GetProjectStatusOrderRequest
	fake := &fakeIssueTrackingClient{
		getProjectStatusOrderFunc: func(ctx context.Context, in *issuetrackingv1.GetProjectStatusOrderRequest) (*issuetrackingv1.GetProjectStatusOrderResponse, error) {
			gotReq = in
			return &issuetrackingv1.GetProjectStatusOrderResponse{
				StatusIdsByColumn: []*issuetrackingv1.StatusIDList{{StatusIds: []string{"1", "2"}}, {StatusIds: []string{"3"}}},
			}, nil
		},
	}
	r := NewRegistry()
	registerJiraChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "jira.getProjectStatusOrder", argsJSON(t, map[string]any{"projectIdOrKey": "PROJ"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("want map result, got %T", result)
	}
	cols, ok := out["statusIdsByColumn"].([][]string)
	if !ok || len(cols) != 2 || len(cols[0]) != 2 || cols[1][0] != "3" {
		t.Fatalf("unexpected columns: %+v", out["statusIdsByColumn"])
	}
	if gotReq.GetProjectIdOrKey() != "PROJ" {
		t.Errorf("expected projectIdOrKey=PROJ, got %q", gotReq.GetProjectIdOrKey())
	}
}
