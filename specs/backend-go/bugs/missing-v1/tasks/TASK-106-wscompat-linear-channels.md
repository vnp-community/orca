# TASK-106: Wire all 19 `linear.*` channels in `wscompat`

**From Solution:** SOL-016
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_linear.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-100, TASK-104, TASK-105
**Status:** `[partial]` — all 19 `linear.*` channel handlers implemented in new file `channels_linear.go`, reusing the same `issueTrackingClient`. `go build`/`go vet`/`go test` clean. NOT wired into `channels.go`'s `RegisterRealChannels` — same "do not edit channels.go" constraint as TASK-100; registered via `registerIssueTrackingOrchestrationChannels` in `channels_issuetracking_orchestration.go` instead. Regression grep guard from this task's Verify step confirmed by inspection: `linear.listTeams` calls `client.ListTeams`, `jira.listProjects` calls `client.ListProjects` — no cross-wiring.

---

## Context

Same `register<Namespace>Channels(r, client)` convention TASK-100 used for
`jira.*`, reusing the SAME `issueTrackingClient` (no new dial). Per
SOL-016, `linear.listIssues` wraps its response in a
`{items, hasMore}` envelope (`LinearCollectionResult<T>`, matching
`normalizeLinearIssueCollectionResult` in
`runtime-linear-client.ts:247`) where `jira.listIssues` returns a bare
array — this is a `wscompat`-layer response-shape difference only; both
route through the same `ListIssuesResponse` proto message.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_linear.go`

```go
// linear.* channel handlers — relay to issue-tracking-service's
// IssueTrackingService, always with Provider: ISSUE_PROVIDER_LINEAR. View
// types mirror frontend/src/shared/types.ts's LinearWorkspace/LinearIssue/
// LinearTeam/LinearCustomViewSummary/LinearComment field-for-field.
package wscompat

import (
	"context"
	"encoding/json"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// ── view types (frontend/src/shared/types.ts) ───────────────────────────

type linearViewerView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type linearWorkspaceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearConnectionStatusView struct {
	Connected           bool                  `json:"connected"`
	Viewer              *linearViewerView     `json:"viewer"`
	Workspaces          []linearWorkspaceView `json:"workspaces,omitempty"`
	ActiveWorkspaceID   string                `json:"activeWorkspaceId,omitempty"`
	SelectedWorkspaceID string                `json:"selectedWorkspaceId,omitempty"`
	CredentialError     string                `json:"credentialError,omitempty"`
}

func toLinearConnectionStatusView(s *issuetrackingv1.ConnectionStatus) linearConnectionStatusView {
	out := linearConnectionStatusView{
		Connected: s.GetConnected(), ActiveWorkspaceID: s.GetActiveWorkspaceId(),
		SelectedWorkspaceID: s.GetSelectedWorkspaceId(), CredentialError: s.GetCredentialError(),
	}
	if s.GetConnected() {
		out.Viewer = &linearViewerView{ID: s.GetViewerId(), Name: s.GetViewerDisplayName(), Email: s.GetViewerEmail()}
	}
	for _, w := range s.GetWorkspaces() {
		out.Workspaces = append(out.Workspaces, linearWorkspaceView{ID: w.GetId(), Name: w.GetName()})
	}
	return out
}

type linearTeamRefView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type linearStateView struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color,omitempty"`
}

type linearUserView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type linearIssueView struct {
	ID          string             `json:"id"`
	Identifier  string             `json:"identifier"`
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	URL         string             `json:"url"`
	Team        *linearTeamRefView `json:"team,omitempty"`
	State       linearStateView    `json:"state"`
	Labels      []string           `json:"labels"`
	Assignee    *linearUserView    `json:"assignee,omitempty"`
}

