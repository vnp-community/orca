# SOL-AT-03: Event-triggered automation dispatch — a NATS JetStream consumer plus trigger-type storage

**Resolves:** [BUG-AT-03](../BUG-AT-03-event-trigger-not-implemented.md)
**Service:** `automation-service` (new consumer) + `task-service`,
`project-service`, `scm-integration-service` (new publishers — see "Cross-
service work needed" below)
**Affected files (proposed):**
- `backend-go/proto/orca/automation/v1/automation.proto`
- `backend-go/services/automation-service/internal/domain/automation.go`, `trigger.go` (new)
- `backend-go/services/automation-service/internal/usecase/create_automation.go`, `update_automation.go`
- `backend-go/services/automation-service/internal/usecase/handle_event_trigger.go` (new)
- `backend-go/services/automation-service/internal/usecase/ports.go`
- `backend-go/services/automation-service/internal/adapter/eventbus/consumer.go` (new)
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ migration: `trigger_type`, `trigger_event`, `trigger_filter_json` columns)
- `backend-go/services/automation-service/cmd/server/main.go` (wire the new consumer)
- Cross-service (flagged, not designed in full): `task-service`, `project-service`, `scm-integration-service` outbox publishers for the events that don't exist yet
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**Channel choice.** `08-inter-service-communication.md:8`: "NATS JetStream
(async) — Domain events other services react to eventually, not
immediately" is precisely BL-AT-03's shape — an automation reacting to
`agent:completed`/`pr:merged`/etc. is exactly the case the table's "why not
the others" column rules out sync gRPC for: "Sync gRPC would couple the
publisher's availability to every subscriber's availability; this is
exactly the coupling event-driven architecture avoids." A design where
`task-service`/`scm-integration-service` call `automation-service`
synchronously on every completion/merge would invert this and is
explicitly the pattern this architecture document rejects.

**Consumer shape — durable, not ephemeral.** `notification-service`'s
existing consumer (`eventbus/consumer.go:6-18`) uses
`SubscribeEphemeral` specifically because *notification* delivery is
per-replica, per-locally-connected-WS-client — "only ONE replica would ever
receive a given event" would be a real bug there. `automation-service` does
not have that problem: it is the system of record for automation dispatch
(`automation-service.md:10-13`), and `RunNow`'s idempotency key
(`(automation_id, request_id)`, §8) already makes a duplicate delivery
harmless. A **durable**, shared JetStream consumer (every replica competes
for the same named consumer, each event delivered to exactly one replica)
is therefore both correct and simpler here — no fan-out problem to solve,
consistent with `08-inter-service-communication.md:42-45`'s "Consumers are
idempotent by construction (dedup on event ID...)" baseline requirement,
which this design satisfies via the *existing* `request_id` uniqueness
mechanism rather than inventing a second dedup table.

**Trigger-type storage.** Grounded directly in BUG-AT-03's own citation of
BL-AT-01's schema (`trigger: {type: cron | manual | event, event?:
string}`) — `automation-service.md` doesn't itself describe an `event`
trigger type (its RPC sketch is schedule/`RunNow`-only, §3), so this is a
genuine, flagged extension of that document's domain model (§4), the same
class of addition [SOL-AT-01](./SOL-AT-01-config-cap-cycle-chain.md) makes
for `project_id`/`actions`.

**Filter matching (BR-AT-09).** No existing precedent in this service;
designed fresh below as a small, closed comparison grammar — deliberately
mirroring `workflow-service.md:338-341`'s `ConditionStepExecutor` posture
("a fixed, closed grammar of comparison operators... no
`eval`/reflection-based evaluation") rather than introducing a second,
looser expression language for what is a structurally identical problem
(match a JSON payload against a simple predicate).

**Cycle detection (BR-AT-10, same rule as BR-AT-01's BR-AT-04).** Modeled
after `workflow-service.md:122-128`'s existing DAG cycle-detection
precedent — `BuildWaves` rejects a template graph with `ErrCyclicDependency`
"same as TS's `WorkflowCycleError`" — reused here as an equivalent
graph-validation pass over the tenant's event-triggered automations, run at
config-save time.

---

## Design — proto (`automation.proto`)

```protobuf
message Automation {
  // ... existing fields, plus SOL-AT-01's project_id/actions ...
  TriggerType trigger_type = 12;      // NEW
  string trigger_event = 13;          // NEW — one of the 5 event names below; empty unless trigger_type=EVENT
  string trigger_filter_json = 14;    // NEW — BR-AT-09, e.g. {"agent":"claude"}; empty = no filter (always matches)
}

