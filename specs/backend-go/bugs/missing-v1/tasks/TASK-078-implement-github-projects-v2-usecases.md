# TASK-078: Add `GitHubProjectsProvider` port + `github.project.*` usecases

**From Solution:** SOL-012 (Design — `usecase/` layer, shape 3)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/usecase/ports.go`, `github_projects_*.go` (new, one per RPC)
**Depends on:** TASK-077
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

`GitHubProjectsProvider` is a **separate, narrower port** from `ScmProvider`
— only the GitHub adapter implements it (TASK-079). Per §4's "each provider
implements a common port" principle, Projects v2 isn't part of the
provider-generic surface at all, since no other provider has it, so it does
not belong on the interface every provider must satisfy. Every usecase here
talks to the injected `githubProjects` port directly (no
`uc.providers.For(...)` provider-keyed lookup — there's no cross-provider
fan-out to do), and rejects a non-GitHub `provider` argument with
`apperrors.KindInvalidArgument` before calling the port, matching the
existing `SCM_PROVIDER_UNSUPPORTED` convention used everywhere else in this
package (e.g. `create_pull_request.go`).

---

## Changes to make

### Step 1: Add `GitHubProjectsProvider` port

**File:** `services/scm-integration-service/internal/usecase/ports.go`

Append to the bottom of the file:

```go
// ProjectFieldValue is a generic key/value field write — mirrors
// scmintegrationv1.ProjectFieldValue 1:1 (Kind: "text" | "number" | "date" |
// "single_select" | "iteration"). See ports.go's package doc comment for why
// GitHubProjectsProvider is a separate interface from ScmProvider.
type ProjectFieldValue struct {
	FieldID string
	Kind    string
	Value   string
}

// Project, ProjectView, ProjectItem, IssueType, AssignableUser, Label,
// ProjectComment, WorkItemDetails mirror their scmintegrationv1 message
// counterparts 1:1 (TASK-077) — usecase/ stays framework-free, so these are
// distinct Go types from the generated proto ones, converted by
// internal/adapter/grpc (TASK-079).
type Project struct {
	ID     string
	Slug   string
	Title  string
	Number int32
	Owner  string
	URL    string
}

type ProjectView struct {
	ID     string
	Name   string
	Layout string
}

type ProjectItem struct {
	ID          string
	Title       string
	ContentType string
	ContentURL  string
	Fields      []ProjectFieldValue
}

type IssueType struct {
	ID          string
	Name        string
	Description string
}

type AssignableUser struct {
	Login     string
	Name      string
	AvatarURL string
}

type Label struct {
	Name        string
	Color       string
	Description string
}

type ProjectComment struct {
	ID     string
	Body   string
	Author string
	URL    string
}

type WorkItemDetails struct {
	Slug   string
	Title  string
	Body   string
	State  string
	URL    string
	Fields []ProjectFieldValue
}

// WorkItemPatch is the shared partial-update shape for
// UpdateIssueBySlug/UpdatePullRequestBySlug — nil pointer fields mean
// "leave unchanged", same convention as IssuePatch.
type WorkItemPatch struct {
	Title        *string
	Body         *string
	State        *string
	AddLabels    []string
	RemoveLabels []string
}

// GitHubProjectsProvider is a SEPARATE, narrower port than ScmProvider —
// only internal/adapter/github implements it (TASK-079). See this file's
// doc comment for why Projects v2 isn't part of the common ScmProvider
// surface at all.
type GitHubProjectsProvider interface {
	ListAccessibleProjects(ctx context.Context, cred Credential) ([]Project, error)
	ResolveProjectRef(ctx context.Context, cred Credential, owner string, number int32) (Project, error)
	ListProjectViews(ctx context.Context, cred Credential, projectSlug string) ([]ProjectView, error)
	ViewProjectTable(ctx context.Context, cred Credential, projectSlug, viewID, pageToken string, pageSize int32) (items []ProjectItem, nextPageToken string, err error)
	UpdateProjectItemField(ctx context.Context, cred Credential, projectSlug, itemID string, field ProjectFieldValue) (ProjectItem, error)
	ClearProjectItemField(ctx context.Context, cred Credential, projectSlug, itemID, fieldID string) (ProjectItem, error)
	GetWorkItemDetailsBySlug(ctx context.Context, cred Credential, itemSlug string) (WorkItemDetails, error)
	UpdateIssueBySlug(ctx context.Context, cred Credential, itemSlug string, patch WorkItemPatch) (WorkItemDetails, error)
	UpdatePullRequestBySlug(ctx context.Context, cred Credential, itemSlug string, patch WorkItemPatch) (WorkItemDetails, error)
	UpdateIssueTypeBySlug(ctx context.Context, cred Credential, itemSlug, issueType string) (WorkItemDetails, error)
	ListIssueTypesBySlug(ctx context.Context, cred Credential, itemSlug string) ([]IssueType, error)
	ListAssignableUsersBySlug(ctx context.Context, cred Credential, itemSlug string) ([]AssignableUser, error)
	ListLabelsBySlug(ctx context.Context, cred Credential, itemSlug string) ([]Label, error)
	AddIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, body string) (ProjectComment, error)
	UpdateIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, commentID, body string) (ProjectComment, error)
	DeleteIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, commentID string) error
}
```

### Step 2: New usecase — `github_projects_update_item_field.go` (full representative)

**File:** `services/scm-integration-service/internal/usecase/github_projects_update_item_field.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// UpdateProjectItemFieldParams — the gRPC adapter routes this call here
// directly rather than through ProviderRegistry.Resolve, since Projects v2
// has no cross-provider fan-out.
type UpdateProjectItemFieldParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	ProjectSlug string
	ItemID      string
	Field       ProjectFieldValue
}

