package wscompat

import (
	"context"
	"errors"
	"testing"

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
