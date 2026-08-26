# TASK-155: Tests for `repo.add`/`list`/`reorder`/`rm`/`update` (project-service group)

**From Solution:** SOL-023 (Bucket 1 + Bucket 2 test plan)
**Priority:** P1
**Service:** `project-service` + `api-gateway`
**File:** `services/project-service/internal/usecase/update_repo_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-151, TASK-152, TASK-153, TASK-154
**Status:** `[ ]` TODO

---

## Changes to make

### New file `services/project-service/internal/usecase/update_repo_test.go`

Table-driven test over `UpdateRepo.Execute`, fake `RepoRepository`
(match whatever fake already exists for `AddRepo`'s/`RemoveRepo`'s tests
in this package — reuse it rather than writing a second fake):

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
	"github.com/stablyai/orca-go/services/project-service/internal/usecase"
)

func TestUpdateRepo_FieldMaskSemantics(t *testing.T) {
	cases := []struct {
		name            string
		existing        domain.Repo
		in              usecase.UpdateRepoInput
		wantURL         string
		wantDisplayName string
	}{
		{
			name:            "empty url and display name leave both unchanged",
			existing:        domain.Repo{ID: "r1", URL: "https://old", DisplayName: "Old"},
			in:              usecase.UpdateRepoInput{RepoID: "r1"},
			wantURL:         "https://old",
			wantDisplayName: "Old",
		},
		{
			name:            "non-empty url overwrites, display name unchanged",
			existing:        domain.Repo{ID: "r1", URL: "https://old", DisplayName: "Old"},
			in:              usecase.UpdateRepoInput{RepoID: "r1", URL: "https://new"},
			wantURL:         "https://new",
			wantDisplayName: "Old",
		},
		{
			name:            "both fields overwrite",
			existing:        domain.Repo{ID: "r1", URL: "https://old", DisplayName: "Old"},
			in:              usecase.UpdateRepoInput{RepoID: "r1", URL: "https://new", DisplayName: "New"},
			wantURL:         "https://new",
			wantDisplayName: "New",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepoRepository{repos: map[string]domain.Repo{"r1": tc.existing}}
			uc := usecase.NewUpdateRepo(repo)
			ctx := tenant.WithTenantID(context.Background(), "t1")

			got, err := uc.Execute(ctx, tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.URL != tc.wantURL || got.DisplayName != tc.wantDisplayName {
				t.Errorf("got {URL:%q DisplayName:%q}, want {URL:%q DisplayName:%q}",
					got.URL, got.DisplayName, tc.wantURL, tc.wantDisplayName)
			}
		})
	}
}
```

Adjust `fakeRepoRepository`/`tenant.WithTenantID` to match whichever
test-fake and tenant-context helpers this package's existing tests
(`add_repo_test.go`, `remove_repo_test.go`, etc.) already use — do not
invent a second fake type if one is reusable.

### `services/api-gateway/internal/adapter/wscompat/channels_test.go`

Add one test per channel (`repo.add`, `repo.list`, `repo.reorder`,
`repo.rm`, `repo.update`), following the exact fake-client shape this
file already uses for `registerGitChannels`'s tests — assert the decoded
WS args become the expected gRPC request fields, and the response passes
through unchanged:

```go
func TestRegisterRepoChannels_AddListReorderRmUpdate(t *testing.T) {
	fake := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerRepoChannels(r, fake, nil)

	t.Run("repo.add", func(t *testing.T) {
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.add",
			[]json.RawMessage{[]byte(`{"projectId":"p1","url":"https://x","displayName":"X"}`)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fake.lastAddRepoReq.GetProjectId() != "p1" || fake.lastAddRepoReq.GetUrl() != "https://x" {
			t.Errorf("unexpected AddRepoRequest: %+v", fake.lastAddRepoReq)
		}
	})

	// repo.list / repo.reorder / repo.rm / repo.update follow the same
	// decode-dispatch-assert shape — one subtest per channel, asserting
	// against the corresponding fake*Req field.
}
```

`fakeProjectServiceClient` needs `AddRepo`/`ListRepos`/`ReorderRepos`/
`RemoveRepo`/`UpdateRepo` methods recording the last request received —
add it to this test file (or a shared test-fakes file in this package if
one already exists) implementing `projectv1.ProjectServiceClient`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/project-service/internal/usecase/... -run TestUpdateRepo -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterRepoChannels -v
```

Expected: all new tests pass.
