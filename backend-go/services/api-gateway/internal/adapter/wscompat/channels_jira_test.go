package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// fakeIssueTrackingClient is shared by channels_jira_test.go and
// channels_linear_test.go (both relay to the same IssueTrackingService) —
// embeds the generated interface so only the methods a given test actually
// exercises need a func field.
type fakeIssueTrackingClient struct {
	issuetrackingv1.IssueTrackingServiceClient

	getConnectionStatusFunc func(ctx context.Context, in *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error)
	createIssueFunc         func(ctx context.Context, in *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error)
	listIssuesFunc          func(ctx context.Context, in *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error)
	getIssueFunc            func(ctx context.Context, in *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error)
	updateIssueFunc         func(ctx context.Context, in *issuetrackingv1.UpdateIssueRequest) (*issuetrackingv1.Issue, error)
	addIssueCommentFunc     func(ctx context.Context, in *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error)
	listIssueCommentsFunc   func(ctx context.Context, in *issuetrackingv1.ListIssueCommentsRequest) (*issuetrackingv1.ListIssueCommentsResponse, error)
	listProjectsFunc        func(ctx context.Context, in *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error)
	listCreateFieldsFunc    func(ctx context.Context, in *issuetrackingv1.ListCreateFieldsRequest) (*issuetrackingv1.ListCreateFieldsResponse, error)
	listTeamsFunc           func(ctx context.Context, in *issuetrackingv1.ListTeamsRequest) (*issuetrackingv1.ListTeamsResponse, error)
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
