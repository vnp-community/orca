# SOL-PW-01: Wire `workflow.hasActiveExecutions` / `task.hasActiveExecutions` into wscompat

**Resolves:** [BUG-PW-01](../BUG-PW-01-workspace-active-executions-unwired.md)
**Service:** `api-gateway` (wscompat only — both gRPC methods already exist and work)
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`registerTaskChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

This is the narrowest possible gap in the whole `logic-v1` batch: both RPCs
the frontend needs are already real, tested, and running —

- `orca.workflow.v1.WorkflowService/HasActiveExecutions` —
  `backend-go/proto/orca/workflow/v1/workflow.proto:47-51` (doc comment:
  "added for Epic C ... to close `project-service.RebindDevServer`'s guard,
  previously a client-side no-op because this RPC didn't exist"),
  implemented at `services/workflow-service/internal/usecase/has_active_executions.go:19-43`
  and already consumed server-side by
  [`project-service.md`](../../../tdd/services/project-service.md) §3's
  `RebindDevServer` saga (`PS->>WF: HasActiveExecutions(projectId)`).
- `orca.task.v1.TaskService/HasActiveExecutions` —
  `backend-go/proto/orca/task/v1/task.proto:19-27` (identical Epic C
  provenance, same doc comment pattern), backing the same
  `RebindDevServer` saga's `PS->>TS: HasActiveExecutions(projectId)` step.

Both are genuinely wired **service-to-service** (project-service calls
them today for the rebind guard, per `project-service.md`'s §3 sequence
diagram, `PS->>WF`/`PS->>TS` steps) — confirmed live via `grep -rn
"hasActiveExecutions|HasActiveExecutions"
services/api-gateway/internal/adapter/wscompat/*.go` returning **zero**
matches, i.e. only the gateway-facing leg is missing, not the capability
itself. BL-PW-01's "Load workspace data (parallel)" step
(`docs/logic/project-workspace/BL-PW-01-workspace-context.md:79-96`) names
`WorkflowService.getActiveExecutions(projectId)` as the 4th parallel leg
alongside `git.status`/`worktree.list`/`fs.readDir` — all three of which
are already real wscompat channels (`channels.go:267-281`,
`channels_worktree.go:159-174`, `channels_git.go:946-975`, per BUG-PW-01's
own citations). This solution is exactly that 4th leg, plus the sibling
`task.*` answer since `WorkspaceContext`'s "active workflows" concept is
really "does this project have execution happening right now" — a
question workflow-service alone cannot fully answer once task-linked
agent runs exist (see [SOL-PW-04](./SOL-PW-04-workspace-integration-event-bus.md)
for that broader linkage) — so exposing both channels, not just
`workflow.*`, matches what the underlying capability actually covers and
costs nothing extra (same registration pattern, one more RPC call).

No proto, usecase, or domain change is needed anywhere — this is pure
`wscompat` wiring, the same shape `registerWorkflowChannels` (`channels_workflow.go:21-99`)
and `registerTaskChannels` (`channels.go:227-257`) already use for their
other four/two channels respectively.

## Design — wiring (wscompat)

Add one channel to each existing registration function, following the
exact `decodeArg` → `AttachIdentity` → RPC call → return shape every
neighboring channel in these two files already uses.

```go
// channels_workflow.go, inside registerWorkflowChannels — alongside
// workflow.execute/.cancel/.template.create/.template.update.
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
	return map[string]bool{"hasActiveExecutions": resp.GetHasActiveExecutions()}, nil
})
```

```go
// channels.go, inside registerTaskChannels — alongside task.create/task.get.
// Update the stale package doc comment above registerTaskChannels
// ("execute/AI-decompose are not wired... still stubs") to also cover
// this addition rather than leaving it further out of date.
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
	return map[string]bool{"hasActiveExecutions": resp.GetHasActiveExecutions()}, nil
})
```

Both return a small `{hasActiveExecutions: bool}` envelope rather than the
raw proto response — the RPC's only payload is that one bool
(`HasActiveExecutionsResponse`), and every other boolean-flavored channel
in this package (e.g. `files.commitUpload`'s `{"ok": true}`, per
`SOL-009` §"wscompat wiring") already returns a small named map rather
than a bare scalar, so the frontend gets a stable, self-describing shape.

Frontend-side, `WorkspaceContext.tsx`'s parallel-load step calls both
channels (`Promise.all([..., callChannel('workflow.hasActiveExecutions',
{projectId}), callChannel('task.hasActiveExecutions', {projectId})])`)
and ORs the two booleans into `activeWorkflowExecutionIds`'s
non-empty/empty determination — that composition is a frontend concern
outside this solution's backend-go scope, noted here only so the two
channels' contract (independent booleans, not a merged shape) is
unambiguous to whoever wires the frontend side.

## Test plan

- `channels_test.go` — `TestRegisterWorkflowChannels_HasActiveExecutions`:
  fake `WorkflowServiceClient` returning `true`/`false`, assert the
  channel returns `{"hasActiveExecutions": true/false}` and that
  `ProjectId` is forwarded from the decoded arg.
- `channels_test.go` — `TestRegisterTaskChannels_HasActiveExecutions`:
  same shape against a fake `TaskServiceClient`.
- Both: assert `AttachIdentity`/tenant metadata is set on the outbound
  context (mirrors the existing `workflow.execute` test's assertion
  pattern) — a tenant leak here would let one tenant's workspace-switch
  query another tenant's execution state.
- Regression guard: a missing/empty `projectId` arg surfaces the RPC's own
  `WORKFLOW_HAS_ACTIVE_EXECUTIONS_INVALID`/task-service equivalent
  `INVALID_ARGUMENT` status untouched — wscompat must not swallow or
  reshape that error.

## References

- `docs/logic/project-workspace/BL-PW-01-workspace-context.md:79-96` — the
  parallel workspace-load step naming `WorkflowService.getActiveExecutions(projectId)`
- `backend-go/proto/orca/workflow/v1/workflow.proto:47-51` — `HasActiveExecutions` RPC + doc comment
- `backend-go/proto/orca/task/v1/task.proto:19-27` — task-service's mirror RPC + doc comment
- `backend-go/services/workflow-service/internal/usecase/has_active_executions.go:19-43` — real usecase
- `specs/backend-go/tdd/services/project-service.md:122-151` (§3 `RebindDevServer` sequence diagram) — confirms both RPCs are already real, service-to-service callers today
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go:1-99` — `registerWorkflowChannels`, the four existing channels this solution's fifth follows
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:222-257` — `registerTaskChannels`, the two existing channels this solution's third follows
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:267-281`, `channels_worktree.go:159-174`, `channels_git.go:946-975` — the three parallel-load legs already wired, cited in BUG-PW-01 as the precedent this closes the gap against
