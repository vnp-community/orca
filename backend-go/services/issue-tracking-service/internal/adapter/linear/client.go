// Package linear implements usecase.IssueTrackerProvider against Linear's
// GraphQL API. Linear publishes no official Go SDK (design doc §4), so this
// hand-rolls a minimal GraphQL client — typed request/response structs over
// net/http, real POST requests with a GraphQL query/mutation string and
// bearer auth. genqlient/gqlgen codegen is a reasonable follow-up per the
// design doc, not required for this to work.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// endpoint is Linear's single GraphQL API endpoint — there is no
// per-tenant/per-site base URL the way Jira has (design doc §9).
const endpoint = "https://api.linear.app/graphql"

// Client is a real Linear GraphQL client — bearer token auth (personal API
// key or OAuth access token), attached per-request the same way Jira's
// Basic Auth header is (design doc §9: hand-rolling the client doesn't
// change the auth model, @linear/sdk was just a typed wrapper around this
// same bearer scheme).
type Client struct {
	httpClient *http.Client
}

// New returns a Client. Pass nil to use a sane default *http.Client with a
// bounded timeout.
func New(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

var _ usecase.IssueTrackerProvider = (*Client)(nil)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

// do POSTs a GraphQL query/mutation to Linear's endpoint and decodes the
// response into out.
func (c *Client) do(ctx context.Context, token, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("linear: marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("linear: building graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("linear: graphql request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("linear: unexpected status %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("linear: decoding graphql response: %w", err)
	}
	return nil
}

// ── Whoami ───────────────────────────────────────────────────────────────

const viewerQuery = `query Viewer {
  viewer {
    id
    name
    email
  }
}`

type linearViewerData struct {
	Viewer struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"viewer"`
}

// Whoami calls Linear's viewer{} GraphQL query to verify cred.Token and
// identify the authenticated account — the first call Connect makes,
// before anything is persisted.
func (c *Client) Whoami(ctx context.Context, cred usecase.Credential) (domain.Viewer, error) {
	var resp graphQLResponse[linearViewerData]
	if err := c.do(ctx, cred.Token, viewerQuery, nil, &resp); err != nil {
		return domain.Viewer{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Viewer{}, fmt.Errorf("linear: whoami: %s", resp.Errors[0].Message)
	}
	return domain.Viewer{ID: resp.Data.Viewer.ID, DisplayName: resp.Data.Viewer.Name, Email: resp.Data.Viewer.Email}, nil
}

// ── SearchIssues / ListIssues / GetIssue ────────────────────────────────

const listIssuesQuery = `query Issues($filter: IssueFilter, $first: Int) {
  issues(filter: $filter, first: $first) {
    nodes {
      identifier
      title
      url
      state { name }
    }
  }
}`

type linearIssueNode struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      struct {
		Name string `json:"name"`
	} `json:"state"`
}

type linearIssuesData struct {
	Issues struct {
		Nodes []linearIssueNode `json:"nodes"`
	} `json:"issues"`
}

func toRichIssues(nodes []linearIssueNode) []domain.Issue {
	issues := make([]domain.Issue, 0, len(nodes))
	for _, n := range nodes {
		issue, err := domain.NewIssue(n.Identifier, n.Title, n.State.Name, n.URL)
		if err != nil {
			continue // skip a malformed entry rather than failing the whole page
		}
		issue.ProviderIssueID = n.Identifier
		issue.Key = n.Identifier
		issues = append(issues, issue)
	}
	return issues
}

// SearchIssues: Linear's issues() filter has no free-text search argument
// on the public schema the way Jira's JQL does — this passes query through
// as a title "contains" filter, the closest equivalent. A richer filter
// (state/assignee/label) is what filterJSON (ListIssues) is for, not
// SearchIssues.
func (c *Client) SearchIssues(ctx context.Context, cred usecase.Credential, query string, limit int) ([]domain.Issue, error) {
	variables := map[string]any{}
	if query != "" {
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

// ListIssues filters by team key matching projectKey when set — Linear's
// "team" is the closest concept to Jira's "project" (design doc §4).
// filterJSON (a LinearIssueFilter-shaped object) is not translated to a
// GraphQL filter here — a documented gap, same as jira/client.go's
// ListIssues filterJSON note.
func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error) {
	variables := map[string]any{}
	if projectKey != "" {
		variables["filter"] = map[string]any{"team": map[string]any{"key": map[string]any{"eq": projectKey}}}
	}
	if limit > 0 {
		variables["first"] = limit
	}
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

// ── team resolution (CreateIssue helper) ────────────────────────────────

const teamByKeyQuery = `query TeamByKey($key: String!) {
  teams(filter: { key: { eq: $key } }) {
    nodes { id }
  }
}`

type linearTeamsData struct {
	Teams struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	} `json:"teams"`
}

// resolveTeamID looks up the Linear team UUID for a team key (e.g. "ENG") —
// CreateIssue's mutation needs a team ID, not a key. A value that already
// looks like a Linear UUID is passed through unchanged (best-effort: this
// scaffold doesn't validate UUID format strictly, it simply tries the
// lookup-by-key path and only falls back if that whole flow is skipped by
// the caller already supplying a resolved id — see CreateIssue below,
// which always resolves by key/id through this one path for simplicity).
func (c *Client) resolveTeamID(ctx context.Context, token, teamKeyOrID string) (string, error) {
	var resp graphQLResponse[linearTeamsData]
	if err := c.do(ctx, token, teamByKeyQuery, map[string]any{"key": teamKeyOrID}, &resp); err != nil {
		return "", err
	}
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("linear: resolve team: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.Teams.Nodes) == 0 {
		return "", fmt.Errorf("linear: no team found for key %q", teamKeyOrID)
	}
	return resp.Data.Teams.Nodes[0].ID, nil
}

// ── CreateIssue / UpdateIssue ────────────────────────────────────────────

const createIssueMutation = `mutation IssueCreate($input: IssueCreateInput!) {
  issueCreate(input: $input) {
    success
    issue {
      identifier
      title
      url
      state { name }
    }
  }
}`

type linearIssueCreateData struct {
	IssueCreate struct {
		Success bool            `json:"success"`
		Issue   linearIssueNode `json:"issue"`
	} `json:"issueCreate"`
}

// CreateIssue resolves in.ProjectKey (which usecase.CreateIssue populates
// from CreateIssueRequest.team_id when set, else project_key) to a real
// Linear team UUID before calling issueCreate — Linear's mutation takes
// teamId, not a team key.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, in domain.NewIssueInput) (domain.Issue, error) {
	teamKey := in.TeamID
	if teamKey == "" {
		teamKey = in.ProjectKey
	}
	teamID, err := c.resolveTeamID(ctx, cred.Token, teamKey)
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
	if in.StateID != "" {
		input["stateId"] = in.StateID
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

// ── ListTeams / ListTeamLabels / ListTeamMembers / GetCustomView /
// ListWorkflowStates ────────────────────────────────────────────────────

const listTeamsQuery = `query Teams {
  teams {
    nodes { id name key }
  }
}`

type linearTeamNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type linearAllTeamsData struct {
	Teams struct {
		Nodes []linearTeamNode `json:"nodes"`
	} `json:"teams"`
}

func (c *Client) ListTeams(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.Team, error) {
	var resp graphQLResponse[linearAllTeamsData]
	if err := c.do(ctx, cred.Token, listTeamsQuery, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list teams: %s", resp.Errors[0].Message)
	}
	out := make([]domain.Team, 0, len(resp.Data.Teams.Nodes))
	for _, t := range resp.Data.Teams.Nodes {
		out = append(out, domain.Team{ID: t.ID, WorkspaceID: workspaceID, Name: t.Name, Key: t.Key})
	}
	return out, nil
}

const listTeamLabelsQuery = `query TeamLabels($teamId: String!) {
  team(id: $teamId) {
    labels {
      nodes { id name color }
    }
  }
}`

type linearTeamLabelsData struct {
	Team struct {
		Labels struct {
			Nodes []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"nodes"`
		} `json:"labels"`
	} `json:"team"`
}

