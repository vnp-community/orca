# TASK-111: Wire `orchestration.dispatchShow` in `wscompat`

**From Solution:** SOL-018
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_orchestration.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-110
**Status:** `[x]` DONE — verified `channels_orchestration.go` implements `orchestration.dispatchShow` exactly as this doc describes: calls `GetDispatchContextForTask`, translates `DispatchContext.handle` to `dispatchView.AssigneeHandle` (the wire-naming fix), returns `{dispatch: nil}` when no dispatch exists, leaves `status` as `""` per the documented adjacent-gap note. Confirmed wired into `RegisterRealChannels`/`main.go`. `go build`/`go vet` clean.

---

## Context

SOL-018 resolves BUG-018's "no `assignee_handle` field" finding as a
wire-naming gap, not a missing field — `DispatchContext.handle` IS the
assignee handle (confirmed against `orchestration-service.md` §4's own
naming and `terminal-orchestration-task-links.ts:59-61`'s exact read,
`result.dispatch?.assignee_handle`). Renaming the proto field was rejected
(used by 3 existing RPCs + 2 REST handlers, a real breaking change for
zero behavioral gain) in favor of translating at the `wscompat` adapter
boundary — this task is that translation, plus wiring
`GetDispatchContextForTask` (TASK-110) as the RPC behind
`orchestration.dispatchShow`.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_orchestration.go`

```go
package wscompat

import (
	"context"
	"encoding/json"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// dispatchView is the wire shape orchestration.dispatchShow returns —
// assignee_handle here is DispatchContext.handle under the name
// terminal-orchestration-task-links.ts:59-61 actually reads. See SOL-018's
// "wire-naming gap, not a missing field" note for why the translation
// happens here rather than as a proto rename.
type dispatchView struct {
	ID                  string `json:"id"`
	OrchestrationTaskID string `json:"orchestration_task_id"`
	AssigneeHandle      string `json:"assignee_handle"`
	Status              string `json:"status"`
}

func registerOrchestrationChannels(r *Registry, client orchestrationv1.OrchestrationServiceClient) {
	r.Register("orchestration.dispatchShow", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type dispatchShowArgs struct {
			Task string `json:"task"`
		}
		in, err := decodeArg[dispatchShowArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetDispatchContextForTask(rpcCtx, &orchestrationv1.GetDispatchContextForTaskRequest{
			OrchestrationTaskId: in.Task,
		})
		if err != nil {
			return nil, err
		}
		dc := resp.GetDispatch()
		if dc == nil {
			// No dispatch yet — matches focusRuntimeOrchestrationTask's own
			// null-safe `result.dispatch?.assignee_handle` read and its
			// client-side "No dispatched terminal for orchestration task"
			// error when absent (terminal-orchestration-task-links.ts:60-63).
			return map[string]any{"dispatch": nil}, nil
		}
		return map[string]any{"dispatch": dispatchView{
			ID:                  dc.GetId(),
			OrchestrationTaskID: dc.GetOrchestrationTaskId(),
			AssigneeHandle:      dc.GetHandle(),
			Status:              "", // DispatchContext proto has no status field yet — see note below, out of this task's scope
		}}, nil
	})
}
```

**Adjacent, smaller gap noted for completeness (do not fix here):**
`DispatchContext` (proto) has no `status` field, even though
`domain.DispatchContext` has one (`Status DispatchStatus`) — out of scope
for BUG-018 (it only reports `assignee_handle`/the missing read RPC). Left
as `""` with this comment rather than guessed at; do not add a `status`
field to the proto as part of this task.

### `channels.go` — thread `orchestrationClient` into `RegisterRealChannels`

This task is independent of TASK-100/TASK-106/TASK-108 (SOL-015/016/017) —
add `orchestrationChannels` alongside whatever those already added.
Against today's baseline:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
	orchestrationClient orchestrationv1.OrchestrationServiceClient, // NEW
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerOrchestrationChannels(r, orchestrationClient) // NEW
}
```

If TASK-100/TASK-106/TASK-108 already landed, add the new
`orchestrationClient` parameter and the `registerOrchestrationChannels(r,
orchestrationClient)` call to whatever version of this function already
exists — parameter order/position doesn't matter as long as every call
site (there is exactly one, `main.go`) is updated to match in the same
commit.

Add `orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"`
to `channels.go`'s import block.

### `cmd/server/main.go`

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter, orchestrationClient)
```

`orchestrationClient` is already dialed at line ~189 for the
`/v1/orchestration` REST routes — no new dial, same pattern TASK-100 used
for `issueTrackingClient`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
grep -rn "RegisterRealChannels(" services/api-gateway
```
