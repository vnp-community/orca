# SOL-018: Add `GetDispatchContextForTask` read RPC — `assignee_handle` already exists under a different name

**Resolves:** [BUG-018](../BUG-018-orchestration-channels-not-implemented.md)
**Service:** `orchestration-service` (new read RPC) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/orchestration/v1/orchestration.proto`
- `backend-go/services/orchestration-service/internal/usecase/get_dispatch_context_for_task.go` (new)
- `backend-go/services/orchestration-service/internal/usecase/ports.go` (extend `DispatchContextRepository`)
- `backend-go/services/orchestration-service/internal/adapter/postgres/repository.go` (new query)
- `backend-go/services/orchestration-service/internal/adapter/grpc/*.go` (wire the new RPC)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_orchestration.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_orchestration_test.go` (new file)
- `backend-go/services/api-gateway/cmd/server/main.go` (pass `orchestrationClient` into `RegisterRealChannels`)
**Status:** 📋 Proposed — not yet implemented

---

## `assignee_handle` isn't missing — it's already there as `handle`, at every layer

BUG-018 states plainly that `DispatchContext` "has no `assignee_handle`
field." Checking the domain model against `orchestration-service.md` §4
narrows this: the TDD's own domain-model bullet for `DispatchContext`
literally names the field `assignee_handle` —

> **`DispatchContext`** — id, `orchestration_task_id`, `assignee_handle`,
> status (...), `failure_count`, `last_heartbeat_at`.

— and the real Go domain type
(`backend-go/services/orchestration-service/internal/domain/orchestration.go:149-166`)
already has this exact field, just spelled `Handle`, populated from the
`handle` argument `CreateDispatchContext`'s usecase takes
(`create_dispatch_context.go:15-19`). The current proto's
`DispatchContext.handle` field carries the same value all the way from
`CreateDispatchContextRequest.handle` — its doc comment ("KeyedAsyncQueue
serialization key",
`orchestration.proto:19-27`) describes what the field is *used for*
mechanically (§6/§8's `HandleSerializer` keys on it), not what it
*represents* semantically, which — per the TDD's own naming and per
`terminal-orchestration-task-links.ts:59-61`'s exact read
(`result.dispatch?.assignee_handle`) — is the assignee terminal handle a
task was dispatched to. So this is a **wire-naming gap, not a missing
field**: the data has existed since `CreateDispatchContext` shipped; it's
never been exposed to a caller under the name the frontend expects.

Propose resolving this at the `wscompat` translation boundary (per
`03-clean-architecture-guidelines.md`'s "adapter translates wire format"
rule) rather than renaming the proto field: `dispatchShow`'s handler maps
`DispatchContext.handle` → JSON `assignee_handle` on the way out, the same
way SOL-015/SOL-016's `wscompat` handlers translate provider-agnostic proto
fields into each namespace's exact expected wire shape. Renaming the proto
field itself was considered and rejected — `handle` is used by 3 existing
RPCs (`CreateDispatchContext`, and `DecisionGate`'s serialization via the
same key) and by 2 existing `httpgateway` REST handlers
(`orchestration_routes.go:47-56`); a rename is a real (if small) breaking
change for zero behavioral gain when a translation at the one new call site
gets the same result for free.

## The genuine gap: no read RPC exists — not in the shipped proto, not in the TDD's own sketch

Unlike the field question above, this part of BUG-018's finding holds
exactly as reported. `orchestration.proto`'s 4 RPCs
(`CreateDispatchContext`, `CreateGate`, `ResolveGate`,
`UpdateTaskStatusAndPromote`) are all writes. Checking against
`orchestration-service.md` §3's own drafted API surface — the TDD's
*target* design, not just what's shipped — confirms there is no
`GetDispatchContext`-shaped RPC there either:
`StartCoordinatorRun`/`GetCoordinatorRun`/`CompleteCoordinatorRun`/
`FailCoordinatorRun`/`CreateDispatchContext`/`RecordHeartbeat`/
`FailDispatch`/`CreateDecisionGate`/`ResolveDecisionGate`/
`ListPendingDecisionGates`/`UpdateTaskStatusAndPromote`/`PostMessage`/
`ListMessages`/`MarkMessageRead`/`GetAgentStatusForHandle` — 15 RPCs, and
`GetCoordinatorRun` is the closest read, but it returns a whole run, not
one task's current dispatch. This is unlike SOL-001/SOL-015, where the TDD
had already sketched the needed RPC and the task was just building it — here
the RPC itself is a **scope addition beyond the TDD**, the same category as
SOL-015's `ListPriorities`/`ListTransitions`/`GetProjectStatusOrder`. Flag
it as such rather than treating it as an oversight in a doc that's
otherwise silent on it.

## Design — Proto addition

```protobuf
service OrchestrationService {
  rpc CreateDispatchContext(CreateDispatchContextRequest) returns (CreateDispatchContextResponse);
  rpc CreateGate(CreateGateRequest) returns (CreateGateResponse);
  rpc ResolveGate(ResolveGateRequest) returns (ResolveGateResponse);
  rpc UpdateTaskStatusAndPromote(UpdateTaskStatusAndPromoteRequest) returns (UpdateTaskStatusAndPromoteResponse);

  // GetDispatchContextForTask is orchestration-service's first read RPC —
  // a scope addition beyond both the shipped proto and
  // orchestration-service.md §3's own drafted surface (neither has a
  // dispatch-context read). Backs orchestration.dispatchShow: "which
  // terminal was this task dispatched to." See SOL-018 for the "not a
  // missing assignee_handle field, a missing read RPC" distinction.
  rpc GetDispatchContextForTask(GetDispatchContextForTaskRequest) returns (GetDispatchContextForTaskResponse);
}

message GetDispatchContextForTaskRequest {
  string orchestration_task_id = 1;
}

message GetDispatchContextForTaskResponse {
  // Unset when no dispatch context exists yet for this task — a legitimate
  // state (the task hasn't been dispatched), not an error. Reuses the
  // existing DispatchContext message; no new message type needed.
  DispatchContext dispatch = 1;
}
```

Additive only — new RPC, new messages, no change to any existing field —
passes `buf breaking` per `08-inter-service-communication.md`'s
conventions.

**Which row to return.** `dispatch_contexts.orchestration_task_id` is not
unique — `FailDispatch`'s existence implies a task can accumulate more than
one dispatch attempt (a failed dispatch, then a retry creates a new row;
§8's circuit-breaker note: `failure_count >= 3` forces `circuit_broken`,
which only makes sense if failed attempts are visible as history, not
overwritten in place). Propose "most recent dispatch context row for this
`orchestration_task_id`, ordered by `created_at DESC`" — matches the
frontend's actual use ("which terminal is this task *currently* running
on," `terminal-orchestration-task-links.ts:50-61`'s focus-the-terminal
flow), not a full history read (that's a separate, unrequested capability;
`ListPendingDecisionGates` is the closest existing precedent for a
list-shaped read if a history view is ever needed later).

---

## Design — `usecase/` layer

```go
// internal/usecase/get_dispatch_context_for_task.go
type GetDispatchContextForTask struct {
    repo DispatchContextRepository
}

func NewGetDispatchContextForTask(repo DispatchContextRepository) *GetDispatchContextForTask {
    return &GetDispatchContextForTask{repo: repo}
}

// Execute returns (context, found, err). found=false with err=nil means
// "no dispatch context exists yet for this task" — a legitimate read
// result the gRPC adapter maps to an unset response field, not a gRPC
// error, since ErrDispatchContextNotFound already exists as a sentinel
// (ports.go:19) for exactly this case.
func (uc *GetDispatchContextForTask) Execute(ctx context.Context, taskID string) (domain.DispatchContext, bool, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return domain.DispatchContext{}, false, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
    }
    if taskID == "" {
        return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_TASK_ID", "orchestration_task_id is required", nil)
    }
    dc, err := uc.repo.GetLatestForTask(ctx, tenantID, taskID)
    if errors.Is(err, ErrDispatchContextNotFound) {
        return domain.DispatchContext{}, false, nil
    }
    if err != nil {
        return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInternal, "ORCH_GET_DISPATCH_FAILED", "failed to look up dispatch context", err)
    }
    return dc, true, nil
}
```

`DispatchContextRepository` (`ports.go:60-75`) gains one method:

```go
// GetLatestForTask returns the most recently created dispatch_contexts row
// for orchestrationTaskID, or ErrDispatchContextNotFound if none exists.
// A task's dispatch_contexts row is not unique (retries after failure
// create new rows, §8's circuit-breaker note) — "latest" is the current
// dispatch, which is what dispatchShow's "which terminal is this on"
// question actually needs, not full attempt history.
GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error)
```

Read-only — no transaction boundary, no `HandleSerializer` routing needed;
none of §8's atomicity table rows apply to a pure read (the table only
covers write chains where a torn read/write can double-dispatch or strand
a task).

`adapter/postgres/repository.go` implements it:

```sql
SELECT id, orchestration_task_id, handle, coordinator_run_id, status,
       failure_count, last_failure, dispatched_at, completed_at, last_heartbeat_at, created_at
FROM dispatch_contexts
WHERE tenant_id = $1 AND orchestration_task_id = $2
ORDER BY created_at DESC
LIMIT 1;
```

---

## Design — `wscompat` wiring (`channels_orchestration.go`)

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
            Status:              "", // DispatchContext proto has no status field yet either — see Test plan note
        }}, nil
    })
}
```

`RegisterRealChannels` gains an `orchestrationClient
orchestrationv1.OrchestrationServiceClient` parameter and a
`registerOrchestrationChannels(r, orchestrationClient)` call —
`orchestrationClient` is already dialed in `main.go` for the `/v1/orchestration`
REST routes (`main.go:273`), so this is threading an existing client into
`RegisterRealChannels`'s call at `main.go:241`, same as SOL-015/SOL-016's
`issueTrackingClient`.

**Adjacent, smaller gap noted for completeness:** `DispatchContext`
(proto) has no `status` field either, even though `domain.DispatchContext`
has one (`Status DispatchStatus`, `orchestration.go:159`) — out of scope for
this fix (BUG-018 only reports `assignee_handle`/the missing read RPC), but
worth a one-line flag rather than silently emitting an always-empty
`status` in the view above. Left as `""` with this comment rather than
guessed at.

---

## Test plan

- `services/orchestration-service/internal/usecase/get_dispatch_context_for_task_test.go`:
  - fake repo returns a context → usecase returns `(dc, true, nil)`.
  - fake repo returns `ErrDispatchContextNotFound` → usecase returns `(zero, false, nil)`, not an error.
  - empty `taskID` → `ORCH_EMPTY_TASK_ID` before the repo is called.
- `services/orchestration-service/internal/adapter/postgres/repository_test.go` (testcontainers): create two dispatch contexts for the same `orchestration_task_id` (simulating a retry after `FailDispatch`) → `GetLatestForTask` returns the one with the later `created_at`, not the first.
- `services/api-gateway/internal/adapter/wscompat/channels_orchestration_test.go`:
  - `TestDispatchShowChannel_ReturnsAssigneeHandle` — fake client returns a `DispatchContext{Handle: "terminal-3"}`, asserts the JSON result has `dispatch.assignee_handle == "terminal-3"` (not `dispatch.handle`) — the regression guard for the wire-naming translation this solution's whole design rests on.
  - `TestDispatchShowChannel_NoDispatchYet_ReturnsNilDispatch` — fake client returns `GetDispatchContextForTaskResponse{}` (unset `Dispatch`), asserts `{"dispatch": null}`, not an error.
  - `TestDispatchShowChannel_PropagatesError` — mirrors `TestDevServerListChannel_PropagatesError`.

## References

- `specs/backend-go/bugs/missing-v1/BUG-018-orchestration-channels-not-implemented.md` — the two findings this solution addresses (field naming + missing read RPC)
- `specs/backend-go/tdd/services/orchestration-service.md` §3 (RPC surface — no read RPC even here), §4 (`DispatchContext`'s `assignee_handle` field name), §8 (atomicity table — confirms reads are out of its scope) — the target design, and where it's silent
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — "adapter translates wire format" rule, the basis for resolving the naming gap at `wscompat` rather than via a proto rename
- `backend-go/proto/orca/orchestration/v1/orchestration.proto:1-93` — current 4-RPC, all-write surface; `DispatchContext.handle`'s existing doc comment
- `backend-go/services/orchestration-service/internal/domain/orchestration.go:149-166` — `domain.DispatchContext.Handle`, the field BUG-018 called missing
- `backend-go/services/orchestration-service/internal/usecase/ports.go:17-23,53-75` — `ErrDispatchContextNotFound` sentinel (already exists) and `DispatchContextRepository` to extend
- `backend-go/services/orchestration-service/internal/usecase/create_dispatch_context.go:12-19` — where `Handle` is populated, confirming it's been the assignee handle since creation
- `backend-go/services/api-gateway/internal/adapter/httpgateway/orchestration_routes.go:19-26` — existing REST routes over the same 4 write RPCs, the reason a proto field rename was rejected in favor of translation-at-the-boundary
- `backend-go/services/api-gateway/cmd/server/main.go:241,273` — existing `orchestrationClient` dial and `RegisterRealChannels` call site to extend
- `frontend/src/renderer/src/components/terminal-pane/terminal-orchestration-task-links.ts:50-72` — the frontend call site, its exact `{task: taskId}` request and `{dispatch: {assignee_handle}}` response read
