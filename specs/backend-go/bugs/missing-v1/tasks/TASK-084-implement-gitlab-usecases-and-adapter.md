# TASK-084: Implement `GitLabMergeRequestProvider` port, usecases, GitLab REST adapter, and gRPC wiring

**From Solution:** SOL-013 (Design — `usecase/` layer, `adapter/external/gitlab/` implementation notes)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/usecase/ports.go`, `list_merge_requests.go` (new), `resolve_merge_request_discussion.go` (new), `get_work_item_details.go` (new), `internal/adapter/gitlab/client.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-083
**Status:** `[ ]` TODO

---

## Context

`ListMergeRequests`/`ResolveDiscussion`/`GetWorkItemDetails` are a
GitLab-only port, same shape/rationale as SOL-012's `GitHubProjectsProvider`
(TASK-078): the usecase talks to the injected GitLab adapter directly, no
`ProviderRegistry.Resolve(...)` fan-out, since these 3 operations don't
belong on the common `ScmProvider` interface (no other provider has GitLab's
`iid`/discussion model). `adapter/external/gitlab` reuses the existing
GitLab REST HTTP client and `RateLimit-*` header parsing already built for
`GetRateLimitStatus` — GitLab has one rate-limit bucket per token, so no
bucket-key change is needed here, unlike SOL-012's GitHub GraphQL addition.

---

## Changes to make

### Step 1: Domain types

**File:** `services/scm-integration-service/internal/domain/scm.go`

Append:

```go
// MergeRequest is GitLab's pull-request-equivalent concept — kept as its
// own domain type (not folded into PullRequest) because it carries fields
// PullRequest doesn't (source/target branch by GitLab's own names,
// discussion counts, GitLab's own merge_status vocabulary), matching
// SOL-013's proto-level MergeRequest message 1:1.
type MergeRequest struct {
	ID                         string
	Repo                       string
	State                      string
	IID                        int32
	Title                      string
	SourceBranch               string
	TargetBranch               string
	Draft                      bool
	DiscussionCount            int32
	UnresolvedDiscussionCount  int32
	MergeStatus                string
}

// MergeRequestDiscussion mirrors scmintegrationv1.MergeRequestDiscussion.
type MergeRequestDiscussion struct {
	ID         string
	Resolved   bool
	ResolvedBy string
}

// WorkItemDetailsGitLab mirrors scmintegrationv1.WorkItemDetailsGitLab —
// named with the GitLab suffix to avoid colliding with SOL-012's
// provider-agnostic WorkItemDetails (usecase package, GitHub Projects v2).
type WorkItemDetailsGitLab struct {
	ID       string
	IID      int32
	ItemType string
	Title    string
	Body     string
	State    string
	URL      string
	Labels   []string
}
```

### Step 2: `GitLabMergeRequestProvider` port

**File:** `services/scm-integration-service/internal/usecase/ports.go`

Append:

```go
// MRFilter narrows a ListMergeRequests call — mirrors IssueFilter's shape.
type MRFilter struct {
	State        string
	SourceBranch string
}

// GitLabMergeRequestProvider is a GitLab-only port, same reasoning as
// GitHubProjectsProvider (see this file's GitHubProjectsProvider doc
// comment): these 3 operations don't belong on the common ScmProvider
// interface since no other provider implements them.
type GitLabMergeRequestProvider interface {
	ListMergeRequests(ctx context.Context, cred Credential, repo string, filter MRFilter) ([]domain.MergeRequest, error)
	ResolveDiscussion(ctx context.Context, cred Credential, repo string, mrIID int32, discussionID string, resolved bool) (domain.MergeRequestDiscussion, error)
	GetWorkItemDetails(ctx context.Context, cred Credential, repo string, iid int32, itemType string) (domain.WorkItemDetailsGitLab, error)
}
```

### Step 3: New usecase — `list_merge_requests.go`

**File:** `services/scm-integration-service/internal/usecase/list_merge_requests.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListMergeRequestsParams struct {
	TenantID     string
	Repo         string
	State        string
	SourceBranch string
}

type ListMergeRequests struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewListMergeRequests(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *ListMergeRequests {
	return &ListMergeRequests{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *ListMergeRequests) Execute(ctx context.Context, in ListMergeRequestsParams) ([]domain.MergeRequest, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	mrs, err := uc.gitlabMRs.ListMergeRequests(ctx, cred, in.Repo, MRFilter{State: in.State, SourceBranch: in.SourceBranch})
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_MERGE_REQUESTS_FAILED", "failed to list merge requests", err)
	}
	return mrs, nil
}
```

