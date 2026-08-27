# SOL-PW-04: Cross-service workspace integration via the already-specified NATS JetStream outbox

**Resolves:** [BUG-PW-04](../BUG-PW-04-workspace-integration-not-implemented.md)
**Service:** `task-service`, `workflow-service`, `orchestration-service`,
`git-gateway-service` (unchanged — see "Where each flow lives"),
`api-gateway` (new consumer), `notification-service` (existing consumer,
now actually fed)
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto` (`task_number`, `pr_url`, `worktree_id` fields; `FindTaskByNumber` RPC)
- `backend-go/proto/orca/workflow/v1/workflow.proto` (`step_type` on `ExecutionEvent`/completion payload — verify field already carries this before adding)
- `backend-go/services/task-service/internal/domain/task.go`, `internal/usecase/ports.go`, `internal/usecase/execute_task.go`, `internal/usecase/update_task.go`, `internal/usecase/find_task_by_number.go` (new)
- `backend-go/services/task-service/internal/adapter/eventbus/` (new — outbox `Store` + enqueue)
- `backend-go/services/task-service/internal/adapter/postgres/` (new `task_outbox` table + migration)
- `backend-go/services/workflow-service/internal/adapter/eventbus/` (new — same shape)
- `backend-go/services/workflow-service/internal/adapter/postgres/` (new `workflow_outbox` table + migration)
- `backend-go/services/orchestration-service/internal/usecase/update_task_status_and_promote.go` (extend to call `task-service.UpdateTask` on coordinator-run completion — port already named `TaskClient`-shaped per its own ports.go, verify before adding a new one)
- `backend-go/services/api-gateway/internal/adapter/wscompat/workspace_events.go` (new — ephemeral NATS subscriber → WS push)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go` (`registerGitDeepChannels`'s `git.commit` handler — commit-message task-ID saga)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go` (PR-creation handler — `pr_url` write-back saga)
- `backend-go/services/notification-service/internal/domain/` (extend `TranslateEvent` for the two new subjects, if user-facing toasts are desired — see "Notification-service's role")
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

BUG-PW-04's own "What backend-go has" section already identifies the
correct foundation: `backend-go/common/eventbus/eventbus.go` and
`backend-go/common/outbox/outbox.go` are real, generic infrastructure —
[`08-inter-service-communication.md`](../../../tdd/architecture/08-inter-service-communication.md)'s
"Event conventions (NATS JetStream)" section specifies exactly this shape
(subject naming `orca.<service>.<entity>.<event>`, transactional outbox,
idempotent consumers) and it is already implemented, tested, generic Go
code sitting unused by every service that would need it for this BL. This
solution's core insight is **not** "invent a new mechanism" — it's "wire
the outbox/eventbus pair every relevant service's own TDD doc already
names but never built":

- `task-service.md` §6's package layout **already lists**
  `adapter/eventbus/ # task.created/task.completed/... via outbox` — this
  solution implements that literal line, not something new.
- `workflow-service.md` §7 **already lists** `workflow.execution.started`/
  `completed`, `workflow.step_failed` as "Publishes... (async, NATS) —
  consumed by `notification-service`". §5 already shows the outbox
  pattern in its own schema section: "Outbox rows... are written in the
  same transaction as the triggering state change... no pattern beyond
  what `05-data-architecture.md` already specifies."
- `02-microservices-decomposition.md`'s dependency graph **already draws**
  `notif -.events.-> wf` and `notif -.events.-> task` (dotted = async
  event-bus edges) — i.e. the "surface a notification" half of BL-PW-04's
  Flow 3 is already an architected edge on paper, just never fed an event.
- `notification-service`'s `HandleIncomingEvent` usecase
  (`services/notification-service/internal/usecase/handle_incoming_event.go:29-45`)
  is a real, generic, subject-agnostic consumer — "translate a consumed
  domain event into a `NotificationEvent`, then fan it out" — that is
  already correctly built to receive whatever this solution starts
  publishing, with zero changes needed to its dedup/translate/broadcast
  pipeline beyond `domain.TranslateEvent` learning the two new subjects
  (a small, additive change, not a redesign).

So the honest framing of this solution is: **four services need their
already-specified outbox wired for real, plus a handful of new,
narrowly-scoped call sites** — not a new architectural component. Where
BL-PW-04's frontend-side sketch (`workspaceEvents` bus: `agent.complete`,
`git.commit`, `worktree.switched`) doesn't map cleanly onto an existing
named event, this solution grounds it in the closest already-specified
backend event instead of inventing new taxonomy, and flags every place it
must genuinely extend the TDD (schema fields, one new consumer package).

