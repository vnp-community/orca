# BUG-AT-01: Automation config CRUD works, but per-project cap, circular-trigger detection, and multi-action chains are all missing

**Business Logic:** [BL-AT-01](../../../../docs/logic/automation/BL-AT-01-cau-hinh-automation.md) — Cấu hình Automation Workflow
**Priority (per spec):** P2
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user can create/list/update/delete an automation with a schedule and exactly one action, and disabling it does stop it from running. But nothing stops them from creating a 21st, 50th, or 500th automation on the same project (BR-AT-02), nothing detects an automation whose action would re-trigger itself (BR-AT-04), and the "chuỗi actions" (ordered list of actions like create_worktree → run_agent → commit → create_pr → notify) the spec's schema describes cannot be expressed at all — an Automation stores only one `step_type`/`step_config_json` pair, not an action list.

---

## Spec summary

BL-AT-01 describes creating an automation with a name, a trigger (cron or event), and an ordered list of actions (`create_worktree`, `run_agent`, `commit`, `create_pr`, `notify`, `cleanup`), then validating and enabling it. Four business rules gate this: cron must be valid before save (BR-AT-01), at most 20 automations per project (BR-AT-02), a disabled automation must never run (BR-AT-03), and circular trigger chains must be detected and rejected (BR-AT-04).

## What backend-go has

- Full CRUD is real and wired end-to-end: `automation-service`'s proto defines `CreateAutomation`/`ListAutomations`/`UpdateAutomation`/`DeleteAutomation`/`RunNow`/`ListRuns`/`HandleExternalTrigger` (`backend-go/proto/orca/automation/v1/automation.proto:16-29`), each backed by a real usecase (`backend-go/services/automation-service/internal/usecase/create_automation.go`, `list_automations.go`, `update_automation.go`, `delete_automation.go`) and a real Postgres repository (`backend-go/services/automation-service/internal/adapter/postgres/repository.go:35-131`, full Create/Get/List/Update/Delete SQL, not stubs).
- All 6 `automation.*` WebSocket channels the frontend calls are registered for real: `automation.create`/`automation.runs` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go:73-117`) and `automation.list`/`automation.update`/`automation.delete` (`channels_automation_task.go:119-193`), plus `automation.runNow` (`channels.go:302` `registerAutomationChannels`) — both `registerAutomationChannels` and `registerAutomationTaskChannels` are called from `RegisterRealChannels` (`channels.go:90,118`). **This supersedes `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md`, which found only `automation.runNow` wired and `list`/`update`/`delete` unbuilt at every layer — that gap has since been closed.**
- REST only exposes a subset: `mountAutomationRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:23-28`) wires `POST /v1/automations/`, `POST /{id}/run`, `GET /{id}/runs`, `POST /{id}/trigger` — but **not** `ListAutomations`/`UpdateAutomation`/`DeleteAutomation`, even though the gRPC methods exist. REST callers of this domain are stuck with the WS-only list/update/delete path.
- BR-AT-01 (cron must be valid before save): implemented, but against a different schedule format than the spec's schema shows. `domain.NewAutomation` rejects an unparseable recurrence string via `NewRecurrenceRule` (`backend-go/services/automation-service/internal/domain/automation.go:110`, `recurrence_rule.go:26-34`) — but the field is `rrule` (RFC 5545), not the spec's literal `cron` field (`"0 9 * * 1-5"`). Validation-before-save exists; the wire format does not match BL-AT-01's documented schema.
- BR-AT-03 (disabled automation never runs): implemented. The scheduler's due-row query filters `WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= now()` (`backend-go/services/automation-service/internal/adapter/postgres/repository.go:149`), so a disabled row is never claimed.

## What's missing

- **BR-AT-02 (max 20 automations per project)**: no limit check anywhere. `CreateAutomation.Execute` (`backend-go/services/automation-service/internal/usecase/create_automation.go:38-97`) never counts existing automations for the tenant before inserting; `AutomationRepository.Create` (`postgres/repository.go:35-45`) has no count/limit guard either. A tenant can create unlimited automations.
- **BR-AT-04 (circular automation trigger detection)**: no such logic exists anywhere in `automation-service` — a repo-wide search for "circular" across `backend-go` turns up nothing related to automation (only an unrelated comment in `api-gateway/internal/adapter/httpgateway/router.go:87` about a WS session). Nothing prevents an automation whose action would re-trigger itself (directly, or via a chain through `HandleExternalTrigger`).
- **Ordered multi-action chains**: the spec's schema (`actions: [{type, params}, ...]`) has no backend-go equivalent. `Automation` (`automation.proto:32-48`) carries exactly one `step_type` + `step_config_json` pair — there is no list of actions, no execution order, and no per-step continue/stop-on-failure semantics (that belongs to BL-AT-02's flow, but the schema gap originates here). `workflow.proto`'s `StepType` enum (`backend-go/proto/orca/workflow/v1/workflow.proto:53-59`) has only `AGENT`/`SHELL`/`NOTIFICATION`/`WEBHOOK`/`CONDITION` — none of the spec's action types (`create_worktree`, `commit`, `create_pr`, `cleanup`) map onto it directly.
- **REST parity**: `ListAutomations`/`UpdateAutomation`/`DeleteAutomation` are gRPC-real but have no REST route in `automation_routes.go` — only reachable via the WS `wscompat` channel.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md` — describes the WS-channel gap as still-open; per this audit it is now resolved (all 6 `automation.*` channels wired). Cite for history, not as an open gap.

## References

- `backend-go/proto/orca/automation/v1/automation.proto:16-29,32-48` — service surface + `Automation` message (single step, no action list)
- `backend-go/proto/orca/workflow/v1/workflow.proto:53-59` — `StepType` enum (5 types, none matching the spec's action vocabulary)
- `backend-go/services/automation-service/internal/usecase/create_automation.go:38-97` — no per-tenant count check
- `backend-go/services/automation-service/internal/domain/automation.go:89-132` — `NewAutomation` validation (rrule format, no cap/cycle check)
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:35-131` — full CRUD SQL
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go:64-194` — all 6 `automation.*` channels
- `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go:23-28` — REST subset (create/run/runs/trigger only)
- `docs/logic/automation/BL-AT-01-cau-hinh-automation.md` — spec
