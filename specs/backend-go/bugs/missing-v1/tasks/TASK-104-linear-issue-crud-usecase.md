# TASK-104: `linear.*` issue-CRUD adapter (`SearchIssues`/`ListIssues`/`GetIssue`/`CreateIssue`/`UpdateIssue`/`AddIssueComment`/`ListIssueComments`) + `CreateProject`/`GetProject`

**From Solution:** SOL-016
**Priority:** P1
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/{usecase,adapter/linear,adapter/jira,adapter/grpc}/*.go`
**Depends on:** TASK-098, TASK-102, TASK-103
**Status:** `[x]` DONE — implemented in worktree `agent-a412325f0d1276bb5` (branch `worktree-agent-a412325f0d1276bb5`), **committed** as `c29ca9e6a`. `go build`/`go vet`/`buf generate`/`buf breaking` clean. Pending merge.

---

## Context

TASK-098 already extended `IssueTrackerProvider`'s issue-CRUD methods and
`jira/client.go`'s implementation; `linear/client.go` still only implements
the old 2-method port (`ListIssues`, `CreateIssue`, positional args) —
this task brings it up to the same 7-method shape, using Linear's GraphQL
API (hand-rolled client, per this package's doc comment) instead of Jira's
REST calls. It also adds `CreateProject`/`GetProject` — shared-concept RPCs
SOL-016 places on the same port (both providers' "project" is a bounded set
of issues with a name/lead/status), wired only by `linear.*` today (no
`jira.createProject` channel exists — see SOL-016's mapping table).

## Changes to make

### 1. `internal/usecase/ports.go` — add `CreateProject`/`GetProject` to `IssueTrackerProvider`

```go
type IssueTrackerProvider interface {
	// ... TASK-098/099's methods, plus:
	CreateProject(ctx context.Context, cred Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error)
	GetProject(ctx context.Context, cred Credential, projectID, workspaceID string) (domain.ProjectRef, error)
}
```

### 2. `internal/usecase/create_project.go`, `get_project.go` (new)

Same provider-parameterized shape as `ListProjects` (TASK-099):

```go
// create_project.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type CreateProjectInput struct {
	Provider    domain.Provider
	TeamID      string // Linear
	Name        string
	Description string
	WorkspaceID string
}

type CreateProject struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewCreateProject(registry ProviderRegistry, credentials CredentialResolver) *CreateProject {
	return &CreateProject{registry: registry, credentials: credentials}
}

func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.ProjectRef, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Name == "" {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_NAME", "name is required", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	project, err := provider.CreateProject(ctx, cred, in.WorkspaceID, in.TeamID, in.Name, in.Description)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREATE_PROJECT_FAILED", "failed to create project", err)
	}
	return project, nil
}
```

`get_project.go` (`GetProjectInput{Provider, ProjectID, WorkspaceID}`,
`provider.GetProject(ctx, cred, in.ProjectID, in.WorkspaceID)`, code
`ISSUETRACKING_GET_PROJECT_FAILED`) follows the identical shape.

### 3. `internal/adapter/linear/client.go` — implement the full port

```go
// ── SearchIssues / ListIssues / GetIssue ────────────────────────────────

func (c *Client) SearchIssues(ctx context.Context, cred usecase.Credential, query string, limit int) ([]domain.Issue, error) {
	variables := map[string]any{}
	if query != "" {
		// Linear's issues() filter has no free-text search argument on the
		// public schema the way Jira's JQL does — this passes query through
		// as a title "contains" filter, the closest equivalent. A richer
		// filter (state/assignee/label) is what filterJSON (ListIssues) is
		// for, not SearchIssues.
		variables["filter"] = map[string]any{"title": map[string]any{"contains": query}}
	}
	if limit > 0 {
		variables["first"] = limit
	}
	var resp graphQLResponse[linearIssuesData]
	if err := c.do(ctx, cred.Token, listIssuesQuery, variables, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: search issues: %s", resp.Errors[0].Message)
	}
	return toRichIssues(resp.Data.Issues.Nodes), nil
}

func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error) {
	variables := map[string]any{}
	if projectKey != "" {
		variables["filter"] = map[string]any{"team": map[string]any{"key": map[string]any{"eq": projectKey}}}
	}
	if limit > 0 {
		variables["first"] = limit
	}
	// filterJSON (LinearIssueFilter-shaped) is not translated to a GraphQL
	// filter here — same documented gap as jira/client.go's ListIssues
	// filterJSON note (TASK-098); a follow-up parses it once a concrete
	// shape is finalized.
	_ = filterJSON
	var resp graphQLResponse[linearIssuesData]
	if err := c.do(ctx, cred.Token, listIssuesQuery, variables, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list issues: %s", resp.Errors[0].Message)
	}
	return toRichIssues(resp.Data.Issues.Nodes), nil
}

const getIssueQuery = `query Issue($id: String!) {
  issue(id: $id) {
    identifier
    title
    description
    url
    state { name }
    team { id name key }
    assignee { id name email }
    labels { nodes { name } }
  }
}`

type linearIssueDetail struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Team struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"team"`
	Assignee *struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"assignee"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

type linearIssueDetailData struct {
	Issue linearIssueDetail `json:"issue"`
}

func toRichIssueDetail(d linearIssueDetail) domain.Issue {
	issue := domain.Issue{
		ID: d.Identifier, ProviderIssueID: d.Identifier, Key: d.Identifier,
		Title: d.Title, DescriptionMarkdown: d.Description, State: d.State.Name,
		WorkflowState: domain.WorkflowState{Name: d.State.Name}, URL: d.URL,
		Project: domain.ProjectRef{ID: d.Team.ID, Key: d.Team.Key, Name: d.Team.Name},
	}
	if d.Assignee != nil {
		issue.Assignee = domain.UserRef{ID: d.Assignee.ID, DisplayName: d.Assignee.Name, Email: d.Assignee.Email}
	}
	for _, l := range d.Labels.Nodes {
		issue.Labels = append(issue.Labels, l.Name)
	}
	return issue
}

func (c *Client) GetIssue(ctx context.Context, cred usecase.Credential, issueID string) (domain.Issue, error) {
	var resp graphQLResponse[linearIssueDetailData]
	if err := c.do(ctx, cred.Token, getIssueQuery, map[string]any{"id": issueID}, &resp); err != nil {
		return domain.Issue{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Issue{}, fmt.Errorf("linear: get issue: %s", resp.Errors[0].Message)
	}
	return toRichIssueDetail(resp.Data.Issue), nil
}

func toRichIssues(nodes []linearIssueNode) []domain.Issue {
	issues := make([]domain.Issue, 0, len(nodes))
	for _, n := range nodes {
		issue, err := domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
		if err != nil {
			continue
		}
		issue.ProviderIssueID = n.Identifier
		issue.Key = n.Identifier
		issues = append(issues, issue)
	}
	return issues
}

// ── CreateIssue ──────────────────────────────────────────────────────────

// CreateIssue uses in.ParentIssueID/team from domain.NewIssueInput.
// ProjectKey (Jira's field) doubles as the Linear team KEY when TeamID
// isn't resolved to a UUID yet — resolveTeamID handles both: a caller
// that already has the team UUID (from ListTeams, TASK-105) can pass it
// directly via a future TeamID-shaped field; this scaffold resolves by
// key, matching the existing resolveTeamID helper.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, in domain.NewIssueInput) (domain.Issue, error) {
	teamID, err := c.resolveTeamID(ctx, cred.Token, in.ProjectKey)
	if err != nil {
		return domain.Issue{}, err
	}
	input := map[string]any{
		"teamId": teamID,
		"title":  in.Title,
	}
	if in.Description != "" {
		input["description"] = in.Description
	}
	if in.AssigneeID != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.PriorityID != "" {
		input["priority"] = in.PriorityID
	}
	if len(in.LabelIDs) > 0 {
		input["labelIds"] = in.LabelIDs
	}
	if in.ParentIssueID != "" {
		input["parentId"] = in.ParentIssueID
	}

	var resp graphQLResponse[linearIssueCreateData]
	if err := c.do(ctx, cred.Token, createIssueMutation, map[string]any{"input": input}, &resp); err != nil {
		return domain.Issue{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Issue{}, fmt.Errorf("linear: create issue: %s", resp.Errors[0].Message)
	}
	if !resp.Data.IssueCreate.Success {
		return domain.Issue{}, fmt.Errorf("linear: issue creation reported failure")
	}
	n := resp.Data.IssueCreate.Issue
	issue, err := domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
	if err != nil {
		return domain.Issue{}, err
	}
	issue.ProviderIssueID = n.Identifier
	issue.Key = n.Identifier
	return issue, nil
}

// ── UpdateIssue ──────────────────────────────────────────────────────────

const updateIssueMutation = `mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
  issueUpdate(id: $id, input: $input) {
    success
    issue { identifier title url state { name } }
  }
}`

func (c *Client) UpdateIssue(ctx context.Context, cred usecase.Credential, in domain.IssueUpdate) (domain.Issue, error) {
	input := map[string]any{}
	if in.Title != "" {
		input["title"] = in.Title
	}
	if in.Description != "" {
		input["description"] = in.Description
	}
	if in.AssigneeID != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.PriorityID != "" {
		input["priority"] = in.PriorityID
	}
	if len(in.LabelIDs) > 0 {
		input["labelIds"] = in.LabelIDs
	}
	if in.WorkflowStateID != "" {
		input["stateId"] = in.WorkflowStateID
	}
	var resp graphQLResponse[linearIssueCreateData]
	if err := c.do(ctx, cred.Token, updateIssueMutation, map[string]any{"id": in.IssueID, "input": input}, &resp); err != nil {
		return domain.Issue{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Issue{}, fmt.Errorf("linear: update issue: %s", resp.Errors[0].Message)
	}
	n := resp.Data.IssueCreate.Issue
	return domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
}

// ── AddIssueComment / ListIssueComments ─────────────────────────────────

const addCommentMutation = `mutation CommentCreate($input: CommentCreateInput!) {
  commentCreate(input: $input) {
    success
    comment { id body createdAt user { id name email } }
  }
}`

type linearCommentNode struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	User      struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
}

type linearCommentCreateData struct {
	CommentCreate struct {
		Success bool              `json:"success"`
		Comment linearCommentNode `json:"comment"`
	} `json:"commentCreate"`
}

func (c *Client) AddIssueComment(ctx context.Context, cred usecase.Credential, issueID, bodyMarkdown string) (domain.IssueComment, error) {
	var resp graphQLResponse[linearCommentCreateData]
	if err := c.do(ctx, cred.Token, addCommentMutation, map[string]any{"input": map[string]any{"issueId": issueID, "body": bodyMarkdown}}, &resp); err != nil {
		return domain.IssueComment{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.IssueComment{}, fmt.Errorf("linear: add comment: %s", resp.Errors[0].Message)
	}
	if !resp.Data.CommentCreate.Success {
		return domain.IssueComment{}, fmt.Errorf("linear: add comment reported failure")
	}
	n := resp.Data.CommentCreate.Comment
	return domain.IssueComment{ID: n.ID, BodyMarkdown: n.Body, Author: domain.UserRef{ID: n.User.ID, DisplayName: n.User.Name, Email: n.User.Email}}, nil
}

const listCommentsQuery = `query IssueComments($id: String!) {
  issue(id: $id) {
    comments {
      nodes { id body createdAt user { id name email } }
    }
  }
}`

type linearIssueCommentsData struct {
	Issue struct {
		Comments struct {
			Nodes []linearCommentNode `json:"nodes"`
		} `json:"comments"`
	} `json:"issue"`
}

func (c *Client) ListIssueComments(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.IssueComment, error) {
	var resp graphQLResponse[linearIssueCommentsData]
	if err := c.do(ctx, cred.Token, listCommentsQuery, map[string]any{"id": issueID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list comments: %s", resp.Errors[0].Message)
	}
	out := make([]domain.IssueComment, 0, len(resp.Data.Issue.Comments.Nodes))
	for _, n := range resp.Data.Issue.Comments.Nodes {
		out = append(out, domain.IssueComment{ID: n.ID, BodyMarkdown: n.Body, Author: domain.UserRef{ID: n.User.ID, DisplayName: n.User.Name, Email: n.User.Email}})
	}
	return out, nil
}

// ── CreateProject / GetProject ──────────────────────────────────────────

const createProjectMutation = `mutation ProjectCreate($input: ProjectCreateInput!) {
  projectCreate(input: $input) {
    success
    project { id name }
  }
}`

type linearProjectCreateData struct {
	ProjectCreate struct {
		Success bool `json:"success"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	} `json:"projectCreate"`
}

func (c *Client) CreateProject(ctx context.Context, cred usecase.Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error) {
	input := map[string]any{"name": name, "teamIds": []string{teamID}}
	if description != "" {
		input["description"] = description
	}
	var resp graphQLResponse[linearProjectCreateData]
	if err := c.do(ctx, cred.Token, createProjectMutation, map[string]any{"input": input}, &resp); err != nil {
		return domain.ProjectRef{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.ProjectRef{}, fmt.Errorf("linear: create project: %s", resp.Errors[0].Message)
	}
	return domain.ProjectRef{ID: resp.Data.ProjectCreate.Project.ID, Name: resp.Data.ProjectCreate.Project.Name, WorkspaceID: workspaceID}, nil
}

const getProjectQuery = `query Project($id: String!) {
  project(id: $id) { id name }
}`

type linearProjectData struct {
	Project struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
}

func (c *Client) GetProject(ctx context.Context, cred usecase.Credential, projectID, workspaceID string) (domain.ProjectRef, error) {
	var resp graphQLResponse[linearProjectData]
	if err := c.do(ctx, cred.Token, getProjectQuery, map[string]any{"id": projectID}, &resp); err != nil {
		return domain.ProjectRef{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.ProjectRef{}, fmt.Errorf("linear: get project: %s", resp.Errors[0].Message)
	}
	return domain.ProjectRef{ID: resp.Data.Project.ID, Name: resp.Data.Project.Name, WorkspaceID: workspaceID}, nil
}
```

### 4. `internal/adapter/jira/client.go` — stub `CreateProject`/`GetProject`

Jira has no `jira.createProject`/`jira.getProject` channel (not in
BUG-015's 19-method list) but must still satisfy the widened
`IssueTrackerProvider` interface:

```go
// CreateProject/GetProject are not wired to any jira.* channel (BUG-015's
// method list has none) — implemented only to satisfy IssueTrackerProvider;
// return a clear unsupported error rather than a silent no-op, per
// SOL-016's own note on this exact situation.
func (c *Client) CreateProject(ctx context.Context, cred usecase.Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error) {
	return domain.ProjectRef{}, fmt.Errorf("jira: CreateProject is not supported — use listProjects/an existing Jira project")
}

func (c *Client) GetProject(ctx context.Context, cred usecase.Credential, projectID, workspaceID string) (domain.ProjectRef, error) {
	return domain.ProjectRef{}, fmt.Errorf("jira: GetProject is not implemented — see listProjects")
}
```

### 5. `internal/adapter/grpc/server.go` — wire `CreateProject`/`GetProject`

Same shape as every other handler — `apperrors.ToGRPCStatus` on failure,
translate `domain.ProjectRef` to `*issuetrackingv1.Project`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/... 2>&1 | head -50
go vet ./services/issue-tracking-service/...
```

Expected: still fails on `ListTeams`/`ListTeamLabels`/`ListTeamMembers`/
`GetCustomView` (TASK-105 adds those) — everything else must compile clean.