### Where each flow lives, and why

| BL-PW-04 flow | Owning service for the *trigger* | Mechanism | Why here, not elsewhere |
|---|---|---|---|
| 1. Agent complete → refresh Git/Explorer + advance linked task | `task-service` (simple path), `orchestration-service`→`task-service` (complex path) | Sync `UpdateTask` write + `task-service`'s own outbox publish on any status transition | `task-service.md` §7 already specifies `orch --> task` as a **synchronous gRPC call-back**, not an event — "It also calls back into `task-service`... to read/update task state as it progresses subtasks." Re-using `UpdateTask` as the single write path for both the simple and complex path means **one** outbox publish point covers both, instead of two independent event producers that could drift. |
| 2. Commit message `#TG-123` → close task + record PR URL | `api-gateway` (synchronous saga at the edge, not an event) | Direct `git-gateway-service.Commit` → regex → `task-service.UpdateTask` | `git-gateway-service.md` §5/§6 is explicit: "a `git.commit` isn't a fact other services react to asynchronously the way `task.completed` is" and this service deliberately has **no** `adapter/eventbus/`. Cross-service aggregation that doesn't fit either owning service's bounded context already has a named home: `gitgateway.proto`'s `DetectWorktrees` doc comment states "that diff happens at api-gateway's edge layer... per `05-data-architecture.md`'s cross-service aggregation rule" — same precedent applies here. A developer also expects their task to flip state *immediately* after the commit UI confirms success, not on eventual NATS delivery — sync fits the UX, not just the architecture. |
| 3. Workflow `create-pr`-shaped step completes → ref sync + notification | `workflow-service` (publishes), `api-gateway` (new ref-sync consumer) + `notification-service` (existing consumer, now fed) | Async, via the `workflow.execution.completed` event `workflow-service.md` §7 already names | Unlike Flow 2, a workflow step's completion is not a synchronous user action waiting on a response — the user isn't staring at a spinner for a background step the way they are for a commit-message submit. Async is the correct shape here, and it's the one flow where the target architecture doc's own dependency graph already draws the edge (`notif -.events.-> wf`). |
| 4. Task → Agent: worktree linkage for later attribution | `project-service` (already captures this — see below), `task-service` (needs its own copy) | Existing `CreateWorktree` saga's `task_id` lineage field, mirrored onto `Task.worktree_id` | Not a new capability — `RecordWorktreeCreatedRequest.task_id` (`project.proto:286`) and `domain.Worktree.TaskID` already exist (§ below). The gap is one-directional: `task-service` has no matching field to look the association up from the *task* side, which Flow 1/2's saga needs. |

## Design — schema/proto extensions

### 1. `task.proto` — human-readable reference, worktree linkage, PR tracking

