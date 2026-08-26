# TASK-079: Implement GitHub Projects v2 GraphQL adapter + wire into gRPC server

**From Solution:** SOL-012 (Design — `adapter/external/github/` implementation notes, shape 3)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/adapter/github/projects.go` (new), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-075 (reuses `Client.graphQLRequest`), TASK-078
**Status:** `[x]` DONE (verified — `go build`/`go vet`/`go test ./internal/adapter/github/...` clean; `*Client` satisfies both `usecase.ScmProvider` and `usecase.GitHubProjectsProvider`)

---

## Context

Implements `usecase.GitHubProjectsProvider` on the **same** `*Client` struct
`internal/adapter/github/client.go` already defines — per SOL-012, this is
still one GitHub adapter package, just a second capability backed by the
same REST client plus the `graphQLRequest` helper TASK-075 added. The true
Projects-board operations (list/resolve/views/table/update-field/clear-field)
are GraphQL; the "BySlug" comment/label/issue-type/assignee operations are
plain REST scoped by the slug's repo (SOL-012's own signature table).

---

## Changes to make

**File:** `services/scm-integration-service/internal/adapter/github/projects.go` (new)

### Step 1: Package header, slug parsing, `var _` assertion

```go
// Package github (this file): implements usecase.GitHubProjectsProvider —
// GitHub Projects v2, GraphQL-only, on the same *Client REST adapter
// already implements usecase.ScmProvider (client.go). See SOL-012 "Design —
// adapter/external/github/ implementation notes".
package github