enum TriggerType {
  TRIGGER_TYPE_UNSPECIFIED = 0; // = CRON, for back-compat with existing rrule-only rows
  TRIGGER_TYPE_CRON = 1;
  TRIGGER_TYPE_MANUAL = 2;      // rrule/dtstart still stored but next_run_at never advances — RunNow-only
  TRIGGER_TYPE_EVENT = 3;
}
```

`CreateAutomationRequest`/`UpdateAutomationRequest` mirror the same three
fields. `trigger_event` is validated server-side against a closed enum of
the five documented event names
(`agent:completed`/`agent:error`/`worktree:created`/`pr:merged`/
`issue:assigned`) — an unrecognized value is rejected at create/update time
(`INVALID_ARGUMENT`), not silently stored and never matched.

## Design — domain

```go
// internal/domain/trigger.go (new)
type TriggerType string

const (
	TriggerTypeCron   TriggerType = "cron"
	TriggerTypeManual TriggerType = "manual"
	TriggerTypeEvent  TriggerType = "event"
)

// EventName is a closed set — the five names BL-AT-03 documents. An
// unrecognized string is rejected by NewAutomation/UpdateAutomation, not
// silently accepted and never matched (which would look like a silently
// broken automation to the user).
type EventName string

const (
	EventAgentCompleted   EventName = "agent:completed"
	EventAgentError       EventName = "agent:error"
	EventWorktreeCreated  EventName = "worktree:created"
	EventPRMerged         EventName = "pr:merged"
	EventIssueAssigned    EventName = "issue:assigned"
)

func (e EventName) Valid() bool { /* one of the 5 above */ }

// TriggerFilter is BR-AT-09's closed comparison grammar — deliberately no
// arbitrary expression evaluation, matching workflow-service.md:338-341's
// ConditionStepExecutor posture for the same class of problem.
// {"field": "agent", "equals": "claude"} — field is dot-path into the
// event payload, "equals" is the only operator v1 needs (BL-AT-03's own
// example is an equality filter); fail-safe-false on a malformed filter
// (an automation with a broken filter never fires rather than firing on
// everything).
type TriggerFilter struct {
	Field  string
	Equals string
}

func (f TriggerFilter) Matches(payload map[string]any) bool { /* dot-path lookup + string-equals */ }
```

`Automation` gains `TriggerType TriggerType`, `TriggerEvent EventName`,
`TriggerFilter *TriggerFilter`. `NewAutomation`'s validation:
`TriggerType == TriggerTypeEvent` requires `TriggerEvent.Valid()`;
`TriggerType != TriggerTypeEvent` requires `TriggerEvent == ""` (no orphaned
event config on a cron automation). `TriggerType` unset defaults to
`TriggerTypeCron` (back-compat: every pre-migration row is implicitly a
cron automation, matching today's exclusively-`rrule` behavior).

## Design — event subscription (`adapter/eventbus/consumer.go`, new)

```go
// Subjects this consumer subscribes to. Mapping from BL-AT-03's 5 event
// names to real (or planned) orca.<service>.<entity>.<event> subjects —
// see "Cross-service work needed" below for which of these already exist.
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},       // -> agent:completed
	{StreamName: "TASK", Subject: "orca.task.task.failed"},          // -> agent:error
	{StreamName: "PROJECT", Subject: "orca.project.worktree.created"}, // -> worktree:created (NEW publisher needed)
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.pull_request.merged"}, // -> pr:merged (NEW)
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.issue.assigned"},      // -> issue:assigned (NEW)
}

type Consumer struct {
	bus     *commoneventbus.Consumer
	dispatch *usecase.HandleEventTrigger
}

func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	for _, b := range Subjects {
		// Durable (NOT SubscribeEphemeral) — see rationale above: every
		// replica competes for the same named consumer, one delivery per
		// event across the whole automation-service fleet.
		go c.bus.Subscribe(ctx, b.StreamName, b.Subject, func(ctx context.Context, event commoneventbus.Event) error {
			return c.dispatch.Execute(ctx, usecase.HandleEventTriggerInput{
				EventID: event.ID, TenantID: event.TenantID,
				EventName: subjectToEventName(b.Subject), Payload: event.Payload,
			})
		})
	}
}
```

```go
// internal/usecase/handle_event_trigger.go (new)
type HandleEventTrigger struct {
	automations AutomationRepository // new method: ListByTrigger(ctx, tenantID, eventName) []Automation
	runNow      *RunNow
}

