# TASK-073: Extend `ScmProvider` port + add usecases for GitHub PR/issue mutations

**From Solution:** SOL-012 (Design — `usecase/` layer, shape 1)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/usecase/ports.go`, `merge_pull_request.go` (new), `request_pull_request_reviewers.go` (new), `remove_pull_request_reviewers.go` (new), `set_pull_request_auto_merge.go` (new), `update_issue.go` (new)
**Depends on:** TASK-071
**Status:** `[ ]` TODO

---

## Context

Adds the 5 new operations to the `ScmProvider` port every adapter
implements, plus one usecase per operation. Follows `create_pull_request.go`'s
exact structure: a `*Params` struct mirroring the RPC request, a validate →
resolve-credential → resolve-provider → delegate → wrap-error pipeline.

---

## Changes to make

### Step 1: Extend `ScmProvider` interface

**File:** `services/scm-integration-service/internal/usecase/ports.go`

Find:

```go
type ScmProvider interface {
	ListIssues(ctx context.Context, cred Credential, repo string, filter IssueFilter) ([]domain.Issue, error)
	CreatePullRequest(ctx context.Context, cred Credential, repo string, input CreatePullRequestInput) (domain.PullRequest, error)
	ListPullRequests(ctx context.Context, cred Credential, repo string) ([]domain.PullRequest, error)
	GetRateLimitStatus(ctx context.Context, cred Credential) (domain.RateLimitStatus, error)
}
```

Replace with:

```go
type ScmProvider interface {
	ListIssues(ctx context.Context, cred Credential, repo string, filter IssueFilter) ([]domain.Issue, error)
	CreatePullRequest(ctx context.Context, cred Credential, repo string, input CreatePullRequestInput) (domain.PullRequest, error)
	ListPullRequests(ctx context.Context, cred Credential, repo string) ([]domain.PullRequest, error)
	GetRateLimitStatus(ctx context.Context, cred Credential) (domain.RateLimitStatus, error)

	// MergePullRequest / RequestPullRequestReviewers / RemovePullRequestReviewers
	// / SetPullRequestAutoMerge / UpdateIssue — see SOL-012 shape 1. GitHub
	// implements all five for real (TASK-075); other adapters return their
	// own package-level ErrCapabilityUnsupported sentinel until wired,
	// mirroring the azuredevops/gitea precedent already in this codebase.
	MergePullRequest(ctx context.Context, cred Credential, repo string, number int32, input MergePullRequestInput) (domain.PullRequest, bool, string, error)
	RequestPullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins, teamSlugs []string) (domain.PullRequest, error)
	RemovePullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins []string) (domain.PullRequest, error)
	SetPullRequestAutoMerge(ctx context.Context, cred Credential, repo string, number int32, enabled bool, mergeMethod string) (domain.PullRequest, error)
	UpdateIssue(ctx context.Context, cred Credential, repo string, number int32, patch IssuePatch) (domain.Issue, error)
}

// MergePullRequestInput carries the merge-method/commit-message fields
// MergePullRequest needs beyond repo+number.
type MergePullRequestInput struct {
	MergeMethod   string
	CommitTitle   string
	CommitMessage string
}

// IssuePatch is UpdateIssue's partial-update shape — nil pointer fields mean
// "leave unchanged", mirroring UpdateIssueRequest's proto3 `optional` fields.
type IssuePatch struct {
	Title        *string
	Body         *string
	State        *string
	AddLabels    []string
	RemoveLabels []string
	Assignees    []string
}
```

`domain.Issue`/`domain.PullRequest` also need a `Number int32` field to carry
the new proto field end to end — add it alongside the existing fields in
`internal/domain/scm.go`:

```go
type Issue struct {
	ID       string
	Provider ScmProvider
	Repo     string
	Title    string
	State    string
	URL      string
	Number   int32
}
```

```go
type PullRequest struct {
	ID         string
	Provider   ScmProvider
	Repo       string
	Title      string
	State      string
	URL        string
	HeadBranch string
	BaseBranch string
	Number     int32
}
```

`NewIssue`/`NewPullRequest` constructors stay as-is (positional args
unaffected — `Number` is set by adapters directly on the struct literal,
same as `Provider`/`Repo` are today in `toDomainPullRequest`-style helpers);
no signature change needed since GitHub's adapter (TASK-075) builds the
struct via `domain.NewPullRequest(...)` and then sets `.Number` afterward.

### Step 2: New usecase — `merge_pull_request.go`

**File:** `services/scm-integration-service/internal/usecase/merge_pull_request.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type MergePullRequestParams struct {
	TenantID      string
	Provider      domain.ScmProvider
	Repo          string
	Number        int32
	MergeMethod   string
	CommitTitle   string
	CommitMessage string
}

