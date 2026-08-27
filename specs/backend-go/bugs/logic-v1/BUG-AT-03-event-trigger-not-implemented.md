# BUG-AT-03: No automation is ever triggered by an Orca event — the event→automation wiring does not exist

**Business Logic:** [BL-AT-03](../../../../docs/logic/automation/BL-AT-03-event-trigger.md) — Kích hoạt Automation theo Sự kiện
**Priority (per spec):** P2
**Status:** NOT_IMPLEMENTED
**Severity:** Medium
**Symptom:** A user configures an automation intending it to fire when "agent:completed" or "pr:merged" happens. Nothing in backend-go ever calls it automatically — no component listens for any of the five documented events and dispatches the matching automation. The only way to run an event-triggered automation today is for some external caller to already know the automation's ID and manually POST to `/v1/automations/{id}/trigger` (or send `automation.runNow`) — which is not event-driven at all, it is just another manual-run entry point.

---

## Spec summary

BL-AT-03 describes automations that fire automatically when one of five Orca-internal events occurs: `agent:completed`, `agent:error`, `worktree:created`, `pr:merged`, `issue:assigned`. Two business rules: event matching must support a filter (e.g. only fire when `agent = "claude"`, BR-AT-09), and an event-triggered automation must not be able to trigger itself (circular prevention, BR-AT-10).

## What backend-go has

- `automation-service` exposes `HandleExternalTrigger` (`backend-go/proto/orca/automation/v1/automation.proto:20-25`), which maps an already-known `automation_id` + `request_id` + opaque `payload_json` onto the same `RunNow` dispatch path, tagging the run `RunTriggerExternal` (`backend-go/services/automation-service/internal/usecase/handle_external_trigger.go:39-45`). This is wired all the way through: gRPC (`internal/adapter/grpc/server.go:103-113`), REST (`POST /v1/automations/{id}/trigger`, `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:26,148-172`), and is a real, callable RPC.
- `Automation`'s schema has a `step_type`/`step_config_json`/`rrule` triple (`automation.proto:32-48`) but **no trigger-type field at all** — there is no `event` column, no stored "trigger on `agent:completed`" configuration, and no filter configuration (e.g. `agent = "claude"`) anywhere in the domain model (`backend-go/services/automation-service/internal/domain/automation.go:59-87`) or the Postgres schema the repository targets (`internal/adapter/postgres/repository.go:36-40` insert column list: `id, tenant_id, name, rrule, dtstart, step_type, step_config_json, enabled, timezone, next_run_at, created_at, updated_at` — no event/filter columns).
- No component anywhere in backend-go subscribes to the five documented event names or calls `HandleExternalTrigger`/`RunNow` in response to them. `notification-service` is the only service with a real eventbus consumer loop (`backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:41-48`), and its subject list is `orca.task.task.completed`, `orca.workflow.execution.completed/failed`, `orca.automation.run.completed`, `orca.credential.credential.rotated`, `orca.orchestration.decision_gate.opened` — none of these are `agent:completed`, `agent:error`, `worktree:created`, `pr:merged`, or `issue:assigned`, and in any case `notification-service` only turns events into user notifications, it never calls back into `automation-service`. `automation-service` itself has zero `eventbus`/`Publish`/`Subscribe` references anywhere under `backend-go/services/automation-service` (confirmed by grep).

## What's missing

- **The entire event→automation dispatch mechanism.** There is no consumer anywhere (in `automation-service` or elsewhere) that listens for `agent:completed`, `agent:error`, `worktree:created`, `pr:merged`, or `issue:assigned` and calls `HandleExternalTrigger`/`RunNow` for the automations configured to react to them. `HandleExternalTrigger` is a pure "someone already decided which automation and called us" RPC — it is not itself triggered by anything.
- **No trigger-type/event storage on `Automation` at all.** The schema has no way to say "this automation's trigger is `event:agent:completed`" — `automation.proto`'s `Automation` message and the Postgres columns are 100% schedule/step-oriented (`rrule`, `step_type`, `step_config_json`). BL-AT-01's own schema (`trigger: {type: cron | manual | event, event?: string}`) has no backend-go equivalent for the `event` case.
- **BR-AT-09 (event filter, e.g. `agent = "claude"`)**: no filter concept exists anywhere — there is no field to store a filter and no matching logic, since there is no event-consumption path to apply a filter within in the first place.
- **BR-AT-10 (event-triggered automation must not trigger itself)**: no cycle-detection code exists for this or any other automation trigger path (see BUG-AT-01's identical finding for BR-AT-04) — moot until the event-consumption path itself exists, but confirmed absent regardless.
- **The five specific events are not even published as domain events under those names** by their owning services (agent completion, PR merge, issue assignment) in a form this service could consume — this audit did not attempt to trace whether e.g. `task-service` or `scm-integration-service` emit anything semantically equivalent, since even if they did, nothing downstream consumes it for automation purposes.

## See also

- None — this gap does not overlap any existing `missing-v1`/`api-v1` bug; `HandleExternalTrigger`'s wiring is covered informationally in `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md`, but that bug is about WS channel registration, not the (entirely separate, and entirely missing) event-listener mechanism this BL requires.

## References

- `backend-go/proto/orca/automation/v1/automation.proto:20-29,32-48` — `HandleExternalTrigger` RPC + `Automation` message (no event/filter fields)
- `backend-go/services/automation-service/internal/usecase/handle_external_trigger.go:1-45` — maps external trigger onto `RunNow`, no event-listening of its own
- `backend-go/services/automation-service/internal/domain/automation.go:59-87` — domain model, no trigger-type/event/filter fields
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:36-40` — insert column list, no event/filter columns
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:41-48` — the only real eventbus consumer in the codebase; wrong subjects, wrong purpose (notifications, not automation dispatch)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:26,148-172` — REST `POST /v1/automations/{id}/trigger`, requires already-known automation_id
- `docs/logic/automation/BL-AT-03-event-trigger.md` — spec