func (uc *HandleEventTrigger) Execute(ctx context.Context, in HandleEventTriggerInput) error {
	tenantCtx := tenant.WithTenantID(ctx, in.TenantID)
	matches, err := uc.automations.ListByTrigger(tenantCtx, in.TenantID, in.EventName)
	if err != nil {
		return err
	}
	var payload map[string]any
	_ = json.Unmarshal([]byte(in.Payload), &payload)
	for _, automation := range matches {
		if !automation.Enabled {
			continue // BR-AT-03 — the existing disabled-never-runs rule, reused verbatim for event dispatch
		}
		if automation.TriggerFilter != nil && !automation.TriggerFilter.Matches(payload) {
			continue // BR-AT-09
		}
		// Deterministic request_id from (event ID, automation ID) —
		// idempotent under JetStream's at-least-once redelivery, same
		// mechanism as scheduler.scheduledRequestID (ticker.go:132-134).
		requestID := fmt.Sprintf("event:%s:%s", in.EventID, automation.ID)
		if _, err := uc.runNow.Execute(tenantCtx, RunNowInput{
			AutomationID: automation.ID, RequestID: requestID, Trigger: domain.RunTriggerEvent,
		}); err != nil {
			// Log and continue — one automation's dispatch failure must not
			// block matching/dispatching the rest of this tenant's automations
			// for the same event (same "fail closed per-run, not per-batch"
			// posture RunNow itself already has for workflow-service errors).
		}
	}
	return nil
}
```

`domain.RunTrigger` (`automation_run.go:36-53`) gains `RunTriggerEvent`
alongside the existing `manual`/`scheduled`/`external` — a fourth, distinct
value (not reusing `RunTriggerExternal`, since `HandleExternalTrigger` is an
authenticated-webhook-initiated call with a caller-supplied `request_id`,
while this is an internal event-bus-initiated call with a
service-derived one — different trust boundaries, per §9 below).

`AutomationRepository.ListByTrigger(ctx, tenantID, eventName)` is a new
query: `SELECT ... WHERE tenant_id = $1 AND trigger_type = 'event' AND
trigger_event = $2 AND enabled = true` — needs
`idx_automations_trigger (tenant_id, trigger_type, trigger_event) WHERE
trigger_type = 'event'`, a new partial index sized to this exact query
shape.

## Design — BR-AT-10 / BR-AT-04: cycle detection

Modeled on `workflow-service.md:122-128`'s `BuildWaves` precedent. At
`CreateAutomation`/`UpdateAutomation` time, for a candidate automation whose
`TriggerType == TriggerTypeEvent`:

1. Build a directed graph over the tenant's event-triggered automations
   (including the candidate): an edge `X -> Y` exists if any of `X`'s
   actions (per SOL-AT-01's `Actions` list) could emit the event `Y` is
   configured to trigger on. The action→event mapping is a small, fixed
   table (mirrors `EventName`'s closed set): `run_agent` (`StepTypeAgent`)
   → `agent:completed`/`agent:error`; `create_worktree` → `worktree:created`;
   `create_pr` → `pr:merged`. (`commit`/`notify`/`cleanup` emit none of the
   5 documented events — no edge.)
2. Run a standard cycle check (DFS with a recursion stack, or Kahn's
   in-degree/BFS — either is fine at this scale; `workflow-service`'s own
   `BuildWaves` uses Kahn's, reused here for consistency) over that graph
   including the candidate node.
3. A cycle found → reject with `FAILED_PRECONDITION
   AUTOMATION_TRIGGER_CYCLE`, naming the cycle's member automation IDs (same
   "offending node set" transparency `ErrCyclicDependency` already gives
   workflow-service callers, `workflow-service.md:128`).

This subsumes BUG-AT-01's BR-AT-04 (a same-automation, single-hop cycle is
just the `X == Y` degenerate case of the same graph) — SOL-AT-01 doesn't
need its own separate implementation once this lands; its "self-reference
guard" section is the placeholder until this ships.

## Design — security (§9)

Per `automation-service.md:293-299,305-310`: an event-triggered run
executes with the *automation's owning user's* permissions
(`RunAutomation` — here, `RunNow`, unchanged) re-validated at dispatch time,
never an elevated "event bus" identity — the JetStream consumer is
purely a trigger source, not a trust boundary, same posture already
documented for the scheduler ticker. This is a materially *lower*-risk
boundary than `HandleExternalTrigger`'s "untrusted caller" note
(`automation-service.md:305-310`): the event payload originates from
another trusted backend-go service via the internal message bus, not an
external webhook caller, so no separate authentication step is needed here
— tenant scoping (`event.TenantID`, part of every event payload per
`08-inter-service-communication.md:34-36`'s "every event payload includes...
tenant ID") is the only boundary this consumer must enforce, via
`tenant.WithTenantID` before every downstream call, exactly as shown above.

## Cross-service work needed (flagged, not designed here)

Per BUG-AT-03's own scope note ("this audit did not attempt to trace
whether ... `task-service` or `scm-integration-service` emit anything
semantically equivalent"), this solution confirms via the subject list
above:

- `orca.task.task.completed`/`orca.task.task.failed` **already exist**
  (`notification-service/internal/adapter/eventbus/consumer.go:45`) — these
  cover `agent:completed`/`agent:error` with no new publisher work, *if*
  task completion/failure is semantically equivalent to agent completion in
  this system (a mapping assumption this solution makes explicit — needs
  confirmation against `task-service`'s own event semantics before
  implementation, not assumed silently).
- `worktree:created` has **no publisher today**. `project-service.md`'s API
  surface (§3) shows `RecordWorktreeCreated` as a synchronous RPC called by
  `git-gateway-service` after a real `git worktree add` succeeds
  (`project-service.md:63-69`'s sequence diagram) — this needs a new outbox
  write inside that same handler, publishing `orca.project.worktree.created`
  per the standard event conventions (`08-inter-service-
  communication.md:32-45`). Out of this solution's file list — belongs to
  whoever owns `project-service`'s `RecordWorktreeCreated` usecase.
- `pr:merged`/`issue:assigned` have **no publisher today**.
  `scm-integration-service`'s proto (confirmed by direct read) has
  `MergePullRequest`/`UpdateIssue` RPCs but no outbox event on either path.
  Same shape of fix: add an outbox write to both usecases, subjects
  `orca.scmintegration.pull_request.merged` /
  `orca.scmintegration.issue.assigned`.

**None of this requires Dev Server Agent (`agent/`) changes** — every event
source is a backend-go service reacting to its own already-owned state
transition (task completion, a git-gateway-service-reported worktree
creation, an SCM provider webhook `scm-integration-service` already
receives), not anything the execution plane needs to emit differently.

## Test plan

- `domain/trigger_test.go`: `TriggerFilter.Matches` — dot-path lookup
  against a nested payload, missing field → `false` (fail-safe), malformed
  filter JSON at parse time → rejected at `NewAutomation`, not deferred to
  match time.
- `domain/automation_test.go`: `TriggerType == Event` requires a valid
  `TriggerEvent`; `TriggerType != Event` rejects a non-empty `TriggerEvent`.
- `usecase/handle_event_trigger_test.go`: fake `AutomationRepository` +
  fake `RunNow`'s dependencies — an event matching 3 automations (1
  disabled, 1 filter-mismatched, 1 matching) dispatches exactly once;
  redelivery of the same `EventID` is a no-op via `RunNow`'s existing
  `request_id` idempotency (assert executor called once total across two
  `Execute` calls with the same `EventID`).
- `usecase/create_automation_test.go` (cycle detection): automation A
  (trigger=`agent:completed`, action=`create_pr`) + automation B
  (trigger=`pr:merged`, action=`run_agent`) — creating B after A exists
  → rejected `AUTOMATION_TRIGGER_CYCLE`; creating either alone succeeds.
- `adapter/eventbus/consumer_test.go`: subject→`EventName` mapping table is
  exhaustive over the 5 documented names (compile-time-checked switch,
  default case fails a test rather than silently dropping an event).
- Integration: a durable JetStream consumer test confirming exactly one of
  N replica instances processes a given event (regression guard against
  accidentally using `SubscribeEphemeral`, which would multiply-dispatch
  across replicas here — the opposite bug from notification-service's).

## References

- `backend-go/proto/orca/automation/v1/automation.proto:32-48` — `Automation` message this extends with trigger fields
- `backend-go/services/automation-service/internal/domain/automation.go:59-132` — domain model, no trigger-type/event/filter
- `backend-go/services/automation-service/internal/domain/automation_run.go:36-53` — `RunTrigger` enum gaining `RunTriggerEvent`
- `backend-go/services/automation-service/internal/usecase/handle_external_trigger.go:1-45` — existing external-trigger path, contrasted in the security section
- `backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:126-134` — `scheduledRequestID`, the deterministic-idempotency-key precedent this solution's event `request_id` derivation mirrors
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:1-18,44-51` — `SubscribeEphemeral` rationale (and why this solution deliberately does NOT reuse it)
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:33,37` — `MergePullRequest`/`UpdateIssue` RPCs confirmed to have no outbox publish today
- `specs/backend-go/tdd/services/automation-service.md:10-13` (system-of-record framing), `:276-280,305-310` (security/trust-boundary posture this solution's §9 reuses)
- `specs/backend-go/tdd/services/workflow-service.md:122-128` (`BuildWaves` cycle-detection precedent), `:338-341` (`ConditionStepExecutor`'s closed-grammar precedent for `TriggerFilter`)
- `specs/backend-go/tdd/services/project-service.md:63-69` — `RecordWorktreeCreated` sequence this solution's `worktree:created` publisher would hook into
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:6-9` (channel-choice table), `:30-45` (event conventions, transactional outbox, idempotent-consumer requirement)
- `docs/logic/automation/BL-AT-03-event-trigger.md` — spec (5 events, BR-AT-09/BR-AT-10)
