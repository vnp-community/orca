# SOL-034: Keep `task.create`/`task.get`, wire `execute`, and build `list`/`update`/`delete`/`getDependencies`/`aiDecompose`/`aiApply` against task-service's real schema

**Resolves:** [BUG-034](../BUG-034-task-channels-not-implemented.md)
**Service:** `task-service` (new RPCs + `SimpleExecutor` real implementation) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto`
- `backend-go/services/task-service/internal/usecase/ports.go` (extend `TaskRepository`/`EdgeRepository`; new `ProjectExecutionResolver` port)
- `backend-go/services/task-service/internal/usecase/list_tasks.go`,
  `update_task.go`, `delete_task.go`, `get_dependencies.go`,
  `ai_decompose.go`, `ai_apply.go` (new)
- `backend-go/services/task-service/internal/adapter/postgres/repository.go`
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go` (real implementation, replaces `StubSimpleExecutor`)
- `backend-go/services/task-service/internal/adapter/grpcclient/relay_executor.go` (new — mirrors git-gateway-service's `RelayExecutor`)
- `backend-go/services/task-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Status:** 📋 Proposed — not yet implemented

---

## Lead finding: keep `task.create`/`task.get`, don't remove them

BUG-034's dead-code finding is real: `registerTaskChannels` wires
`task.create`/`task.get` against genuine `CreateTask`/`GetTask` RPCs, but
neither appears in the frontend's actual `task.*` call table
(`rpc-catalog.md:435-445`). **Recommendation: keep both, add the 7 real
methods alongside — do not remove them.** Concretely, not neutrally:

- Removing working code backed by a real, tested RPC to close an
  *unconfirmed* dead-code hypothesis is the riskier move — BUG-034 itself
  says "it's possible they're reached some other way this audit doesn't
  cover... but that could not be confirmed," which is exactly the kind of
  uncertainty that should block a deletion, not justify one.
  `CreateTask`/`GetTask` are also structurally load-bearing for this
  proposal's own new usecases (`list`, `update`, `delete`,
  `getDependencies` all need `Get`-shaped reads; nothing here would be
  simplified by deleting them).
- The actual fix for the naming-drift symptom is narrower: add a doc
  comment at `registerTaskChannels`'s definition flagging that
  `task.create`/`task.get` have no confirmed frontend call site as of this
  audit (mirroring `simple_executor.go`'s existing "STUB in this scaffold"
  comment convention), and open a fast-follow task to `grep` the frontend
  bundle (not just `rpc-catalog.md`) for any caller before ever removing
  them. That closes the finding without a risky deletion on an unconfirmed
  premise.

```go
// channels.go — add above registerTaskChannels
// task.create/task.get have no confirmed frontend call site in
// specs/frontend/api/rpc-catalog.md's task.* table (BUG-034/SOL-034) —
// kept because CreateTask/GetTask back real, working usecases and this
// package's own list/update/delete/getDependencies usecases below reuse
// Get directly. Do not delete without first confirming via a full
// frontend-source grep, not just the rpc-catalog audit, that nothing
// reaches these two channels.
```

---

## The 7 real methods

### `execute` — wrapper-only for the RPC, real work for the executors

`task.execute` already has a complete RPC/usecase/REST path
(`task.proto:16`, `execute_task.go`, `task_routes.go:25`) — wiring the
`wscompat` channel is a thin wrapper identical in shape to Part 1 of
SOL-032/SOL-033. The real gap, per BUG-034, is that both
`SimpleExecutor` and `ComplexExecutor` are stubs
(`grpcclient/simple_executor.go`, `complex_executor.go`) that return a
synthesized ref without calling anything. This proposal makes
**`SimpleExecutor` real**; `ComplexExecutor` is scoped out (see below).

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

#### Making `SimpleExecutor` real: the `agent.exec` relay

Per `task-service.md` §2/§3.1/§7 and
`08-inter-service-communication.md`'s "only two Go services that talk to
the execution plane" rule (`infra-fleet-service` and
`git-gateway-service`), **`task-service` cannot call the Dev Server Agent
directly** — it must go through `infra-fleet-service`'s gRPC surface as a
client, exactly the way `git-gateway-service`'s `RelayExecutor` does
(`relay_executor.go`), reusing the *same* generic `Relay(connectionId,
method, paramsJson)` RPC (`infra-fleet-service`'s `usecase/relay.go`) —
not a new relay mechanism.

The gap `StubSimpleExecutor`'s doc comment already names: `SimpleExecutor.Execute`
only receives `(tenantID, taskID, requestID)` — no `connectionId`. A real
implementation needs one more resolution step first: `taskID` →
`project_id` (already a real column, `task.tasks.project_id`, added for
Epic C — `HasActiveExecutions`' own doc comment confirms it) → the
project's bound dev server's `connectionId`, via `infra-fleet-service`'s
existing `ResolveConnection` RPC (the same one `git-gateway-service`'s
`ConnectionResolver` calls). Propose a new port for this step:

```go
// ports.go — new port
// ProjectExecutionResolver resolves a project's execution target
// (connectionId, or none for host-local) via infra-fleet-service —
// task-service never calls project-service or infra-fleet-service
// directly from this port's consumer (SimpleExecutor); the resolution
// itself lives in internal/adapter/grpcclient, mirroring
// git-gateway-service's ConnectionResolver split.
type ProjectExecutionResolver interface {
	ResolveConnection(ctx context.Context, tenantID, projectID string) (connectionID string, connected bool, err error)
}
```

```go
// adapter/grpcclient/simple_executor.go — real implementation
type SimpleExecutor struct {
	tasks    usecase.TaskRepository
	resolver usecase.ProjectExecutionResolver
	relay    infrafleetv1.InfraFleetServiceClient
}

func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	task, err := s.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: load task: %w", err)
	}
	connectionID, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("simple_executor: resolve connection: %w", err)
	}
	if !connected {
		// Per git-gateway-service.md §8's precedent: a resolve failure or
		// not-connected connectionId is a real error, never a silent
		// local fallback — task-service has no local `agent.exec`
		// equivalent of its own (unlike git-gateway-service's §2 step 3,
		// there is no "this service's own host" case for task execution).
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", "task's project has no connected dev server", nil)
	}

	paramsJSON, _ := json.Marshal(map[string]any{
		"taskId":    taskID,
		"title":     task.Title,
		"requestId": requestID,
	})
	resp, err := s.relay.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: "agent.exec", ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return "", fmt.Errorf("simple_executor: relay agent.exec: %w", err)
	}
	var result struct{ ExecutionRef string `json:"executionRef"` }
	_ = json.Unmarshal([]byte(resp.GetResultJson()), &result)
	return result.ExecutionRef, nil
}
```

This is a real, non-stub implementation for the simple path. Note two
honest limits carried over unchanged from the existing scaffold, not
solved by this proposal: (1) task-service still has no execution-completion
callback (`execute_task.go`'s existing doc comment) — `agent.exec`'s
result here is the *dispatch* acknowledgment, not a completion signal,
so `HasActiveExecutions`' one-way-transition caveat is unaffected by this
change; (2) the `agent.exec` param/result JSON shape above is a best-effort
mirror of `git-gateway-service`'s own `relay_executor.go` caveat — verify
against `specs/agent/api/agent-rpc-catalog-runtime.md`'s real
`agent.exec` handler contract before finalizing field names.

#### `ComplexExecutor` — explicitly out of scope here

Per task-service.md §3.1, the complex path hands off to
`orchestration-service`'s coordinator, which itself sequences
subtask/dependency execution and reaches the Dev Server Agent for worker
dispatch — this is materially larger than a single relay call (it needs
`orchestration-service`'s own coordinator design, not just a client dial).
**Scope this as a separate follow-up proposal**, not solved alongside
`SimpleExecutor` here — bundling them would make this proposal's review
surface (and risk) disproportionate to `SimpleExecutor`'s single-relay-call
fix. `StubComplexExecutor` stays a stub after this proposal ships; a task
with any subtask/dependency edge (`isComplex` in `execute_task.go`) still
gets a synthesized placeholder ref, same as today — flag this explicitly
in the PR description so it isn't mistaken for a full fix of `execute`.

### `aiDecompose` — relay design, explicit

Per task-service.md §3.2 and confirmed by the same
`08-inter-service-communication.md` "only two Go services talk to the
execution plane" rule used above: **`task-service` initiates this relay
itself**, going through `infra-fleet-service` as the relay client — not
`git-gateway-service` (wrong domain) and not a direct agent connection
(not one of the two permitted services). The call shape mirrors
`git-gateway-service`'s `GenerateCommitMessage`/§3.1 pattern exactly:
gather context locally (task title, description, ancestor/subtree context
via existing `TaskRepository`/`EdgeRepository` reads — no dispatch needed,
this is task-service's own Postgres data), resolve AI provider/account
context via `ai-provider-service` (a second, ai-provider-specific gRPC
client, distinct from the `infra-fleet-service` relay client), then relay
the actual completion call through the same `infrafleetv1.Relay` RPC
`SimpleExecutor` above now uses, with method `"ai.complete"`.

```protobuf
rpc AIDecompose(AIDecomposeRequest) returns (AIDecomposeResponse);
rpc AIApply(AIApplyRequest) returns (AIApplyResponse);