func toLinearIssueView(i *issuetrackingv1.Issue) linearIssueView {
	v := linearIssueView{
		ID: i.GetId(), Identifier: i.GetKey(), Title: i.GetTitle(),
		Description: i.GetDescriptionMarkdown(), URL: i.GetUrl(),
		Labels: i.GetLabels(), State: linearStateView{Name: i.GetState()},
	}
	if p := i.GetProject(); p != nil {
		v.Team = &linearTeamRefView{ID: p.GetId(), Key: p.GetKey(), Name: p.GetName()}
	}
	if a := i.GetAssignee(); a != nil {
		v.Assignee = &linearUserView{ID: a.GetId(), Name: a.GetDisplayName(), Email: a.GetEmail()}
	}
	return v
}

type linearCommentView struct {
	ID   string          `json:"id"`
	Body string          `json:"body"`
	User *linearUserView `json:"user,omitempty"`
}

func toLinearCommentView(c *issuetrackingv1.IssueComment) linearCommentView {
	v := linearCommentView{ID: c.GetId(), Body: c.GetBodyMarkdown()}
	if a := c.GetAuthor(); a != nil {
		v.User = &linearUserView{ID: a.GetId(), Name: a.GetDisplayName(), Email: a.GetEmail()}
	}
	return v
}

type linearTeamView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type linearProjectView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearLabelView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type linearMemberView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearCustomViewView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
}

// ── registerLinearChannels ──────────────────────────────────────────────

