# TASK-082: Tests for `github.*`/`github.project.*` usecases, adapter, and wscompat channels

**From Solution:** SOL-012 (Test plan)
**Priority:** P1
**Service:** `scm-integration-service` + `api-gateway`
**File:** `services/scm-integration-service/internal/usecase/scm_provider_dispatch_test.go`, `merge_pull_request_test.go` (new), `services/scm-integration-service/internal/adapter/github/projects_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_github_test.go` (new)
**Depends on:** TASK-073, TASK-074, TASK-075, TASK-076, TASK-078, TASK-079, TASK-080, TASK-081
**Status:** `[ ]` TODO

---

## Context

Follows this codebase's existing fake-based usecase test pattern
(`scm_provider_dispatch_test.go`'s `fakeProvider`/`fakeRegistry`/
`fakeCredentialResolver`) and `channels_test.go`'s fake-gRPC-client wscompat
test pattern exactly.

---

## Changes to make

### Step 1: Extend `fakeProvider` with the 7 new `ScmProvider` methods

**File:** `services/scm-integration-service/internal/usecase/scm_provider_dispatch_test.go`

`fakeProvider` must implement every `ScmProvider` method (TASK-073/074
extended the interface) or every existing test in this package fails to
compile. Add to the `fakeProvider` struct:

```go
	mergedPR       domain.PullRequest
	merged         bool
	mergeSHA       string
	mergeErr       error
	reviewersPR    domain.PullRequest
	reviewersErr   error
	autoMergePR    domain.PullRequest
	autoMergeErr   error
	updatedIssue   domain.Issue
	updateIssueErr error
	branchPR       domain.PullRequest
	branchFound    bool
	branchErr      error
	slugOwner      string
	slugName       string
	slugErr        error
```

And the methods:

```go
func (f *fakeProvider) MergePullRequest(ctx context.Context, cred Credential, repo string, number int32, input MergePullRequestInput) (domain.PullRequest, bool, string, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.mergeErr != nil {
		return domain.PullRequest{}, false, "", f.mergeErr
	}
	return f.mergedPR, f.merged, f.mergeSHA, nil
}

func (f *fakeProvider) RequestPullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins, teamSlugs []string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.reviewersErr != nil {
		return domain.PullRequest{}, f.reviewersErr
	}
	return f.reviewersPR, nil
}

func (f *fakeProvider) RemovePullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins []string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.reviewersErr != nil {
		return domain.PullRequest{}, f.reviewersErr
	}
	return f.reviewersPR, nil
}

func (f *fakeProvider) SetPullRequestAutoMerge(ctx context.Context, cred Credential, repo string, number int32, enabled bool, mergeMethod string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.autoMergeErr != nil {
		return domain.PullRequest{}, f.autoMergeErr
	}
	return f.autoMergePR, nil
}

func (f *fakeProvider) UpdateIssue(ctx context.Context, cred Credential, repo string, number int32, patch IssuePatch) (domain.Issue, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.updateIssueErr != nil {
		return domain.Issue{}, f.updateIssueErr
	}
	return f.updatedIssue, nil
}

func (f *fakeProvider) GetPullRequestForBranch(ctx context.Context, cred Credential, repo, headBranch string) (domain.PullRequest, bool, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.branchErr != nil {
		return domain.PullRequest{}, false, f.branchErr
	}
	return f.branchPR, f.branchFound, nil
}

func (f *fakeProvider) ResolveRepoSlug(ctx context.Context, cred Credential, candidate string) (string, string, error) {
	f.lastCred = cred
	f.calls++
	if f.slugErr != nil {
		return "", "", f.slugErr
	}
	return f.slugOwner, f.slugName, nil
}
```

### Step 2: New usecase test — `merge_pull_request_test.go` (full representative)

**File:** `services/scm-integration-service/internal/usecase/merge_pull_request_test.go`

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestMergePullRequest_Success(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "closed", "url", "head", "base")
	provider := &fakeProvider{mergedPR: pr, merged: true, mergeSHA: "abc123"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry)

	result, err := uc.Execute(context.Background(), MergePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1, MergeMethod: "squash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Merged || result.SHA != "abc123" {
		t.Errorf("expected merged=true sha=abc123, got merged=%v sha=%s", result.Merged, result.SHA)
	}
	if provider.calls != 1 {
		t.Errorf("expected provider called exactly once, got %d", provider.calls)
	}
}