```protobuf
message Task {
  string id = 1;
  string tenant_id = 2;
  string title = 3;
  string status = 4;
  string parent_id = 5;
  string project_id = 6;
  // Added for Flow 2/4 (SOL-PW-04): a per-tenant sequential number so a
  // commit message can reference "#TG-42" without embedding a UUID.
  // Assigned once at CreateTask, immutable, backed by a per-tenant
  // sequence (see Data model note below) — never reused even if the task
  // is deleted, matching GitHub/Jira issue-number semantics developers
  // already expect from a "#TG-N" style reference.
  int64 task_number = 7;
  // Mirrors project-service's Worktree.TaskID (project.proto:286) from
  // the *task* side — task-service cannot join project-service's table
  // directly (02-microservices-decomposition.md's "no service reads
  // another service's tables" rule), so this is written back explicitly
  // by whichever saga first associates the two (see "Wiring" below).
  // Empty until an agent run or commit-close saga sets it.
  string worktree_id = 8;
  // Set by the PR-creation write-back saga (Flow 2's second half) — empty
  // until a PR referencing this task's #TG-N is created.
  string pr_url = 9;
}

message UpdateTaskRequest {
  string id = 1;
  google.protobuf.StringValue title = 2;
  google.protobuf.StringValue status = 3;
  // Both optional/unset-means-no-change, matching the existing
  // StringValue wrapper convention on this message.
  google.protobuf.StringValue pr_url = 4;
  google.protobuf.StringValue worktree_id = 5;
}

// FindTaskByNumber resolves a project-scoped "#TG-N" reference to a task
// id — the read path the commit-message regex saga (Flow 2) needs.
// Project-scoped, not tenant-wide: two different projects can each have
// their own #TG-42, matching how GitHub/Jira issue numbers are
// repo/project-scoped, not org-wide.
rpc FindTaskByNumber(FindTaskByNumberRequest) returns (FindTaskByNumberResponse);
message FindTaskByNumberRequest {
  string project_id = 1;
  int64 task_number = 2;
}
message FindTaskByNumberResponse {
  Task task = 1; // empty/NOT_FOUND if no task in project_id has this number
}
```

`task_number` needs a per-tenant-or-per-project monotonic sequence in
Postgres (`CREATE SEQUENCE` scoped via a `(project_id)`-partitioned
counter table, or a `nextval` on a per-project sequence created
lazily at first `CreateTask` for that project) — flagged as a genuine
schema addition beyond `task-service.md` §5's current `tasks` table,
which has no such column today. This is additive-only (a new nullable-
then-backfilled column plus a new sequence), consistent with
`05-data-architecture.md`'s expand/contract migration discipline.

### 2. `workflow.proto` — tagging step completions for Flow 3's consumer

`ExecutionEvent` (the `StreamExecutionEvents` stream type, `workflow.proto`
— not read in full for this solution; verify its exact fields before
implementing) needs a `step_type` (or equivalent) field on whatever
message backs `workflow.execution.completed`'s outbox payload, so
`api-gateway`'s new consumer (§ below) can filter for steps whose output
looks like a PR-creation result without workflow-service needing to know
anything about git or PRs itself — it stays a generic "this step, of this
type, in this execution, finished with this output" fact. **workflow-service
does not gain any git-gateway-service knowledge** — the interpretation
("this was a create-pr step") happens entirely in the consumer, keeping
`workflow-service.md` §2's "does not own... git-gateway/`scm.*` calls
directly" boundary intact.

## Design — `task-service` usecase/adapter layer

```go
// internal/usecase/ports.go (extended)
type OutboxStore interface {
    // Enqueue writes an outbox row in the SAME transaction as the
    // caller's domain write — per 05-data-architecture.md's outbox
    // pattern, this is a repository-layer concern (adapter/postgres),
    // not a separate call after commit.
    Enqueue(ctx context.Context, tx pgx.Tx, subject string, payload any) error
}
```

```go
// internal/usecase/update_task.go (extended)
// UpdateTask already exists per task.proto's RPC list — extending it to
// publish is the one behavior change this solution makes to an existing
// usecase, not a new one.
func (uc *UpdateTask) Execute(ctx context.Context, in UpdateTaskInput) (domain.Task, error) {
	return uc.repo.WithTx(ctx, func(tx pgx.Tx) (domain.Task, error) {
		task, err := uc.repo.UpdateTx(ctx, tx, in) // existing write path
		if err != nil {
			return domain.Task{}, err
		}
		// Only a genuine status transition is publish-worthy — a title-only
		// edit isn't a fact any of this BL's consumers care about.
		if in.Status != nil && *in.Status != task.PreviousStatus {
			if err := uc.outbox.Enqueue(ctx, tx, "orca.task.task.statuschanged", TaskStatusChangedPayload{
				TaskID: task.ID, ProjectID: task.ProjectID, WorktreeID: task.WorktreeID,
				PreviousStatus: task.PreviousStatus, NewStatus: task.Status,
			}); err != nil {
				return domain.Task{}, err
			}
		}
		return task, nil
	})
}
```

