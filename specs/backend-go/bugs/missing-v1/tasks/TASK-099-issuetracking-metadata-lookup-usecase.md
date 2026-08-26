# TASK-099: Metadata-lookup usecase group (`ListProjects`/`ListIssueTypes`/`ListCreateFields`/`ListAssignableUsers`/`ListPriorities`/`ListTransitions`/`GetProjectStatusOrder`)

**From Solution:** SOL-015
**Priority:** P1
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/{domain,usecase,adapter/jira,adapter/grpc}/*.go`
**Depends on:** TASK-096, TASK-097, TASK-098
**Status:** `[x]` DONE (verified) — all 7 usecases (list_projects/list_issue_types/list_create_fields/list_assignable_users/list_priorities/list_transitions/get_project_status_order) + jira/client.go implementations + grpc handlers wired. `go build`/`go vet`/`go test` clean. `GetProjectStatusOrder` intentionally returns a documented "not yet implemented" error (no live Jira Cloud site available to confirm the Agile-API response shape) — matches the task's own sketch.

---

## Context

7 read-only lookups the frontend's Jira create/edit-issue forms need
(project list, issue types, dynamic create-screen fields, assignable
users) plus the 3 scope additions SOL-015 flagged beyond the TDD
(`ListPriorities`, `ListTransitions`, `GetProjectStatusOrder`). All 7 share
one shape: resolve credential, call one `IssueTrackerProvider` method, no
mutation, no retry concerns. Grouped into one task per the assignment's
"group these, don't write 7 separate tasks" instruction.

## Changes to make

### 1. `internal/domain/issue.go` — add `CreateField`, `Transition`, `ProjectStatusOrder`

```go
type CreateFieldOption struct {
	ID    string
	Value string
	Name  string
}

type CreateField struct {
	Key           string
	Name          string
	Required      bool
	SchemaType    string
	SchemaItems   string
	SchemaCustom  string
	AllowedValues []CreateFieldOption
}

type Transition struct {
	ID   string
	Name string
	To   WorkflowState
}

// ProjectStatusOrder is Jira's per-project Kanban column grouping — a list
// of columns, each an ordered list of status ids in that column. No Linear
// equivalent (Linear's ListWorkflowStates already returns an ordered flat
// list — see TASK-105's teamStates).
type ProjectStatusOrder struct {
	StatusIDsByColumn [][]string
}
```

### 2. `internal/usecase/ports.go` — extend `IssueTrackerProvider`

```go
type IssueTrackerProvider interface {
	// ... TASK-098's methods, plus:
	ListProjects(ctx context.Context, cred Credential, workspaceID string) ([]domain.ProjectRef, error)
	ListIssueTypes(ctx context.Context, cred Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error)
	ListCreateFields(ctx context.Context, cred Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error)
	ListAssignableUsers(ctx context.Context, cred Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error)
	ListPriorities(ctx context.Context, cred Credential) ([]domain.PriorityRef, error)
	ListTransitions(ctx context.Context, cred Credential, issueID string) ([]domain.Transition, error)
	GetProjectStatusOrder(ctx context.Context, cred Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error)
}
```

### 3. `internal/usecase/list_projects.go`, `list_issue_types.go`, `list_create_fields.go`, `list_assignable_users.go`, `list_priorities.go`, `list_transitions.go`, `get_project_status_order.go` (new)

One usecase struct per method, all sharing this shape (shown for
`ListCreateFields`, the representative case SOL-015's own design section
sketches):

```go
// list_create_fields.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListCreateFieldsInput struct {
	ProjectIDOrKey string
	IssueTypeID    string
	WorkspaceID    string
}

// ListCreateFields is Jira-only — Linear has no dynamic per-issue-type
// create-screen field concept, so this usecase always resolves against
// domain.ProviderJira, never a caller-supplied provider. Mirrors
// jira/client.go's existing internal listIssueTypes helper's own doc
// comment anticipating this exact RPC.
type ListCreateFields struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListCreateFields(registry ProviderRegistry, credentials CredentialResolver) *ListCreateFields {
	return &ListCreateFields{registry: registry, credentials: credentials}
}

func (uc *ListCreateFields) Execute(ctx context.Context, in ListCreateFieldsInput) ([]domain.CreateField, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderJira)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for jira", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderJira, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no jira credential available", err)
	}
	fields, err := provider.ListCreateFields(ctx, cred, in.ProjectIDOrKey, in.IssueTypeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_CREATE_FIELDS_FAILED", "failed to list create fields", err)
	}
	return fields, nil
}
```

`ListIssueTypes` (`ListIssueTypesInput{ProjectIDOrKey, WorkspaceID}`, code
`ISSUETRACKING_LIST_ISSUE_TYPES_FAILED`) and `ListPriorities`
(`ListPrioritiesInput{WorkspaceID}`, code
`ISSUETRACKING_LIST_PRIORITIES_FAILED`) and `GetProjectStatusOrder`
(`GetProjectStatusOrderInput{ProjectIDOrKey, WorkspaceID}`, code
`ISSUETRACKING_GET_STATUS_ORDER_FAILED`) are Jira-only the same way —
always `domain.ProviderJira`, never a caller-supplied provider field (Jira
issue types / global priorities / Kanban column order have no Linear
analog in this proto).

`ListProjects` and `ListAssignableUsers` ARE provider-parameterized
(`linear.listTeams`... no — `linear.listProjects` doesn't exist per
SOL-016's own table, but `ListAssignableUsers` is explicitly shared with
Linear per SOL-015/SOL-016's mapping tables), so their `Input` structs
carry a `Provider domain.Provider` field and their `Execute` validates it
with `in.Provider.Valid()` before resolving, matching `ListIssues`'s shape
exactly (not the Jira-only shape above):

```go
// list_projects.go
type ListProjectsInput struct {
	Provider    domain.Provider
	WorkspaceID string
}
// Execute: in.Provider.Valid() check, registry.Resolve(in.Provider),
// credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID),
// provider.ListProjects(ctx, cred, in.WorkspaceID), code
// ISSUETRACKING_LIST_PROJECTS_FAILED.

// list_assignable_users.go
type ListAssignableUsersInput struct {
	Provider       domain.Provider
	ProjectIDOrKey string
	IssueID        string
	WorkspaceID    string
}
// Execute: same provider-parameterized shape, code
// ISSUETRACKING_LIST_ASSIGNABLE_USERS_FAILED.
```

`ListTransitions` is provider-parameterized too (SOL-016's table lists no
Linear `teamStates`-equivalent reuse of it, but the RPC itself is
provider-agnostic in the proto — Linear's issue transitions are just its
workflow states reachable from the current one; TASK-104/105 may choose not
to wire a `linear.listTransitions` channel since BUG-016's 19-method list
doesn't include one, but the usecase stays provider-agnostic for
consistency with the proto):

```go
// list_transitions.go
type ListTransitionsInput struct {
	Provider    domain.Provider
	IssueID     string
	WorkspaceID string
}
// Execute: same provider-parameterized shape, code
// ISSUETRACKING_LIST_TRANSITIONS_FAILED.
```

### 4. `internal/adapter/jira/client.go` — implement the 7 methods

```go
func (c *Client) ListProjects(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.ProjectRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/project/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list projects request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list projects request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list projects", resp)
	}
	var parsed struct {
		Values []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list projects response: %w", err)
	}
	out := make([]domain.ProjectRef, 0, len(parsed.Values))
	for _, p := range parsed.Values {
		out = append(out, domain.ProjectRef{ID: p.ID, Key: p.Key, Name: p.Name, WorkspaceID: workspaceID})
	}
	return out, nil
}

// ListIssueTypes is the exported wrapper this file's TODO (see the
// defaultIssueType doc comment) anticipated — thin wrapper over the
// existing unexported listIssueTypes, now reachable from the gRPC surface.
func (c *Client) ListIssueTypes(ctx context.Context, cred usecase.Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error) {
	types, err := c.listIssueTypes(ctx, cred, projectIDOrKey)
	if err != nil {
		return nil, err
	}
	out := make([]domain.IssueTypeRef, 0, len(types))
	for _, t := range types {
		out = append(out, domain.IssueTypeRef{ID: t.ID, Name: t.Name, Subtask: t.Subtask})
	}
	return out, nil
}

func (c *Client) ListCreateFields(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/createmeta/" +
		url.PathEscape(projectIDOrKey) + "/issuetypes/" + url.PathEscape(issueTypeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list create fields request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list create fields request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list create fields", resp)
	}
	var parsed struct {
		Fields []struct {
			FieldID  string `json:"fieldId"`
			Name     string `json:"name"`
			Required bool   `json:"required"`
			Schema   struct {
				Type   string `json:"type"`
				Items  string `json:"items"`
				Custom string `json:"custom"`
			} `json:"schema"`
			AllowedValues []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
				Name  string `json:"name"`
			} `json:"allowedValues"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list create fields response: %w", err)
	}
	out := make([]domain.CreateField, 0, len(parsed.Fields))
	for _, f := range parsed.Fields {
		cf := domain.CreateField{
			Key: f.FieldID, Name: f.Name, Required: f.Required,
			SchemaType: f.Schema.Type, SchemaItems: f.Schema.Items, SchemaCustom: f.Schema.Custom,
		}
		for _, av := range f.AllowedValues {
			cf.AllowedValues = append(cf.AllowedValues, domain.CreateFieldOption{ID: av.ID, Value: av.Value, Name: av.Name})
		}
		out = append(out, cf)
	}
	return out, nil
}

func (c *Client) ListAssignableUsers(ctx context.Context, cred usecase.Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u, _ := url.Parse(strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/user/assignable/search")
	q := u.Query()
	if issueID != "" {
		q.Set("issueKey", issueID)
	} else {
		q.Set("project", projectIDOrKey)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list assignable users request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list assignable users request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list assignable users", resp)
	}
	var parsed []struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
		AvatarUrls  struct {
			Size32 string `json:"32x32"`
		} `json:"avatarUrls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list assignable users response: %w", err)
	}
	out := make([]domain.UserRef, 0, len(parsed))
	for _, u := range parsed {
		out = append(out, domain.UserRef{ID: u.AccountID, DisplayName: u.DisplayName, Email: u.EmailAddress, AvatarURL: u.AvatarUrls.Size32})
	}
	return out, nil
}

func (c *Client) ListPriorities(ctx context.Context, cred usecase.Credential) ([]domain.PriorityRef, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/priority"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list priorities request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list priorities request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list priorities", resp)
	}
	var parsed []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list priorities response: %w", err)
	}
	out := make([]domain.PriorityRef, 0, len(parsed))
	for _, p := range parsed {
		out = append(out, domain.PriorityRef{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

func (c *Client) ListTransitions(ctx context.Context, cred usecase.Credential, issueID string) ([]domain.Transition, error) {
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/issue/" + url.PathEscape(issueID) + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: building list transitions request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: list transitions request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, jiraStatusError("list transitions", resp)
	}
	var parsed struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("jira: decoding list transitions response: %w", err)
	}
	out := make([]domain.Transition, 0, len(parsed.Transitions))
	for _, t := range parsed.Transitions {
		out = append(out, domain.Transition{
			ID: t.ID, Name: t.Name,
			To: domain.WorkflowState{ID: t.To.ID, Name: t.To.Name, Category: t.To.StatusCategory.Key},
		})
	}
	return out, nil
}

func (c *Client) GetProjectStatusOrder(ctx context.Context, cred usecase.Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error) {
	// Jira Cloud has no single "column order" REST endpoint — this reads
	// the project's board configuration via the Agile API
	// (/rest/agile/1.0/board?projectKeyOrId=...  then
	// /rest/agile/1.0/board/{id}/configuration's columnConfig.columns,
	// each column's statuses list giving one entry of
	// StatusIdsByColumn). Left as a documented gap rather than guessed at:
	// TODO wire the real /rest/agile/1.0 board-lookup chain once a live
	// Jira Cloud site is available to confirm the exact response shape —
	// same "no live system to confirm against" caveat
	// relay_executor.go's doc comment already established as this
	// codebase's convention for exactly this situation.
	return domain.ProjectStatusOrder{}, fmt.Errorf("jira: GetProjectStatusOrder not yet implemented — see TODO in this method")
}
```

### 5. `internal/adapter/grpc/server.go` — wire the 7 new RPCs

Same translate-request → call-usecase → `apperrors.ToGRPCStatus` →
translate-response shape as every other handler in this file. Add
`toProtoProjects`/`toProtoIssueTypes`/`toProtoCreateFields`/
`toProtoUsers`/`toProtoPriorities`/`toProtoTransitions`/
`toProtoStatusOrder` translation helpers next to the existing
`toProtoIssue`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/... 2>&1 | head -50
go vet ./services/issue-tracking-service/internal/usecase/... ./services/issue-tracking-service/internal/adapter/jira/...
```

Expected: still fails inside `internal/adapter/linear/client.go` (TASK-104/
TASK-105 fix it) — everything else compiles.
