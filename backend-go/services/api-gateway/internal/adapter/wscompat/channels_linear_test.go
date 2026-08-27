package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

func TestLinearListIssuesChannel_ReturnsItemsHasMoreEnvelope(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssuesFunc: func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
			return &issuetrackingv1.ListIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "ENG-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.listIssues", argsJSON(t, map[string]any{"teamKey": "ENG"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	envelope, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("want map envelope, got %T", result)
	}
	if _, hasItems := envelope["items"]; !hasItems {
		t.Fatal("linear.listIssues must return an {items, hasMore} envelope, not a bare array — regression guard vs jira.listIssues's bare-array shape")
	}
	if envelope["hasMore"] != false {
		t.Errorf("want hasMore=false, got %v", envelope["hasMore"])
	}
}

func TestLinearStatusChannel_UsesLinearProvider(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getConnectionStatusFunc: func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
				t.Fatalf("want linear provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.ConnectionStatus{Connected: true}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearConnectionStatusView)
	if !ok || !view.Connected {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearCreateIssueChannel_MapsTeamIDAndStateID(t *testing.T) {
	var gotReq *issuetrackingv1.CreateIssueRequest
	fake := &fakeIssueTrackingClient{
		createIssueFunc: func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
			gotReq = in
			return &issuetrackingv1.CreateIssueResponse{Issue: &issuetrackingv1.Issue{Id: "1", Key: "ENG-2", Title: in.GetTitle()}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	args := argsJSON(t, map[string]any{"teamId": "team-1", "title": "New task", "stateId": "state-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.createIssue", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTeamId() != "team-1" || gotReq.GetStateId() != "state-1" {
		t.Fatalf("request fields not mapped correctly: %+v", gotReq)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
		t.Fatalf("want linear provider, got %v", gotReq.GetProvider())
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true {
		t.Fatalf("unexpected result shape: %+v", result)
	}
}

func TestLinearListTeamsChannel_NeverReturnsJiraProjectRows(t *testing.T) {
	// Regression guard for SOL-016's "no false unification" design: a fake
	// client that would answer ListProjects with Jira-shaped data must
	// never be reachable from linear.listTeams — registerLinearChannels's
	// linear.listTeams handler only ever calls client.ListTeams.
	fake := &fakeIssueTrackingClient{
		listTeamsFunc: func(ctx context.Context, in *issuetrackingv1.ListTeamsRequest) (*issuetrackingv1.ListTeamsResponse, error) {
			return &issuetrackingv1.ListTeamsResponse{Teams: []*issuetrackingv1.Team{{Id: "team-1", Name: "Engineering", Key: "ENG"}}}, nil
		},
		listProjectsFunc: func(ctx context.Context, in *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error) {
			t.Fatal("linear.listTeams must never call ListProjects")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.listTeams", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearTeamView)
	if !ok || len(views) != 1 || views[0].Key != "ENG" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearUpdateIssueChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("state transition rejected")
	fake := &fakeIssueTrackingClient{
		updateIssueFunc: func(ctx context.Context, in *issuetrackingv1.UpdateIssueRequest) (*issuetrackingv1.Issue, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.updateIssue", argsJSON(t, map[string]any{"issueId": "ENG-1", "stateId": "state-2"}))
	if err != nil {
		t.Fatalf("linear.updateIssue must resolve, not reject, on a provider error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestLinearTestConnectionChannel_ReturnsOKFalseOnProviderRejection(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		testConnectionFunc: func(ctx context.Context, in *issuetrackingv1.TestConnectionRequest) (*issuetrackingv1.TestConnectionResult, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
				t.Fatalf("want linear provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.TestConnectionResult{Ok: false, Error: "invalid token"}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.testConnection", argsJSON(t, map[string]any{"workspaceId": "ws-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false || out["error"] != "invalid token" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearConnectChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.ConnectRequest
	fake := &fakeIssueTrackingClient{
		connectFunc: func(ctx context.Context, in *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
			gotReq = in
			return &issuetrackingv1.ConnectionStatus{ViewerId: "usr-1", ViewerDisplayName: "Ada", ViewerEmail: "ada@x.com"}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "linear.connect", argsJSON(t, map[string]any{"apiToken": "tok"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true {
		t.Fatalf("unexpected result shape: %+v", result)
	}
	viewer, ok := out["viewer"].(linearViewerView)
	if !ok || viewer.ID != "usr-1" {
		t.Fatalf("unexpected viewer: %+v", out["viewer"])
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
		t.Errorf("expected linear provider, got %v", gotReq.GetProvider())
	}
}

func TestLinearConnectChannel_ProviderErrorReturnsOKFalse(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		connectFunc: func(ctx context.Context, in *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
			return nil, errors.New("invalid API key")
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.connect", argsJSON(t, map[string]any{"apiToken": "bad"}))
	if err != nil {
		t.Fatalf("linear.connect must resolve, not reject, on an auth failure: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestLinearDisconnectChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.DisconnectRequest
	fake := &fakeIssueTrackingClient{
		disconnectFunc: func(ctx context.Context, in *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.disconnect", argsJSON(t, map[string]any{"workspaceId": "ws-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR || gotReq.GetWorkspaceId() != "ws-1" {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
}

func TestLinearSelectWorkspaceChannel_Success(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		selectWorkspaceFunc: func(ctx context.Context, in *issuetrackingv1.SelectWorkspaceRequest) (*issuetrackingv1.ConnectionStatus, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
				t.Fatalf("want linear provider, got %v", in.GetProvider())
			}
			return &issuetrackingv1.ConnectionStatus{Connected: true, SelectedWorkspaceId: in.GetWorkspaceId()}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.selectWorkspace", argsJSON(t, map[string]any{"workspaceId": "ws-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearConnectionStatusView)
	if !ok || view.SelectedWorkspaceID != "ws-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearSearchIssuesChannel_MapsFields(t *testing.T) {
	var gotReq *issuetrackingv1.SearchIssuesRequest
	fake := &fakeIssueTrackingClient{
		searchIssuesFunc: func(ctx context.Context, in *issuetrackingv1.SearchIssuesRequest) (*issuetrackingv1.SearchIssuesResponse, error) {
			gotReq = in
			return &issuetrackingv1.SearchIssuesResponse{Issues: []*issuetrackingv1.Issue{{Id: "1", Key: "ENG-1", Title: "Bug"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.searchIssues", argsJSON(t, map[string]any{"query": "bug", "limit": 5}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearIssueView)
	if !ok || len(views) != 1 || views[0].Identifier != "ENG-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetQuery() != "bug" || gotReq.GetLimit() != 5 {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR {
		t.Errorf("expected linear provider, got %v", gotReq.GetProvider())
	}
}

func TestLinearGetIssueChannel_MapsFields(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getIssueFunc: func(ctx context.Context, in *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error) {
			if in.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR || in.GetIssueId() != "issue-1" {
				t.Fatalf("unexpected request: %+v", in)
			}
			return &issuetrackingv1.Issue{Id: "issue-1", Key: "ENG-9", Title: "Task"}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.getIssue", argsJSON(t, map[string]any{"issueId": "issue-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearIssueView)
	if !ok || view.Identifier != "ENG-9" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearAddIssueCommentChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.AddIssueCommentRequest
	fake := &fakeIssueTrackingClient{
		addIssueCommentFunc: func(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error) {
			gotReq = in
			return &issuetrackingv1.IssueComment{Id: "c1"}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.addIssueComment", argsJSON(t, map[string]any{"issueId": "issue-1", "body": "a comment"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != true || out["id"] != "c1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR || gotReq.GetBodyMarkdown() != "a comment" {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
}

func TestLinearAddIssueCommentChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("issue not found")
	fake := &fakeIssueTrackingClient{
		addIssueCommentFunc: func(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.addIssueComment", argsJSON(t, map[string]any{"issueId": "issue-1", "body": "a comment"}))
	if err != nil {
		t.Fatalf("linear.addIssueComment must resolve, not reject, on a provider error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["ok"] != false {
		t.Fatalf("want {ok:false, error:...}, got %+v", result)
	}
}

func TestLinearIssueCommentsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listIssueCommentsFunc: func(ctx context.Context, in *issuetrackingv1.ListIssueCommentsRequest) (*issuetrackingv1.ListIssueCommentsResponse, error) {
			return &issuetrackingv1.ListIssueCommentsResponse{Comments: []*issuetrackingv1.IssueComment{{Id: "c1", BodyMarkdown: "hi"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.issueComments", argsJSON(t, map[string]any{"issueId": "issue-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearCommentView)
	if !ok || len(views) != 1 || views[0].Body != "hi" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearCreateProjectChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.CreateProjectRequest
	fake := &fakeIssueTrackingClient{
		createProjectFunc: func(ctx context.Context, in *issuetrackingv1.CreateProjectRequest) (*issuetrackingv1.Project, error) {
			gotReq = in
			return &issuetrackingv1.Project{Id: "p1", Name: in.GetName()}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.createProject", argsJSON(t, map[string]any{"teamId": "team-1", "name": "Roadmap", "workspaceId": "ws-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearProjectView)
	if !ok || view.Name != "Roadmap" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetTeamId() != "team-1" || gotReq.GetWorkspaceId() != "ws-1" {
		t.Errorf("request fields not mapped correctly: %+v", gotReq)
	}
}

func TestLinearGetProjectChannel_Success(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		getProjectFunc: func(ctx context.Context, in *issuetrackingv1.GetProjectRequest) (*issuetrackingv1.Project, error) {
			if in.GetProjectId() != "p1" {
				t.Fatalf("want projectId=p1, got %q", in.GetProjectId())
			}
			return &issuetrackingv1.Project{Id: "p1", Name: "Roadmap"}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.getProject", argsJSON(t, map[string]any{"projectId": "p1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearProjectView)
	if !ok || view.Name != "Roadmap" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearTeamStatesChannel_ReturnsViews(t *testing.T) {
	var gotReq *issuetrackingv1.ListWorkflowStatesRequest
	fake := &fakeIssueTrackingClient{
		listWorkflowStatesFunc: func(ctx context.Context, in *issuetrackingv1.ListWorkflowStatesRequest) (*issuetrackingv1.ListWorkflowStatesResponse, error) {
			gotReq = in
			return &issuetrackingv1.ListWorkflowStatesResponse{States: []*issuetrackingv1.WorkflowState{{Id: "s1", Name: "In Progress", Category: "started"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.teamStates", argsJSON(t, map[string]any{"teamId": "team-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearStateView)
	if !ok || len(views) != 1 || views[0].Name != "In Progress" || views[0].Type != "started" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetTeamId() != "team-1" {
		t.Errorf("expected teamId=team-1, got %q", gotReq.GetTeamId())
	}
}

func TestLinearTeamLabelsChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listTeamLabelsFunc: func(ctx context.Context, in *issuetrackingv1.ListTeamLabelsRequest) (*issuetrackingv1.ListTeamLabelsResponse, error) {
			return &issuetrackingv1.ListTeamLabelsResponse{Labels: []*issuetrackingv1.Label{{Id: "l1", Name: "bug", Color: "red"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.teamLabels", argsJSON(t, map[string]any{"teamId": "team-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearLabelView)
	if !ok || len(views) != 1 || views[0].Name != "bug" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearTeamMembersChannel_ReturnsViews(t *testing.T) {
	fake := &fakeIssueTrackingClient{
		listTeamMembersFunc: func(ctx context.Context, in *issuetrackingv1.ListTeamMembersRequest) (*issuetrackingv1.ListTeamMembersResponse, error) {
			return &issuetrackingv1.ListTeamMembersResponse{Members: []*issuetrackingv1.Member{{Id: "m1", DisplayName: "Ada"}}}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.teamMembers", argsJSON(t, map[string]any{"teamId": "team-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]linearMemberView)
	if !ok || len(views) != 1 || views[0].Name != "Ada" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLinearGetCustomViewChannel_Success(t *testing.T) {
	var gotReq *issuetrackingv1.GetCustomViewRequest
	fake := &fakeIssueTrackingClient{
		getCustomViewFunc: func(ctx context.Context, in *issuetrackingv1.GetCustomViewRequest) (*issuetrackingv1.CustomView, error) {
			gotReq = in
			return &issuetrackingv1.CustomView{Id: in.GetViewId(), Name: "My View", Model: in.GetModel()}, nil
		},
	}
	r := NewRegistry()
	registerLinearChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "linear.getCustomView", argsJSON(t, map[string]any{"viewId": "view-1", "model": "issue"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(linearCustomViewView)
	if !ok || view.ID != "view-1" || view.Model != "issue" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetViewId() != "view-1" {
		t.Errorf("expected viewId=view-1, got %q", gotReq.GetViewId())
	}
}
