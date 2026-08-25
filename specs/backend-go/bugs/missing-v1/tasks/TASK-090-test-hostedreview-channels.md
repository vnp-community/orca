# TASK-090: Tests for `hostedReview.*` eligibility usecase and wscompat channels

**From Solution:** SOL-014 (Test plan)
**Priority:** P1
**Service:** `scm-integration-service` + `api-gateway`
**File:** `services/scm-integration-service/internal/usecase/check_hosted_review_eligibility_test.go` (new), `services/scm-integration-service/internal/adapter/github/branch_exists_test.go` (new), `services/scm-integration-service/internal/adapter/gitlab/branch_exists_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_hostedreview_test.go` (new)
**Depends on:** TASK-087, TASK-088, TASK-089
**Status:** `[ ]` TODO

---

## Changes to make

### Step 1: `CheckHostedReviewEligibility` usecase test — table-driven over the 4 outcomes

**File:** `services/scm-integration-service/internal/usecase/check_hosted_review_eligibility_test.go`

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeBranchAwareProvider extends fakeProvider (scm_provider_dispatch_test.go)
// with BranchExists — the one ScmProvider method
// CheckHostedReviewEligibility exercises that fakeProvider doesn't already
// cover from SOL-012's tests (TASK-082 Step 1 adds BranchExists to
// fakeProvider directly; this file assumes that's already done and sets
// branchExists/branchExistsErr on the shared fakeProvider).

func TestCheckHostedReviewEligibility_NotConnected(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: false})
	provider := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "NOT_CONNECTED" {
		t.Fatalf("expected NOT_CONNECTED, got %+v", result)
	}
	if provider.calls != 0 {
		t.Errorf("expected provider.BranchExists NOT called when auth check fails first, got %d calls", provider.calls)
	}
}

func TestCheckHostedReviewEligibility_BranchNotFound(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "BRANCH_NOT_FOUND" {
		t.Fatalf("expected BRANCH_NOT_FOUND, got %+v", result)
	}
}

func TestCheckHostedReviewEligibility_ReviewAlreadyExists(t *testing.T) {
	existing, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url", "feature-x", "main")
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: true, branchPR: existing, branchFound: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "REVIEW_ALREADY_EXISTS" {
		t.Fatalf("expected REVIEW_ALREADY_EXISTS, got %+v", result)
	}
	if result.ExistingPullRequest.ID != "1" {
		t.Errorf("expected ExistingPullRequest to be set, got %+v", result.ExistingPullRequest)
	}
}

func TestCheckHostedReviewEligibility_Eligible(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: true, branchFound: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Eligible || result.IneligibleReason != "" {
		t.Fatalf("expected eligible with no reason, got %+v", result)
	}
}

func TestCheckHostedReviewEligibility_PropagatesBranchExistsFailure(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExistsErr: errors.New("provider unavailable")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	_, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err == nil {
		t.Fatal("expected an error when BranchExists fails")
	}
}
```

`fakeProvider` needs `branchExists bool`, `branchExistsErr error` fields and
a `BranchExists` method added (TASK-082 Step 1 covers the interface-shape
addition; add it there so `merge_pull_request_test.go` and this file share
one fake):

```go
func (f *fakeProvider) BranchExists(ctx context.Context, cred Credential, repo, branch string) (bool, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.branchExistsErr != nil {
		return false, f.branchExistsErr
	}
	return f.branchExists, nil
}
```

`fakeCredentialResolverConnectedFlag` is a small new fake specific to this
test file — `GetAuthStatus.Execute` calls `CredentialResolver.Resolve` and
treats a `nil` error as "connected" (see `get_auth_status.go`); this fake
returns `nil` when `connected: true` and a sentinel error otherwise:

```go
type fakeCredentialResolverConnectedFlag struct {
	connected bool
}

