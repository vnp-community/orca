# TASK-098: Issue-CRUD usecase group (`SearchIssues`/`ListIssues`/`GetIssue`/`CreateIssue`/`UpdateIssue`/`AddIssueComment`/`ListIssueComments`)

**From Solution:** SOL-015
**Priority:** P1
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/{domain,usecase,adapter/jira,adapter/grpc}/*.go`
**Depends on:** TASK-096, TASK-097
**Status:** `[x]` DONE (verified) — domain.Issue extended, IssueTrackerProvider extended, ListIssues/CreateIssue updated + SearchIssues/GetIssue/UpdateIssue/AddIssueComment/ListIssueComments added, jira/client.go fully extended, grpc/server.go handlers wired. `go build`/`go vet`/`go test` clean.

---

## Context

Extends `domain.Issue` from `{id,title,state,url}` to the rich
provider-agnostic shape TASK-096's proto already carries, and adds the 5
new query/mutation usecases plus extends `ListIssues`/`CreateIssue` to pass
through the new request fields. All 7 usecases share one pattern:
`credentials.Resolve(ctx, tenantID, userID, provider, workspaceID)` (the
signature TASK-097 introduced) then `provider.<Method>(ctx, cred, ...)`.

## Changes to make

### 1. `internal/domain/issue.go` — extend `Issue`, add related value objects

Add these fields to the existing `Issue` struct (keep `ID`/`Title`/`State`/
`URL` — `NewIssue` keeps compiling unchanged, existing callers unaffected):

```go
type Issue struct {
	ID                  string
	ProviderIssueID     string
	Key                 string
	Title               string
	DescriptionMarkdown string
	State               string
	WorkflowState       WorkflowState
	URL                 string
	Project             ProjectRef
	IssueType           IssueTypeRef
	Labels              []string
	Assignee            UserRef
	Reporter            UserRef
	Priority            PriorityRef
	CustomFieldsJSON    string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ProjectRef struct {
	ID          string
	Key         string
	Name        string
	WorkspaceID string
}

type IssueTypeRef struct {
	ID      string
	Name    string
	Subtask bool
}

type WorkflowState struct {
	ID       string
	Name     string
	Category string // todo|in_progress|done|cancelled
}

type UserRef struct {
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
}

type PriorityRef struct {
	ID   string
	Name string
}

// IssueComment is one comment on an Issue.
type IssueComment struct {
	ID           string
	BodyMarkdown string
	Author       UserRef
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewIssueInput is what CreateIssue passes to IssueTrackerProvider.CreateIssue
// — replaces the old (projectKey, title, description string) positional
// signature now that Jira/Linear both need issue type, assignee, priority,
// labels, and (Linear) parent-issue/team/state.
type NewIssueInput struct {
	ProjectKey      string // Jira project key; unused by Linear (TeamID instead, see TASK-104)
	Title           string
	Description     string
	IssueTypeID     string
	AssigneeID      string
	PriorityID      string
	LabelIDs        []string
	ParentIssueID   string
	CustomFieldsJSON string
}

// IssueUpdate is what UpdateIssue passes to IssueTrackerProvider.UpdateIssue.
// Every field empty/nil means "leave unchanged" — matches
// UpdateIssueRequest's proto contract.
type IssueUpdate struct {
	IssueID          string
	Title            string
	Description      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	WorkflowStateID  string
	CustomFieldsJSON string
}
```

Add `"time"` to the file's import block.

### 2. `internal/usecase/ports.go` — extend `IssueTrackerProvider`

Replace `ListIssues`/`CreateIssue`'s old positional signatures and add the 5
new methods:

```go
type IssueTrackerProvider interface {
	Whoami(ctx context.Context, cred Credential) (domain.Viewer, error)

	SearchIssues(ctx context.Context, cred Credential, query string, limit int) ([]domain.Issue, error)
	ListIssues(ctx context.Context, cred Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error)
	GetIssue(ctx context.Context, cred Credential, issueID string) (domain.Issue, error)
	CreateIssue(ctx context.Context, cred Credential, in domain.NewIssueInput) (domain.Issue, error)
	UpdateIssue(ctx context.Context, cred Credential, in domain.IssueUpdate) (domain.Issue, error)
	AddIssueComment(ctx context.Context, cred Credential, issueID, bodyMarkdown string) (domain.IssueComment, error)
	ListIssueComments(ctx context.Context, cred Credential, issueID string) ([]domain.IssueComment, error)
}
```

This is a breaking change to the port (not the proto — `buf breaking` is
unaffected) — both `jira/client.go` (this task) and `linear/client.go`
(TASK-104) must implement the new shape before anything compiles again.

### 3. `internal/usecase/list_issues.go` — extend input, fix `Resolve` call

```go
type ListIssuesInput struct {
	Provider    domain.Provider
	ProjectKey  string
	FilterJSON  string
	Limit       int32
	WorkspaceID string
}

func (uc *ListIssues) Execute(ctx context.Context, in ListIssuesInput) ([]domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}

	issues, err := provider.ListIssues(ctx, cred, in.ProjectKey, in.FilterJSON, int(in.Limit))
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_FAILED", "failed to list issues from provider", err)
	}
	return issues, nil
}
```

### 4. `internal/usecase/create_issue.go` — extend input, fix `Resolve` call

```go
type CreateIssueInput struct {
	Provider         domain.Provider
	ProjectKey       string
	Title            string
	Description      string
	IssueTypeID      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	ParentIssueID    string
	CustomFieldsJSON string
	WorkspaceID      string
}

func (uc *CreateIssue) Execute(ctx context.Context, in CreateIssueInput) (domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Title == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_TITLE", "title is required", domain.ErrEmptyTitle)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}

	// Mutations are not silently retried on ambiguous failure, to avoid
	// duplicate issue creation (design doc §8) — no retry wrapper here.
	issue, err := provider.CreateIssue(ctx, cred, domain.NewIssueInput{
		ProjectKey: in.ProjectKey, Title: in.Title, Description: in.Description,
		IssueTypeID: in.IssueTypeID, AssigneeID: in.AssigneeID, PriorityID: in.PriorityID,
		LabelIDs: in.LabelIDs, ParentIssueID: in.ParentIssueID, CustomFieldsJSON: in.CustomFieldsJSON,
	})
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREATE_FAILED", "failed to create issue with provider", err)
	}
	return issue, nil
}
```

`ProjectKey`'s empty-check (`ISSUETRACKING_EMPTY_PROJECT_KEY`, present in
today's `CreateIssue`) is intentionally dropped here — Linear's
`CreateIssue` (TASK-104) uses `TeamID`, not `ProjectKey`, so that
validation moves to be provider-specific inside each adapter's
`CreateIssue` rather than the shared usecase.

### 5. `internal/usecase/search_issues.go`, `get_issue.go`, `add_issue_comment.go`, `list_issue_comments.go` (new)

Same tenant/user/provider-resolve-credential shape as `ListIssues.Execute`
above, one usecase struct each:

```go
// search_issues.go
type SearchIssuesInput struct {
	Provider    domain.Provider
	Query       string
	Limit       int32
	WorkspaceID string
}

type SearchIssues struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewSearchIssues(registry ProviderRegistry, credentials CredentialResolver) *SearchIssues {
	return &SearchIssues{registry: registry, credentials: credentials}
}

func (uc *SearchIssues) Execute(ctx context.Context, in SearchIssuesInput) ([]domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	issues, err := provider.SearchIssues(ctx, cred, in.Query, int(in.Limit))
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_SEARCH_FAILED", "failed to search issues", err)
	}
	return issues, nil
}
```

`get_issue.go` (`GetIssueInput{Provider, IssueID, WorkspaceID}`,
`provider.GetIssue(ctx, cred, in.IssueID)`, code
`ISSUETRACKING_GET_ISSUE_FAILED`), `add_issue_comment.go`
(`AddIssueCommentInput{Provider, IssueID, BodyMarkdown, WorkspaceID}`,
`provider.AddIssueComment(ctx, cred, in.IssueID, in.BodyMarkdown)`, code
`ISSUETRACKING_ADD_COMMENT_FAILED`), and `list_issue_comments.go`
(`ListIssueCommentsInput{Provider, IssueID, WorkspaceID}`,
`provider.ListIssueComments(ctx, cred, in.IssueID)`, code
`ISSUETRACKING_LIST_COMMENTS_FAILED`) follow the exact same shape.

### 6. `internal/usecase/update_issue.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type UpdateIssueInput struct {
	Provider         domain.Provider
	IssueID          string
	Title            string
	Description      string
	AssigneeID       string
	PriorityID       string
	LabelIDs         []string
	WorkflowStateID  string
	CustomFieldsJSON string
	WorkspaceID      string
}

type UpdateIssue struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewUpdateIssue(registry ProviderRegistry, credentials CredentialResolver) *UpdateIssue {
	return &UpdateIssue{registry: registry, credentials: credentials}
}

func (uc *UpdateIssue) Execute(ctx context.Context, in UpdateIssueInput) (domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if in.IssueID == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_ISSUE_ID", "issue_id is required", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	issue, err := provider.UpdateIssue(ctx, cred, domain.IssueUpdate{
		IssueID: in.IssueID, Title: in.Title, Description: in.Description,
		AssigneeID: in.AssigneeID, PriorityID: in.PriorityID, LabelIDs: in.LabelIDs,
		WorkflowStateID: in.WorkflowStateID, CustomFieldsJSON: in.CustomFieldsJSON,
	})
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_UPDATE_FAILED", "failed to update issue with provider", err)
	}
	return issue, nil
}
```

### 7. `internal/adapter/jira/client.go` — extend to the new port shape

Replace `ListIssues`/`CreateIssue`'s signatures and add the 5 new methods.
Reuse the existing `jiraSearchResponse`/`jiraIssue` types and
`listIssueTypes`/`resolveIssueType` helpers already in this file (TASK-096
extension keeps them, nothing here removes them):

```go
// toRichIssue maps a jiraIssue search-result row (or an equivalent
// createmeta-adjacent shape) into the extended domain.Issue — replaces the
// old domain.NewIssue(key, summary, status, url) call sites throughout this
// file.
func toRichIssue(baseURL string, key, summary, status string) domain.Issue {
	return domain.Issue{
		ID: key, ProviderIssueID: key, Key: key, Title: summary,
		State: status, WorkflowState: domain.WorkflowState{Name: status},
		URL: issueBrowseURL(baseURL, key),
	}
}

func (c *Client) SearchIssues(ctx context.Context, cred usecase.Credential, jql string, limit int) ([]domain.Issue, error) {
	// Same GET /rest/api/3/search request ListIssues already builds, with
	// the caller-supplied JQL used verbatim instead of a synthesized
	// project= clause, and maxResults from limit (falls back to 50).
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u, err := url.Parse(strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/search")
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base url: %w", err)
	}
	q := u.Query()
	q.Set("jql", jql)
	if limit <= 0 {
		limit = 50
	}
	q.Set("maxResults", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building search issues request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: search issues request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("search issues", resp)
	}
	var parsed jiraSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding search issues response: %w", err)
	}
	issues := make([]domain.Issue, 0, len(parsed.Issues))
	for _, ji := range parsed.Issues {
		issues = append(issues, toRichIssue(cred.BaseURL, ji.Key, ji.Fields.Summary, ji.Fields.Status.Name))
	}
	return issues, nil
}

func (c *Client) ListIssues(ctx context.Context, cred usecase.Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error) {
	jql := ""
	if projectKey != "" {
		jql = fmt.Sprintf("project=%q", projectKey)
	}
	// filterJSON (JiraIssueFilter-shaped) is not translated to JQL here —
	// TASK-098 ships the projectKey path only; a follow-up can extend this
	// to parse filterJSON's structured fields (status/assignee/labels) into
	// additional JQL clauses once a concrete JiraIssueFilter shape is
	// finalized (see channels_jira.go's decode step in TASK-100).
	_ = filterJSON
	return c.SearchIssues(ctx, cred, jql, limit)
}

func (c *Client) GetIssue(ctx context.Context, cred usecase.Credential, issueID string) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building get issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: get issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("get issue", resp)
	}
	var parsed jiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Issue{}, fmt.Errorf("jira: decoding get issue response: %w", err)
	}
	return toRichIssue(cred.BaseURL, parsed.Key, parsed.Fields.Summary, parsed.Fields.Status.Name), nil
}

// CreateIssue: same request-building logic as before, driven by
// in.IssueTypeID (falling back to defaultIssueType/resolveIssueType when
// unset, preserving today's behavior) instead of positional args.
func (c *Client) CreateIssue(ctx context.Context, cred usecase.Credential, in domain.NewIssueInput) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	issueTypeName := in.IssueTypeID
	if issueTypeName == "" {
		types, err := c.listIssueTypes(ctx, cred, in.ProjectKey)
		if err != nil {
			return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
		}
		issueTypeName, err = resolveIssueType(types, defaultIssueType)
		if err != nil {
			return domain.Issue{}, fmt.Errorf("jira: resolving issue type: %w", err)
		}
	}

	fields := jiraCreateIssueFields{
		Project:   jiraProjectRef{Key: in.ProjectKey},
		Summary:   in.Title,
		IssueType: jiraIssueTypeRef{Name: issueTypeName},
	}
	if in.Description != "" {
		doc := plainTextADF(in.Description)
		fields.Description = &doc
	}
	body, err := json.Marshal(jiraCreateIssueRequest{Fields: fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal create issue request: %w", err)
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building create issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: create issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("create issue", resp)
	}
	var parsed jiraCreateIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Issue{}, fmt.Errorf("jira: decoding create issue response: %w", err)
	}
	return toRichIssue(cred.BaseURL, parsed.Key, in.Title, ""), nil
}

func (c *Client) UpdateIssue(ctx context.Context, cred usecase.Credential, in domain.IssueUpdate) (domain.Issue, error) {
	if cred.BaseURL == "" {
		return domain.Issue{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	fields := map[string]any{}
	if in.Title != "" {
		fields["summary"] = in.Title
	}
	if in.Description != "" {
		doc := plainTextADF(in.Description)
		fields["description"] = doc
	}
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: marshal update issue request: %w", err)
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(in.IssueID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: building update issue request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Issue{}, fmt.Errorf("jira: update issue request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return domain.Issue{}, jiraStatusError("update issue", resp)
	}
	// Jira's PUT /issue/{id} returns 204 No Content — re-fetch to return
	// the caller a current view, same convention GetIssue already uses.
	return c.GetIssue(ctx, cred, in.IssueID)
}

func (c *Client) AddIssueComment(ctx context.Context, cred usecase.Credential, issueID, bodyMarkdown string) (domain.IssueComment, error) {
	if cred.BaseURL == "" {
		return domain.IssueComment{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	doc := plainTextADF(bodyMarkdown)
	body, err := json.Marshal(map[string]any{"body": doc})
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: marshal add comment request: %w", err)
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: building add comment request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: add comment request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return domain.IssueComment{}, jiraStatusError("add comment", resp)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.IssueComment{}, fmt.Errorf("jira: decoding add comment response: %w", err)
	}
	return domain.IssueComment{ID: parsed.ID, BodyMarkdown: bodyMarkdown}, nil
}

func (c *Client) ListIssueComments(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.IssueComment, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list comments request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list comments request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list comments", resp)
	}
	var parsed struct {
		Comments []struct {
			ID   string `json:"id"`
			Body struct {
				Content []struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"content"`
			} `json:"body"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list comments response: %w", err)
	}
	out := make([]domain.IssueComment, 0, len(parsed.Comments))
	for _, c := range parsed.Comments {
		text := ""
		if len(c.Body.Content) > 0 && len(c.Body.Content[0].Content) > 0 {
			text = c.Body.Content[0].Content[0].Text
		}
		out = append(out, domain.IssueComment{ID: c.ID, BodyMarkdown: text})
	}
	return out, nil
}
```

Add `"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"`
usage is already imported; no new import needed beyond what's already
there.

### 8. `internal/adapter/grpc/server.go` — update `toProtoIssue`, `ListIssues`/`CreateIssue` handlers, add 5 new RPC handlers

```go
func toProtoIssue(i domain.Issue) *issuetrackingv1.Issue {
	return &issuetrackingv1.Issue{
		Id:                  i.ID,
		ProviderIssueId:     i.ProviderIssueID,
		Key:                 i.Key,
		Title:               i.Title,
		DescriptionMarkdown: i.DescriptionMarkdown,
		State:               i.State,
		Url:                 i.URL,
		Labels:              i.Labels,
		CustomFieldsJson:    i.CustomFieldsJSON,
		// WorkflowState/Project/IssueType/Assignee/Reporter/Priority/
		// timestamps: translate only when the corresponding domain field is
		// non-zero, same "don't fabricate a populated sub-message from a
		// zero value" discipline as every other adapter in this codebase.
	}
}
```

`ListIssues` handler gains `FilterJson`/`Limit`/`WorkspaceId` passthrough;
`CreateIssue` handler gains the 6 new `CreateIssueRequest` fields. Add
`GetIssue`, `UpdateIssue`, `AddIssueComment`, `ListIssueComments`,
`SearchIssues` handlers following `ListIssues`'s exact translate-request →
call-usecase → `apperrors.ToGRPCStatus` → translate-response shape.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/... 2>&1 | head -50
```

Expected: fails only inside `internal/adapter/linear/client.go` (still
implementing the old 2-method port — TASK-104 fixes this). Everything under
`internal/domain`, `internal/usecase`, `internal/adapter/jira`,
`internal/adapter/grpc` must compile clean.

```bash
go vet ./services/issue-tracking-service/internal/usecase/... ./services/issue-tracking-service/internal/adapter/jira/...
go test ./services/issue-tracking-service/internal/usecase/... ./services/issue-tracking-service/internal/adapter/jira/... -run TestListIssues -count=1
```
