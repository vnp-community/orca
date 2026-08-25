# TASK-074: Extend `ScmProvider` port + add usecases for repo/branch resolution

**From Solution:** SOL-012 (Design — `usecase/` layer, shape 2), SOL-014 (references `GetPullRequestForBranch`)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/usecase/ports.go`, `get_pull_request_for_branch.go` (new), `resolve_repo_slug.go` (new)
**Depends on:** TASK-072, TASK-073
**Status:** `[x]` DONE (verified — `go build ./internal/usecase/...` clean)

---

## Context

`GetPullRequestForBranch` is provider-generic (used by both
`github.prForBranch` and `hostedReview.forBranch`, SOL-014/TASK-089) —
lives on the common `ScmProvider` port, not a GitHub-only interface.
`ResolveRepoSlug` is GitHub-specific in practice (`github.repoSlug`) but
kept on the same common port for symmetry — other adapters return
`ErrCapabilityUnsupported` until wired.

---

## Changes to make

### Step 1: Extend `ScmProvider` interface

**File:** `services/scm-integration-service/internal/usecase/ports.go`

Find (added by TASK-073):

```go
	UpdateIssue(ctx context.Context, cred Credential, repo string, number int32, patch IssuePatch) (domain.Issue, error)
}
```

Replace with:

```go
	UpdateIssue(ctx context.Context, cred Credential, repo string, number int32, patch IssuePatch) (domain.Issue, error)

	// GetPullRequestForBranch — provider-generic; backs github.prForBranch
	// AND hostedReview.forBranch (SOL-014). found=false + zero-value
	// PullRequest means "no open PR/MR for this branch", not an error.
	GetPullRequestForBranch(ctx context.Context, cred Credential, repo, headBranch string) (pr domain.PullRequest, found bool, err error)
	// ResolveRepoSlug — github.repoSlug. Resolves candidate (a remote URL,
	// "owner/name", or bare name) to a canonical owner/name pair.
	ResolveRepoSlug(ctx context.Context, cred Credential, candidate string) (owner, name string, err error)
}
```

### Step 2: New usecase — `get_pull_request_for_branch.go`

**File:** `services/scm-integration-service/internal/usecase/get_pull_request_for_branch.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetPullRequestForBranchParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	HeadBranch string
}

type GetPullRequestForBranchResult struct {
	PullRequest domain.PullRequest
	Found       bool
}

type GetPullRequestForBranch struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewGetPullRequestForBranch(credentials CredentialResolver, providers ProviderRegistry) *GetPullRequestForBranch {
	return &GetPullRequestForBranch{credentials: credentials, providers: providers}
}

func (uc *GetPullRequestForBranch) Execute(ctx context.Context, in GetPullRequestForBranchParams) (GetPullRequestForBranchResult, error) {
	if in.TenantID == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.HeadBranch == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_BRANCH", "head_branch is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	pr, found, err := provider.GetPullRequestForBranch(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInternal, "SCM_GET_PR_FOR_BRANCH_FAILED", "failed to get pull request for branch", err)
	}
	return GetPullRequestForBranchResult{PullRequest: pr, Found: found}, nil
}
```

### Step 3: New usecase — `resolve_repo_slug.go`

**File:** `services/scm-integration-service/internal/usecase/resolve_repo_slug.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ResolveRepoSlugParams struct {
	TenantID  string
	Provider  domain.ScmProvider
	Candidate string
}

type ResolveRepoSlugResult struct {
	Owner string
	Name  string
	Slug  string
}

type ResolveRepoSlug struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewResolveRepoSlug(credentials CredentialResolver, providers ProviderRegistry) *ResolveRepoSlug {
	return &ResolveRepoSlug{credentials: credentials, providers: providers}
}

func (uc *ResolveRepoSlug) Execute(ctx context.Context, in ResolveRepoSlugParams) (ResolveRepoSlugResult, error) {
	if in.TenantID == "" {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Candidate == "" {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_CANDIDATE", "candidate is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	owner, name, err := provider.ResolveRepoSlug(ctx, cred, in.Candidate)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInternal, "SCM_RESOLVE_REPO_SLUG_FAILED", "failed to resolve repo slug", err)
	}
	return ResolveRepoSlugResult{Owner: owner, Name: name, Slug: owner + "/" + name}, nil
}
```

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./internal/usecase/... 2>&1 | head -50
```

Expected: `internal/usecase` builds against the extended interface once
TASK-073's stub methods (bitbucket/azuredevops/gitea) also cover these two
new methods — add them in the same pass as TASK-073's stub step.