message AIDecomposeRequest {
  string task_id = 1;
}
message SubtaskProposal {
  string title = 1;
  string description = 2;
}
message AIDecomposeResponse {
  repeated SubtaskProposal proposals = 1; // review-before-commit, §3.2 — not yet written to task_edges
}

// AIApply commits a (possibly user-edited) proposal set from a prior
// AIDecompose call — the two-step review-before-commit shape TS already
// has, per task-service.md §3.2.
message AIApplyRequest {
  string task_id = 1;
  repeated SubtaskProposal proposals = 2;
}
message AIApplyResponse {
  repeated Task created_subtasks = 1;
}
```

```go
// usecase/ai_decompose.go
func (uc *AIDecompose) Execute(ctx context.Context, in AIDecomposeInput) ([]domain.SubtaskProposal, error) {
	task, err := uc.tasks.Get(ctx, in.TenantID, in.TaskID)
	if err != nil {
		return nil, err
	}
	providerCtx, err := uc.aiProvider.ResolveContext(ctx, in.TenantID, in.UserID)
	if err != nil {
		return nil, err
	}
	connectionID, connected, err := uc.resolver.ResolveConnection(ctx, in.TenantID, task.ProjectID)
	if err != nil || !connected {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "TASK_AI_DECOMPOSE_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}
	prompt := buildDecomposePrompt(task, providerCtx)
	content, err := uc.relay.Complete(ctx, connectionID, prompt) // same AICompleter-shaped port git-gateway-service defines
	if err != nil {
		return nil, err
	}
	return parseSubtaskProposals(content), nil
}
```

`AIApply` is a plain write usecase: for each `SubtaskProposal`, call the
same path `CreateTask`'s usecase already uses to insert a task row plus a
`parent_child` `task_edges` row — no new persistence primitive, just reuse
of `CreateTask`+`AddEdge`'s existing logic, invoked N times in one
transaction (one failed subtask insert should not leave a partial subtree,
per the transactional-consistency posture task-service.md §8 already
applies to `AddEdge`'s cycle-check-then-write).

### `list`/`update`/`delete`/`getDependencies` — CRUD against the real schema

Grounded in the actual columns (`migrations/0001_init.up.sql`,
`0002_task_project_execution_tracking.up.sql`): `task.tasks(id, tenant_id,
title, status, parent_id, project_id, created_at, updated_at)`,
`task.task_edges(id, tenant_id, from_task_id, to_task_id, edge_type,
created_at)`. Note this scaffold's schema already narrower than
`task-service.md` §5's fuller sketch (no `description`/`complexity`/
`assignee_id`/`active_execution_id` columns) — this proposal's fields
follow what's actually there, same "extend the real thing, not the doc
sketch" posture SOL-033 takes for `automation.Automation`.

```protobuf
rpc ListTasks(ListTasksRequest) returns (ListTasksResponse);
rpc UpdateTask(UpdateTaskRequest) returns (UpdateTaskResponse);
rpc DeleteTask(DeleteTaskRequest) returns (google.protobuf.Empty);
rpc GetDependencies(GetDependenciesRequest) returns (GetDependenciesResponse);

message ListTasksRequest {
  string tenant_id = 1;
  string project_id = 2; // optional filter
  string page_token = 3;
  int32 page_size = 4;
}
message ListTasksResponse {
  repeated Task tasks = 1;
  string next_page_token = 2;
}

// UpdateTask: wrapper-typed optional fields, same field-mask shape as
// SOL-033's UpdateAutomationRequest — a status-only edit (the common
// case, e.g. marking done) shouldn't require resending title/parent_id.
message UpdateTaskRequest {
  string id = 1;
  google.protobuf.StringValue title = 2;
  google.protobuf.StringValue status = 3;
}
message UpdateTaskResponse {
  Task task = 1;
}

message DeleteTaskRequest {
  string id = 1;
}

// GetDependencies walks depends_on edges FROM task_id — distinct from
// AddEdge (write) and distinct from GetAncestors/GetSubtree (parent_child,
// not sketched in the generated proto yet either — see Known-gaps note
// below), per task-service.md §3.
message GetDependenciesRequest {
  string task_id = 1;
}
message GetDependenciesResponse {
  repeated Task dependencies = 1;
}
```

```go
// usecase/get_dependencies.go — reuses EdgeRepository.ListFrom, which
// ExecuteTask's isComplex already calls for the identical edge kind —
// no new repository method needed for the edge read itself, only the
// task-hydration step (edge -> full Task) that ListFrom's callers so far
// haven't needed.
func (uc *GetDependencies) Execute(ctx context.Context, tenantID, taskID string) ([]domain.Task, error) {
	edges, err := uc.edges.ListFrom(ctx, tenantID, taskID, domain.EdgeKindDependsOn)
	if err != nil {
		return nil, err
	}
	tasks := make([]domain.Task, 0, len(edges))
	for _, e := range edges {
		t, err := uc.tasks.Get(ctx, tenantID, e.ToTaskID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
```

`UpdateTask` deliberately does NOT become the general mechanism that
clears `StatusInProgress` back out (the one-way-transition gap
`execute_task.go`'s doc comment names) — allowing an arbitrary client-driven
`status` write to double as an execution-completion callback would let a
buggy or malicious client mark a still-running task `done` early. Propose
`UpdateTask`'s domain-layer status setter reject transitions *into*
`in_progress` (that's `ExecuteTask`'s job only) but otherwise allow normal
open/done/cancelled edits — leaving the real completion-callback design
(likely `orchestration-service`/`infra-fleet-service` calling back into
task-service, not a REST/WS-facing `UpdateTask`) as the separate,
larger piece of follow-up work `execute_task.go` already flags.

`TaskRepository`/`EdgeRepository` gain `List`, `Update` (status/title), and
`Delete` methods — same shape as SOL-033's `AutomationRepository`
extension, omitted here for space; `Delete` needs no explicit
`task_edges`/`task_grants` cleanup since both reference `tasks(id)` with
`ON DELETE CASCADE` (`migrations/0001_init.up.sql`).

---

## `wscompat` wiring — all 7

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
		TaskID    string                    `json:"taskId"`
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

// task.execute — see the earlier "execute" section for its handler body.
```

---

## Test plan

- `usecase/list_tasks_test.go`, `update_task_test.go`, `delete_task_test.go`,
  `get_dependencies_test.go` — fakes for `TaskRepository`/`EdgeRepository`,
  no real Postgres, per `03-clean-architecture-guidelines.md`.
- `usecase/ai_decompose_test.go` — fake `ProjectExecutionResolver`,
  `AIProviderClient`, and `AICompleter`-shaped relay port; assert the
  not-connected case returns `TASK_AI_DECOMPOSE_NO_CONNECTION` rather than
  silently returning an empty proposal list.
- `usecase/update_task_test.go` — explicit case asserting a status
  transition request targeting `in_progress` is rejected (the guard against
  `UpdateTask` becoming an unintended completion-callback surface).
- `adapter/grpcclient/simple_executor_test.go` — replace/extend the
  existing stub's test with a real one: fake `TaskRepository` (returns a
  task with a `ProjectID`), fake `ProjectExecutionResolver`, fake
  `infrafleetv1.InfraFleetServiceClient` (same fake-the-generated-client
  pattern `git-gateway-service`'s `grpcclient_test.go` and
  `wscompat/channels_test.go` both already use) — assert `Relay` is called
  with method `"agent.exec"` and the not-connected case returns a typed
  error instead of a synthesized placeholder ref (locks in that the stub
  behavior is actually gone).
- `adapter/postgres/repository_test.go` — `testcontainers-go` integration
  tests for `List`/`Update`/`Delete`, including a cascade-delete assertion
  (delete a task with a `task_edges` row, confirm the edge is gone).
- `adapter/grpc/server_test.go` — contract tests for the 6 new RPCs.
- `wscompat/channels_test.go` — one test per new channel, following
  `TestDevServerListChannel_Success`'s shape; a
  `TestTaskCreateGetChannels_StillRegistered` regression test asserting
  `task.create`/`task.get` remain registered (guards the "keep, don't
  remove" decision above against a future contributor treating the
  dead-code finding as license to delete them).
- **`ComplexExecutor` is explicitly out of scope for this test plan** — no
  test here should assert it does anything beyond its current stub
  behavior; a follow-up proposal owns its test plan.

## References

- `specs/backend-go/tdd/services/task-service.md` — full service design;
  §2 (bounded context — execution and AI inference both delegated, never
  local), §3/§3.1/§3.2 (API surface, execution dispatch, AI decomposition
  relay pattern), §4 (domain model), §5 (schema — compare against the
  scaffold's actually-narrower columns), §7 (dependency graph — confirms
  `task-service --> infra-fleet-service` and the `orch --> task` callback
  edge), §9 (grant resolution is unaffected by this proposal), §10
  (migration notes, "one prescribed behavior change" = the AI relay)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` —
  "Talking to the Dev Server Agent" section; "only two Go services... talk
  to the execution plane" rule this proposal's `aiDecompose`/`SimpleExecutor`
  design is built around (task-service relays through infra-fleet-service,
  never directly)
- `specs/backend-go/bugs/missing-v1/BUG-034-task-channels-not-implemented.md`
  — full gap table, the dead-code finding this proposal's lead section
  responds to, and the executor-stub caveat on `task.execute`
- `backend-go/proto/orca/task/v1/task.proto` — current 7-RPC surface this
  proposal extends; note `Task.project_id`'s Epic C origin, load-bearing
  for `SimpleExecutor`'s and `aiDecompose`'s connection-resolution step
- `backend-go/services/task-service/internal/usecase/execute_task.go` —
  the real complexity-branch logic and the honest stub/one-way-transition
  caveats this proposal explicitly preserves (completion callback) or fixes
  (`SimpleExecutor`'s relay)
- `backend-go/services/task-service/internal/usecase/ports.go:97-116` —
  `SimpleExecutor`/`ComplexExecutor` port definitions and their "STUB in
  this scaffold" doc comments
- `backend-go/services/task-service/internal/usecase/has_active_executions.go`
  — confirms `task.tasks.project_id` is real and already load-bearing
  elsewhere, and documents the one-way-transition limitation this
  proposal's `UpdateTask` guard is designed not to accidentally paper over
- `backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`,
  `complex_executor.go` — current stub implementations this proposal
  replaces (`SimpleExecutor`) or explicitly leaves alone (`ComplexExecutor`)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
  — the `Relay`-RPC-based pattern `SimpleExecutor`'s and `aiDecompose`'s
  new relay clients mirror
- `backend-go/services/infra-fleet-service/internal/usecase/relay.go` —
  the generic `Relay(connectionId, method, params)` RPC both new relay
  paths dispatch through; no new infra-fleet-service work needed, same as
  SOL-032
- `backend-go/services/task-service/migrations/0001_init.up.sql`,
  `0002_task_project_execution_tracking.up.sql` — the actual current
  schema this proposal's SQL/field choices are grounded in
- `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go:1-27`
  — REST equivalents already calling all 7 RPCs
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:182-215`
  — `registerTaskChannels`, the existing `create`/`get` wiring this
  proposal keeps and extends
- `specs/frontend/api/rpc-catalog.md:435-445` — full `task.*` frontend
  call-site table (7 real methods; confirms `create`/`get` absence)
