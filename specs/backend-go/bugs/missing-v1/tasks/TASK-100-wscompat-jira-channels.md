# TASK-100: Wire all 19 `jira.*` channels in `wscompat`

**From Solution:** SOL-015
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_jira.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-096, TASK-097, TASK-098, TASK-099
**Status:** `[partial]` — implemented as a standalone file registering into `channels_issuetracking_orchestration.go` (per cross-group convention, `channels.go` untouched). Worktree `agent-a412325f0d1276bb5`, committed as `c29ca9e6a`. **Integration note:** needs `registerIssueTrackingOrchestrationChannels(r, issueTrackingClient, orchestrationClient, infraFleetClient)` added to `RegisterRealChannels`/`main.go` — all 3 clients already dialed there.

---

## Context

Follows `channels.go`'s established `register<Namespace>Channels(r, client)`
convention (`registerDevServerChannels`, `registerGitChannels`) — one new
file, one `Registry.Register` call per channel, `decodeArg[T](args, 0)` to
pull typed args, `rpcTimeout`-scoped gRPC call, a view-translation helper
mapping the provider-agnostic proto response onto the exact field names
`frontend/src/shared/jira-types.ts` expects (`siteUrl` not `url`, `key` not
`provider_issue_id`, `sites` not `workspaces`, etc.) — per
`03-clean-architecture-guidelines.md`'s "adapter translates wire format"
rule, this translation belongs here, not inside `issue-tracking-service`.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_jira.go`

```go
// jira.* channel handlers — relay to issue-tracking-service's
// IssueTrackingService (extended in TASK-096..099), always with
// Provider: ISSUE_PROVIDER_JIRA. View types below mirror
// frontend/src/shared/jira-types.ts field-for-field; see that file before
// changing a field name here.
package wscompat