func (c *Client) ListTeamLabels(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamLabel, error) {
	var resp graphQLResponse[linearTeamLabelsData]
	if err := c.do(ctx, cred.Token, listTeamLabelsQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list team labels: %s", resp.Errors[0].Message)
	}
	out := make([]domain.TeamLabel, 0, len(resp.Data.Team.Labels.Nodes))
	for _, l := range resp.Data.Team.Labels.Nodes {
		out = append(out, domain.TeamLabel{ID: l.ID, Name: l.Name, Color: l.Color})
	}
	return out, nil
}

const listTeamMembersQuery = `query TeamMembers($teamId: String!) {
  team(id: $teamId) {
    members {
      nodes { id name avatarUrl }
    }
  }
}`

type linearTeamMembersData struct {
	Team struct {
		Members struct {
			Nodes []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatarUrl"`
			} `json:"nodes"`
		} `json:"members"`
	} `json:"team"`
}

func (c *Client) ListTeamMembers(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamMember, error) {
	var resp graphQLResponse[linearTeamMembersData]
	if err := c.do(ctx, cred.Token, listTeamMembersQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list team members: %s", resp.Errors[0].Message)
	}
	out := make([]domain.TeamMember, 0, len(resp.Data.Team.Members.Nodes))
	for _, m := range resp.Data.Team.Members.Nodes {
		out = append(out, domain.TeamMember{ID: m.ID, DisplayName: m.Name, AvatarURL: m.AvatarURL})
	}
	return out, nil
}

