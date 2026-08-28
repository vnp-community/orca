# SOL-AT-01: Per-project cap, action-chain schema, and a self-reference guard for automation config

**Resolves:** [BUG-AT-01](../BUG-AT-01-cau-hinh-automation-partial.md)
**Service:** `automation-service` (+ `api-gateway` REST parity)
**Affected files (proposed):**
- `backend-go/proto/orca/automation/v1/automation.proto`
- `backend-go/services/automation-service/internal/domain/automation.go`
- `backend-go/services/automation-service/internal/domain/automation_action.go` (new)
- `backend-go/services/automation-service/internal/usecase/create_automation.go`, `update_automation.go`
- `backend-go/services/automation-service/internal/usecase/ports.go`
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ new migration)
- `backend-go/services/automation-service/internal/usecase/run_now.go` (multi-action loop; shared with SOL-AT-02)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**Per-project cap (BR-AT-02).** The currently-implemented `domain.Automation`
has no `ProjectID` at all (`automation.go:59-87`), so "per project" cannot be
enforced today — only a per-tenant count would be possible. The TDD's own
domain model already carries this field:
`automation-service.md:94` lists `project_id (logical FK → project-service)`
as part of `Automation`, and `automation-service.md:140` shows it as a
nullable Postgres column (`project_id UUID`) already in ADR-021's migration
`0021` DDL. The current schema in
`backend-go/services/automation-service/internal/adapter/postgres/repository.go:35-45`
targets a narrower, already-diverged table (no `project_id` column at all —
confirmed by the insert column list). This solution promotes `project_id`
from "documented but unimplemented" to a real column, closing the gap
directly rather than inventing a new mechanism.