func registerLinearChannels(r *Registry, client issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("linear.status", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetConnectionStatus(rpcCtx, &issuetrackingv1.GetConnectionStatusRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
		})
		if err != nil {
			return nil, err
		}
		return toLinearConnectionStatusView(resp), nil
	})

	r.Register("linear.testConnection", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type testConnArgs struct {
			WorkspaceID string `json:"workspaceId"`
		}
		in, _ := decodeArg[testConnArgs](args, 0)
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.TestConnection(rpcCtx, &issuetrackingv1.TestConnectionRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		if !resp.GetOk() {
			return map[string]any{"ok": false, "error": resp.GetError()}, nil
		}
		return map[string]any{"ok": true, "viewer": linearViewerView{}}, nil
	})

	r.Register("linear.connect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type connectArgs struct {
			APIToken string `json:"apiToken"`
		}
		in, err := decodeArg[connectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.Connect(rpcCtx, &issuetrackingv1.ConnectRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, Token: in.APIToken,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "viewer": linearViewerView{
			ID: resp.GetViewerId(), Name: resp.GetViewerDisplayName(), Email: resp.GetViewerEmail(),
		}}, nil
	})

	r.Register("linear.disconnect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type disconnectArgs struct {
			WorkspaceID string `json:"workspaceId"`
		}
		in, _ := decodeArg[disconnectArgs](args, 0)
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err := client.Disconnect(rpcCtx, &issuetrackingv1.DisconnectRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, WorkspaceId: in.WorkspaceID,
		})
		return nil, err
	})

	r.Register("linear.selectWorkspace", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type selectArgs struct {
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[selectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SelectWorkspace(rpcCtx, &issuetrackingv1.SelectWorkspaceRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return toLinearConnectionStatusView(resp), nil
	})

	r.Register("linear.searchIssues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type searchArgs struct {
			Query       string `json:"query"`
			Limit       int32  `json:"limit"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[searchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SearchIssues(rpcCtx, &issuetrackingv1.SearchIssuesRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, Query: in.Query, Limit: in.Limit, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]linearIssueView, 0, len(resp.GetIssues()))
		for _, i := range resp.GetIssues() {
			views = append(views, toLinearIssueView(i))
		}
		return views, nil
	})

	r.Register("linear.listIssues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			TeamKey     string          `json:"teamKey"`
			Filter      json.RawMessage `json:"filter"`
			Limit       int32           `json:"limit"`
			WorkspaceID string          `json:"workspaceId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssues(rpcCtx, &issuetrackingv1.ListIssuesRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, ProjectKey: in.TeamKey,
			FilterJson: string(in.Filter), Limit: in.Limit, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]linearIssueView, 0, len(resp.GetIssues()))
		for _, i := range resp.GetIssues() {
			views = append(views, toLinearIssueView(i))
		}
		// LinearCollectionResult<T> envelope — the one shape difference from
		// jira.listIssues's bare array, per SOL-016's own note. hasMore is
		// always false: ListIssuesResponse carries no pagination cursor yet.
		return map[string]any{"items": views, "hasMore": false}, nil
	})

	r.Register("linear.getIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			IssueID     string `json:"issueId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetIssue(rpcCtx, &issuetrackingv1.GetIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, IssueId: in.IssueID, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return toLinearIssueView(resp), nil
	})

	r.Register("linear.createIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			TeamID      string   `json:"teamId"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			StateID     string   `json:"stateId"`
			AssigneeID  string   `json:"assigneeId"`
			LabelIDs    []string `json:"labelIds"`
			ParentID    string   `json:"parentId"`
			WorkspaceID string   `json:"workspaceId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateIssue(rpcCtx, &issuetrackingv1.CreateIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
			TeamId: in.TeamID, Title: in.Title, Description: in.Description, StateId: in.StateID,
			AssigneeId: in.AssigneeID, LabelIds: in.LabelIDs, ParentIssueId: in.ParentID, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "issue": toLinearIssueView(resp.GetIssue())}, nil
	})

	r.Register("linear.updateIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			IssueID     string   `json:"issueId"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			StateID     string   `json:"stateId"`
			AssigneeID  string   `json:"assigneeId"`
			LabelIDs    []string `json:"labelIds"`
			WorkspaceID string   `json:"workspaceId"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssue(rpcCtx, &issuetrackingv1.UpdateIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, IssueId: in.IssueID,
			Title: in.Title, Description: in.Description, WorkflowStateId: in.StateID,
			AssigneeId: in.AssigneeID, LabelIds: in.LabelIDs, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "issue": toLinearIssueView(resp)}, nil
	})

	r.Register("linear.addIssueComment", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addCommentArgs struct {
			IssueID     string `json:"issueId"`
			Body        string `json:"body"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[addCommentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.AddIssueComment(rpcCtx, &issuetrackingv1.AddIssueCommentRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, IssueId: in.IssueID, BodyMarkdown: in.Body, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "id": resp.GetId()}, nil
	})

	r.Register("linear.issueComments", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentsArgs struct {
			IssueID     string `json:"issueId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[commentsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssueComments(rpcCtx, &issuetrackingv1.ListIssueCommentsRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR, IssueId: in.IssueID, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]linearCommentView, 0, len(resp.GetComments()))
		for _, c := range resp.GetComments() {
			views = append(views, toLinearCommentView(c))
		}
		return views, nil
	})

	r.Register("linear.createProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createProjectArgs struct {
			TeamID      string `json:"teamId"`
			Name        string `json:"name"`
			Description string `json:"description"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[createProjectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateProject(rpcCtx, &issuetrackingv1.CreateProjectRequest{
			WorkspaceId: in.WorkspaceID, TeamId: in.TeamID, Name: in.Name, Description: in.Description,
		})
		if err != nil {
			return nil, err
		}
		return linearProjectView{ID: resp.GetId(), Name: resp.GetName()}, nil
	})

	r.Register("linear.getProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getProjectArgs struct {
			ProjectID   string `json:"projectId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[getProjectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetProject(rpcCtx, &issuetrackingv1.GetProjectRequest{
			ProjectId: in.ProjectID, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return linearProjectView{ID: resp.GetId(), Name: resp.GetName()}, nil
	})

	r.Register("linear.teamStates", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type teamStatesArgs struct {
			TeamID      string `json:"teamId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[teamStatesArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListWorkflowStates(rpcCtx, &issuetrackingv1.ListWorkflowStatesRequest{
			TeamId: in.TeamID, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]linearStateView, 0, len(resp.GetStates()))
		for _, s := range resp.GetStates() {
			views = append(views, linearStateView{Name: s.GetName(), Type: s.GetCategory()})
		}
		return views, nil
	})

	r.Register("linear.listTeams", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listTeamsArgs struct {
			WorkspaceID string `json:"workspaceId"`
		}
		in, _ := decodeArg[listTeamsArgs](args, 0) // workspaceId optional, per runtime-linear-client.ts:365
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListTeams(rpcCtx, &issuetrackingv1.ListTeamsRequest{WorkspaceId: in.WorkspaceID})
		if err != nil {
			return nil, err
		}
		views := make([]linearTeamView, 0, len(resp.GetTeams()))
		for _, t := range resp.GetTeams() {
			views = append(views, linearTeamView{ID: t.GetId(), Name: t.GetName(), Key: t.GetKey()})
		}
		return views, nil
	})

	r.Register("linear.teamLabels", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type teamLabelsArgs struct {
			TeamID      string `json:"teamId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[teamLabelsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListTeamLabels(rpcCtx, &issuetrackingv1.ListTeamLabelsRequest{TeamId: in.TeamID, WorkspaceId: in.WorkspaceID})
		if err != nil {
			return nil, err
		}
		views := make([]linearLabelView, 0, len(resp.GetLabels()))
		for _, l := range resp.GetLabels() {
			views = append(views, linearLabelView{ID: l.GetId(), Name: l.GetName(), Color: l.GetColor()})
		}
		return views, nil
	})

	r.Register("linear.teamMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type teamMembersArgs struct {
			TeamID      string `json:"teamId"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[teamMembersArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListTeamMembers(rpcCtx, &issuetrackingv1.ListTeamMembersRequest{TeamId: in.TeamID, WorkspaceId: in.WorkspaceID})
		if err != nil {
			return nil, err
		}
		views := make([]linearMemberView, 0, len(resp.GetMembers()))
		for _, m := range resp.GetMembers() {
			views = append(views, linearMemberView{ID: m.GetId(), Name: m.GetDisplayName()})
		}
		return views, nil
	})

	r.Register("linear.getCustomView", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getCustomViewArgs struct {
			ViewID      string `json:"viewId"`
			Model       string `json:"model"`
			WorkspaceID string `json:"workspaceId"`
		}
		in, err := decodeArg[getCustomViewArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetCustomView(rpcCtx, &issuetrackingv1.GetCustomViewRequest{
			ViewId: in.ViewID, Model: in.Model, WorkspaceId: in.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return linearCustomViewView{ID: resp.GetId(), Name: resp.GetName(), Model: resp.GetModel()}, nil
	})
}
```

Channel-count check: 19 `r.Register("linear....` calls above — matches
BUG-016's 19-method list (status, testConnection, connect, disconnect,
selectWorkspace, searchIssues, listIssues, getIssue, createIssue,
updateIssue, addIssueComment, issueComments, createProject, getProject,
teamStates, listTeams, teamLabels, teamMembers, getCustomView).

### `channels.go` — wire `registerLinearChannels`

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
	issueTrackingClient issuetrackingv1.IssueTrackingServiceClient,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerJiraChannels(r, issueTrackingClient)
	registerLinearChannels(r, issueTrackingClient) // NEW — same client, no new dial
}
```

No `main.go` change beyond what TASK-100 already did — `issueTrackingClient`
is already threaded through.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
```

Regression guard for the "false unification" SOL-016 explicitly avoided:
grep confirms `linear.listTeams`'s handler calls `client.ListTeams` (not
`client.ListProjects`) and `jira.listProjects`'s handler (TASK-100) calls
`client.ListProjects` (not `client.ListTeams`):

```bash
grep -A3 'r.Register("linear.listTeams"' services/api-gateway/internal/adapter/wscompat/channels_linear.go | grep -q "client.ListTeams(" && echo OK
grep -A3 'r.Register("jira.listProjects"' services/api-gateway/internal/adapter/wscompat/channels_jira.go | grep -q "client.ListProjects(" && echo OK
```