func (f *fakeCredentialResolverConnectedFlag) Resolve(ctx context.Context, tenantID string, provider domain.ScmProvider) (Credential, error) {
	if f.connected {
		return Credential{Token: "tok"}, nil
	}
	return Credential{}, errors.New("not connected")
}
```

Before writing this, read `get_auth_status.go`'s actual `Execute` body and
match this fake to its real not-connected-detection contract exactly (it
may check a specific sentinel error type rather than "any error" — adjust
`fakeCredentialResolverConnectedFlag` to match, don't assume).

### Step 2: `BranchExists` adapter tests

**File:** `services/scm-integration-service/internal/adapter/github/branch_exists_test.go`

```go
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestBranchExists_ReturnsTrueOn200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "o/r", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true on 200")
	}
}

func TestBranchExists_ReturnsFalseOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "o/r", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists=false on 404")
	}
}
```

Add the identical pair (`services/scm-integration-service/internal/adapter/gitlab/branch_exists_test.go`)
against `gitlab.Client.BranchExists`.

### Step 3: wscompat channel tests

**File:** `services/api-gateway/internal/adapter/wscompat/channels_hostedreview_test.go`

```go
package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

type fakeScmIntegrationClientHostedReview struct {
	scmintegrationv1.ScmIntegrationServiceClient

	getPullRequestForBranchFunc func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error)
	checkEligibilityFunc        func(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest) (*scmintegrationv1.HostedReviewEligibility, error)
}

func (f *fakeScmIntegrationClientHostedReview) GetPullRequestForBranch(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest, _ ...grpc.CallOption) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
	return f.getPullRequestForBranchFunc(ctx, in)
}

func (f *fakeScmIntegrationClientHostedReview) CheckHostedReviewEligibility(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest, _ ...grpc.CallOption) (*scmintegrationv1.HostedReviewEligibility, error) {
	return f.checkEligibilityFunc(ctx, in)
}

func TestHostedReviewForBranchChannel_ReturnsNilWhenNotFound(t *testing.T) {
	fake := &fakeScmIntegrationClientHostedReview{
		getPullRequestForBranchFunc: func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
			return &scmintegrationv1.GetPullRequestForBranchResponse{Found: false}, nil
		},
	}

	r := NewRegistry()
	registerHostedReviewChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "hostedReview.forBranch",
		argsJSON(t, map[string]any{"provider": "github", "repo": "o/r", "headBranch": "feature-x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result when not found, got %+v", result)
	}
}

func TestHostedReviewGetCreationEligibilityChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.CheckHostedReviewEligibilityRequest
	fake := &fakeScmIntegrationClientHostedReview{
		checkEligibilityFunc: func(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest) (*scmintegrationv1.HostedReviewEligibility, error) {
			gotReq = in
			return &scmintegrationv1.HostedReviewEligibility{Eligible: true}, nil
		},
	}

	r := NewRegistry()
	registerHostedReviewChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "hostedReview.getCreationEligibility",
		argsJSON(t, map[string]any{"provider": "gitlab", "repo": "group/project", "headBranch": "feature-x", "baseBranch": "main"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.HostedReviewEligibility)
	if !ok || !resp.GetEligible() {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB {
		t.Errorf("expected SCM_PROVIDER_GITLAB from parseWSProvider(\"gitlab\"), got %v", gotReq.GetProvider())
	}
}
```

Add `TestHostedReviewCreateChannel_Success` (asserts `hostedReview.create`
maps to `CreatePullRequest` and returns `resp.GetPullRequest()` unwrapped),
following the same fake-embed-and-override shape.

### Step 4: Contract test

Add `TestHostedReviewCreateChannelMatchesRESTContract`, mirroring TASK-082
Step 6 / TASK-086 Step 4: `hostedReview.create` channel and `POST
/v1/scm/pull-requests` (`scm_routes.go`) both resolve to `CreatePullRequest`
with an identical response shape.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go test ./internal/usecase/... ./internal/adapter/github/... ./internal/adapter/gitlab/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v

cd /opt/repos/orca/backend-go
buf breaking proto --against '.git#branch=main'
```

Expected: all new and existing tests pass; `buf breaking` reports no
breaking changes across TASK-087's additions.
