# TASK-088: Add `BranchExists` to `ScmProvider` + `CheckHostedReviewEligibility` usecase

**From Solution:** SOL-014 (Design — Proto addition's `BranchExists` note, `usecase/` layer)
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `services/scm-integration-service/internal/usecase/ports.go`, `check_hosted_review_eligibility.go` (new), `internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea}/client.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-072, TASK-073, TASK-074, TASK-084, TASK-087
**Status:** `[ ]` TODO

---

## Context

`BranchExists` is a plain existence read every provider's REST API supports
uniformly (GET a ref/branch by name, 200 vs 404) — per SOL-014 it belongs on
the **common** `ScmProvider` port, unlike SOL-012/SOL-013's provider-specific
additions. `CheckHostedReviewEligibility` composes 3 signals entirely in
`usecase/`: `GetAuthStatus` (existing), `BranchExists` (this task),
`GetPullRequestForBranch` (TASK-074) — no new provider-adapter surface
beyond `BranchExists` itself.

---

## Changes to make

### Step 1: Extend `ScmProvider` interface

**File:** `services/scm-integration-service/internal/usecase/ports.go`

Find (added by TASK-074):

```go
	// ResolveRepoSlug — github.repoSlug. Resolves candidate (a remote URL,
	// "owner/name", or bare name) to a canonical owner/name pair.
	ResolveRepoSlug(ctx context.Context, cred Credential, candidate string) (owner, name string, err error)
}
```

Replace with:

```go
	// ResolveRepoSlug — github.repoSlug. Resolves candidate (a remote URL,
	// "owner/name", or bare name) to a canonical owner/name pair.
	ResolveRepoSlug(ctx context.Context, cred Credential, candidate string) (owner, name string, err error)

	// BranchExists — a plain existence read every provider's REST API
	// supports uniformly, unlike SOL-012/SOL-013's provider-specific
	// additions. Backs CheckHostedReviewEligibility's step 2 (SOL-014).
	BranchExists(ctx context.Context, cred Credential, repo, branch string) (bool, error)
}
```

### Step 2: New usecase — `check_hosted_review_eligibility.go`

**File:** `services/scm-integration-service/internal/usecase/check_hosted_review_eligibility.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type CheckHostedReviewEligibilityParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	HeadBranch string
	BaseBranch string
}

// HostedReviewEligibility mirrors scmintegrationv1.HostedReviewEligibility.
// IneligibleReason is one of "NOT_CONNECTED" | "BRANCH_NOT_FOUND" |
// "REVIEW_ALREADY_EXISTS" (empty when Eligible is true).
type HostedReviewEligibility struct {
	Eligible             bool
	IneligibleReason     string
	ExistingPullRequest  domain.PullRequest
}

// CheckHostedReviewEligibility fans out across 3 existing/already-proposed
// capabilities — GetAuthStatus, BranchExists, GetPullRequestForBranch — per
// scm-integration-service.md §6's check_hosted_review_eligibility.go package
// note ("fans out across configured providers"). No ProviderRegistry.For()
// fan-out across multiple providers happens here — "fans out" in the TDD
// refers to fanning out across these 3 checks for the ONE requested
// provider, not iterating every configured provider.
type CheckHostedReviewEligibility struct {
	credentials   CredentialResolver
	providers     ProviderRegistry
	getAuthStatus *GetAuthStatus
}

func NewCheckHostedReviewEligibility(credentials CredentialResolver, providers ProviderRegistry, getAuthStatus *GetAuthStatus) *CheckHostedReviewEligibility {
	return &CheckHostedReviewEligibility{credentials: credentials, providers: providers, getAuthStatus: getAuthStatus}
}

