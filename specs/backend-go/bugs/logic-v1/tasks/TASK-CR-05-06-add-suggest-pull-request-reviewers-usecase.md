# TASK-CR-05-06: Add `SuggestPullRequestReviewers` usecase and wire it into the gRPC server

**From Solution:** SOL-CR-05
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/suggest_pull_request_reviewers.go` (new), `backend-go/services/scm-integration-service/internal/adapter/grpc/server.go`, `backend-go/services/scm-integration-service/cmd/server/main.go`
**Depends on:** TASK-CR-05-04, TASK-CR-05-05
**Status:** `[x]` DONE — SuggestPullRequestReviewers usecase + gRPC handler + main.go wiring added; suggest_pull_request_reviewers_test.go covers stop-at-first-found and all-paths-tried-when-none-found, passing

---

## Context

BR-CR-18's "suggest reviewers from CODEOWNERS, if present" is answered by
trying each canonical CODEOWNERS path in order (mirroring GitHub's own
lookup order) and stopping at the first one found — an empty result is not
an error, since most repos have no CODEOWNERS file at all.

## Changes to make

### 1. `internal/usecase/suggest_pull_request_reviewers.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SuggestPullRequestReviewersParams struct {
	TenantID     string
	Provider     domain.ScmProvider
	Repo         string
	BaseRef      string
	ChangedFiles []string
}

type SuggestedReviewers struct {
	ReviewerLogins []string
	TeamSlugs      []string
	Found          bool
}

// codeownersPaths mirrors GitHub's own documented lookup order.
var codeownersPaths = []string{"CODEOWNERS", ".github/CODEOWNERS", ".gitlab/CODEOWNERS", "docs/CODEOWNERS"}

type SuggestPullRequestReviewers struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewSuggestPullRequestReviewers(credentials CredentialResolver, providers ProviderRegistry) *SuggestPullRequestReviewers {
	return &SuggestPullRequestReviewers{credentials: credentials, providers: providers}
}

func (uc *SuggestPullRequestReviewers) Execute(ctx context.Context, in SuggestPullRequestReviewersParams) (SuggestedReviewers, error) {
	if in.TenantID == "" {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return SuggestedReviewers{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	for _, path := range codeownersPaths {
		content, found, err := provider.GetRepoFileContent(ctx, cred, in.Repo, path, in.BaseRef)
		if err != nil {
			return SuggestedReviewers{}, apperrors.New(apperrors.KindInternal, "SCM_CODEOWNERS_FETCH_FAILED", "failed to fetch CODEOWNERS", err)
		}
		if !found {
			continue
		}
		logins, teams := MatchOwners(ParseCodeowners(content), in.ChangedFiles)
		return SuggestedReviewers{ReviewerLogins: logins, TeamSlugs: teams, Found: true}, nil
	}
	return SuggestedReviewers{Found: false}, nil // no CODEOWNERS anywhere — not an error, BR-CR-18 says "if present"
}
```

### 2. `internal/adapter/grpc/server.go`

Add a `suggestPullRequestReviewers *usecase.SuggestPullRequestReviewers`
field to `Server`, thread it through the constructor's parameter list
(same pattern as `createPullRequest`), and add the RPC handler:

```go
func (s *Server) SuggestPullRequestReviewers(ctx context.Context, req *scmintegrationv1.SuggestPullRequestReviewersRequest) (*scmintegrationv1.SuggestPullRequestReviewersResponse, error) {
	result, err := s.suggestPullRequestReviewers.Execute(ctx, usecase.SuggestPullRequestReviewersParams{
		TenantID:     req.GetTenantId(),
		Provider:     toDomainProvider(req.GetProvider()), // this file's existing proto<->domain provider mapping helper (used by CreatePullRequest above)
		Repo:         req.GetRepo(),
		BaseRef:      req.GetBaseRef(),
		ChangedFiles: req.GetChangedFiles(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err) // this file's existing apperrors->gRPC status mapping helper
	}
	return &scmintegrationv1.SuggestPullRequestReviewersResponse{
		ReviewerLogins:  result.ReviewerLogins,
		TeamSlugs:       result.TeamSlugs,
		CodeownersFound: result.Found,
	}, nil
}
```

### 3. `cmd/server/main.go`

```go
suggestPullRequestReviewersUC := usecase.NewSuggestPullRequestReviewers(credentials, registry)
```

Add it to the `Server` constructor call alongside `createPullRequestUC`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./...
go test ./internal/usecase/... -run TestSuggestPullRequestReviewers -v
```

Add `suggest_pull_request_reviewers_test.go`: tries all 4 canonical paths
in order, stops at first `found=true` (assert the fake provider's
`GetRepoFileContent` call count); returns `Found: false` (not an error)
when none exist.
