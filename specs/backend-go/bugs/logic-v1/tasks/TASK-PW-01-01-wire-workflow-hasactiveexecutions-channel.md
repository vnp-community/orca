# TASK-PW-01-01: Wire `workflow.hasActiveExecutions` wscompat channel

**From Solution:** SOL-PW-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
**Depends on:** none
**Status:** [x] DONE — `workflow.hasActiveExecutions` channel wired in channels_workflow.go; TestRegisterWorkflowChannels_HasActiveExecutions passes.

---

## Context

`WorkflowService.HasActiveExecutions` is a real, tested RPC
(`backend-go/proto/orca/workflow/v1/workflow.proto:45-50`) already called
service-to-service by `project-service.RebindDevServer`, but no wscompat
channel exposes it to the frontend's `WorkspaceContext` parallel-load step.
This task adds the missing gateway-facing leg — pure wiring, no proto or
usecase change. The response field is `has_active` (not
`has_active_executions`) — confirm this against `workflow.proto:149-151`
before wiring the getter.

## Changes to make

In `registerWorkflowChannels` (`channels_workflow.go`), add a fifth
registration before the function's closing `}` (after
`workflow.template.update`):

```go
r.Register("workflow.hasActiveExecutions", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type hasActiveArgs struct {
		ProjectID string `json:"projectId"`
	}
	in, err := decodeArg[hasActiveArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	resp, err := client.HasActiveExecutions(ctx, &workflowv1.HasActiveExecutionsRequest{ProjectId: in.ProjectID})
	if err != nil {
		return nil, err
	}
	return map[string]bool{"hasActiveExecutions": resp.GetHasActive()}, nil
})
```

Note the response envelope key (`hasActiveExecutions`) is deliberately
different from the proto field name (`has_active`) — this matches
`BL-PW-01-workspace-context.md`'s frontend contract naming, not the wire
field name.

Add a test to `channels_workflow_test.go` (or `channels_test.go`, wherever
the existing `workflow.execute`/`.cancel` tests live) —
`TestRegisterWorkflowChannels_HasActiveExecutions`:

```go
func TestRegisterWorkflowChannels_HasActiveExecutions(t *testing.T) {
	fake := &fakeWorkflowServiceClient{ /* returns HasActiveExecutionsResponse{HasActive: true} */ }
	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"},
		"workflow.hasActiveExecutions", []json.RawMessage{[]byte(`{"projectId":"p1"}`)})

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"hasActiveExecutions": true}, result)
	assert.Equal(t, "p1", fake.lastRequest.GetProjectId())
	// assert tenant metadata was attached to the outbound ctx, matching
	// workflow.execute's existing test pattern — a tenant leak here would
	// let one tenant's workspace-switch query another tenant's execution state.
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterWorkflowChannels_HasActiveExecutions -v
```

Expected: clean build; new test passes; a missing/empty `projectId` arg
surfaces `decodeArg`'s own error, not a swallowed/reshaped one.