func (uc *CheckHostedReviewEligibility) Execute(ctx context.Context, in CheckHostedReviewEligibilityParams) (HostedReviewEligibility, error) {
	if in.TenantID == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.HeadBranch == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_BRANCH", "head_branch is required", nil)
	}

	// 1. Auth — cheapest check, and every subsequent check is meaningless
	// without it, so fail fast here rather than attempting a branch lookup
	// with no usable credential.
	connected, err := uc.getAuthStatus.Execute(ctx, GetAuthStatusInput{TenantID: in.TenantID, Provider: in.Provider})
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_GET_AUTH_STATUS_FAILED", "failed to check auth status", err)
	}
	if !connected {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "NOT_CONNECTED"}, nil
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	// 2. Branch existence.
	exists, err := provider.BranchExists(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_BRANCH_EXISTS_CHECK_FAILED", "failed to check branch existence", err)
	}
	if !exists {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "BRANCH_NOT_FOUND"}, nil
	}

	// 3. Existing open PR/MR for this branch — TASK-074's GetPullRequestForBranch.
	pr, found, err := provider.GetPullRequestForBranch(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_GET_PR_FOR_BRANCH_FAILED", "failed to check for an existing pull request", err)
	}
	if found {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "REVIEW_ALREADY_EXISTS", ExistingPullRequest: pr}, nil
	}

	return HostedReviewEligibility{Eligible: true}, nil
}
```

`GetAuthStatus.Execute` already returns `(bool, error)` (see
`get_auth_status.go`) — reused here as an injected `*GetAuthStatus`
dependency (constructed once in `main.go`, same instance
`RegisterScmIntegrationServiceServer`'s `GetAuthStatus` RPC already uses),
not a re-implementation of its logic.

### Step 3: `BranchExists` per provider adapter

**GitHub** — `services/scm-integration-service/internal/adapter/github/client.go`:

```go
// BranchExists calls GitHub's REST API: GET /repos/{repo}/branches/{branch}
// — 200 means it exists, 404 means it doesn't (not an error in either case).
func (c *Client) BranchExists(ctx context.Context, cred usecase.Credential, repo, branch string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/branches/%s", c.baseURL, repo, url.PathEscape(branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("github: build branch exists request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("github: branch exists request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github: branch exists: unexpected status %d", resp.StatusCode)
	}
}
```

(The local variable `url` shadows the `net/url` package import in this
function — rename the local to `reqURL`, matching this adapter's existing
convention elsewhere in `client.go`, and keep `net/url` imported for
`url.PathEscape`.)

**GitLab** — `services/scm-integration-service/internal/adapter/gitlab/client.go`:

```go
// BranchExists calls GitLab's REST API: GET
// /projects/{id}/repository/branches/{branch} — 200 exists, 404 doesn't.
func (c *Client) BranchExists(ctx context.Context, cred usecase.Credential, repo, branch string) (bool, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/repository/branches/%s", c.baseURL, projectPath(repo), url.PathEscape(branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("gitlab: build branch exists request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("gitlab: branch exists request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("gitlab: branch exists: unexpected status %d", resp.StatusCode)
	}
}
```

**Bitbucket / Azure DevOps / Gitea** — same 200-vs-404 shape against each
provider's own branch-ref endpoint (add to each adapter's `client.go`,
following the GitHub/GitLab methods above verbatim except for URL/headers):

| Provider | Endpoint | Notes |
|---|---|---|
| Bitbucket | `GET /2.0/repositories/{workspace}/{repo_slug}/refs/branches/{name}` | `repo` is already `workspace/repo_slug`-shaped, same convention as this adapter's other methods |
| Azure DevOps | `GET https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/refs?filter=heads/{branch}&api-version=7.1` | Always returns 200; existence is `len(response.value) > 0`, not the status code — decode `{"value": [...]}"` and check its length instead of switching on status |
| Gitea | `GET /repos/{owner}/{repo}/branches/{branch}` | GitHub-shaped API (README) — identical 200/404 shape to GitHub's version above |

### Step 4: gRPC server + composition root wiring

Follow TASK-076/TASK-084's exact pattern:
- `internal/adapter/grpc/server.go`: add `checkHostedReviewEligibility
  *usecase.CheckHostedReviewEligibility` field + constructor param, plus a
  `CheckHostedReviewEligibility` RPC method mapping
  `req.Get*()` → `usecase.CheckHostedReviewEligibilityParams` →
  `apperrors.ToGRPCStatus` on error → `&scmintegrationv1.HostedReviewEligibility{
  Eligible: result.Eligible, IneligibleReason: result.IneligibleReason,
  ExistingPullRequest: toProtoPullRequest(result.ExistingPullRequest)}` on
  success (only set `ExistingPullRequest` when `IneligibleReason ==
  "REVIEW_ALREADY_EXISTS"`, mirroring TASK-076's
  `GetPullRequestForBranch`'s conditional `PullRequest` field).
- `cmd/server/main.go`: construct `checkHostedReviewEligibilityUC :=
  usecase.NewCheckHostedReviewEligibility(credentials, registry,
  getAuthStatusUC)` (reusing the existing `getAuthStatusUC` variable already
  constructed for the `GetAuthStatus` RPC) and pass it into `scmgrpc.New(...)`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./... && go vet ./...
go test ./... -count=1
```

Expected: clean build; every provider adapter satisfies the extended
`usecase.ScmProvider` interface (`BranchExists` included);
`CheckHostedReviewEligibility` RPC wired end to end.