type MergePullRequestResult struct {
	PullRequest domain.PullRequest
	Merged      bool
	SHA         string
}

type MergePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewMergePullRequest(credentials CredentialResolver, providers ProviderRegistry) *MergePullRequest {
	return &MergePullRequest{credentials: credentials, providers: providers}
}

func (uc *MergePullRequest) Execute(ctx context.Context, in MergePullRequestParams) (MergePullRequestResult, error) {
	if in.TenantID == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Number == 0 {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_NUMBER", "number is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	pr, merged, sha, err := provider.MergePullRequest(ctx, cred, in.Repo, in.Number, MergePullRequestInput{
		MergeMethod:   in.MergeMethod,
		CommitTitle:   in.CommitTitle,
		CommitMessage: in.CommitMessage,
	})
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_MERGE_PULL_REQUEST_FAILED", "failed to merge pull request", err)
	}
	return MergePullRequestResult{PullRequest: pr, Merged: merged, SHA: sha}, nil
}
```

### Step 3: New usecases — same shape, one file each

Each of the remaining four follows `merge_pull_request.go`'s exact
validate → resolve-credential → resolve-provider → delegate → wrap pipeline.
Write them as follows.

**File:** `services/scm-integration-service/internal/usecase/request_pull_request_reviewers.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type RequestPullRequestReviewersParams struct {
	TenantID       string
	Provider       domain.ScmProvider
	Repo           string
	Number         int32
	ReviewerLogins []string
	TeamSlugs      []string
}

type RequestPullRequestReviewers struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewRequestPullRequestReviewers(credentials CredentialResolver, providers ProviderRegistry) *RequestPullRequestReviewers {
	return &RequestPullRequestReviewers{credentials: credentials, providers: providers}
}

func (uc *RequestPullRequestReviewers) Execute(ctx context.Context, in RequestPullRequestReviewersParams) (domain.PullRequest, error) {
	if in.TenantID == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	pr, err := provider.RequestPullRequestReviewers(ctx, cred, in.Repo, in.Number, in.ReviewerLogins, in.TeamSlugs)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_REQUEST_PR_REVIEWERS_FAILED", "failed to request pull request reviewers", err)
	}
	return pr, nil
}
```

**File:** `services/scm-integration-service/internal/usecase/remove_pull_request_reviewers.go` — identical shape to
`RequestPullRequestReviewers` above, but:
- struct named `RemovePullRequestReviewers`, params `RemovePullRequestReviewersParams` (no `TeamSlugs` field)
- delegates to `provider.RemovePullRequestReviewers(ctx, cred, in.Repo, in.Number, in.ReviewerLogins)`
- error code `SCM_REMOVE_PR_REVIEWERS_FAILED`

**File:** `services/scm-integration-service/internal/usecase/set_pull_request_auto_merge.go` — identical shape, but:
- struct named `SetPullRequestAutoMerge`, params `SetPullRequestAutoMergeParams{TenantID, Provider, Repo, Number, Enabled bool, MergeMethod string}`
- delegates to `provider.SetPullRequestAutoMerge(ctx, cred, in.Repo, in.Number, in.Enabled, in.MergeMethod)`
- error code `SCM_SET_PR_AUTO_MERGE_FAILED`

**File:** `services/scm-integration-service/internal/usecase/update_issue.go` — identical shape, but:
- struct named `UpdateIssue`, params `UpdateIssueParams{TenantID, Provider, Repo, Number, Patch IssuePatch}`
- returns `domain.Issue`, delegates to `provider.UpdateIssue(ctx, cred, in.Repo, in.Number, in.Patch)`
- error code `SCM_UPDATE_ISSUE_FAILED`

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... 2>&1 | head -50
```

Expected at this point: build fails only on `internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea}` not
implementing the 5 new `ScmProvider` methods yet — that's TASK-075 (GitHub)
and out-of-scope stub methods for the other four providers (add a
one-line `return ..., ErrCapabilityUnsupported` per new method to each of
`bitbucket`, `azuredevops`, `gitea`'s `client.go`, mirroring their existing
`ErrCapabilityUnsupported` methods, so the interface is satisfied — GitLab's
methods are added in TASK-084). No other package should fail to build.
