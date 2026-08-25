# TASK-086: Tests for `gitlab.*` usecases, adapter, and wscompat channels

**From Solution:** SOL-013 (Test plan)
**Priority:** P1
**Service:** `scm-integration-service` + `api-gateway`
**File:** `services/scm-integration-service/internal/usecase/list_merge_requests_test.go` (new), `services/scm-integration-service/internal/adapter/gitlab/discussions_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_gitlab_test.go` (new)
**Depends on:** TASK-083, TASK-084, TASK-085
**Status:** `[ ]` TODO

---

## Changes to make

### Step 1: `fakeGitLabMergeRequestProvider` test double

**File:** `services/scm-integration-service/internal/usecase/list_merge_requests_test.go`

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeGitLabMergeRequestProvider is an in-memory GitLabMergeRequestProvider
// — mirrors fakeProvider's recording-fields pattern (scm_provider_dispatch_test.go).
type fakeGitLabMergeRequestProvider struct {
	mrs    []domain.MergeRequest
	mrsErr error

	disc    domain.MergeRequestDiscussion
	discErr error

	details    domain.WorkItemDetailsGitLab
	detailsErr error

	lastCred   Credential
	lastRepo   string
	lastFilter MRFilter
	calls      int
}

func (f *fakeGitLabMergeRequestProvider) ListMergeRequests(ctx context.Context, cred Credential, repo string, filter MRFilter) ([]domain.MergeRequest, error) {
	f.lastCred, f.lastRepo, f.lastFilter = cred, repo, filter
	f.calls++
	if f.mrsErr != nil {
		return nil, f.mrsErr
	}
	return f.mrs, nil
}

func (f *fakeGitLabMergeRequestProvider) ResolveDiscussion(ctx context.Context, cred Credential, repo string, mrIID int32, discussionID string, resolved bool) (domain.MergeRequestDiscussion, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.discErr != nil {
		return domain.MergeRequestDiscussion{}, f.discErr
	}
	return f.disc, nil
}

func (f *fakeGitLabMergeRequestProvider) GetWorkItemDetails(ctx context.Context, cred Credential, repo string, iid int32, itemType string) (domain.WorkItemDetailsGitLab, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.detailsErr != nil {
		return domain.WorkItemDetailsGitLab{}, f.detailsErr
	}
	return f.details, nil
}

func TestListMergeRequests_MapsFilterAndReturnsResults(t *testing.T) {
	mrs := []domain.MergeRequest{{ID: "1", Repo: "group/project", IID: 42, Title: "Fix bug", State: "opened"}}
	provider := &fakeGitLabMergeRequestProvider{mrs: mrs}
	uc := NewListMergeRequests(&fakeCredentialResolver{token: "tok"}, provider)

	got, err := uc.Execute(context.Background(), ListMergeRequestsParams{
		TenantID: "tenant-1", Repo: "group/project", State: "opened", SourceBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].IID != 42 {
		t.Fatalf("expected one MR with iid 42, got %+v", got)
	}
	if provider.lastFilter.State != "opened" || provider.lastFilter.SourceBranch != "feature-x" {
		t.Errorf("expected filter to be passed through, got %+v", provider.lastFilter)
	}
}

func TestListMergeRequests_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeGitLabMergeRequestProvider{mrsErr: errors.New("gitlab unavailable")}
	uc := NewListMergeRequests(&fakeCredentialResolver{token: "tok"}, provider)

	_, err := uc.Execute(context.Background(), ListMergeRequestsParams{TenantID: "tenant-1", Repo: "group/project"})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestListMergeRequests_RequiresTenantAndRepo(t *testing.T) {
	uc := NewListMergeRequests(&fakeCredentialResolver{}, &fakeGitLabMergeRequestProvider{})
	cases := []ListMergeRequestsParams{{Repo: "group/project"}, {TenantID: "tenant-1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
```

Add matching `TestResolveMergeRequestDiscussion_*` and
`TestGetWorkItemDetails_*` tests (success, provider-failure,
missing-required-field) in their own `resolve_merge_request_discussion_test.go`
/ `get_work_item_details_test.go`, same shape.

### Step 2: GitLab adapter test — discussion resolve

**File:** `services/scm-integration-service/internal/adapter/gitlab/discussions_test.go`

```go
package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestResolveDiscussion_SendsResolvedQueryParam(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotMethod = r.URL.Path, r.URL.RawQuery, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"disc-1","notes":[{"resolved":true,"resolved_by":{"username":"alice"}}]}`))
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	disc, err := client.ResolveDiscussion(context.Background(), usecase.Credential{Token: "tok"}, "group/project", 42, "disc-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/projects/group%2Fproject/merge_requests/42/discussions/disc-1" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotQuery != "resolved=true" {
		t.Errorf("expected resolved=true query param, got %q", gotQuery)
	}
	if !disc.Resolved || disc.ResolvedBy != "alice" {
		t.Errorf("unexpected discussion: %+v", disc)
	}
}
```

Add a `TestListMergeRequests_FiltersByStateAndSourceBranch` httptest
fixture asserting the query string built in TASK-084 Step 6, and a
`TestGetWorkItemDetails_SelectsEndpointByItemType` asserting `item_type ==
"issue"` hits `/issues/{iid}` while any other value hits
`/merge_requests/{iid}`.

### Step 3: wscompat channel tests

**File:** `services/api-gateway/internal/adapter/wscompat/channels_gitlab_test.go`

```go
package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

type fakeScmIntegrationClientGitLab struct {
	scmintegrationv1.ScmIntegrationServiceClient

	listMergeRequestsFunc func(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest) (*scmintegrationv1.ListMergeRequestsResponse, error)
}

func (f *fakeScmIntegrationClientGitLab) ListMergeRequests(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListMergeRequestsResponse, error) {
	return f.listMergeRequestsFunc(ctx, in)
}

func TestGitLabListMRsChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.ListMergeRequestsRequest
	fake := &fakeScmIntegrationClientGitLab{
		listMergeRequestsFunc: func(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest) (*scmintegrationv1.ListMergeRequestsResponse, error) {
			gotReq = in
			return &scmintegrationv1.ListMergeRequestsResponse{
				MergeRequests: []*scmintegrationv1.MergeRequest{{Iid: 42, Title: "Fix bug"}},
			}, nil
		},
	}

	r := NewRegistry()
	registerGitLabChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "gitlab.listMRs",
		argsJSON(t, map[string]any{"repo": "group/project", "state": "opened"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.ListMergeRequestsResponse)
	if !ok || len(resp.GetMergeRequests()) != 1 || resp.GetMergeRequests()[0].GetIid() != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetRepo() != "group/project" || gotReq.GetState() != "opened" {
		t.Errorf("expected repo=group/project state=opened, got repo=%s state=%s", gotReq.GetRepo(), gotReq.GetState())
	}
}
```

Add `TestGitLabResolveMRDiscussionChannel_Success` and
`TestGitLabWorkItemDetailsChannel_Success`, following the same
fake-embed-and-override shape, one method-override field per channel this
file's real client calls.

### Step 4: Contract test

Add `TestGitLabRateLimitChannelMatchesRESTContract`, mirroring TASK-082
Step 6's GitHub version: `gitlab.rateLimit` and `GET
/v1/scm/rate-limit?provider=gitlab` both resolve to `GetRateLimitStatus`
with an identical response shape.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go test ./internal/usecase/... ./internal/adapter/gitlab/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v

cd /opt/repos/orca/backend-go
buf breaking proto --against '.git#branch=main'
```

Expected: all new and existing tests pass; `buf breaking` reports no
breaking changes across TASK-083's additions.