`ExecuteTask` (simple path, `internal/usecase/execute_task.go:48`) calls
`UpdateTask` internally once the (synchronous, per the interpretation
below) `SimpleExecutor` call returns — success transitions status toward
whatever "advance to review" maps to in this service's status vocabulary,
failure leaves status untouched with an error recorded. **Interpretation
flag**: `TaskServiceExecuteResponse.execution_ref` (`task.proto:129-131`,
"opaque handle into infra-fleet-service or orchestration-service") reads
as if `Execute` might be fire-and-forget, but every other dispatch this
system's TDD describes is synchronous-with-deadline (`git-gateway-service.md`
§2's resolve→dispatch→translate, `workflow-service.md` §8's "a step's own
timeout... enforced independently via `context.WithDeadline`") — there is
no async job/webhook pattern named anywhere else in this TDD set. This
solution treats `execution_ref` as an opaque *audit* handle, not evidence
of async completion, and places the `UpdateTask` call at the point
`ExecuteTask.Execute`'s underlying `SimpleExecutor.Execute` call returns.
If a future `infra-fleet-service` redesign makes agent dispatch genuinely
async, this call site moves to wherever that completion signal lands —
flagged explicitly as an assumption to confirm before implementation, not
papered over.

`orchestration-service`'s complex path (`internal/usecase/update_task_status_and_promote.go`
— already exists per the file listing, confirming this service already
has *a* task-status-promotion usecase; verify its current body calls
`task-service.UpdateTask` or a stand-in before assuming it needs new
plumbing vs. just needing its existing call path completed) is the other
caller into the same `UpdateTask` RPC — per `task-service.md` §7's
`orch --> task` edge, this is a **direct gRPC call**, not a second event
producer, so both paths converge on the one outbox publish point above.

## Design — `workflow-service` usecase/adapter layer

Same outbox shape, applied at the point `Execute`'s wave-dispatch loop
(`workflow-service.md` §7's diagram, step `L: step_executions row updated`
→ `N: execution.status = completed/failed`) transitions an execution to a
terminal state — this is exactly the `workflow.execution.completed`/
`workflow.step_failed` events §7 already names, just actually published
now via the same transactional-outbox pattern task-service uses above. No
new usecase; extend whatever usecase currently writes `executions.status`
to also call `outbox.Enqueue` in the same transaction.

## Design — `api-gateway` wscompat: two new pieces

### 1. Ephemeral NATS consumer → WS push (Flow 1 + Flow 3's ref-sync/refresh half)