### Step 4: New usecase — `resolve_merge_request_discussion.go`

**File:** `services/scm-integration-service/internal/usecase/resolve_merge_request_discussion.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ResolveMergeRequestDiscussionParams struct {
	TenantID        string
	Repo            string
	MergeRequestIID int32
	DiscussionID    string
	Resolved        bool
}

type ResolveMergeRequestDiscussion struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewResolveMergeRequestDiscussion(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *ResolveMergeRequestDiscussion {
	return &ResolveMergeRequestDiscussion{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *ResolveMergeRequestDiscussion) Execute(ctx context.Context, in ResolveMergeRequestDiscussionParams) (domain.MergeRequestDiscussion, error) {
	if in.TenantID == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.DiscussionID == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_DISCUSSION_ID", "discussion_id is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	disc, err := uc.gitlabMRs.ResolveDiscussion(ctx, cred, in.Repo, in.MergeRequestIID, in.DiscussionID, in.Resolved)
	if err != nil {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInternal, "SCM_RESOLVE_MR_DISCUSSION_FAILED", "failed to resolve merge request discussion", err)
	}
	return disc, nil
}
```

### Step 5: New usecase — `get_work_item_details.go`

**File:** `services/scm-integration-service/internal/usecase/get_work_item_details.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetWorkItemDetailsParams struct {
	TenantID string
	Repo     string
	IID      int32
	ItemType string
}

type GetWorkItemDetails struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewGetWorkItemDetails(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *GetWorkItemDetails {
	return &GetWorkItemDetails{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *GetWorkItemDetails) Execute(ctx context.Context, in GetWorkItemDetailsParams) (domain.WorkItemDetailsGitLab, error) {
	if in.TenantID == "" {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	details, err := uc.gitlabMRs.GetWorkItemDetails(ctx, cred, in.Repo, in.IID, in.ItemType)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInternal, "SCM_GET_WORK_ITEM_DETAILS_FAILED", "failed to get work item details", err)
	}
	return details, nil
}
```

### Step 6: GitLab REST adapter implementation

**File:** `services/scm-integration-service/internal/adapter/gitlab/client.go`

Append (after the existing `GetRateLimitStatus` method):

```go
var _ usecase.GitLabMergeRequestProvider = (*Client)(nil)

// gitlabDiscussionCount mirrors GitLab's own field naming for
// merge_requests list responses when discussion counts are requested.
type gitlabMergeRequestFull struct {
	ID                         int    `json:"id"`
	IID                        int    `json:"iid"`
	Title                      string `json:"title"`
	State                      string `json:"state"`
	WebURL                     string `json:"web_url"`
	SourceBranch               string `json:"source_branch"`
	TargetBranch               string `json:"target_branch"`
	Draft                      bool   `json:"draft"`
	UserNotesCount             int    `json:"user_notes_count"`
	MergeStatus                string `json:"detailed_merge_status"`
}

func toDomainMergeRequest(repo string, gm gitlabMergeRequestFull) domain.MergeRequest {
	return domain.MergeRequest{
		ID: strconv.Itoa(gm.ID), Repo: repo, State: gm.State, IID: int32(gm.IID),
		Title: gm.Title, SourceBranch: gm.SourceBranch, TargetBranch: gm.TargetBranch,
		Draft: gm.Draft, DiscussionCount: int32(gm.UserNotesCount), MergeStatus: gm.MergeStatus,
	}
}

// ListMergeRequests calls GitLab's REST API: GET
// /projects/{id}/merge_requests, filtered by state/source_branch.
func (c *Client) ListMergeRequests(ctx context.Context, cred usecase.Credential, repo string, filter usecase.MRFilter) ([]domain.MergeRequest, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests", c.baseURL, projectPath(repo))
	q := url.Values{}
	if filter.State != "" {
		q.Set("state", filter.State)
	}
	if filter.SourceBranch != "" {
		q.Set("source_branch", filter.SourceBranch)
	}
	if enc := q.Encode(); enc != "" {
		reqURL += "?" + enc
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build list merge requests request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: list merge requests request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: list merge requests: unexpected status %d", resp.StatusCode)
	}
	var raw []gitlabMergeRequestFull
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("gitlab: decode list merge requests response: %w", err)
	}
	out := make([]domain.MergeRequest, 0, len(raw))
	for _, gm := range raw {
		out = append(out, toDomainMergeRequest(repo, gm))
	}
	return out, nil
}

// ResolveDiscussion calls GitLab's REST API: PUT
// /projects/{id}/merge_requests/{iid}/discussions/{discussion_id}?resolved={bool}.
func (c *Client) ResolveDiscussion(ctx context.Context, cred usecase.Credential, repo string, mrIID int32, discussionID string, resolved bool) (domain.MergeRequestDiscussion, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests/%d/discussions/%s?resolved=%t",
		c.baseURL, projectPath(repo), mrIID, url.PathEscape(discussionID), resolved)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, nil)
	if err != nil {
		return domain.MergeRequestDiscussion{}, fmt.Errorf("gitlab: build resolve discussion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.MergeRequestDiscussion{}, fmt.Errorf("gitlab: resolve discussion request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.MergeRequestDiscussion{}, fmt.Errorf("gitlab: resolve discussion: unexpected status %d", resp.StatusCode)
	}
	var raw struct {
		ID       string `json:"id"`
		Notes    []struct {
			Resolved bool `json:"resolved"`
			ResolvedBy struct {
				Username string `json:"username"`
			} `json:"resolved_by"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.MergeRequestDiscussion{}, fmt.Errorf("gitlab: decode resolve discussion response: %w", err)
	}
	disc := domain.MergeRequestDiscussion{ID: raw.ID, Resolved: resolved}
	if len(raw.Notes) > 0 {
		disc.Resolved = raw.Notes[0].Resolved
		disc.ResolvedBy = raw.Notes[0].ResolvedBy.Username
	}
	return disc, nil
}