func TestMergePullRequest_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{mergeErr: errors.New("merge conflict")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), MergePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1,
	})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestMergePullRequest_RequiresTenantRepoNumber(t *testing.T) {
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}}
	uc := NewMergePullRequest(&fakeCredentialResolver{}, registry)

	cases := []MergePullRequestParams{
		{Repo: "o/r", Number: 1},
		{TenantID: "tenant-1", Number: 1},
		{TenantID: "tenant-1", Repo: "o/r"},
	}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
```

### Step 3: Remaining usecase tests — same shape

Add one `Test<Type>_Success` / `Test<Type>_PropagatesProviderFailure` pair
per remaining usecase (TASK-073/074/078's 22 usecases:
`RequestPullRequestReviewers`, `RemovePullRequestReviewers`,
`SetPullRequestAutoMerge`, `UpdateIssue`, `GetPullRequestForBranch`,
`ResolveRepoSlug`, and the 16 `GitHubProjectsProvider`-backed ones), each in
its own `<snake_case_name>_test.go`, following `TestMergePullRequest_*`'s
exact fake-provider/fake-registry/fake-credential-resolver shape. For the
`GitHubProjectsProvider`-backed usecases, add a matching `fakeGitHubProjects`
test double (implementing all 16 `GitHubProjectsProvider` methods,
mirroring `fakeProvider`'s recording-fields pattern) in
`scm_provider_dispatch_test.go` alongside `fakeProvider`, plus a
`TestUpdateProjectItemField_RejectsNonGitHubProvider` case asserting the
`SCM_PROVIDER_UNSUPPORTED` rejection path (TASK-078 Step 2) — the one
validation rule specific to this port that `fakeProvider`'s tests don't
otherwise exercise.

### Step 4: GitHub Projects v2 GraphQL adapter test

**File:** `services/scm-integration-service/internal/adapter/github/projects_test.go`

```go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestListAccessibleProjects_ParsesGraphQLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			// This test overrides DefaultGraphQLURL is not possible without a
			// constructor param — see the note below before writing this test.
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"viewer": map[string]any{
					"projectsV2": map[string]any{
						"nodes": []map[string]any{
							{"id": "PVT_1", "number": 7, "title": "Roadmap", "url": "https://github.com/orgs/acme/projects/7",
								"owner": map[string]any{"login": "acme"}},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	// graphQLRequest (TASK-075) posts to the package-level constant
	// DefaultGraphQLURL, not c.baseURL — to point this test at httptest,
	// either (a) add a graphQLURL field to Client with the same
	// nil/empty-defaults-to-DefaultGraphQLURL convention baseURL already
	// uses (preferred — mirrors New(httpClient, baseURL)'s existing
	// pattern, requires updating graphQLRequest to use c.graphQLURL instead
	// of the constant), or (b) skip this test at the unit level and rely on
	// the wscompat contract test (TASK-082 Step 6) plus manual verification
	// against a real GitHub App. Prefer (a): it costs one field + one line
	// in New(), and makes every GraphQL-backed method in this package
	// (TASK-079) unit-testable the same way baseURL already makes the REST
	// methods testable via client_test.go.
	client := New(server.Client(), server.URL)
	client.graphQLURL = server.URL + "/graphql" // add this field per the note above

	projects, err := client.ListAccessibleProjects(context.Background(), usecase.Credential{Token: "tok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Slug != "acme/7" {
		t.Fatalf("expected one project with slug acme/7, got %+v", projects)
	}
}
```

Before writing this test, apply the small adjustment its own comment
describes: add a `graphQLURL string` field to `Client` (`client.go`), default
it to `DefaultGraphQLURL` in `New` the same way `baseURL` defaults to
`DefaultBaseURL`, and change `graphQLRequest` (TASK-075) to POST to
`c.graphQLURL` instead of the `DefaultGraphQLURL` constant directly. Add at
least one more test covering `UpdateProjectItemField`'s field-kind-to-
GraphQL-input mapping (`"text"`/`"number"`/`"date"`/`"single_select"`,
TASK-079 Step 4) with an httptest fixture asserting the request body's
`variables.value` shape per kind, and one for `GetWorkItemDetailsBySlug`
(REST-backed) asserting the `owner/repo#number` slug parses correctly.

### Step 5: wscompat channel tests

**File:** `services/api-gateway/internal/adapter/wscompat/channels_github_test.go`

```go
package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// fakeScmIntegrationClient is a minimal test double for
// scmintegrationv1.ScmIntegrationServiceClient — same embed-and-override
// shape as fakeInfraFleetClient (channels_test.go).
type fakeScmIntegrationClient struct {
	scmintegrationv1.ScmIntegrationServiceClient

	mergePullRequestFunc func(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest) (*scmintegrationv1.MergePullRequestResponse, error)
}

func (f *fakeScmIntegrationClient) MergePullRequest(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest, _ ...grpc.CallOption) (*scmintegrationv1.MergePullRequestResponse, error) {
	return f.mergePullRequestFunc(ctx, in)
}

func TestGitHubMergePRChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.MergePullRequestRequest
	fake := &fakeScmIntegrationClient{
		mergePullRequestFunc: func(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest) (*scmintegrationv1.MergePullRequestResponse, error) {
			gotReq = in
			return &scmintegrationv1.MergePullRequestResponse{Merged: true, Sha: "abc123"}, nil
		},
	}

	r := NewRegistry()
	registerGitHubChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "github.mergePR",
		argsJSON(t, map[string]any{"repo": "o/r", "number": 42, "mergeMethod": "squash"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.MergePullRequestResponse)
	if !ok || !resp.GetMerged() || resp.GetSha() != "abc123" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB {
		t.Errorf("expected SCM_PROVIDER_GITHUB, got %v", gotReq.GetProvider())
	}
	if gotReq.GetRepo() != "o/r" || gotReq.GetNumber() != 42 {
		t.Errorf("expected repo=o/r number=42, got repo=%s number=%d", gotReq.GetRepo(), gotReq.GetNumber())
	}
}

func TestGitHubPRForBranchChannel_ReturnsNilWhenNotFound(t *testing.T) {
	fake := &fakeScmIntegrationClient{}
	// getPullRequestForBranchFunc wired the same way as mergePullRequestFunc
	// above, returning &scmintegrationv1.GetPullRequestForBranchResponse{Found: false}.
	// Add the corresponding method override + field to fakeScmIntegrationClient
	// following the same embed-and-override pattern.
	_ = fake
	t.Skip("wire GetPullRequestForBranch on fakeScmIntegrationClient per the comment above, then assert result == nil, err == nil")
}
```

Extend `fakeScmIntegrationClient` with one method-override field per
`github.*`/`github.project.*` channel this file's real client calls (26
total: 10 from TASK-080, 16 from TASK-081), and write one
`Test<Channel>Channel_Success` per channel following
`TestGitHubMergePRChannel_Success`'s shape — decode args, assert the
outbound request's field mapping, assert the returned result. Include:
- `TestGitHubPRForBranchChannel_ReturnsNilWhenNotFound` — finish the skipped
  test above (asserts `github.prForBranch` returns `nil, nil` when
  `Found == false`, mirroring SOL-014's `hostedReview.forBranch` test case).
- `TestGitHubUpdateIssueChannel_OmitsUnsetOptionalFields` — asserts that
  when `title`/`body`/`state` are absent from `args[0]`, the outbound
  `UpdateIssueRequest`'s corresponding `optional` fields are `nil`
  (`req.Title == nil`), not empty-string-set.

### Step 6: Contract test

Add `TestGitHubRateLimitChannelMatchesRESTContract` (or a `_test.go` file of
your choosing) asserting `github.rateLimit`'s channel and
`GET /v1/scm/rate-limit?provider=github` (`scm_routes.go`) both resolve to
`GetRateLimitStatus` with an identical response shape — same
regression-guard pattern SOL-001/SOL-012 use elsewhere: construct both
paths against the same `fakeScmIntegrationClient`, assert `reflect.DeepEqual`
(or field-by-field) equality between the channel's returned `any` and the
REST handler's JSON body.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go test ./internal/usecase/... ./internal/adapter/github/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v

cd /opt/repos/orca/backend-go
buf breaking proto --against '.git#branch=main'
```

Expected: all new and existing tests pass; `buf breaking` reports no
breaking changes across TASK-071/072/077's additions.