import (
	"context"
	"encoding/json"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// ── view types (frontend/src/shared/jira-types.ts) ─────────────────────────

type jiraViewerView struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type jiraSiteView struct {
	ID          string `json:"id"`
	SiteURL     string `json:"siteUrl"`
	DisplayName string `json:"displayName"`
	AccountID   string `json:"accountId"`
}

type jiraConnectionStatusView struct {
	Connected       bool           `json:"connected"`
	Viewer          *jiraViewerView `json:"viewer"`
	Sites           []jiraSiteView `json:"sites,omitempty"`
	ActiveSiteID    string         `json:"activeSiteId,omitempty"`
	SelectedSiteID  string         `json:"selectedSiteId,omitempty"`
	CredentialError string         `json:"credentialError,omitempty"`
}

func toJiraConnectionStatusView(s *issuetrackingv1.ConnectionStatus) jiraConnectionStatusView {
	out := jiraConnectionStatusView{
		Connected: s.GetConnected(), ActiveSiteID: s.GetActiveWorkspaceId(),
		SelectedSiteID: s.GetSelectedWorkspaceId(), CredentialError: s.GetCredentialError(),
	}
	if s.GetConnected() {
		out.Viewer = &jiraViewerView{AccountID: s.GetViewerId(), DisplayName: s.GetViewerDisplayName(), Email: s.GetViewerEmail()}
	}
	for _, w := range s.GetWorkspaces() {
		out.Sites = append(out.Sites, jiraSiteView{ID: w.GetId(), SiteURL: w.GetUrl(), DisplayName: w.GetName()})
	}
	return out
}

type jiraProjectView struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type jiraIssueTypeView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type jiraUserView struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

type jiraPriorityView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraStatusView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraIssueView struct {
	ID        string             `json:"id"`
	Key       string             `json:"key"`
	Title     string             `json:"title"`
	Description string           `json:"description,omitempty"`
	URL       string             `json:"url"`
	Project   *jiraProjectView   `json:"project,omitempty"`
	IssueType *jiraIssueTypeView `json:"issueType,omitempty"`
	Status    jiraStatusView     `json:"status"`
	Labels    []string           `json:"labels"`
	Assignee  *jiraUserView      `json:"assignee,omitempty"`
	Reporter  *jiraUserView      `json:"reporter,omitempty"`
	Priority  *jiraPriorityView  `json:"priority,omitempty"`
}

func toJiraIssueView(i *issuetrackingv1.Issue) jiraIssueView {
	v := jiraIssueView{
		ID: i.GetId(), Key: i.GetKey(), Title: i.GetTitle(),
		Description: i.GetDescriptionMarkdown(), URL: i.GetUrl(),
		Labels: i.GetLabels(), Status: jiraStatusView{Name: i.GetState()},
	}
	if p := i.GetProject(); p != nil {
		v.Project = &jiraProjectView{ID: p.GetId(), Key: p.GetKey(), Name: p.GetName()}
	}
	if it := i.GetIssueType(); it != nil {
		v.IssueType = &jiraIssueTypeView{ID: it.GetId(), Name: it.GetName(), Subtask: it.GetSubtask()}
	}
	if a := i.GetAssignee(); a != nil {
		v.Assignee = &jiraUserView{AccountID: a.GetId(), DisplayName: a.GetDisplayName(), Email: a.GetEmail()}
	}
	if r := i.GetReporter(); r != nil {
		v.Reporter = &jiraUserView{AccountID: r.GetId(), DisplayName: r.GetDisplayName(), Email: r.GetEmail()}
	}
	if pr := i.GetPriority(); pr != nil {
		v.Priority = &jiraPriorityView{ID: pr.GetId(), Name: pr.GetName()}
	}
	return v
}

type jiraCommentView struct {
	ID   string        `json:"id"`
	Body string        `json:"body"`
	User *jiraUserView `json:"user,omitempty"`
}

func toJiraCommentView(c *issuetrackingv1.IssueComment) jiraCommentView {
	v := jiraCommentView{ID: c.GetId(), Body: c.GetBodyMarkdown()}
	if a := c.GetAuthor(); a != nil {
		v.User = &jiraUserView{AccountID: a.GetId(), DisplayName: a.GetDisplayName(), Email: a.GetEmail()}
	}
	return v
}

// ── registerJiraChannels ────────────────────────────────────────────────

func registerJiraChannels(r *Registry, client issuetrackingv1.IssueTrackingServiceClient) {
	r.Register("jira.status", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetConnectionStatus(rpcCtx, &issuetrackingv1.GetConnectionStatusRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
		})
		if err != nil {
			return nil, err
		}
		return toJiraConnectionStatusView(resp), nil
	})

	r.Register("jira.connect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type connectArgs struct {
			SiteURL  string `json:"siteUrl"`
			Email    string `json:"email"`
			APIToken string `json:"apiToken"`
		}
		in, err := decodeArg[connectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.Connect(rpcCtx, &issuetrackingv1.ConnectRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
			SiteUrl: in.SiteURL, Email: in.Email, Token: in.APIToken,
		})
		if err != nil {
			// jiraConnect's return type is a discriminated union
			// ({ok:true,viewer}|{ok:false,error}), not a thrown error — the
			// frontend call site (runtime-jira-client.ts) expects a
			// resolved value, not a rejected promise, on auth failure.
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "viewer": jiraViewerView{
			AccountID: resp.GetViewerId(), DisplayName: resp.GetViewerDisplayName(), Email: resp.GetViewerEmail(),
		}}, nil
	})

	r.Register("jira.disconnect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type disconnectArgs struct {
			SiteID string `json:"siteId"`
		}
		in, _ := decodeArg[disconnectArgs](args, 0) // siteId optional — jiraDisconnect(undefined) disconnects all
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err := client.Disconnect(rpcCtx, &issuetrackingv1.DisconnectRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, WorkspaceId: in.SiteID,
		})
		return nil, err
	})

	r.Register("jira.selectSite", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type selectSiteArgs struct {
			SiteID string `json:"siteId"`
		}
		in, err := decodeArg[selectSiteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SelectWorkspace(rpcCtx, &issuetrackingv1.SelectWorkspaceRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		return toJiraConnectionStatusView(resp), nil
	})

	r.Register("jira.testConnection", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type testConnArgs struct {
			SiteID string `json:"siteId"`
		}
		in, _ := decodeArg[testConnArgs](args, 0)
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.TestConnection(rpcCtx, &issuetrackingv1.TestConnectionRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		if !resp.GetOk() {
			return map[string]any{"ok": false, "error": resp.GetError()}, nil
		}
		return map[string]any{"ok": true, "viewer": jiraViewerView{}}, nil
	})

	r.Register("jira.searchIssues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type searchArgs struct {
			JQL    string `json:"jql"`
			Limit  int32  `json:"limit"`
			SiteID string `json:"siteId"`
		}
		in, err := decodeArg[searchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SearchIssues(rpcCtx, &issuetrackingv1.SearchIssuesRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, Query: in.JQL, Limit: in.Limit, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraIssueView, 0, len(resp.GetIssues()))
		for _, i := range resp.GetIssues() {
			views = append(views, toJiraIssueView(i))
		}
		return views, nil
	})

	r.Register("jira.listIssues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectKey string `json:"projectKey"`
			Filter     json.RawMessage `json:"filter"`
			Limit      int32  `json:"limit"`
			SiteID     string `json:"siteId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssues(rpcCtx, &issuetrackingv1.ListIssuesRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, ProjectKey: in.ProjectKey,
			FilterJson: string(in.Filter), Limit: in.Limit, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraIssueView, 0, len(resp.GetIssues()))
		for _, i := range resp.GetIssues() {
			views = append(views, toJiraIssueView(i))
		}
		return views, nil
	})

	r.Register("jira.getIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			IssueIDOrKey string `json:"issueIdOrKey"`
			SiteID       string `json:"siteId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetIssue(rpcCtx, &issuetrackingv1.GetIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, IssueId: in.IssueIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		return toJiraIssueView(resp), nil
	})

	r.Register("jira.createIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			ProjectKey string   `json:"projectKey"`
			Title      string   `json:"title"`
			Description string  `json:"description"`
			IssueType  string   `json:"issueType"`
			AssigneeID string   `json:"assigneeId"`
			PriorityID string   `json:"priorityId"`
			LabelIDs   []string `json:"labelIds"`
			SiteID     string   `json:"siteId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateIssue(rpcCtx, &issuetrackingv1.CreateIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
			ProjectKey: in.ProjectKey, Title: in.Title, Description: in.Description,
			IssueTypeId: in.IssueType, AssigneeId: in.AssigneeID, PriorityId: in.PriorityID,
			LabelIds: in.LabelIDs, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil // JiraMutationResult shape
		}
		return map[string]any{"ok": true, "issue": toJiraIssueView(resp.GetIssue())}, nil
	})

	r.Register("jira.updateIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			IssueIDOrKey string   `json:"issueIdOrKey"`
			Title        string   `json:"title"`
			Description  string   `json:"description"`
			AssigneeID   string   `json:"assigneeId"`
			PriorityID   string   `json:"priorityId"`
			LabelIDs     []string `json:"labelIds"`
			TransitionID string   `json:"transitionId"`
			SiteID       string   `json:"siteId"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssue(rpcCtx, &issuetrackingv1.UpdateIssueRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, IssueId: in.IssueIDOrKey,
			Title: in.Title, Description: in.Description, AssigneeId: in.AssigneeID,
			PriorityId: in.PriorityID, LabelIds: in.LabelIDs, WorkflowStateId: in.TransitionID, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "issue": toJiraIssueView(resp)}, nil
	})

	r.Register("jira.addIssueComment", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addCommentArgs struct {
			IssueIDOrKey string `json:"issueIdOrKey"`
			Body         string `json:"body"`
			SiteID       string `json:"siteId"`
		}
		in, err := decodeArg[addCommentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.AddIssueComment(rpcCtx, &issuetrackingv1.AddIssueCommentRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, IssueId: in.IssueIDOrKey, BodyMarkdown: in.Body, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return map[string]any{"ok": false, "error": err.Error()}, nil // JiraCommentResult shape
		}
		return map[string]any{"ok": true, "id": resp.GetId()}, nil
	})

	r.Register("jira.issueComments", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentsArgs struct {
			IssueIDOrKey string `json:"issueIdOrKey"`
			SiteID       string `json:"siteId"`
		}
		in, err := decodeArg[commentsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssueComments(rpcCtx, &issuetrackingv1.ListIssueCommentsRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, IssueId: in.IssueIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraCommentView, 0, len(resp.GetComments()))
		for _, c := range resp.GetComments() {
			views = append(views, toJiraCommentView(c))
		}
		return views, nil
	})

	r.Register("jira.listProjects", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listProjectsArgs struct {
			SiteID string `json:"siteId"`
		}
		in, _ := decodeArg[listProjectsArgs](args, 0)
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListProjects(rpcCtx, &issuetrackingv1.ListProjectsRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraProjectView, 0, len(resp.GetProjects()))
		for _, p := range resp.GetProjects() {
			views = append(views, jiraProjectView{ID: p.GetId(), Key: p.GetKey(), Name: p.GetName()})
		}
		return views, nil
	})

	r.Register("jira.listIssueTypes", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listTypesArgs struct {
			ProjectIDOrKey string `json:"projectIdOrKey"`
			SiteID         string `json:"siteId"`
		}
		in, err := decodeArg[listTypesArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssueTypes(rpcCtx, &issuetrackingv1.ListIssueTypesRequest{
			ProjectIdOrKey: in.ProjectIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraIssueTypeView, 0, len(resp.GetIssueTypes()))
		for _, t := range resp.GetIssueTypes() {
			views = append(views, jiraIssueTypeView{ID: t.GetId(), Name: t.GetName(), Subtask: t.GetSubtask()})
		}
		return views, nil
	})

	r.Register("jira.listCreateFields", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listFieldsArgs struct {
			ProjectIDOrKey string `json:"projectIdOrKey"`
			IssueTypeID    string `json:"issueTypeId"`
			SiteID         string `json:"siteId"`
		}
		in, err := decodeArg[listFieldsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListCreateFields(rpcCtx, &issuetrackingv1.ListCreateFieldsRequest{
			ProjectIdOrKey: in.ProjectIDOrKey, IssueTypeId: in.IssueTypeID, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetFields(), nil // field names already match JiraCreateField via protojson-adjacent shape; see Test plan note
	})

	r.Register("jira.listAssignableUsers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listUsersArgs struct {
			ProjectIDOrKey string `json:"projectIdOrKey"`
			IssueIDOrKey   string `json:"issueIdOrKey"`
			SiteID         string `json:"siteId"`
		}
		in, err := decodeArg[listUsersArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListAssignableUsers(rpcCtx, &issuetrackingv1.ListAssignableUsersRequest{
			Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
			ProjectIdOrKey: in.ProjectIDOrKey, IssueId: in.IssueIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		views := make([]jiraUserView, 0, len(resp.GetUsers()))
		for _, u := range resp.GetUsers() {
			views = append(views, jiraUserView{AccountID: u.GetId(), DisplayName: u.GetDisplayName(), Email: u.GetEmail()})
		}
		return views, nil
	})

	r.Register("jira.listPriorities", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listPrioritiesArgs struct {
			SiteID string `json:"siteId"`
		}
		in, _ := decodeArg[listPrioritiesArgs](args, 0)
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListPriorities(rpcCtx, &issuetrackingv1.ListPrioritiesRequest{WorkspaceId: in.SiteID})
		if err != nil {
			return nil, err
		}
		views := make([]jiraPriorityView, 0, len(resp.GetPriorities()))
		for _, p := range resp.GetPriorities() {
			views = append(views, jiraPriorityView{ID: p.GetId(), Name: p.GetName()})
		}
		return views, nil
	})

	r.Register("jira.listTransitions", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listTransitionsArgs struct {
			IssueIDOrKey string `json:"issueIdOrKey"`
			SiteID       string `json:"siteId"`
		}
		in, err := decodeArg[listTransitionsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListTransitions(rpcCtx, &issuetrackingv1.ListTransitionsRequest{
			IssueId: in.IssueIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTransitions(), nil
	})

	r.Register("jira.getProjectStatusOrder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type statusOrderArgs struct {
			ProjectIDOrKey string `json:"projectIdOrKey"`
			SiteID         string `json:"siteId"`
		}
		in, err := decodeArg[statusOrderArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetProjectStatusOrder(rpcCtx, &issuetrackingv1.GetProjectStatusOrderRequest{
			ProjectIdOrKey: in.ProjectIDOrKey, WorkspaceId: in.SiteID,
		})
		if err != nil {
			return nil, err
		}
		cols := make([][]string, 0, len(resp.GetStatusIdsByColumn()))
		for _, c := range resp.GetStatusIdsByColumn() {
			cols = append(cols, c.GetStatusIds())
		}
		return map[string]any{"statusIdsByColumn": cols}, nil
	})
}
```

Channel-count check: 19 `r.Register("jira....` calls above — matches
BUG-015's 19-method list exactly (status, connect, disconnect, selectSite,
testConnection, searchIssues, listIssues, getIssue, createIssue,
updateIssue, addIssueComment, issueComments, listProjects, listIssueTypes,
listCreateFields, listAssignableUsers, listPriorities, listTransitions,
getProjectStatusOrder).

### `channels.go` — wire `issueTrackingClient` into `RegisterRealChannels`

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
	issueTrackingClient issuetrackingv1.IssueTrackingServiceClient, // NEW
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
	registerJiraChannels(r, issueTrackingClient) // NEW
}
```

Add `issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"`
to `channels.go`'s import block.

### `cmd/server/main.go` — pass the already-dialed client

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter, issueTrackingClient)
```

`issueTrackingClient` is already dialed at line ~175 for the `/v1/issues`
REST routes — no new dial.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
```

Expected: clean build. `wscompat.RegisterRealChannels`'s new parameter
means every other call site (there is exactly one, `main.go`) must be
updated in the same commit — verify with:

```bash
grep -rn "RegisterRealChannels(" services/api-gateway
```
