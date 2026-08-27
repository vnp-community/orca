# TASK-222: Wire `task.execute` wscompat channel; document `task.create`/`task.get`'s unconfirmed frontend call site

**From Solution:** SOL-034 ("Lead finding" + `execute` section)
**Priority:** P0 — zero new backend risk, ship first
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[x]` DONE — `task.execute` registered in `channels_automation_task.go`'s `registerTaskCRUDChannels`. `go build`/`go vet`/`go test` clean.

---

## Context

Two independent, small changes bundled into one task since both are pure
`wscompat`-file edits with no other dependency:

1. **`task.execute`** already has a complete RPC/usecase/REST path
   (`task.proto:16`, `execute_task.go`, `task_routes.go:25`) — wiring the
   channel is a thin wrapper, identical in shape to SOL-032/SOL-033's Part
   1 wins.
2. **Lead finding**: `task.create`/`task.get` are wired against genuine
   `CreateTask`/`GetTask` RPCs but neither appears in the frontend's actual
   `task.*` call table (`rpc-catalog.md:435-445`) — BUG-034's dead-code
   finding. Per SOL-034: **keep both, do not remove them.** Removing
   working code backed by a real, tested RPC to close an *unconfirmed*
   dead-code hypothesis is the riskier move, and `CreateTask`/`GetTask` are
   structurally load-bearing for TASK-223's new usecases (`list`, `update`,
   `delete`, `getDependencies` all need `Get`-shaped reads). This task
   closes the finding with a doc comment, not a deletion.

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Wire `task.execute`

Add to `registerTaskChannels`, after the existing `task.get` registration:

```go
	r.Register("task.execute", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type executeArgs struct {
			TaskID    string `json:"taskId"`
			RequestID string `json:"requestId"`
		}
		in, err := decodeArg[executeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Execute(ctx, &taskv1.TaskServiceExecuteRequest{TaskId: in.TaskID, RequestId: in.RequestID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

### Step 2: Document `task.create`/`task.get`'s dead-code finding

Replace the existing comment above `registerTaskChannels`:

```go
// ── task.* (subset: create/get — the DAG/grant channels backing
// task-service's real BFS/cycle-detection logic; execute/AI-decompose are
// not wired since they depend on infra-fleet-service/orchestration-service,
// still stubs — see task-service's own README) ─────────────────────────
```

with:

```go
// ── task.* ───────────────────────────────────────────────────────────────
//
// task.create/task.get have no confirmed frontend call site in
// specs/frontend/api/rpc-catalog.md's task.* table (BUG-034/SOL-034) — kept
// because CreateTask/GetTask back real, working usecases and this
// package's own list/update/delete/getDependencies usecases (SOL-034 Part
// 2, TASK-223/TASK-225) reuse Get directly. Do not delete without first
// confirming via a full frontend-source grep, not just the rpc-catalog
// audit, that nothing reaches these two channels.
//
// task.execute is real end-to-end as of this comment
// (execute_task.go/task_routes.go); aiDecompose/aiApply and
// list/update/delete/getDependencies are tracked in TASK-223 through
// TASK-226.
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -run TestTask -v
```

Expected: clean build; a
`TestTaskCreateGetChannels_StillRegistered` regression test (added in
TASK-226) is not required by this task but this task must not remove
either channel's registration.