```go
// internal/adapter/wscompat/workspace_events.go
//
// Subscribes an ephemeral (per-replica, not durable/competing-consumer —
// see common/eventbus.Consumer.Subscribe's doc comment on why: every
// api-gateway replica must independently learn about the event to push it
// to whichever locally-held WS connections it owns, not have JetStream
// round-robin the message to only one replica) consumer to
// orca.task.task.statuschanged and orca.workflow.execution.completed, and
// pushes a workspace.event WS frame to every connection subscribed to the
// event's project_id. This is the direct backend counterpart to
// BL-PW-04's frontend-side workspaceEvents sketch — 08-inter-service-
// communication.md's API Gateway responsibility #5 ("Manages WebSocket
// sessions for real-time surfaces... pipes frames both directions")
// already names this as api-gateway's job for exactly this shape of
// real-time surface.
func RegisterWorkspaceEventBridge(consumer *eventbus.Consumer, sessions *wsSessionRegistry, gitClient gitgatewayv1.GitGatewayServiceClient) error {
	return consumer.SubscribeEphemeral(ctx, "workflow-events", "orca.workflow.execution.completed", func(ctx context.Context, ev eventbus.Event) error {
		var payload workflowExecutionCompletedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err // NAK — redelivery may succeed if this was transient
		}
		// Flow 3's ref-sync half: only for steps whose step_type marks them
		// as a PR-creation-shaped step (see workflow.proto's new step_type
		// field) — every other step's completion just pushes a
		// notification-shaped frame, no git call.
		if payload.HasCreatePRStep() {
			if _, err := gitClient.Fetch(ctx, &gitgatewayv1.FetchRequest{WorktreeId: payload.WorktreeID}); err != nil {
				slog.WarnContext(ctx, "workspace-event: ref-sync fetch failed", "err", err)
				// Do not fail the whole handler — the notification half
				// (below) should still reach the user even if ref-sync did
				// not; log and continue rather than NAK-ing a message whose
				// retry would just repeat the same failing Fetch call.
			}
		}
		sessions.PushToProject(payload.ProjectID, WorkspaceEventFrame{Type: "workflow.execution.completed", Payload: payload})
		return nil
	})
	// A second SubscribeEphemeral call, same shape, for
	// orca.task.task.statuschanged — pushes {Type: "agent.complete"} (or
	// "task.statuschanged", whichever name the frontend's workspaceEvents
	// sketch settles on) frames, and additionally triggers the file
	// explorer's auto-refresh trigger BUG-PW-02 is blocked on: the
	// frontend's existing workspace.refreshFileTree channel
	// (channels_git.go:946-975) is already real, this bridge is the only
	// missing piece — once the frame arrives, the frontend calls the
	// channel it already has, no new git-gateway-service work needed.
}
```