**Multi-action chains.** `automation-service.md:44-46` states explicitly:
"an automation run is structurally a one-step (**or few-step**) workflow
execution, not a different kind of thing" — the TDD already anticipates more
than one step per automation, even though its RPC sketch (§3) and the
implemented proto (`automation.proto:32-48`) only carry a single
`step_type`/`step_config_json` pair today. `workflow-service.md`'s
`ExecuteAdHocStep` (§3.1) is explicitly "one step definition ... no
`dependsOn`, no template" — deliberately NOT the DAG/wave machinery
(`workflow-service.md:250-265`, Kahn's-algorithm wave computation) that a
branching, parallel DAG would need. BL-AT-01's "chuỗi actions" is a strictly
**ordered, sequential** list (`create_worktree → run_agent → commit →
create_pr → notify`), not a DAG — so the correct extension is N sequential
`ExecuteAdHocStep` calls from `RunNow`, not routing automation through
`workflow-service.Execute`'s full template/DAG path. Building a DAG for a
linear chain would pull in inheritance resolution, wave concurrency, and
persisted-template semantics (`workflow-service.md:212-242`) that this
BL doesn't need and that the automation TDD never asked for.

**Circular-trigger detection (BR-AT-04).** BUG-AT-03's BR-AT-10 is the same
rule under a different code, scoped to event triggers specifically. Full
graph-cycle detection is designed once, in
[SOL-AT-03](./SOL-AT-03-event-trigger.md), since a cycle can only actually
form once event-triggered dispatch exists (BUG-AT-03's own finding: "moot
until the event-consumption path itself exists"). This solution adds the one
piece of BR-AT-04 that applies regardless of trigger type and belongs at
config-save time: reject an automation whose action list contains a
"trigger automation" reference to its own `id` (the trivial, single-hop,
same-document self-reference) — see "Design — domain" below.

**REST parity.** `automation_routes.go:22-27` mounts only
create/run/runs/trigger; `ListAutomations`/`UpdateAutomation`/
`DeleteAutomation` are real gRPC methods
(`automation.proto:27-29`) with no REST route. `08-inter-service-
communication.md:54-56`: "Routes REST requests to the appropriate service
via `grpc-gateway`... no separately maintained REST layer" — `api-gateway`
hand-writes this translation today (`automation_routes.go`'s own doc
comment cites `mountUsageRoutes`'s precedent), so the fix is three more
hand-written handlers following the exact shape already used for the four
that exist, not a new pattern.

---

## Design — proto (`automation.proto`)

```protobuf
message Automation {
  string id = 1;
  string tenant_id = 2;
  string project_id = 9;      // NEW — logical FK -> project-service.projects; empty = unscoped (back-compat)
  string name = 3;
  string rrule = 4;
  // step_type/step_config_json (fields 5/6) are DEPRECATED but left on the
  // wire for one release: UpdateAutomation on a pre-migration row without
  // `actions` set still round-trips through them. New/updated automations
  // populate `actions` instead — see AutomationAction below.
  string step_config_json = 5 [deprecated = true];
  orca.workflow.v1.StepType step_type = 6 [deprecated = true];
  bool enabled = 7;
  string dtstart = 8;
  string timezone = 10;        // renumbered from 9 to make room for project_id above
  repeated AutomationAction actions = 11; // NEW — ordered chain, BR-AT-01's schema
}

// AutomationAction is one step in an automation's ordered action chain.
// Mirrors BL-AT-01's `actions: [{type, params}, ...]` schema field-for-field
// (`type` -> step_type, `params` -> step_config_json), plus the
// continue/stop-on-failure switch BL-AT-02's main flow requires.
message AutomationAction {
  orca.workflow.v1.StepType step_type = 1;
  string step_config_json = 2;
  // on_failure controls whether RunNow's action loop continues to the next
  // action or stops the run — see BL-AT-02 "continuing or stopping per
  // config on failure". Default (unspecified) = STOP, the safer default for
  // a chain whose later steps (commit/create_pr) usually depend on an
  // earlier one (create_worktree) having actually happened.
  OnFailurePolicy on_failure = 3;
}

enum OnFailurePolicy {
  ON_FAILURE_POLICY_UNSPECIFIED = 0; // = STOP
  ON_FAILURE_POLICY_STOP = 1;
  ON_FAILURE_POLICY_CONTINUE = 2;
}
```

`CreateAutomationRequest`/`UpdateAutomationRequest` gain the same
`project_id` and `repeated AutomationAction actions` fields (mirroring
`Automation`'s shape, per this proto's existing convention of request
messages mirroring the entity 1:1 — see `automation-service.md:79-84`'s
`RunNowRequest` sketch and the current `CreateAutomationRequest`'s existing
1:1 mirror of `Automation`).

## Design — domain

```go
// internal/domain/automation_action.go (new)
type OnFailurePolicy string

const (
	OnFailureStop     OnFailurePolicy = "stop"
	OnFailureContinue OnFailurePolicy = "continue"
)

func (p OnFailurePolicy) Valid() bool { /* stop|continue */ }

type AutomationAction struct {
	StepType       StepType
	StepConfigJSON string
	OnFailure      OnFailurePolicy // defaults to OnFailureStop, mirrors proto default
}
```

`Automation` (`automation.go:59-87`) gains `ProjectID string` and
`Actions []AutomationAction`, replacing the single `StepType`/
`StepConfigJSON` pair as the source of truth for new rows. `NewAutomation`
gains a length check: `len(actions) == 0` returns `ErrEmptyActions`
(replacing `ErrEmptyStepConfig`'s role); a single-element `actions` list is
still valid (the common case — most automations have exactly one action) and
is exactly what dispatches through `ExecuteAdHocStep` unchanged in
`RunNow` (SOL-AT-02).

**Self-reference guard (BR-AT-04, config-time slice).** If BL-AT-01's action
vocabulary is later extended with a "trigger another automation" action type
(not present in backend-go today — `create_worktree`/`run_agent`/`commit`/
`create_pr`/`notify`/`cleanup` map to `StepType`'s existing five values plus
SOL-AT-04's new `cleanup_worktrees`, none of which reference another
automation by id), `NewAutomation`/`UpdateAutomation` must reject any action
whose config references the automation's own `id`. Until such an action type
exists, this guard is a no-op by construction — flagged here so it isn't
silently dropped if that action type is added later, not implemented as
dead code today.

## Design — usecase: per-project cap (BR-AT-02)

```go
// internal/usecase/ports.go — new method on the existing AutomationRepository port
type AutomationRepository interface {
	// ... existing methods ...
	CountByProject(ctx context.Context, tenantID, projectID string) (int, error)
}
```

```go
// internal/usecase/create_automation.go — inside Execute, before domain.NewAutomation
if in.ProjectID != "" {
	count, err := uc.repo.CountByProject(ctx, tenantID, in.ProjectID)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_COUNT_FAILED", "failed to count existing automations", err)
	}
	if count >= maxAutomationsPerProject { // = 20, BR-AT-02
		return domain.Automation{}, apperrors.New(apperrors.KindFailedPrecondition, "AUTOMATION_PROJECT_LIMIT_EXCEEDED", "project already has 20 automations", nil)
	}
}
```

`CountByProject` backs onto a `WHERE tenant_id = $1 AND project_id = $2`
count query — cheap, indexed once `idx_automations_project (tenant_id,
project_id)` is added alongside the new column (mirrors the existing
`idx_automations_tenant` index shape, `automation-service.md:161`). Race
window (two concurrent creates both reading count=19) is the same
class of race `RunNow`'s idempotency section already accepts as
inherent to at-least-once/concurrent systems (`automation-service.md:276-
280`, "two replicas racing ... is expected occasionally"); a `CHECK`-based
hard cap isn't practical in Postgres without a trigger, and BR-AT-02's cap
is a UX guard against runaway automation creation, not a security boundary
— an off-by-one under a genuine race is an acceptable trade-off, consistent
with the at-least-once posture this service already embraces elsewhere.
`ProjectID == ""` (unscoped/back-compat automations) skips the cap entirely
— pre-migration rows and any deliberately-global automation aren't capped
by a project they don't belong to.

## Design — usecase: multi-action dispatch (shared surface with SOL-AT-02)

`RunNow.Execute` (`run_now.go:44-147`) changes its single
`uc.executor.ExecuteAdHocStep(...)` call (line 111) into a loop over
`automation.Actions`, in order:

```go
for i, action := range automation.Actions {
	result, execErr := uc.executor.ExecuteAdHocStep(ctx, ExecuteAdHocStepInput{
		TenantID: tenantID, StepType: action.StepType,
		StepConfigJSON: action.StepConfigJSON,
		RequestID: fmt.Sprintf("%s:%d", in.RequestID, i), // per-action idempotency suffix
	})
	// record a per-action result on the run (see AutomationRun.ActionResults
	// below); on execErr or result.Status=="failed": if action.OnFailure ==
	// OnFailureStop (or unset), break and mark the whole run Failed; if
	// OnFailureContinue, record the failure and proceed to the next action.
}
```

`AutomationRun` (`automation_run.go:73-84`) gains
`ActionResults []ActionResult` (`{Index int; Status string; OutputJSON,
ErrorMessage string}`), replacing the single `OutputJSON`/`ErrorMessage`
pair as the primary record for multi-action runs (both fields are kept,
now holding the *last* action's output, for backward wire-compatibility with
any caller still reading them directly). The full timeout/retention/
concurrency wrapping around this loop is SOL-AT-02's concern — this file
only introduces the loop shape and the schema it iterates over.

## Design — wiring: REST parity

```go
// automation_routes.go — mountAutomationRoutes gains 3 routes
sub.Get("/", handleListAutomations(client))
sub.Patch("/{id}", handleUpdateAutomation(client))
sub.Delete("/{id}", handleDeleteAutomation(client))
```

Each new handler follows `handleCreateAutomation`/`handleRunNow`'s existing
shape exactly (`identityFromContext` → `AttachIdentity` → gRPC call →
`writeJSON`/`writeGRPCError`) — no new pattern, direct 1:1 mirrors of the
`automation.*` WS channels already wired in `channels_automation_task.go`.

## Test plan

- `domain/automation_test.go`: `NewAutomation` rejects `len(actions) == 0`;
  accepts a single-action list unchanged (regression guard for existing
  single-step automations); `OnFailurePolicy` defaults to `stop` when unset.
- `usecase/create_automation_test.go`: fake `AutomationRepository.CountByProject`
  returns 20 → `AUTOMATION_PROJECT_LIMIT_EXCEEDED`; returns 19 → succeeds;
  `ProjectID == ""` → cap skipped regardless of count.
- `usecase/run_now_test.go` (shared with SOL-AT-02): a 3-action chain where
  action 2 fails with `OnFailureStop` → run marked `failed`, action 3 never
  dispatched (assert fake executor call count == 2); same chain with
  `OnFailureContinue` on action 2 → all 3 actions dispatched, run's
  `ActionResults` records action 2's failure, final run status determined by
  action 3's outcome (or `failed` if any non-continue action failed — exact
  policy documented in `run_automation.go`'s doc comment when implemented).
- `adapter/postgres/repository_test.go`: `CountByProject` scoped correctly
  across two projects in the same tenant (no cross-project leakage).
- `httpgateway/automation_routes_test.go`: new List/Update/Delete handlers
  round-trip against a fake `AutomationServiceClient`, matching the existing
  four handlers' test shape.
- Migration test: a pre-migration row with `step_type`/`step_config_json`
  set and `actions` empty still dispatches correctly via the deprecated-field
  fallback path (backward compatibility during the expand/contract window).

## References

- `backend-go/proto/orca/automation/v1/automation.proto:32-48,50-58,115-125` — current single-step `Automation`/request shapes this extends
- `backend-go/services/automation-service/internal/domain/automation.go:59-132` — current domain model, no `ProjectID`/`Actions`
- `backend-go/services/automation-service/internal/usecase/create_automation.go:38-97` — no per-project count check
- `backend-go/services/automation-service/internal/usecase/run_now.go:44-147`, esp. `:109-116` — single `ExecuteAdHocStep` call this solution loops
- `backend-go/services/automation-service/internal/domain/automation_run.go:73-84` — `AutomationRun` shape gaining `ActionResults`
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:35-45` — insert column list, no `project_id`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:21-28` — REST subset this extends
- `specs/backend-go/tdd/services/automation-service.md:44-46` (few-step framing), `:79-84` (request-mirrors-entity convention), `:94` (`project_id` in domain model), `:140,161` (Postgres column + index precedent), `:276-280` (accepted at-least-once race posture)
- `specs/backend-go/tdd/services/workflow-service.md:74-86` (`ExecuteAdHocStep`'s single-step, no-DAG scope), `:250-265` (DAG/wave machinery this solution deliberately avoids for a linear chain)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:54-56` (REST-via-`grpc-gateway` convention `api-gateway`'s hand-written translation follows)
- `docs/logic/automation/BL-AT-01-cau-hinh-automation.md` — spec (`actions` schema, BR-AT-01/02/03/04)