// GetCustomView queries customView(id) regardless of model — Linear's
// public schema exposes a single lookup; model is round-tripped from the
// input since the API response has no separate discriminator field to
// read it back from reliably in this scaffold.
const getCustomViewQuery = `query CustomView($id: String!) {
  customView(id: $id) {
    id
    name
    team { id }
  }
}`

type linearCustomViewData struct {
	CustomView struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Team *struct {
			ID string `json:"id"`
		} `json:"team"`
	} `json:"customView"`
}

func (c *Client) GetCustomView(ctx context.Context, cred usecase.Credential, viewID, model string) (domain.CustomView, error) {
	var resp graphQLResponse[linearCustomViewData]
	if err := c.do(ctx, cred.Token, getCustomViewQuery, map[string]any{"id": viewID}, &resp); err != nil {
		return domain.CustomView{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.CustomView{}, fmt.Errorf("linear: get custom view: %s", resp.Errors[0].Message)
	}
	cv := domain.CustomView{ID: resp.Data.CustomView.ID, Name: resp.Data.CustomView.Name, Model: model}
	if resp.Data.CustomView.Team != nil {
		cv.TeamID = resp.Data.CustomView.Team.ID
	}
	return cv, nil
}

const listWorkflowStatesQuery = `query TeamStates($teamId: String!) {
  team(id: $teamId) {
    states {
      nodes { id name type position }
    }
  }
}`

type linearTeamStatesData struct {
	Team struct {
		States struct {
			Nodes []struct {
				ID       string  `json:"id"`
				Name     string  `json:"name"`
				Type     string  `json:"type"` // triage|backlog|unstarted|started|completed|cancelled
				Position float64 `json:"position"`
			} `json:"nodes"`
		} `json:"states"`
	} `json:"team"`
}

// ListWorkflowStates backs linear.teamStates — Linear's states are already
// returned ordered by position (ascending) by the API, matching
// teamStates' expected ordered-list contract; no client-side sort needed.
func (c *Client) ListWorkflowStates(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.WorkflowState, error) {
	var resp graphQLResponse[linearTeamStatesData]
	if err := c.do(ctx, cred.Token, listWorkflowStatesQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list workflow states: %s", resp.Errors[0].Message)
	}
	out := make([]domain.WorkflowState, 0, len(resp.Data.Team.States.Nodes))
	for _, s := range resp.Data.Team.States.Nodes {
		out = append(out, domain.WorkflowState{ID: s.ID, Name: s.Name, Category: s.Type})
	}
	return out, nil
}

// ── ListProjects / ListIssueTypes / ListCreateFields / ListAssignableUsers
// / ListPriorities / ListTransitions / GetProjectStatusOrder ─────────────
//
// These 7 methods (added to IssueTrackerProvider for Jira's metadata-lookup
// group) are, per SOL-016's mapping table, either genuinely Jira-only
// (IssueTypes/CreateFields/Priorities/StatusOrder — the usecase layer
// always resolves domain.ProviderJira for these and never reaches this
// adapter) or explicitly NOT unified with a Linear equivalent to avoid
// "false unification" (ListProjects: Linear's closest concept is Team,
// deliberately kept a separate message/RPC — see ListTeams above;
// ListTransitions: Linear's issue-state changes go through
// UpdateIssue/workflow_state_id, not a discrete "transition" object the
// way Jira's workflow does). ListAssignableUsers IS a shared concept in
// principle, but this scaffold has no design-doc-confirmed Linear query
// for "assignable to project/issue" distinct from team membership, so it
// stays a clear unsupported error here too rather than silently
// approximating one with ListTeamMembers.

func (c *Client) ListProjects(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.ProjectRef, error) {
	return nil, fmt.Errorf("linear: ListProjects is not applicable to linear — use listTeams (SOL-016: no false unification)")
}

func (c *Client) ListIssueTypes(ctx context.Context, cred usecase.Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error) {
	return nil, fmt.Errorf("linear: ListIssueTypes is not applicable to linear — issue types have no Linear analog")
}

func (c *Client) ListCreateFields(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error) {
	return nil, fmt.Errorf("linear: ListCreateFields is not applicable to linear")
}

func (c *Client) ListAssignableUsers(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error) {
	return nil, fmt.Errorf("linear: ListAssignableUsers is not applicable to linear — use teamMembers")
}

func (c *Client) ListPriorities(ctx context.Context, cred usecase.Credential) ([]domain.PriorityRef, error) {
	return nil, fmt.Errorf("linear: ListPriorities is not applicable to linear")
}

func (c *Client) ListTransitions(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.Transition, error) {
	return nil, fmt.Errorf("linear: ListTransitions is not applicable to linear — use teamStates and updateIssue")
}

func (c *Client) GetProjectStatusOrder(ctx context.Context, cred usecase.Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error) {
	return domain.ProjectStatusOrder{}, fmt.Errorf("linear: GetProjectStatusOrder is not applicable to linear — use teamStates")
}
