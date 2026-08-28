# TASK-011: Tests for `normalizeNilSlices` and each confirmed BUG-005 channel

**From Solution:** SOL-005
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/registry_test.go` (extends TASK-002's new file), `channels_tenant_project_test.go`, `channels_repo_ssh_status_workspace_test.go`, `channels_team_test.go`, `channels_credentials_test.go`
**Depends on:** TASK-010
**Status:** `[x]` DONE — unit tests (`TestNormalizeNilSlices`, `TestNormalizeNilSlices_JSONShape`) plus 2 tests not in the original plan (`TestNormalizeNilSlices_ProtoMessagePassesThroughUntouched`, `TestNormalizeNilSlices_NonProtoPointerIsNotSpeciallyHandled` — added to lock in TASK-010's proto-safety correction) in `registry_test.go`; one regression test per confirmed channel added directly to each channel's existing test file: `TestProjectGroupListChannel_EmptyResult_ReturnsEmptyArrayNotNull` (`channels_tenant_project_test.go`), a `ssh.listTargets empty result...` subtest inside `TestRegisterSshChannels` (`channels_repo_ssh_status_workspace_test.go`), `TestTeamListChannel_EmptyResult_ReturnsEmptyArrayNotNull` (`channels_team_test.go`), `TestCredentialsList_EmptyResult_ReturnsEmptyArrayNotNull` (`channels_credentials_test.go`). All pass; `go test ./services/api-gateway/... -count=1` clean.

---

## Context

Direct unit coverage for `normalizeNilSlices`'s reflection logic, plus one
regression test per confirmed BUG-005 instance
(`projectGroup.list`/`ssh.listTargets`/`team.list`/`credentials.list`)
proving the actual dispatched JSON is `[]`/`{"services":[]}`, not `null`,
when the upstream response is empty.

## Changes to make

### Step 1 — unit tests for `normalizeNilSlices`, in `registry_test.go`

```go
func TestNormalizeNilSlices(t *testing.T) {
	type withList struct {
		Groups []string
		Name   string
	}

	cases := []struct {
		name string
		in   any
		want any
	}{
		{"nil top-level slice", []string(nil), []string{}},
		{"non-nil empty slice unchanged", []string{}, []string{}},
		{"populated slice unchanged", []string{"a"}, []string{"a"}},
		{"nil field one level in", withList{Groups: nil, Name: "x"}, withList{Groups: []string{}, Name: "x"}},
		{"populated field one level in unchanged", withList{Groups: []string{"g1"}, Name: "x"}, withList{Groups: []string{"g1"}, Name: "x"}},
		{"nil input passes through", nil, nil},
		{"non-slice scalar passes through", 42, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeNilSlices(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("normalizeNilSlices(%#v) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

// TestNormalizeNilSlices_JSONShape is the shape-level assertion that
// actually matters for BUG-005 — reflect.DeepEqual on a Go nil vs. []T
// slice can look equivalent in some comparisons; what the frontend
// receives is the JSON encoding, so assert that directly too.
func TestNormalizeNilSlices_JSONShape(t *testing.T) {
	got := normalizeNilSlices([]string(nil))
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("expected JSON [], got %s", b)
	}
}
```

Add `"reflect"` to this test file's imports if TASK-002 didn't already.

### Step 2 — regression test per confirmed channel

Following each file's existing fake-client test pattern (see
`channels_repo_ssh_status_workspace_test.go`'s `TestRegisterSshChannels`
for the shape to copy), add one test per channel asserting an empty
upstream response dispatches to `[]`/`{"services":[]}`, not `null`:

```go
// channels_tenant_project_test.go — add:
func TestProjectGroupListChannel_EmptyResult_ReturnsEmptyArrayNotNull(t *testing.T) {
	fake := &fakeProjectClient{listProjectGroupsFunc: func(ctx context.Context, in *projectv1.ListProjectGroupsRequest) (*projectv1.ListProjectGroupsResponse, error) {
		return &projectv1.ListProjectGroupsResponse{}, nil // Groups left nil
	}}
	r := NewRegistry()
	registerTenantProjectChannels(r, fake /* + whatever other args this func's real signature needs */)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "projectGroup.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("expected [], got %s", b)
	}
}
```

Repeat the same shape for:
- `channels_repo_ssh_status_workspace_test.go`: `ssh.listTargets` with a
  fake `ListSshTargets` returning `&infrafleetv1.ListSshTargetsResponse{}`
  (nil `SshTargets`) → expect `[]`.
- `channels_team_test.go`: `team.list` with a fake `ListTeams` returning
  `&tenantv1.ListTeamsResponse{}` (nil `Teams`) → expect `[]`.
- `channels_credentials_test.go`: `credentials.list` with both fake
  clients returning zero credentials → expect
  `{"services":[],"mode":"server"}` (adjust the exact expected JSON to
  match `handleCredentialsList`'s real response struct's field names/`mode`
  value — check the real current handler before hardcoding the expected
  string).

For each, use whatever fake-client construction pattern that specific test
file already established for OTHER tests in the same file (e.g.
`fakeProjectClient`/`fakeRepoSshStatusWorkspaceInfraFleetClient`) rather
than inventing a new one — these fakes already exist for each file's
existing coverage; only the specific test case (empty response) is new.

## Verify

```bash
cd backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1 -v -run 'TestNormalizeNilSlices|TestProjectGroupListChannel_EmptyResult|TestSshListTargetsChannel_EmptyResult|TestTeamListChannel_EmptyResult|TestCredentialsListChannel_EmptyResult'
```

Expected: all new tests pass, confirming `result:null` → `result:[]`
(or the equivalent wrapped shape) for all 4 confirmed BUG-005 channels.