type UpdateProjectItemField struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewUpdateProjectItemField(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *UpdateProjectItemField {
	return &UpdateProjectItemField{credentials: credentials, githubProjects: githubProjects}
}

func (uc *UpdateProjectItemField) Execute(ctx context.Context, in UpdateProjectItemFieldParams) (ProjectItem, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ProjectSlug == "" {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_PROJECT_SLUG", "project_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return ProjectItem{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	item, err := uc.githubProjects.UpdateProjectItemField(ctx, cred, in.ProjectSlug, in.ItemID, in.Field)
	if err != nil {
		return ProjectItem{}, apperrors.New(apperrors.KindInternal, "SCM_UPDATE_PROJECT_ITEM_FIELD_FAILED", "failed to update project item field", err)
	}
	return item, nil
}
```

### Step 3: Remaining 15 usecases — same shape, one file each

Every remaining `GitHubProjectsProvider` method gets its own usecase file,
following `UpdateProjectItemField`'s exact pipeline (reject non-GitHub
provider → validate required params → resolve credential → delegate to
`uc.githubProjects.<Method>` → wrap error with a `SCM_<SCREAMING_SNAKE>_FAILED`
code). Create these files:

| File | Type name | `Execute` params → `githubProjects` call |
|---|---|---|
| `github_projects_list_accessible.go` | `ListAccessibleProjects` | `(TenantID, Provider)` → `ListAccessibleProjects(ctx, cred)` |
| `github_projects_resolve_ref.go` | `ResolveProjectRef` | `(TenantID, Provider, Owner, Number)` → `ResolveProjectRef(ctx, cred, in.Owner, in.Number)` |
| `github_projects_list_views.go` | `ListProjectViews` | `(TenantID, Provider, ProjectSlug)` → `ListProjectViews(ctx, cred, in.ProjectSlug)` |
| `github_projects_view_table.go` | `ViewProjectTable` | `(TenantID, Provider, ProjectSlug, ViewID, PageToken, PageSize)` → `ViewProjectTable(ctx, cred, in.ProjectSlug, in.ViewID, in.PageToken, in.PageSize)`, returns `(items []ProjectItem, nextPageToken string, error)` |
| `github_projects_clear_item_field.go` | `ClearProjectItemField` | `(TenantID, Provider, ProjectSlug, ItemID, FieldID)` → `ClearProjectItemField(ctx, cred, in.ProjectSlug, in.ItemID, in.FieldID)` |
| `github_projects_get_work_item_details.go` | `GetWorkItemDetailsBySlug` | `(TenantID, Provider, ItemSlug)` → `GetWorkItemDetailsBySlug(ctx, cred, in.ItemSlug)` |
| `github_projects_update_issue_by_slug.go` | `UpdateIssueBySlug` | `(TenantID, Provider, ItemSlug, Patch WorkItemPatch)` → `UpdateIssueBySlug(ctx, cred, in.ItemSlug, in.Patch)` |
| `github_projects_update_pr_by_slug.go` | `UpdatePullRequestBySlug` | `(TenantID, Provider, ItemSlug, Patch WorkItemPatch)` → `UpdatePullRequestBySlug(ctx, cred, in.ItemSlug, in.Patch)` |
| `github_projects_update_issue_type_by_slug.go` | `UpdateIssueTypeBySlug` | `(TenantID, Provider, ItemSlug, IssueType string)` → `UpdateIssueTypeBySlug(ctx, cred, in.ItemSlug, in.IssueType)` |
| `github_projects_list_issue_types_by_slug.go` | `ListIssueTypesBySlug` | `(TenantID, Provider, ItemSlug)` → `ListIssueTypesBySlug(ctx, cred, in.ItemSlug)` |
| `github_projects_list_assignable_users_by_slug.go` | `ListAssignableUsersBySlug` | `(TenantID, Provider, ItemSlug)` → `ListAssignableUsersBySlug(ctx, cred, in.ItemSlug)` |
| `github_projects_list_labels_by_slug.go` | `ListLabelsBySlug` | `(TenantID, Provider, ItemSlug)` → `ListLabelsBySlug(ctx, cred, in.ItemSlug)` |
| `github_projects_add_issue_comment_by_slug.go` | `AddIssueCommentBySlug` | `(TenantID, Provider, ItemSlug, Body string)` → `AddIssueCommentBySlug(ctx, cred, in.ItemSlug, in.Body)` |
| `github_projects_update_issue_comment_by_slug.go` | `UpdateIssueCommentBySlug` | `(TenantID, Provider, ItemSlug, CommentID, Body string)` → `UpdateIssueCommentBySlug(ctx, cred, in.ItemSlug, in.CommentID, in.Body)` |
| `github_projects_delete_issue_comment_by_slug.go` | `DeleteIssueCommentBySlug` | `(TenantID, Provider, ItemSlug, CommentID string)` → returns only `error`; `DeleteIssueCommentBySlug(ctx, cred, in.ItemSlug, in.CommentID)` |

Every one of these constructors is `New<Type>(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *<Type>`,
same as `NewUpdateProjectItemField` — copy `github_projects_update_item_field.go`
verbatim per file, rename the type/params struct/table row's field list, and
swap the single delegate call + its error code (`SCM_<NAME>_FAILED`, screaming
snake of the RPC name).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./internal/usecase/... 2>&1 | head -80
```

Expected: `internal/usecase` builds clean. `GitHubProjectsProvider` has no
implementation yet (TASK-079) — that's fine, nothing in this package
references a concrete implementation, only the interface.
