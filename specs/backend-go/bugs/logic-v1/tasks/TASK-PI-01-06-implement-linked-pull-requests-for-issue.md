# TASK-PI-01-06: `GetLinkedPullRequestsForIssue` usecase + adapters

**From Solution:** SOL-PI-01
**Priority:** P2 — genuine addition, no TDD precedent, lower priority than the filter/cache fix
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/get_linked_pull_requests_for_issue.go` (new), `internal/usecase/ports.go`, `internal/adapter/github/client.go`, `internal/adapter/gitlab/client.go`
**Depends on:** TASK-PI-01-01
**Status:** `[ ]` TODO

---

## Context

BUG-PI-01's issue-detail view has no way to see which PRs reference an
issue — `GetPullRequestForBranch` resolves by branch, not by issue. This is
this solution's one genuinely new RPC (no TDD precedent), degrading per
provider like `GetBoardView` already does for capability gaps.

## Changes to make

### 1. `ports.go` — add to `ScmProvider`

```go
// GetLinkedPullRequestsForIssue — GitHub: parsed from the issue's timeline
// "cross-referenced" events / closing-keyword search; GitLab: the related
// MRs endpoint. capabilitySupported=false means this provider has no cheap
// query for this — return an empty list, not an error.
GetLinkedPullRequestsForIssue(ctx context.Context, cred Credential, repo string, issueNumber int32) (prs []domain.PullRequest, capabilitySupported bool, err error)
```

### 2. `internal/usecase/get_linked_pull_requests_for_issue.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetLinkedPullRequestsForIssueInput struct {
	TenantID    string
	Provider    domain.ScmProvider
	Repo        string
	IssueNumber int32
}

type GetLinkedPullRequestsForIssueOutput struct {
	PullRequests           []domain.PullRequest
	CapabilityUnsupported  bool
}

type GetLinkedPullRequestsForIssue struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewGetLinkedPullRequestsForIssue(credentials CredentialResolver, providers ProviderRegistry) *GetLinkedPullRequestsForIssue {
	return &GetLinkedPullRequestsForIssue{credentials: credentials, providers: providers}
}

func (uc *GetLinkedPullRequestsForIssue) Execute(ctx context.Context, in GetLinkedPullRequestsForIssueInput) (GetLinkedPullRequestsForIssueOutput, error) {
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	prs, supported, err := provider.GetLinkedPullRequestsForIssue(ctx, cred, in.Repo, in.IssueNumber)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInternal, "SCM_LINKED_PRS_FAILED", "failed to fetch linked pull requests", err)
	}
	if !supported {
		return GetLinkedPullRequestsForIssueOutput{CapabilityUnsupported: true}, nil // degrade, not fail
	}
	return GetLinkedPullRequestsForIssueOutput{PullRequests: prs}, nil
}
```

### 3. GitHub adapter — implement via the issue timeline's
`cross-referenced` events (`GET /repos/{owner}/{repo}/issues/{number}/timeline`),
filtering to `source.issue.pull_request` entries; `capabilitySupported=true`.

### 4. GitLab adapter — implement via
`GET /projects/:id/issues/:iid/related_merge_requests`; `capabilitySupported=true`.

### 5. Other providers (Bitbucket/Azure DevOps/Gitea)

Return `nil, false, nil` (capability unsupported) until wired — same
placeholder posture `MergePullRequest` etc. already use for unimplemented
adapters.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
