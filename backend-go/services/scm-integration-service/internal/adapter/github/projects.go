// Package github (this file): implements usecase.GitHubProjectsProvider —
// GitHub Projects v2, GraphQL-only, on the same *Client REST adapter
// already implements usecase.ScmProvider (client.go). See SOL-012 "Design —
// adapter/external/github/ implementation notes".
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

var _ usecase.GitHubProjectsProvider = (*Client)(nil)

// parseProjectSlug splits "owner/number" (as produced by ResolveProjectRef)
// back into its parts.
func parseProjectSlug(slug string) (owner string, number int32, err error) {
	idx := strings.LastIndex(slug, "/")
	if idx < 0 {
		return "", 0, fmt.Errorf("github: invalid project slug %q, want owner/number", slug)
	}
	n, err := strconv.Atoi(slug[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("github: invalid project slug %q: %w", slug, err)
	}
	return slug[:idx], int32(n), nil
}

// parseItemSlug splits "owner/repo#number" (the by-slug addressing scheme
// BUG-012 found missing server-side) into a repo path + issue/PR number.
func parseItemSlug(slug string) (repo string, number int32, err error) {
	idx := strings.LastIndex(slug, "#")
	if idx < 0 {
		return "", 0, fmt.Errorf("github: invalid item slug %q, want owner/repo#number", slug)
	}
	n, err := strconv.Atoi(slug[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("github: invalid item slug %q: %w", slug, err)
	}
	return slug[:idx], int32(n), nil
}

// projectV2Owner mirrors the "owner on Organization | User" union every
// Projects v2 GraphQL response below returns.
type projectV2Owner struct {
	Login string `json:"login"`
}

type projectV2Node struct {
	ID     string         `json:"id"`
	Number int32          `json:"number"`
	Title  string         `json:"title"`
	URL    string         `json:"url"`
	Owner  projectV2Owner `json:"owner"`
}

// resolveProjectID looks up a project's GraphQL node id from an owner+number
// pair — organization projects and user projects are different GraphQL root
// fields, so this tries organization first and falls back to user.
func (c *Client) resolveProjectID(ctx context.Context, cred usecase.Credential, owner string, number int32) (projectV2Node, error) {
	const orgQuery = `query($owner: String!, $number: Int!) {
		organization(login: $owner) {
			projectV2(number: $number) { id number title url owner { ... on Organization { login } ... on User { login } } }
		}
	}`
	var orgResp struct {
		Organization struct {
			ProjectV2 projectV2Node `json:"projectV2"`
		} `json:"organization"`
	}
	if err := c.graphQLRequest(ctx, cred, orgQuery, map[string]any{"owner": owner, "number": number}, &orgResp); err == nil && orgResp.Organization.ProjectV2.ID != "" {
		return orgResp.Organization.ProjectV2, nil
	}

	const userQuery = `query($owner: String!, $number: Int!) {
		user(login: $owner) {
			projectV2(number: $number) { id number title url owner { ... on Organization { login } ... on User { login } } }
		}
	}`
	var userResp struct {
		User struct {
			ProjectV2 projectV2Node `json:"projectV2"`
		} `json:"user"`
	}
	if err := c.graphQLRequest(ctx, cred, userQuery, map[string]any{"owner": owner, "number": number}, &userResp); err != nil {
		return projectV2Node{}, fmt.Errorf("github: resolve project %s/%d: %w", owner, number, err)
	}
	if userResp.User.ProjectV2.ID == "" {
		return projectV2Node{}, fmt.Errorf("github: project %s/%d not found", owner, number)
	}
	return userResp.User.ProjectV2, nil
}

func toUsecaseProject(n projectV2Node) usecase.Project {
	return usecase.Project{
		ID: n.ID, Slug: fmt.Sprintf("%s/%d", n.Owner.Login, n.Number),
		Title: n.Title, Number: n.Number, Owner: n.Owner.Login, URL: n.URL,
	}
}

// ListAccessibleProjects calls GitHub's GraphQL API: viewer.projectsV2.
func (c *Client) ListAccessibleProjects(ctx context.Context, cred usecase.Credential) ([]usecase.Project, error) {
	const query = `query {
		viewer {
			projectsV2(first: 50) { nodes { id number title url owner { ... on Organization { login } ... on User { login } } } }
		}
	}`
	var resp struct {
		Viewer struct {
			ProjectsV2 struct {
				Nodes []projectV2Node `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"viewer"`
	}
	if err := c.graphQLRequest(ctx, cred, query, nil, &resp); err != nil {
		return nil, fmt.Errorf("github: list accessible projects: %w", err)
	}
	out := make([]usecase.Project, 0, len(resp.Viewer.ProjectsV2.Nodes))
	for _, n := range resp.Viewer.ProjectsV2.Nodes {
		out = append(out, toUsecaseProject(n))
	}
	return out, nil
}

// ResolveProjectRef calls GitHub's GraphQL API: organization.projectV2(number:)
// (falling back to user.projectV2(number:) — see resolveProjectID).
func (c *Client) ResolveProjectRef(ctx context.Context, cred usecase.Credential, owner string, number int32) (usecase.Project, error) {
	n, err := c.resolveProjectID(ctx, cred, owner, number)
	if err != nil {
		return usecase.Project{}, err
	}
	return toUsecaseProject(n), nil
}

// ListProjectViews calls GitHub's GraphQL API: projectV2.views.
func (c *Client) ListProjectViews(ctx context.Context, cred usecase.Credential, projectSlug string) ([]usecase.ProjectView, error) {
	owner, number, err := parseProjectSlug(projectSlug)
	if err != nil {
		return nil, err
	}
	proj, err := c.resolveProjectID(ctx, cred, owner, number)
	if err != nil {
		return nil, err
	}
	const query = `query($id: ID!) {
		node(id: $id) {
			... on ProjectV2 { views(first: 20) { nodes { id name layout } } }
		}
	}`
	var resp struct {
		Node struct {
			Views struct {
				Nodes []struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Layout string `json:"layout"`
				} `json:"nodes"`
			} `json:"views"`
		} `json:"node"`
	}
	if err := c.graphQLRequest(ctx, cred, query, map[string]any{"id": proj.ID}, &resp); err != nil {
		return nil, fmt.Errorf("github: list project views: %w", err)
	}
	out := make([]usecase.ProjectView, 0, len(resp.Node.Views.Nodes))
	for _, v := range resp.Node.Views.Nodes {
		out = append(out, usecase.ProjectView{ID: v.ID, Name: v.Name, Layout: v.Layout})
	}
	return out, nil
}

// projectV2ItemFieldValue mirrors one node of a ProjectV2Item's
// fieldValues connection — only the two field-value shapes this adapter
// maps (text, single-select) are modeled; others decode to a zero value
// rather than erroring, since an unmapped field kind shouldn't break the
// whole table read.
type projectV2ItemFieldValue struct {
	Text  string `json:"text"`
	Name  string `json:"name"`
	Field struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"field"`
}

// ViewProjectTable calls GitHub's GraphQL API: projectV2.items, paginated.
func (c *Client) ViewProjectTable(ctx context.Context, cred usecase.Credential, projectSlug, viewID, pageToken string, pageSize int32) ([]usecase.ProjectItem, string, error) {
	owner, number, err := parseProjectSlug(projectSlug)
	if err != nil {
		return nil, "", err
	}
	proj, err := c.resolveProjectID(ctx, cred, owner, number)
	if err != nil {
		return nil, "", err
	}
	if pageSize <= 0 {
		pageSize = 25
	}
	const query = `query($id: ID!, $first: Int!, $after: String) {
		node(id: $id) {
			... on ProjectV2 {
				items(first: $first, after: $after) {
					pageInfo { hasNextPage endCursor }
					nodes {
						id
						content {
							... on Issue { title url }
							... on PullRequest { title url }
							... on DraftIssue { title }
						}
						fieldValues(first: 20) {
							nodes {
								... on ProjectV2ItemFieldTextValue { text field { ... on ProjectV2FieldCommon { id name } } }
								... on ProjectV2ItemFieldSingleSelectValue { name field { ... on ProjectV2FieldCommon { id name } } }
							}
						}
					}
				}
			}
		}
	}`
	variables := map[string]any{"id": proj.ID, "first": pageSize}
	// viewID is accepted for API-surface parity with the frontend's per-view
	// table read, but GitHub's items connection is not itself filterable by
	// view — the caller's default-view semantics are honored implicitly
	// (there is no separate per-view item query in the Projects v2 API).
	_ = viewID
	if pageToken != "" {
		variables["after"] = pageToken
	}
	var resp struct {
		Node struct {
			Items struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					ID      string `json:"id"`
					Content struct {
						Title string `json:"title"`
						URL   string `json:"url"`
					} `json:"content"`
					FieldValues struct {
						Nodes []projectV2ItemFieldValue `json:"nodes"`
					} `json:"fieldValues"`
				} `json:"nodes"`
			} `json:"items"`
		} `json:"node"`
	}
	if err := c.graphQLRequest(ctx, cred, query, variables, &resp); err != nil {
		return nil, "", fmt.Errorf("github: view project table: %w", err)
	}
	items := make([]usecase.ProjectItem, 0, len(resp.Node.Items.Nodes))
	for _, n := range resp.Node.Items.Nodes {
		fields := make([]usecase.ProjectFieldValue, 0, len(n.FieldValues.Nodes))
		for _, fv := range n.FieldValues.Nodes {
			value := fv.Text
			if value == "" {
				value = fv.Name
			}
			fields = append(fields, usecase.ProjectFieldValue{FieldID: fv.Field.ID, Kind: "text", Value: value})
		}
		items = append(items, usecase.ProjectItem{
			ID: n.ID, Title: n.Content.Title, ContentType: "issue", ContentURL: n.Content.URL, Fields: fields,
		})
	}
	nextPageToken := ""
	if resp.Node.Items.PageInfo.HasNextPage {
		nextPageToken = resp.Node.Items.PageInfo.EndCursor
	}
	return items, nextPageToken, nil
}

// UpdateProjectItemField calls GitHub's GraphQL API:
// updateProjectV2ItemFieldValue. The mutation's `value` input is a typed
// union keyed by field.kind — this adapter handles "text"/"number"/"date"/
// "single_select" (the kinds SOL-012's ProjectFieldValue doc comment names);
// "iteration" needs the iteration's own node id, not modeled here yet — an
// unsupported kind returns an error rather than silently no-op'ing.
func (c *Client) UpdateProjectItemField(ctx context.Context, cred usecase.Credential, projectSlug, itemID string, field usecase.ProjectFieldValue) (usecase.ProjectItem, error) {
	owner, number, err := parseProjectSlug(projectSlug)
	if err != nil {
		return usecase.ProjectItem{}, err
	}
	proj, err := c.resolveProjectID(ctx, cred, owner, number)
	if err != nil {
		return usecase.ProjectItem{}, err
	}

	var valueInput map[string]any
	switch field.Kind {
	case "text":
		valueInput = map[string]any{"text": field.Value}
	case "number":
		n, convErr := strconv.ParseFloat(field.Value, 64)
		if convErr != nil {
			return usecase.ProjectItem{}, fmt.Errorf("github: field value %q is not a number: %w", field.Value, convErr)
		}
		valueInput = map[string]any{"number": n}
	case "date":
		valueInput = map[string]any{"date": field.Value}
	case "single_select":
		valueInput = map[string]any{"singleSelectOptionId": field.Value}
	default:
		return usecase.ProjectItem{}, fmt.Errorf("github: unsupported project field kind %q", field.Kind)
	}

	const mutation = `mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $value: ProjectV2FieldValue!) {
		updateProjectV2ItemFieldValue(input: {projectId: $projectId, itemId: $itemId, fieldId: $fieldId, value: $value}) {
			projectV2Item { id }
		}
	}`
	if err := c.graphQLRequest(ctx, cred, mutation, map[string]any{
		"projectId": proj.ID, "itemId": itemID, "fieldId": field.FieldID, "value": valueInput,
	}, nil); err != nil {
		return usecase.ProjectItem{}, fmt.Errorf("github: update project item field: %w", err)
	}
	return usecase.ProjectItem{ID: itemID, Fields: []usecase.ProjectFieldValue{field}}, nil
}

// ClearProjectItemField calls GitHub's GraphQL API: clearProjectV2ItemFieldValue.
func (c *Client) ClearProjectItemField(ctx context.Context, cred usecase.Credential, projectSlug, itemID, fieldID string) (usecase.ProjectItem, error) {
	owner, number, err := parseProjectSlug(projectSlug)
	if err != nil {
		return usecase.ProjectItem{}, err
	}
	proj, err := c.resolveProjectID(ctx, cred, owner, number)
	if err != nil {
		return usecase.ProjectItem{}, err
	}
	const mutation = `mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!) {
		clearProjectV2ItemFieldValue(input: {projectId: $projectId, itemId: $itemId, fieldId: $fieldId}) {
			projectV2Item { id }
		}
	}`
	if err := c.graphQLRequest(ctx, cred, mutation, map[string]any{
		"projectId": proj.ID, "itemId": itemID, "fieldId": fieldID,
	}, nil); err != nil {
		return usecase.ProjectItem{}, fmt.Errorf("github: clear project item field: %w", err)
	}
	return usecase.ProjectItem{ID: itemID}, nil
}

// GetWorkItemDetailsBySlug calls GitHub's REST API: GET
// /repos/{owner}/{repo}/issues/{number} — works for both issues and PRs,
// since GitHub's issues endpoint returns PRs too (see githubIssue.PullRequest
// in client.go).
func (c *Client) GetWorkItemDetailsBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) (usecase.WorkItemDetails, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	raw, err := c.getIssueOrPR(ctx, cred, repo, number)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	return usecase.WorkItemDetails{Slug: itemSlug, Title: raw.Title, Body: raw.Body, State: raw.State, URL: raw.HTMLURL}, nil
}

// githubIssueOrPR mirrors the subset of GitHub's issues-endpoint response
// this file's "BySlug" methods need (title/body/state/url) — a superset of
// githubIssue (client.go) that also carries body, which ListIssues never
// needed.
type githubIssueOrPR struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
}

// getIssueOrPR is the shared GET /repos/{repo}/issues/{number} call every
// "BySlug" method below builds on.
func (c *Client) getIssueOrPR(ctx context.Context, cred usecase.Credential, repo string, number int32) (githubIssueOrPR, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return githubIssueOrPR{}, fmt.Errorf("github: build get work item details request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return githubIssueOrPR{}, fmt.Errorf("github: get work item details request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return githubIssueOrPR{}, fmt.Errorf("github: get work item details: unexpected status %d", resp.StatusCode)
	}
	var raw githubIssueOrPR
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return githubIssueOrPR{}, fmt.Errorf("github: decode get work item details response: %w", err)
	}
	return raw, nil
}

// UpdateIssueBySlug calls GitHub's REST API: PATCH /repos/{repo}/issues/{number}
// for title/body/state, then POST/DELETE .../issues/{number}/labels for
// add/remove label deltas — same body/label-delta shape as UpdateIssue
// (client.go), against usecase.WorkItemPatch instead of usecase.IssuePatch.
func (c *Client) UpdateIssueBySlug(ctx context.Context, cred usecase.Credential, itemSlug string, patch usecase.WorkItemPatch) (usecase.WorkItemDetails, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	if err := c.patchIssueFields(ctx, cred, repo, number, patch.Title, patch.Body, patch.State); err != nil {
		return usecase.WorkItemDetails{}, err
	}
	if err := c.applyLabelDeltas(ctx, cred, repo, number, patch.AddLabels, patch.RemoveLabels); err != nil {
		return usecase.WorkItemDetails{}, err
	}
	return c.GetWorkItemDetailsBySlug(ctx, cred, itemSlug)
}

// UpdatePullRequestBySlug calls GitHub's REST API: PATCH
// /repos/{repo}/pulls/{number} — title/body/state only (no labels on PRs
// via this endpoint).
func (c *Client) UpdatePullRequestBySlug(ctx context.Context, cred usecase.Credential, itemSlug string, patch usecase.WorkItemPatch) (usecase.WorkItemDetails, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	body, err := json.Marshal(struct {
		Title *string `json:"title,omitempty"`
		Body  *string `json:"body,omitempty"`
		State *string `json:"state,omitempty"`
	}{Title: patch.Title, Body: patch.Body, State: patch.State})
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: encode update pull request body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewReader(body))
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: build update pull request request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: update pull request request failed: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: update pull request: unexpected status %d", resp.StatusCode)
	}
	return c.GetWorkItemDetailsBySlug(ctx, cred, itemSlug)
}

// UpdateIssueTypeBySlug calls GitHub's REST API: PATCH
// /repos/{repo}/issues/{number} with {"type": issueType} — GitHub's
// issue-type field, an org-scoped feature.
func (c *Client) UpdateIssueTypeBySlug(ctx context.Context, cred usecase.Credential, itemSlug, issueType string) (usecase.WorkItemDetails, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	body, err := json.Marshal(struct {
		Type string `json:"type"`
	}{Type: issueType})
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: encode update issue type body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewReader(body))
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: build update issue type request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: update issue type request failed: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: update issue type: unexpected status %d", resp.StatusCode)
	}
	return c.GetWorkItemDetailsBySlug(ctx, cred, itemSlug)
}

// ListIssueTypesBySlug calls GitHub's REST API: GET /orgs/{org}/issue-types
// — org is the slug's repo owner segment.
func (c *Client) ListIssueTypesBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) ([]usecase.IssueType, error) {
	repo, _, err := parseItemSlug(itemSlug)
	if err != nil {
		return nil, err
	}
	org := repo
	if idx := strings.Index(repo, "/"); idx >= 0 {
		org = repo[:idx]
	}
	reqURL := fmt.Sprintf("%s/orgs/%s/issue-types", c.baseURL, org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list issue types request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list issue types request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list issue types: unexpected status %d", resp.StatusCode)
	}
	var raw []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list issue types response: %w", err)
	}
	out := make([]usecase.IssueType, 0, len(raw))
	for _, t := range raw {
		out = append(out, usecase.IssueType{ID: strconv.Itoa(t.ID), Name: t.Name, Description: t.Description})
	}
	return out, nil
}

// ListAssignableUsersBySlug calls GitHub's REST API: GET /repos/{repo}/assignees.
func (c *Client) ListAssignableUsersBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) ([]usecase.AssignableUser, error) {
	repo, _, err := parseItemSlug(itemSlug)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/repos/%s/assignees", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list assignable users request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list assignable users request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list assignable users: unexpected status %d", resp.StatusCode)
	}
	var raw []struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list assignable users response: %w", err)
	}
	out := make([]usecase.AssignableUser, 0, len(raw))
	for _, u := range raw {
		out = append(out, usecase.AssignableUser{Login: u.Login, Name: u.Name, AvatarURL: u.AvatarURL})
	}
	return out, nil
}

// ListLabelsBySlug calls GitHub's REST API: GET /repos/{repo}/labels.
func (c *Client) ListLabelsBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) ([]usecase.Label, error) {
	repo, _, err := parseItemSlug(itemSlug)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/repos/%s/labels", c.baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list labels request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list labels request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list labels: unexpected status %d", resp.StatusCode)
	}
	var raw []struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list labels response: %w", err)
	}
	out := make([]usecase.Label, 0, len(raw))
	for _, l := range raw {
		out = append(out, usecase.Label{Name: l.Name, Color: l.Color, Description: l.Description})
	}
	return out, nil
}

// AddIssueCommentBySlug calls GitHub's REST API: POST
// /repos/{repo}/issues/{number}/comments.
func (c *Client) AddIssueCommentBySlug(ctx context.Context, cred usecase.Credential, itemSlug, body string) (usecase.ProjectComment, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.ProjectComment{}, err
	}
	reqBody, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: body})
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: encode add issue comment body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: build add issue comment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: add issue comment request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return usecase.ProjectComment{}, fmt.Errorf("github: add issue comment: unexpected status %d", resp.StatusCode)
	}
	return decodeGitHubComment(resp)
}

// ListIssueCommentsBySlug calls GitHub's REST API: GET
// /repos/{repo}/issues/{number}/comments — the read half missing from the
// existing Add/Update/DeleteIssueCommentBySlug group (BUG-PI-01 step 6).
func (c *Client) ListIssueCommentsBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) ([]usecase.ProjectComment, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build list issue comments request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list issue comments request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list issue comments: unexpected status %d", resp.StatusCode)
	}
	var raw []githubComment
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode list issue comments response: %w", err)
	}
	out := make([]usecase.ProjectComment, 0, len(raw))
	for _, c := range raw {
		out = append(out, usecase.ProjectComment{ID: strconv.Itoa(c.ID), Body: c.Body, Author: c.User.Login, URL: c.HTMLURL})
	}
	return out, nil
}

// UpdateIssueCommentBySlug calls GitHub's REST API: PATCH
// /repos/{repo}/issues/comments/{commentId} — commentId is NOT
// repo-number-scoped, GitHub addresses comments by a global id, passed
// straight through from commentID.
func (c *Client) UpdateIssueCommentBySlug(ctx context.Context, cred usecase.Credential, itemSlug, commentID, body string) (usecase.ProjectComment, error) {
	repo, _, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.ProjectComment{}, err
	}
	reqBody, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: body})
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: encode update issue comment body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/comments/%s", c.baseURL, repo, commentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: build update issue comment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: update issue comment request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return usecase.ProjectComment{}, fmt.Errorf("github: update issue comment: unexpected status %d", resp.StatusCode)
	}
	return decodeGitHubComment(resp)
}

// DeleteIssueCommentBySlug calls GitHub's REST API: DELETE
// /repos/{repo}/issues/comments/{commentId} — a 204 or 404 (already
// deleted) is success.
func (c *Client) DeleteIssueCommentBySlug(ctx context.Context, cred usecase.Credential, itemSlug, commentID string) error {
	repo, _, err := parseItemSlug(itemSlug)
	if err != nil {
		return err
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/comments/%s", c.baseURL, repo, commentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("github: build delete issue comment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: delete issue comment request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("github: delete issue comment: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// githubComment mirrors the fields this file's comment methods need from
// GitHub's issue-comment response shape.
type githubComment struct {
	ID      int    `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

func decodeGitHubComment(resp *http.Response) (usecase.ProjectComment, error) {
	var raw githubComment
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return usecase.ProjectComment{}, fmt.Errorf("github: decode comment response: %w", err)
	}
	return usecase.ProjectComment{ID: strconv.Itoa(raw.ID), Body: raw.Body, Author: raw.User.Login, URL: raw.HTMLURL}, nil
}

// patchIssueFields is UpdateIssueBySlug's title/body/state PATCH step,
// shared with client.go's UpdateIssue via the same field-presence check.
func (c *Client) patchIssueFields(ctx context.Context, cred usecase.Credential, repo string, number int32, title, body, state *string) error {
	if title == nil && body == nil && state == nil {
		return nil
	}
	reqBody, err := json.Marshal(struct {
		Title *string `json:"title,omitempty"`
		Body  *string `json:"body,omitempty"`
		State *string `json:"state,omitempty"`
	}{Title: title, Body: body, State: state})
	if err != nil {
		return fmt.Errorf("github: encode update issue body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("github: build update issue request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: update issue request failed: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: update issue: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// applyLabelDeltas is UpdateIssueBySlug's add/remove-label step, mirroring
// client.go's UpdateIssue label-delta logic.
func (c *Client) applyLabelDeltas(ctx context.Context, cred usecase.Credential, repo string, number int32, addLabels, removeLabels []string) error {
	if len(addLabels) > 0 {
		body, _ := json.Marshal(struct {
			Labels []string `json:"labels"`
		}{Labels: addLabels})
		reqURL := fmt.Sprintf("%s/repos/%s/issues/%d/labels", c.baseURL, repo, number)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("github: build add labels request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+cred.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("github: add labels request failed: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("github: add labels: unexpected status %d", resp.StatusCode)
		}
	}
	for _, label := range removeLabels {
		reqURL := fmt.Sprintf("%s/repos/%s/issues/%d/labels/%s", c.baseURL, repo, number, url.PathEscape(label))
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
		if err != nil {
			return fmt.Errorf("github: build remove label request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+cred.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("github: remove label request failed: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("github: remove label %q: unexpected status %d", label, resp.StatusCode)
		}
	}
	return nil
}
