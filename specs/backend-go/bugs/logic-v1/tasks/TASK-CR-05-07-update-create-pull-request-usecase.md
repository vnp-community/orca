# TASK-CR-05-07: Enforce branch-pushed precondition, thread `Draft`, and compose linked-issue auto-update into `CreatePullRequest`

**From Solution:** SOL-CR-05
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go`, `backend-go/services/scm-integration-service/internal/adapter/grpc/server.go`, `backend-go/services/scm-integration-service/cmd/server/main.go`
**Depends on:** TASK-CR-05-03
**Status:** `[x]` DONE — CreatePullRequest rewritten with BR-CR-17 BranchExists precondition, BR-CR-20 Draft + ErrCapabilityUnsupported mapping, BR-CR-19 best-effort linked-issue update; grpc server.go + main.go updated; 4 new regression tests plus existing dispatch test passing

---

## Context

Three gaps land here: BR-CR-17 (branch must be pushed before a PR can be
created — `BranchExists` already exists and already backs
`CheckHostedReviewEligibility`'s step 2, this task calls it one more place),
BR-CR-20 (thread `Draft` through, map a provider's
`domain.ErrCapabilityUnsupported` to a typed precondition error), and
BR-CR-19 (best-effort linked-issue state update via the existing
`UpdateIssue` usecase, composed in-process the same way
`GenerateCommitMessage` composes `getStatus`/`getDiff`/`history` in
`git-gateway-service`, SOL-CR-04).

## Changes to make

### 1. `internal/usecase/create_pull_request.go`

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type CreatePullRequestParams struct {
	TenantID          string
	Provider          domain.ScmProvider
	Repo              string
	Title             string
	Body              string
	HeadBranch        string
	BaseBranch        string
	Draft             bool // NEW — BR-CR-20
	LinkedIssueNumber int32 // NEW — BR-CR-19; 0 means "no linked issue"
}

// CreatePullRequestResult carries the created PR plus a non-fatal
// linked-issue-update error, if any — see the Execute doc comment below
// for why a failed issue update never turns a successful PR creation into
// a failed call.
type CreatePullRequestResult struct {
	PullRequest             domain.PullRequest
	LinkedIssueUpdateError  string
}

type CreatePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	updateIssue *UpdateIssue // NEW — in-process composition, mirrors GenerateCommitMessage's pattern (SOL-CR-04)
}

func NewCreatePullRequest(credentials CredentialResolver, providers ProviderRegistry, updateIssue *UpdateIssue) *CreatePullRequest {
	return &CreatePullRequest{credentials: credentials, providers: providers, updateIssue: updateIssue}
}

func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (CreatePullRequestResult, error) {
	if in.TenantID == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Title == "" {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_TITLE", "title is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	// BR-CR-17 — reuse BranchExists (already implemented, already used by
	// CheckHostedReviewEligibility) as an explicit precondition inside
	// CreatePullRequest itself.
	exists, err := provider.BranchExists(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_BRANCH_EXISTS_CHECK_FAILED", "failed to verify branch was pushed", err)
	}
	if !exists {
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_BRANCH_NOT_PUSHED", "branch must be pushed to the remote before a pull request can be created", nil)
	}

	pr, err := provider.CreatePullRequest(ctx, cred, in.Repo, CreatePullRequestInput{
		Title: in.Title, Body: in.Body, HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch,
		Draft: in.Draft, // BR-CR-20
	})
	if err != nil {
		if in.Draft && errors.Is(err, domain.ErrCapabilityUnsupported) {
			return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_DRAFT_UNSUPPORTED", "this provider does not support draft pull requests", err)
		}
		return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREATE_PULL_REQUEST_FAILED", "failed to create pull request", err)
	}

	result := CreatePullRequestResult{PullRequest: pr}
	// BR-CR-19 — best-effort: the PR is already real at this point; a
	// failed issue update must not look like a failed PR creation to the
	// caller, so this error is carried in the result, not returned as the
	// call's error.
	if in.LinkedIssueNumber != 0 {
		state := "in_review" // provider-appropriate mapping is UpdateIssue/the provider adapter's own concern, unchanged by this task
		if _, err := uc.updateIssue.Execute(ctx, UpdateIssueParams{
			TenantID: in.TenantID, Provider: in.Provider, Repo: in.Repo,
			Number: in.LinkedIssueNumber, Patch: IssuePatch{State: &state},
		}); err != nil {
			result.LinkedIssueUpdateError = err.Error()
		}
	}
	return result, nil
}
```

### 2. `internal/adapter/grpc/server.go`

Update the `Server` struct's `createPullRequest` field and constructor
parameter list — no change needed there beyond what TASK-CR-05-06 already
touches for `suggestPullRequestReviewers` (this task doesn't add a new
field, `createPullRequest`'s own field/type is unchanged, only its
constructor call in `main.go` gains an argument — see below).

Update `CreatePullRequest`'s handler:

```go
func (s *Server) CreatePullRequest(ctx context.Context, req *scmintegrationv1.CreatePullRequestRequest) (*scmintegrationv1.CreatePullRequestResponse, error) {
	result, err := s.createPullRequest.Execute(ctx, usecase.CreatePullRequestParams{
		TenantID:          req.GetTenantId(),
		Provider:          toDomainProvider(req.GetProvider()),
		Repo:              req.GetRepo(),
		Title:             req.GetTitle(),
		Body:              req.GetBody(),
		HeadBranch:        req.GetHeadBranch(),
		BaseBranch:        req.GetBaseBranch(),
		Draft:             req.GetDraft(),
		LinkedIssueNumber: req.GetLinkedIssueNumber(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &scmintegrationv1.CreatePullRequestResponse{
		PullRequest:             toProtoPullRequest(result.PullRequest),
		LinkedIssueUpdateError:  result.LinkedIssueUpdateError,
	}, nil
}
```

Update `toProtoPullRequest` to echo `Draft`:

```go
func toProtoPullRequest(pr domain.PullRequest) *scmintegrationv1.PullRequest {
	return &scmintegrationv1.PullRequest{
		Id:     pr.ID,
		Url:    pr.URL,
		State:  pr.State,
		Number: pr.Number,
		Draft:  pr.Draft, // NEW
	}
}
```

### 3. `cmd/server/main.go`

```go
createPullRequestUC := usecase.NewCreatePullRequest(credentials, registry, updateIssueUC)
```

`updateIssueUC` is already constructed at `main.go:166` — ensure it's
constructed before this line (move it up if needed, same ordering concern
as TASK-CR-04-03's `historyUC`).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./...
go vet ./...
go test ./internal/usecase/... -run TestCreatePullRequest -v
```

Add cases to `create_pull_request_test.go`:
- `BranchExists=false` → `SCM_BRANCH_NOT_PUSHED`, `provider.CreatePullRequest`
  never called (assert zero calls) — regression guard for BR-CR-17.
- `Draft=true` against a fake provider returning an error wrapping
  `domain.ErrCapabilityUnsupported` → `SCM_DRAFT_UNSUPPORTED`, not a
  generic internal error.
- `LinkedIssueNumber` set, `UpdateIssue`'s fake returns an error → result
  still has the created `PullRequest` and a non-empty
  `LinkedIssueUpdateError`, method itself returns `err == nil` —
  regression guard against BR-CR-19's failure mode rolling back or masking
  a successful PR creation.
- `LinkedIssueNumber` unset (0) → `updateIssue` never called.
