# TASK-PI-01-05: `ListIssueCommentsBySlug` usecase + GitHub/GitLab adapters

**From Solution:** SOL-PI-01
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/list_issue_comments.go` (new), `internal/usecase/ports.go`, `internal/adapter/github/client.go`, `internal/adapter/gitlab/client.go`
**Depends on:** TASK-PI-01-01
**Status:** `[ ]` TODO

---

## Context

`AddIssueCommentBySlug`/`UpdateIssueCommentBySlug`/`DeleteIssueCommentBySlug`
exist but there is no way to read the existing comment thread
(BUG-PI-01's step-6 gap). This completes the `*BySlug` comment RPC group the
TDD's §3 sketch already implies.

## Changes to make

### 1. `ports.go` — extend the `*BySlug`-capable provider port

Find wherever `GetWorkItemDetailsBySlug`/`AddIssueCommentBySlug` are declared
on the provider interface (same interface `ScmProvider` extends, or a
sibling `SlugProvider` port — match whatever this codebase currently uses)
and add:

```go
ListIssueCommentsBySlug(ctx context.Context, cred Credential, itemSlug string) ([]domain.ProjectComment, error)
```

(If `domain.ProjectComment` doesn't yet exist as a Go type mirroring the
proto message, add it next to `domain.Issue`/`domain.PullRequest` in
`internal/domain/scm.go`: `type ProjectComment struct { ID, Body, Author, URL string }`.)

### 2. `internal/usecase/list_issue_comments.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListIssueCommentsBySlugInput struct {
	TenantID string
	ItemSlug string
}

type ListIssueCommentsBySlug struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewListIssueCommentsBySlug(credentials CredentialResolver, providers ProviderRegistry) *ListIssueCommentsBySlug {
	return &ListIssueCommentsBySlug{credentials: credentials, providers: providers}
}

func (uc *ListIssueCommentsBySlug) Execute(ctx context.Context, in ListIssueCommentsBySlugInput) ([]domain.ProjectComment, error) {
	if in.ItemSlug == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_SLUG", "item_slug is required", nil)
	}
	provider, cred, err := resolveSlugProvider(ctx, uc.credentials, uc.providers, in.TenantID, in.ItemSlug)
	if err != nil {
		return nil, err
	}
	comments, err := provider.ListIssueCommentsBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ISSUE_COMMENTS_FAILED", "failed to list issue comments", err)
	}
	return comments, nil
}
```

`resolveSlugProvider` should reuse whatever helper `add_issue_comment_by_slug.go`
(or equivalent) already uses to resolve provider+credential from a slug —
match that file's existing pattern exactly rather than duplicating logic.

### 3. GitHub adapter (`internal/adapter/github/client.go`)

```go
// ListIssueCommentsBySlug — GET /repos/{owner}/{repo}/issues/{number}/comments,
// parses item_slug the same way AddIssueCommentBySlug already does.
func (c *Client) ListIssueCommentsBySlug(ctx context.Context, cred Credential, itemSlug string) ([]domain.ProjectComment, error) {
	owner, repo, number, err := parseIssueSlug(itemSlug) // reuse existing slug parser
	if err != nil {
		return nil, err
	}
	// ... GET request, map response items to []domain.ProjectComment{ID, Body, Author, URL} ...
}
```

### 4. GitLab adapter (`internal/adapter/gitlab/client.go`)

Same shape against GitLab's `GET /projects/:id/issues/:iid/notes` endpoint.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