import (
	"context"
	"fmt"
	"net/http"
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
```

### Step 2: `resolveProjectID` — shared node-id lookup for the GraphQL methods

```go
// projectV2Owner mirrors the "owner on Organization | User" union every
// Projects v2 GraphQL response below returns.
type projectV2Owner struct {
	Login string `json:"login"`
}

type projectV2Node struct {
	ID     string          `json:"id"`
	Number int32           `json:"number"`
	Title  string          `json:"title"`
	URL    string          `json:"url"`
	Owner  projectV2Owner  `json:"owner"`
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
```

### Step 3: `ListAccessibleProjects` / `ResolveProjectRef` / `ListProjectViews`

```go
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
```

### Step 4: `ViewProjectTable` / `UpdateProjectItemField` / `ClearProjectItemField`

```go
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
```

### Step 5: REST-backed "BySlug" methods — one full representative + a table for the rest

```go
// GetWorkItemDetailsBySlug calls GitHub's REST API: GET
// /repos/{owner}/{repo}/issues/{number} — works for both issues and PRs,
// since GitHub's issues endpoint returns PRs too (see githubIssue.PullRequest
// in client.go).
func (c *Client) GetWorkItemDetailsBySlug(ctx context.Context, cred usecase.Credential, itemSlug string) (usecase.WorkItemDetails, error) {
	repo, number, err := parseItemSlug(itemSlug)
	if err != nil {
		return usecase.WorkItemDetails{}, err
	}
	url := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: build get work item details request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: get work item details request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: get work item details: unexpected status %d", resp.StatusCode)
	}
	var raw struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return usecase.WorkItemDetails{}, fmt.Errorf("github: decode get work item details response: %w", err)
	}
	return usecase.WorkItemDetails{Slug: itemSlug, Title: raw.Title, Body: raw.Body, State: raw.State, URL: raw.HTMLURL}, nil
}
```

Add `"encoding/json"` to the import block.

The remaining 9 "BySlug" methods follow this exact
parse-slug → REST call → map-to-usecase-type shape (all scoped by the
slug's repo, all reusing `parseItemSlug` above and the same
header-setting convention as `GetWorkItemDetailsBySlug` and `client.go`'s
existing methods):

| Method | GitHub REST endpoint | Notes |
|---|---|---|
| `UpdateIssueBySlug` | `PATCH /repos/{repo}/issues/{number}` + label add/remove | Same body/label-delta shape as `UpdateIssue` (TASK-075) — reuse that logic against `WorkItemPatch` instead of `usecase.IssuePatch`, return via `GetWorkItemDetailsBySlug` after |
| `UpdatePullRequestBySlug` | `PATCH /repos/{repo}/pulls/{number}` | title/body/state only (no labels on PRs via this endpoint) |
| `UpdateIssueTypeBySlug` | `PATCH /repos/{repo}/issues/{number}` with `{"type": issueType}` | GitHub's issue-type field, org-scoped feature |
| `ListIssueTypesBySlug` | `GET /orgs/{org}/issue-types` | `org` = the slug's repo owner segment |
| `ListAssignableUsersBySlug` | `GET /repos/{repo}/assignees` | |
| `ListLabelsBySlug` | `GET /repos/{repo}/labels` | |
| `AddIssueCommentBySlug` | `POST /repos/{repo}/issues/{number}/comments` | body `{"body": body}`; response maps to `usecase.ProjectComment{ID, Body, Author: user.login, URL: html_url}` |
| `UpdateIssueCommentBySlug` | `PATCH /repos/{repo}/issues/comments/{commentId}` | `commentId` is NOT repo-number-scoped — GitHub addresses comments by a global id, passed straight through from `commentID` |
| `DeleteIssueCommentBySlug` | `DELETE /repos/{repo}/issues/comments/{commentId}` | returns only `error`; a 204 or 404 (already deleted) is success |

Write each following `GetWorkItemDetailsBySlug`'s and `client.go`'s
`UpdateIssue`/`ListIssues` exact structure (build request, set headers,
`c.httpClient.Do`, check status, decode, map to the `usecase` type).

---

## Wire into `internal/adapter/grpc/server.go` and `cmd/server/main.go`

### `server.go`

Add 16 fields to `Server` (one `*usecase.<Type>` per Step-2/3-of-TASK-078
usecase), 16 constructor parameters, and 16 RPC methods following
`UpdateProjectItemField`'s shape:

```go
func (s *Server) UpdateProjectItemField(ctx context.Context, req *scmintegrationv1.UpdateProjectItemFieldRequest) (*scmintegrationv1.ProjectItem, error) {
	item, err := s.updateProjectItemField.Execute(ctx, usecase.UpdateProjectItemFieldParams{
		TenantID: req.GetTenantId(), Provider: domain.ScmProviderGitHub,
		ProjectSlug: req.GetProjectSlug(), ItemID: req.GetItemId(),
		Field: usecase.ProjectFieldValue{FieldID: req.GetField().GetFieldId(), Kind: req.GetField().GetKind(), Value: req.GetField().GetValue()},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoProjectItem(item), nil
}

func toProtoProjectItem(item usecase.ProjectItem) *scmintegrationv1.ProjectItem {
	fields := make([]*scmintegrationv1.ProjectFieldValue, 0, len(item.Fields))
	for _, f := range item.Fields {
		fields = append(fields, &scmintegrationv1.ProjectFieldValue{FieldId: f.FieldID, Kind: f.Kind, Value: f.Value})
	}
	return &scmintegrationv1.ProjectItem{Id: item.ID, Title: item.Title, ContentType: item.ContentType, ContentUrl: item.ContentURL, Fields: fields}
}
```

`Provider: domain.ScmProviderGitHub` is hardcoded per RPC (not read from
`req.GetProvider()`) only for the 16 Projects v2 RPCs — TASK-077's proto
messages for this sub-surface deliberately have **no** `provider` field
(GitHub-only by construction, per SOL-012's framing); every other new
`toProto*`/mapping helper here follows the exact same
request-field-to-usecase-param mapping already established in TASK-076's
`server.go` additions. Write the remaining 15 RPC methods the same way,
each converting its request message 1:1 into the matching usecase's
`*Params` struct and its response type via a `toProto<Type>` helper mirroring
`toProtoProjectItem` above (`toProtoProject`, `toProtoProjectView`,
`toProtoWorkItemDetails`, `toProtoIssueType`, `toProtoAssignableUser`,
`toProtoLabel`, `toProtoProjectComment`).

### `main.go`

After TASK-076's usecase construction block, add:

```go
	listAccessibleProjectsUC := usecase.NewListAccessibleProjects(credentials, githubProjectsAdapter)
	resolveProjectRefUC := usecase.NewResolveProjectRef(credentials, githubProjectsAdapter)
	listProjectViewsUC := usecase.NewListProjectViews(credentials, githubProjectsAdapter)
	viewProjectTableUC := usecase.NewViewProjectTable(credentials, githubProjectsAdapter)
	updateProjectItemFieldUC := usecase.NewUpdateProjectItemField(credentials, githubProjectsAdapter)
	clearProjectItemFieldUC := usecase.NewClearProjectItemField(credentials, githubProjectsAdapter)
	getWorkItemDetailsBySlugUC := usecase.NewGetWorkItemDetailsBySlug(credentials, githubProjectsAdapter)
	updateIssueBySlugUC := usecase.NewUpdateIssueBySlug(credentials, githubProjectsAdapter)
	updatePullRequestBySlugUC := usecase.NewUpdatePullRequestBySlug(credentials, githubProjectsAdapter)
	updateIssueTypeBySlugUC := usecase.NewUpdateIssueTypeBySlug(credentials, githubProjectsAdapter)
	listIssueTypesBySlugUC := usecase.NewListIssueTypesBySlug(credentials, githubProjectsAdapter)
	listAssignableUsersBySlugUC := usecase.NewListAssignableUsersBySlug(credentials, githubProjectsAdapter)
	listLabelsBySlugUC := usecase.NewListLabelsBySlug(credentials, githubProjectsAdapter)
	addIssueCommentBySlugUC := usecase.NewAddIssueCommentBySlug(credentials, githubProjectsAdapter)
	updateIssueCommentBySlugUC := usecase.NewUpdateIssueCommentBySlug(credentials, githubProjectsAdapter)
	deleteIssueCommentBySlugUC := usecase.NewDeleteIssueCommentBySlug(credentials, githubProjectsAdapter)
```

where `githubProjectsAdapter` is the **same** `*github.Client` instance
already constructed for `registry` — find:

```go
	registry := providerregistry.New(map[domain.ScmProvider]usecase.ScmProvider{
		domain.ScmProviderGitHub:      scmgithub.New(nil, cfg.GitHubBaseURL),
```

Replace with:

```go
	githubProjectsAdapter := scmgithub.New(nil, cfg.GitHubBaseURL)
	registry := providerregistry.New(map[domain.ScmProvider]usecase.ScmProvider{
		domain.ScmProviderGitHub:      githubProjectsAdapter,
```

(reusing the variable in place of the inline `scmgithub.New(...)` call —
same underlying `*Client`, satisfying both `usecase.ScmProvider` and
`usecase.GitHubProjectsProvider` from one instance, per SOL-012's "still
one ScmProvider(+GitHubProjectsProvider) implementation" design note).

Then pass all 16 new usecases into `scmgrpc.New(...)` alongside TASK-076's
7, and add matching parameters/fields to `Server`/`New` in `server.go`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... && go vet ./...
go test ./internal/adapter/github/... -count=1
```

Expected: clean build; `*Client` satisfies both `usecase.ScmProvider` and
`usecase.GitHubProjectsProvider`; `Server` satisfies the generated
`ScmIntegrationServiceServer` interface in full.
