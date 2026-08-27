# TASK-225: Wire `task.list`/`update`/`delete`/`getDependencies`/`aiDecompose`/`aiApply` wscompat channels

**From Solution:** SOL-034 (`wscompat` wiring section)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-223, TASK-224
**Status:** `[x]` DONE — `task.list`/`task.update`/`task.delete`/`task.getDependencies`/`task.aiDecompose`/`task.aiApply` all registered in `channels_automation_task.go`'s `registerTaskCRUDChannels`. `go build`/`go vet`/`go test` clean.

---

## Context

Wires the remaining 6 `task.*` frontend channels once TASK-223/TASK-224's
RPCs exist, completing the full 7-real-method `task.*` catalog (`execute`
was wired in TASK-222; `create`/`get` were already wired and are kept per
TASK-222's doc comment).

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add to `registerTaskChannels`, after TASK-222's `task.execute`:

```go
	r.Register("task.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ListTasks(ctx, &taskv1.ListTasksRequest{
			TenantId: id.TenantID, ProjectId: in.ProjectID, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("task.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID     string  `json:"id"`
			Title  *string `json:"title"`
			Status *string `json:"status"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &taskv1.UpdateTaskRequest{Id: in.ID}
		if in.Title != nil {
			req.Title = wrapperspb.String(*in.Title)
		}
		if in.Status != nil {
			req.Status = wrapperspb.String(*in.Status)
		}
		resp, err := client.UpdateTask(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp.GetTask(), nil
	})

	r.Register("task.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteTask(ctx, &taskv1.DeleteTaskRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil
	})

	r.Register("task.getDependencies", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type depsArgs struct {
			TaskID string `json:"taskId"`
		}
		in, err := decodeArg[depsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetDependencies(ctx, &taskv1.GetDependenciesRequest{TaskId: in.TaskID})
		if err != nil {
			return nil, err
		}
		return resp.GetDependencies(), nil
	})

	r.Register("task.aiDecompose", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type decomposeArgs struct {
			TaskID string `json:"taskId"`
		}
		in, err := decodeArg[decomposeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.AIDecompose(ctx, &taskv1.AIDecomposeRequest{TaskId: in.TaskID})
		if err != nil {
			return nil, err
		}
		return resp.GetProposals(), nil
	})

	r.Register("task.aiApply", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type applyArgs struct {
			TaskID    string                   `json:"taskId"`
			Proposals []taskv1.SubtaskProposal `json:"proposals"`
		}
		in, err := decodeArg[applyArgs](args, 0)
		if err != nil {
			return nil, err
		}
		proposals := make([]*taskv1.SubtaskProposal, len(in.Proposals))
		for i := range in.Proposals {
			proposals[i] = &in.Proposals[i]
		}
		resp, err := client.AIApply(ctx, &taskv1.AIApplyRequest{TaskId: in.TaskID, Proposals: proposals})
		if err != nil {
			return nil, err
		}
		return resp.GetCreatedSubtasks(), nil
	})
```

Add `"google.golang.org/protobuf/types/known/wrapperspb"` to this file's
import block if TASK-219 (automation's channels) hasn't already added it —
`wscompat` needs it exactly once regardless of how many `*.update` channels
use it.

Update the `task.*` channel-count doc comment (from TASK-222) to reflect
all 7 real methods plus the 2 kept-but-unconfirmed ones now wired:

```go
// ── task.* ───────────────────────────────────────────────────────────────
//
// task.create/task.get have no confirmed frontend call site in
// specs/frontend/api/rpc-catalog.md's task.* table (BUG-034/SOL-034) — kept
// per TASK-222's doc comment, do not delete without a full frontend-source
// grep confirming they're unreachable.
//
// All 7 of the frontend's real task.* methods are now wired: execute
// (TASK-222), list/update/delete/getDependencies (TASK-223/TASK-225),
// aiDecompose/aiApply (TASK-224/TASK-225). ComplexExecutor remains a stub
// (task.execute's complex branch) — see TASK-224's Context note; not part
// of this pass's scope.
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
```
