# TASK-PW-01-02: Wire `task.hasActiveExecutions` wscompat channel

**From Solution:** SOL-PW-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** [x] DONE — `task.hasActiveExecutions` channel wired in channels.go (no AttachIdentity, matches task.create/get convention); TestRegisterTaskChannels_HasActiveExecutions passes alongside TestTaskCreateGetChannels_StillRegistered.

---

## Context

`TaskService.HasActiveExecutions` (`backend-go/proto/orca/task/v1/task.proto:133-139`)
is the sibling RPC to TASK-PW-01-01's `workflow.hasActiveExecutions` — same
gap, same fix shape, in `registerTaskChannels` instead of
`registerWorkflowChannels`. The response field is `has_active`, not
`has_active_executions` — confirm against `task.proto:137-139` before
wiring the getter.

## Changes to make

In `channels.go`'s `registerTaskChannels` (currently `task.create`/
`task.get` only, lines ~227-260), add a third registration before the
function's closing `}`:

```go
r.Register("task.hasActiveExecutions", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type hasActiveArgs struct {
		ProjectID string `json:"projectId"`
	}
	in, err := decodeArg[hasActiveArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.HasActiveExecutions(ctx, &taskv1.HasActiveExecutionsRequest{ProjectId: in.ProjectID})
	if err != nil {
		return nil, err
	}
	return map[string]bool{"hasActiveExecutions": resp.GetHasActive()}, nil
})
```

Unlike TASK-PW-01-01's `workflow.*` sibling, `task.create`/`task.get`
in this file do not call `gatewaygrpc.AttachIdentity` on their own
context — match the existing local convention in this function rather
than introducing it unilaterally; if `TaskServiceClient`'s tenant
scoping is actually handled elsewhere (e.g. an interceptor on `client`'s
underlying `ClientConn`), verify that before assuming this channel needs
`AttachIdentity` added.

Also update this file's stale package doc comment immediately above
`registerTaskChannels` (the one noting "execute/AI-decompose are not
wired... still stubs") to mention `hasActiveExecutions` is now real,
rather than leaving it further out of date.

Add a test — `TestRegisterTaskChannels_HasActiveExecutions` — in
`channels_automation_task_test.go` (the file already covering
`registerTaskChannels`, per its existing `TestTaskCreateGetChannels_StillRegistered`
test):

```go
func TestRegisterTaskChannels_HasActiveExecutions(t *testing.T) {
	fake := &fakeTaskServiceClient{}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"},
		"task.hasActiveExecutions", []json.RawMessage{[]byte(`{"projectId":"p1"}`)})

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"hasActiveExecutions": false}, result)
	assert.Equal(t, "p1", fake.lastHasActiveExecutionsRequest.GetProjectId())
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterTaskChannels_HasActiveExecutions -v
```

Expected: clean build; new test passes alongside the existing
`TestTaskCreateGetChannels_StillRegistered` test (no regression).