// GetWorkItemDetails calls GitLab's REST API: GET
// /projects/{id}/merge_requests/{iid} or /projects/{id}/issues/{iid},
// selected by itemType — GitLab addresses issues and MRs by separate iid
// sequences, not a shared "work item" ID space.
func (c *Client) GetWorkItemDetails(ctx context.Context, cred usecase.Credential, repo string, iid int32, itemType string) (domain.WorkItemDetailsGitLab, error) {
	segment := "merge_requests"
	if itemType == "issue" {
		segment = "issues"
	}
	reqURL := fmt.Sprintf("%s/projects/%s/%s/%d", c.baseURL, projectPath(repo), segment, iid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, fmt.Errorf("gitlab: build get work item details request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, fmt.Errorf("gitlab: get work item details request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.WorkItemDetailsGitLab{}, fmt.Errorf("gitlab: get work item details: unexpected status %d", resp.StatusCode)
	}
	var raw struct {
		ID          int      `json:"id"`
		IID         int      `json:"iid"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		State       string   `json:"state"`
		WebURL      string   `json:"web_url"`
		Labels      []string `json:"labels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.WorkItemDetailsGitLab{}, fmt.Errorf("gitlab: decode get work item details response: %w", err)
	}
	return domain.WorkItemDetailsGitLab{
		ID: strconv.Itoa(raw.ID), IID: int32(raw.IID), ItemType: itemType,
		Title: raw.Title, Body: raw.Description, State: raw.State, URL: raw.WebURL, Labels: raw.Labels,
	}, nil
}
```

Add `"net/url"` and `"net/http"` to the import block if not already
present (both already are, per `client.go`'s existing imports — `url` is
already imported for `projectPath`).

### Step 7: gRPC server + composition root wiring

Follow TASK-076's exact pattern:
- `internal/adapter/grpc/server.go`: add `listMergeRequests
  *usecase.ListMergeRequests`, `resolveMergeRequestDiscussion
  *usecase.ResolveMergeRequestDiscussion`, `getWorkItemDetails
  *usecase.GetWorkItemDetails` fields + constructor params, plus
  `ListMergeRequests`/`ResolveMergeRequestDiscussion`/`GetWorkItemDetails`
  RPC methods mapping `req.Get*()` → `usecase.*Params` → `apperrors.ToGRPCStatus`
  on error → a `toProtoMergeRequest`/`toProtoMergeRequestDiscussion`/
  `toProtoWorkItemDetailsGitLab` mapping helper on success (same shape as
  `toProtoPullRequest`).
- `cmd/server/main.go`: construct `gitlabAdapter := scmgitlab.New(nil,
  cfg.GitLabBaseURL)` (reusing the same instance already in `registry`'s
  map, same pattern as TASK-079's `githubProjectsAdapter`), then
  `usecase.NewListMergeRequests(credentials, gitlabAdapter)` etc., and pass
  all 3 into `scmgrpc.New(...)`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... && go vet ./...
go test ./... -count=1
```

Expected: clean build; `*Client` (gitlab) satisfies both `usecase.ScmProvider`
and `usecase.GitLabMergeRequestProvider`; `Server` satisfies the generated
interface in full.
