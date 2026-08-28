# TASK-PI-04-02: `ScmProvider.SubmitReview` port + `submit_review.go` usecase

**From Solution:** SOL-PI-04
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/ports.go`, `backend-go/services/scm-integration-service/internal/usecase/submit_review.go` (new), `backend-go/services/scm-integration-service/internal/domain/scm.go`
**Depends on:** TASK-PI-04-01
**Status:** `[x] DONE — domain ReviewType/ReviewComment/ReviewInput/Review + ScmProvider.SubmitReview port + submit_review.go usecase (BR-PI-10/BR-PI-11 re-validated) + grpc handler.`

---

## Context

BR-PI-10 ("must have at least 1 comment to submit review") and BR-PI-11
(default to Request Changes when `review_type` is unspecified) are
business rules about request shape, validated here in the usecase as a
belt-and-braces re-check even though `api-gateway`'s composition step
(TASK-PI-04-05) validates BR-PI-10 first too — so a future second caller of
`SubmitReview` can't bypass the rule by skipping the gateway path.

## Changes to make

### 1. `domain/scm.go` — add `ReviewType`/`ReviewComment`/`ReviewInput`/`Review`

```go
type ReviewType string

const (
	ReviewTypeUnspecified    ReviewType = ""
	ReviewTypeComment        ReviewType = "comment"
	ReviewTypeApprove        ReviewType = "approve"
	ReviewTypeRequestChanges ReviewType = "request_changes"
)

type ReviewComment struct {
	Path string
	Line int32
	Body string
}

type ReviewInput struct {
	Type     ReviewType
	Summary  string
	Comments []ReviewComment
}

type Review struct {
	ID         string
	ReviewerID string
	State      ReviewType
	SubmittedAt string
	Comments   []ReviewComment
	URL        string
}
```

### 2. `ports.go` — extend `ScmProvider`

```go
SubmitReview(ctx context.Context, cred Credential, repo string, prNumber int32, in domain.ReviewInput) (domain.Review, error)
```

### 3. `internal/usecase/submit_review.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SubmitReviewParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	PRNumber   int32
	ReviewType domain.ReviewType
	Summary    string
	Comments   []domain.ReviewComment
}

type SubmitReview struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewSubmitReview(credentials CredentialResolver, providers ProviderRegistry) *SubmitReview {
	return &SubmitReview{credentials: credentials, providers: providers}
}

func (uc *SubmitReview) Execute(ctx context.Context, in SubmitReviewParams) (domain.Review, error) {
	if len(in.Comments) == 0 { // BR-PI-10, re-validated here
		return domain.Review{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_REVIEW_EMPTY_COMMENTS", "at least one comment is required to submit a review", nil)
	}
	reviewType := in.ReviewType
	if reviewType == domain.ReviewTypeUnspecified {
		reviewType = domain.ReviewTypeRequestChanges // BR-PI-11
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	review, err := provider.SubmitReview(ctx, cred, in.Repo, in.PRNumber, domain.ReviewInput{
		Type: reviewType, Summary: in.Summary, Comments: in.Comments,
	})
	if err != nil {
		return domain.Review{}, apperrors.New(apperrors.KindInternal, "SCM_SUBMIT_REVIEW_FAILED", "failed to submit review", err)
	}
	return review, nil
}
```

### 4. gRPC handler wiring

Add a `SubmitReview` method to `internal/adapter/grpc/server.go`, translating
`SubmitReviewRequest`/`Review` to/from `SubmitReviewParams`/`domain.Review` —
follow the existing `MergePullRequest` handler's exact shape.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