`wsSessionRegistry`/`WorkspaceEventFrame` are new but small — a
project-id-keyed multimap of live WS connections, the same shape
`08-inter-service-communication.md`'s point 5 already describes
api-gateway owning ("any api-gateway replica can handle any connection...
state that mattered lives in the owning service, not the gateway" — here
the "state" is just an ephemeral in-memory routing table, not business
state, so this doesn't violate that stateless-by-design framing).

### 2. Two synchronous sagas at existing channel handlers (Flow 2)

```go
// channels_git.go's git.commit handler, after client.Commit succeeds:
sha := resp.GetCommitSha()
for _, taskNum := range extractTaskReferences(in.Message) { // regex: #TG-(\d+)
	task, err := taskClient.FindTaskByNumber(ctx, &taskv1.FindTaskByNumberRequest{
		ProjectId: projectIDForWorktree, TaskNumber: taskNum,
	})
	if err != nil || task.GetTask() == nil {
		continue // no matching task in this project — not an error, just no-op
	}
	done := "done"
	_, _ = taskClient.UpdateTask(ctx, &taskv1.UpdateTaskRequest{
		Id: task.GetTask().GetId(), Status: wrapperspb.String(done),
	})
	// Best-effort — a task-close failure must never fail the commit
	// response the user is already looking at; log and move on.
}
```

```go
// channels_scm.go's PR-creation handler, after CreatePullRequest succeeds
// — same regex, this time against the PR title/description/branch name,
// writing prUrl back via UpdateTask's new field instead of status.
```

`projectIDForWorktree` needs resolving from `worktree_id` — either via a
`project-service.GetProjectContext`-style lookup (already on
`project-service.md` §3's RPC list) cached the same way
`git-gateway-service.md` §8 already caches `worktree_id → connectionId`,
or by having the frontend pass `projectId` alongside `worktreeId` in the
existing `git.commit` channel args (cheaper, no new call, but trusts a
client-supplied value the handler should still validate against the
resolved worktree before using it for a cross-project task lookup).

## Notification-service's role

`notification-service`'s `HandleIncomingEvent` (already real) becomes a
second, independent consumer of the same two subjects — no coordination
needed with the `api-gateway` bridge above, since NATS JetStream fans one
published event out to every distinct durable/ephemeral consumer group
that subscribes, per `common/eventbus/eventbus.go`'s own doc comment on
competing-consumer vs. fan-out semantics. The only change needed there is
`domain.TranslateEvent` learning `orca.task.task.statuschanged` and
`orca.workflow.execution.completed` as recognized subjects (small,
additive — matches the file's own "translate a consumed domain event"
framing, no new architecture).

## Test plan

- `task-service/internal/usecase/update_task_test.go` — a status-changing
  update enqueues exactly one outbox row in the same fake transaction; a
  title-only update enqueues none.
- `task-service/internal/usecase/find_task_by_number_test.go` — resolves
  within a project, returns not-found across a different project's
  matching number (regression guard for the project-scoping decision).
- `workflow-service` — equivalent outbox-enqueue test at whichever usecase
  transitions `executions.status` to a terminal value.
- `orchestration-service/internal/usecase/update_task_status_and_promote_test.go` —
  extend/confirm this already calls (or now calls) `task-service.UpdateTask`
  on coordinator-run completion for a complex task.
- `api-gateway/internal/adapter/wscompat/workspace_events_test.go` — fake
  `eventbus.Consumer`, assert a `workflow.execution.completed` event with
  a create-PR-flavored `step_type` triggers `GitGatewayServiceClient.Fetch`
  with the event's `worktree_id`, and that a non-PR step type does not.
- `channels_git_test.go` — `git.commit` with a `#TG-42` message calls
  `FindTaskByNumber` then `UpdateTask(status=done)`; a message with no
  `#TG-` reference calls neither; a `FindTaskByNumber` NOT_FOUND is
  swallowed, not surfaced as a commit failure.
- End-to-end (integration, real NATS in CI per `05-data-architecture.md`'s
  outbox-relay testing convention): a task status update through
  `task-service` is observed, within a bounded poll window, on a test
  subscriber to `orca.task.task.statuschanged` — validates the whole
  outbox → relay → JetStream path, not just each service's unit tests.

## References

- `specs/backend-go/bugs/logic-v1/BUG-PW-04-workspace-integration-not-implemented.md` — full problem statement, all four flows, absence-of-evidence greps this solution closes
- `docs/logic/project-workspace/BL-PW-04-workspace-integration.md:19-127` — the four flows and `workspaceEvents` sketch
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:30-45` (event conventions), `:58-67` (API Gateway responsibility #5, WS session management for real-time surfaces)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166` (dependency graph — `notif -.events.-> wf`/`task`, `orch --> task` edges already drawn)
- `specs/backend-go/tdd/services/task-service.md:74-85` (§3.1 Execute's complexity branch, `active_execution_id`), `:232-249` (§6 package layout naming `adapter/eventbus/` and `task.completed`), `:255-284` (§7 dependencies, `orch --> task` callback edge)
- `specs/backend-go/tdd/services/workflow-service.md:203-210` (§5 outbox rows in schema section), `:283-285` (§7 "Publishes... consumed by notification-service")
- `specs/backend-go/tdd/services/git-gateway-service.md:169-177,208-216` (§5/§6 — deliberately no database, no `adapter/eventbus/`, "a `git.commit` isn't a fact other services react to asynchronously")
- `backend-go/common/eventbus/eventbus.go:1-20,100-125` — package doc, `Subscribe` vs. `SubscribeEphemeral` semantics this solution relies on for the durable-vs-fan-out consumer choice
- `backend-go/common/outbox/outbox.go:1-24` — package doc, the relay loop every new outbox adopts unchanged
- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:29-45` — the existing generic consumer this solution finally feeds
- `backend-go/proto/orca/task/v1/task.proto:44-67` (Task/UpdateTaskRequest — fields this solution extends), `:124-131` (`TaskServiceExecuteRequest`/`Response` — the `execution_ref` interpretation flagged above)
- `backend-go/proto/orca/project/v1/project.proto:258-265,286,343-349` — existing `task_id` lineage fields on `CreateWorktreeRequest`/`WorktreeLineageEntry`, the precedent Flow 4's `Task.worktree_id` mirrors
- `backend-go/services/orchestration-service/internal/usecase/update_task_status_and_promote.go` — file existence confirms this service already has a task-promotion usecase to extend/verify, not build from scratch
- `specs/backend-go/bugs/logic-v1/BUG-PW-02-file-explorer-dir-entry-and-auto-refresh-gaps.md` — the file-explorer auto-refresh consumer this solution's Flow 1 bridge unblocks
