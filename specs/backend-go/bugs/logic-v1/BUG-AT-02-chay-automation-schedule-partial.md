# BUG-AT-02: Scheduled runs dispatch for real, but timeout, run-history retention, and concurrent-run prevention are all unenforced

**Business Logic:** [BL-AT-02](../../../../docs/logic/automation/BL-AT-02-chay-automation.md) — Chạy Automation theo Cron Schedule
**Priority (per spec):** P2
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A scheduled automation does fire and does record a real run outcome (succeeded/failed) — this is the one part of the flow backend-go visibly fixed over the old TS backend. But a stuck action can hang forever (no 2-hour timeout), a chatty automation's run-history table grows without bound (no 30-record retention), and firing the same automation twice in quick succession (e.g. a manual "Run now" click while a scheduled tick is mid-flight) starts two concurrent executions rather than being blocked.

---

## Spec summary

BL-AT-02 describes a scheduler that checks every 30 seconds, detects due automations, creates a run record, executes the automation's actions in sequence (continuing or stopping per config on failure), updates the run's final status, and sends a result notification. Four business rules: missed runs must catch up at app startup (BR-AT-05), a run must time out after 2 hours (BR-AT-06), run history keeps only the 30 most recent records (BR-AT-07), and concurrent runs of the same automation must be prevented (BR-AT-08).

## What backend-go has

- A real ticker-based scheduler: `Ticker.Run` (`backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:49-61`) ticks on `cfg.SchedulerInterval` (default 1 minute, not the spec's 30s — `backend-go/services/automation-service/internal/config/config.go:16`), calling `tick` → `ClaimDue` → `dispatch` for each due automation.
- Claiming is transactionally safe: `AutomationRepository.ClaimDue` (`backend-go/services/automation-service/internal/adapter/postgres/repository.go:140-177`) uses `SELECT ... FOR UPDATE SKIP LOCKED` so concurrent replicas never double-claim the same due row, and the claim transaction stays open until `Commit`/`Rollback` (`ticker.go:79`, `ports.go:85-107`).
- Execution is real, not stubbed: `RunNow.Execute` (`backend-go/services/automation-service/internal/usecase/run_now.go:44-147`) creates a `pending` run, transitions it to `running`, calls `workflow-service.ExecuteAdHocStep` over real gRPC (`internal/adapter/grpcclient/workflow_client.go:41-60`), and marks the run `succeeded`/`failed` based on the actual result — this closes the "TS Gap 3" documented in `automation.proto:9-13` and in `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md`'s "context-only note," which is now resolved (a `run_now_e2e_test.go` end-to-end test exists specifically to catch a regression back to the old skipped/unavailable behavior).
- BR-AT-05 (missed-run catch-up) is implicitly covered: `ClaimDue`'s query (`WHERE enabled AND next_run_at <= now()`) claims any overdue row the moment the scheduler next ticks, regardless of how long it was overdue — since `automation-service` is a continuously-running server process (not the old desktop-app-offline model the spec's wording assumes), this is a structural equivalent to "catch-up on startup," not a literal implementation of it.
- Idempotency (a *narrower* guard than BR-AT-08, not a substitute for it) is real: `RunNow.Execute` checks `FindByRequestID` before creating a new run (`run_now.go:61-65`) and re-checks on unique-constraint race (`run_now.go:95-97`), so the exact same `(automation_id, request_id)` never double-dispatches.

## What's missing

- **BR-AT-06 (2-hour run timeout)**: no timeout is applied anywhere in the dispatch path. `RunNow.Execute` calls `uc.executor.ExecuteAdHocStep(ctx, ...)` (`run_now.go:111`) using whatever context it was handed, with no `context.WithTimeout`/deadline wrapping the call in `run_now.go`, `ticker.go`, or `grpcclient/workflow_client.go:41-60`. A hung `workflow-service` call blocks the run (and the scheduler's claim transaction, per `ticker.go`'s per-automation loop) indefinitely.
- **BR-AT-07 (run history keeps 30 most recent records)**: no retention/pruning logic exists. `AutomationRunRepository.ListByAutomation` (`postgres/repository.go:291-322`) only paginates existing rows; nothing deletes or archives rows past the 30th, and `Create` (`postgres/repository.go:241-255`) never checks or trims history before inserting. Run history for a frequently-scheduled automation grows unbounded.
- **BR-AT-08 (concurrent run of same automation prevented)**: not enforced. The only de-duplication is exact-`request_id` idempotency (`run_now.go:61-65`) — two different request IDs for the *same automation* (e.g. a manual `RunNow` fired while the scheduler's own scheduled tick for that automation is still `running`) both proceed to call `workflow-service` concurrently. Neither `RunNow.Execute` nor `ClaimDue`'s query checks for an existing `running` row for the same `automation_id` before dispatching another.
- **Sequential multi-action execution with per-step continue/stop config**: since `Automation` carries a single `step_type`/`step_config_json` (see BUG-AT-01), there is no "FOR each action: execute, continue on success, stop-or-continue per config on failure" loop anywhere in `RunNow` or `workflow-service`'s `ExecuteAdHocStep` path — this part of the spec's main flow has no backend-go equivalent to evaluate.
- **Result notification** ("Gửi notification kết quả (nếu cấu hình)"): `notification-service` does subscribe to `orca.automation.run.completed` (`backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:47`), but nothing in `automation-service` — `run_now.go`, `ticker.go`, or the postgres adapters — publishes to that subject; there is no `eventbus`/publisher import anywhere under `services/automation-service` (confirmed by grep). The consumer side exists; the producer side does not, so no run-completion notification can ever actually fire today.
- **30-second scheduler cadence**: default interval is 1 minute (`config.go:16`), not 30 seconds as the spec's main flow step 1 states — a minor deviation, not a correctness bug, but worth flagging if the SRS intends the tighter cadence.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-033-automation-channels-partially-implemented.md` — its "context-only note" flags `RunNow`'s execution path as needing runtime verification; this audit confirms the execution path itself (usecase/gRPC wiring) is real, but the timeout/retention/concurrency rules around it are not.

## References

- `backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:49-134` — ticker loop, dispatch, no timeout wrapping
- `backend-go/services/automation-service/internal/usecase/run_now.go:44-147` — core dispatch interactor, idempotency-only dedup
- `backend-go/services/automation-service/internal/adapter/grpcclient/workflow_client.go:41-60` — outbound call, no deadline set
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:140-177,241-322` — `ClaimDue` (no running-lock check), run CRUD (no retention)
- `backend-go/services/automation-service/internal/config/config.go:14-22` — default 1-minute interval, 50-row batch size
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:41-48` — subscribes to `orca.automation.run.completed`, but nothing publishes it
- `backend-go/proto/orca/automation/v1/automation.proto:9-13` — "closes TS Gap 3" doc comment
- `docs/logic/automation/BL-AT-02-chay-automation.md` — spec
