# SOL-AT-02: 2-hour run timeout, 30-record retention, concurrent-run lock, and the missing run-completion publish

**Resolves:** [BUG-AT-02](../BUG-AT-02-chay-automation-schedule-partial.md)
**Service:** `automation-service`
**Affected files (proposed):**
- `backend-go/services/automation-service/internal/usecase/run_now.go`
- `backend-go/services/automation-service/internal/usecase/ports.go`
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ new migration: partial unique index, retention query)
- `backend-go/services/automation-service/internal/adapter/eventbus/` (new — outbox publisher)
- `backend-go/services/automation-service/internal/config/config.go`
- `backend-go/services/automation-service/internal/adapter/grpcclient/workflow_client.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**Timeout (BR-AT-06).** `workflow-service.md:311-316` already establishes
the pattern this needs: "a step's own timeout (default 30 minutes, matching
TS) is enforced independently via `context.WithDeadline` from the step's
start — the 5s intra-cluster default is for control-plane calls only, not
long-running step execution." Automation's 2-hour budget is a **different,
outer** timeout: it bounds the whole dispatched run (which, per
[SOL-AT-01](./SOL-AT-01-config-cap-cycle-chain.md), may now be a multi-action
chain — several sequential `ExecuteAdHocStep` calls, each individually
bounded by `workflow-service`'s own 30-minute per-step deadline), not any
one step. This is consistent with `08-inter-service-communication.md:26-28`'s
"Deadlines are mandatory on every outbound call... default 5s for
intra-cluster calls, overridable per call site with a documented reason" —
automation-service's `RunNow`→`ExecuteAdHocStep` call is exactly such an
overridden, documented case, same as `workflow-service`'s own per-step
override already is.

**Retention (BR-AT-07).** `automation-service.md:184` already defines
`idx_automation_runs_automation ON automation_runs (automation_id,
created_at DESC)` — an index shaped precisely for "find the Nth-most-recent
row for this automation," i.e. already anticipates a retention query even
though no pruning logic calls it today (confirmed absent per
`postgres/repository.go:241-322`'s `Create`/`ListByAutomation`, neither of
which prunes).

**Concurrency guard (BR-AT-08).** The service already has the *pattern* this
needs: `run_now.go:89-98` catches a unique-constraint violation on
`(automation_id, request_id)` and re-queries rather than treating the race
as fatal — "the idempotency key is what makes a duplicate claim harmless,
not the claim mechanism alone" (`automation-service.md:276-280`). BR-AT-08
is the same shape of problem (two different `request_id`s for the *same
automation*, one manual + one scheduled) solved the same way: a second
Postgres uniqueness constraint, this time scoped to "at most one `running`
row per automation," not to a specific `request_id`.

**Result notification.** `automation-service.md:220` already names the
missing piece in its own package layout: `eventbus/ # outbox publisher:
automation.run.completed, ...` — the package is documented as part of this
service's shape but doesn't exist yet (confirmed by BUG-AT-02's grep: zero
`eventbus`/`Publish` references under `services/automation-service`).
`08-inter-service-communication.md:38-41`: "Publishing goes through the
transactional outbox pattern... never a direct publish call inside a
request handler, which would risk publishing an event for a transaction
that later rolls back" — this governs exactly where the publish call must
live: inside the same transaction as the terminal `UpdateStatus` write, not
appended after it.

---

## Design — BR-AT-06: 2-hour run timeout

```go
// internal/usecase/run_now.go — wraps the action-loop from SOL-AT-01
const runTimeout = 2 * time.Hour // BR-AT-02, automation-service.md's own
                                  // deadline-override convention (08-inter-
                                  // service-communication.md:26-28)

func (uc *RunNow) Execute(ctx context.Context, in RunNowInput) (domain.AutomationRun, error) {
	// ... tenant/idempotency/pending-run setup unchanged ...
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	for i, action := range automation.Actions {
		result, execErr := uc.executor.ExecuteAdHocStep(runCtx, ExecuteAdHocStepInput{ /* ... */ })
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			// Whole-run budget exhausted mid-chain — mark Failed with a
			// distinct reason so BR-AT-06's timeout is distinguishable in
			// run history from an ordinary step failure.
			failed, _ := running.MarkFailed(time.Now().UTC(), "automation run exceeded 2h timeout")
			_ = uc.runs.UpdateStatus(ctx, failed) // ctx, not runCtx — runCtx is already expired
			return failed, apperrors.New(apperrors.KindDeadlineExceeded, "AUTOMATION_RUN_TIMEOUT", "run exceeded 2h timeout", runCtx.Err())
		}
		// ... existing per-action success/failure handling from SOL-AT-01 ...
	}
}
```

`grpcclient/workflow_client.go:41-60`'s `ExecuteAdHocStep` call already
receives whatever `ctx` `RunNow` hands it — no change needed there beyond
confirming it propagates the deadline rather than replacing it with its own
(the file has no `context.WithTimeout` of its own today per BUG-AT-02's
finding, so it's a pure pass-through — safe by default here).

## Design — BR-AT-07: 30-record retention

```go
// internal/usecase/ports.go — new method on AutomationRunRepository
type AutomationRunRepository interface {
	// ... existing methods ...
	// PruneOldRuns deletes every automation_runs row for automationID beyond
	// the `keep` most recent (by created_at DESC) — BR-AT-07.
	PruneOldRuns(ctx context.Context, tenantID, automationID string, keep int) error
}
```

```sql
-- adapter/postgres/repository.go — PruneOldRuns
DELETE FROM automation.automation_runs
WHERE tenant_id = $1 AND automation_id = $2
  AND id NOT IN (
    SELECT id FROM automation.automation_runs
    WHERE tenant_id = $1 AND automation_id = $2
    ORDER BY created_at DESC
    LIMIT $3
  );
```

Uses `idx_automation_runs_automation (automation_id, created_at DESC)`
(`automation-service.md:184`) for the inner `ORDER BY ... LIMIT` — no new
index required. Called from `RunNow.Execute` **after** the run reaches a
terminal status (`succeeded`/`failed`/timeout), best-effort (a prune
failure logs but never fails the run itself — pruning is housekeeping, not
part of the run's own correctness, same "best-effort" posture
`RemoveWorktree.Execute`'s bookkeeping write already uses for an analogous
non-critical side effect, `remove_worktree.go:41-43`). `keep = 30` is a
package-level constant next to `runTimeout` above.

## Design — BR-AT-08: concurrent-run prevention

**Schema**: a partial unique index, migration alongside SOL-AT-01's:

```sql
CREATE UNIQUE INDEX idx_automation_runs_one_running
  ON automation.automation_runs (automation_id)
  WHERE status = 'running';
```

Postgres enforces "at most one `running` row per `automation_id`" natively —
no read-then-write race window, matching the existing
`idx_automation_runs_request_id` unique-index pattern
(`automation-service.md:185`) this service already relies on for
`request_id` idempotency.

**Usecase**: `run_now.go`'s existing transition from `pending` → `running`
(`:101-107`, `pending.MarkRunning` → `uc.runs.UpdateStatus`) is the write
that would violate this index for a second concurrent dispatch of the same
automation. Catch it exactly like the existing `request_id` race
(`:89-98`):

```go
if err := uc.runs.UpdateStatus(ctx, running); err != nil {
	if isUniqueViolation(err, "idx_automation_runs_one_running") {
		// Another dispatch (scheduler tick or manual RunNow) for this
		// automation is already running — BR-AT-08. Not an error: the
		// caller gets back the run that's actually in flight, same
		// "duplicate claim is harmless" posture as the request_id race.
		if existing, found, ferr := uc.runs.FindRunning(ctx, tenantID, automation.ID); ferr == nil && found {
			return existing, nil
		}
	}
	return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
}
```

`FindRunning(ctx, tenantID, automationID)` is a new
`AutomationRunRepository` method — `SELECT ... WHERE automation_id = $1 AND
status = 'running' LIMIT 1`, backed by the same partial index. Unlike
`request_id`-based idempotency (which returns the *same logical* run for a
retried dispatch), this path returns a *different* run — the one already in
flight — to the *newer* caller, which is the correct BR-AT-08 semantics:
"don't start a second execution," not "pretend the new request was already
served."

**Why not `ClaimDue`'s query instead**: `ClaimDue`
(`postgres/repository.go:140-177`) claims from the `automations` table on
`next_run_at`, which has no visibility into `automation_runs.status` — and a
manual `RunNow` call (not from the ticker at all) needs the same guard.
Enforcing at the `automation_runs` write is the one location both dispatch
paths (`RunNow`'s gRPC handler and `Ticker.dispatch`) already funnel
through (`automation-service.md:86-88`: "one code path calls
`workflow-service` regardless of trigger source"), so it's the correct
single enforcement point, not a duplicate check added to both callers.

## Design — result notification publish

```go
// internal/adapter/eventbus/publisher.go (new package, per
// automation-service.md:220's already-documented layout)
type RunCompletedPublisher struct {
	outbox commoneventbus.OutboxWriter // per 05-data-architecture.md's transactional outbox
}

func (p *RunCompletedPublisher) PublishRunCompleted(ctx context.Context, tx pgx.Tx, run domain.AutomationRun) error {
	// Written in the SAME transaction as the terminal UpdateStatus write —
	// 08-inter-service-communication.md:38-41's "never a direct publish call
	// inside a request handler" rule. Subject: orca.automation.run.completed
	// (already the exact subject notification-service subscribes to,
	// eventbus/consumer.go:48).
	return p.outbox.Write(ctx, tx, commoneventbus.OutboxEntry{
		Subject: "orca.automation.run.completed",
		TenantID: run.TenantID,
		Payload: runCompletedPayload{AutomationID: run.AutomationID, RunID: run.ID, Status: string(run.Status)},
	})
}
```

`AutomationRunRepository.UpdateStatus` (`postgres/repository.go:275-289`)
changes from a bare `pool.Exec` to a `pool.Begin`-wrapped transaction that
performs the status `UPDATE` and the outbox `INSERT` together, only for
**terminal** transitions (`run.Status.Terminal()`, per
`automation_run.go:32-34`'s existing helper) — intermediate `pending`→
`running` writes don't publish anything, matching BL-AT-02's "gửi
notification kết quả" (result notification, i.e. only on completion).

## Design — 30-second scheduler cadence (minor)

`config.go:16`'s `defaultSchedulerInterval` changes from `time.Minute` to
`30 * time.Second`, matching BL-AT-02's main-flow step 1 literally. Flagged
as low-risk/low-priority per BUG-AT-02's own framing ("a minor deviation,
not a correctness bug") — included here only because it's a one-line change
riding alongside this bug's other fixes, not because it needs its own
design.

## Test plan

- `usecase/run_now_test.go`: a fake `WorkflowStepExecutor` that never
  returns (blocks on `ctx.Done()`) → `RunNow.Execute` returns
  `AUTOMATION_RUN_TIMEOUT` within the test's shortened `runTimeout` override
  (inject via a package-level var or constructor param for testability, not
  a hardcoded const); run row lands `failed` with the timeout reason.
- `usecase/run_now_test.go`: 31 prior runs for one automation, `keep=30` →
  after a new dispatch, `PruneOldRuns` reduces the fake repo's stored count
  to exactly 30, newest-first.
- `usecase/run_now_test.go`: two concurrent `Execute` calls for the same
  automation (different `request_id`s) against a fake repo that simulates
  the partial-unique-index violation on the second `UpdateStatus` call →
  second call returns the first call's run via `FindRunning`, never
  dispatches to the executor a second time (assert executor call count == 1).
- `adapter/postgres/repository_test.go` (integration, real Postgres): the
  partial unique index actually rejects a second concurrent `running` insert
  for the same `automation_id`; a `succeeded`/`failed` row for the same
  automation does NOT conflict (index only applies `WHERE status =
  'running'`).
- `adapter/eventbus/publisher_test.go`: `PublishRunCompleted` writes exactly
  one outbox row per terminal transition, zero for `pending`→`running`;
  a rolled-back transaction leaves no outbox row (verifies the same-tx
  requirement, not an after-the-fact publish).
- End-to-end regression guard (mirrors the existing `run_now_e2e_test.go`
  cited in BUG-AT-02): a real notification-service consumer receiving
  `orca.automation.run.completed` after this fix, where today it never
  fires at all.

## References

- `backend-go/services/automation-service/internal/usecase/run_now.go:44-147`, esp. `:89-98` (existing race-tolerant pattern this solution's BR-AT-08 fix mirrors), `:109-126` (dispatch call this solution wraps in a timeout)
- `backend-go/services/automation-service/internal/adapter/postgres/repository.go:140-177` (`ClaimDue`, no running-lock), `:241-322` (run CRUD, no retention/prune), `:275-289` (`UpdateStatus`, becomes tx-wrapped)
- `backend-go/services/automation-service/internal/adapter/grpcclient/workflow_client.go:41-60` — outbound call, confirmed pass-through of caller's `ctx`
- `backend-go/services/automation-service/internal/domain/automation_run.go:29-34` (`RunStatus.Terminal()`, reused as the publish gate)
- `backend-go/services/automation-service/internal/config/config.go:14-22` — default interval this solution changes to 30s
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:44-51` — the existing, already-correct subscriber this solution finally gives a publisher
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:41-43` — best-effort bookkeeping precedent this solution's prune-failure handling follows
- `specs/backend-go/tdd/services/automation-service.md:44-46,86-88` (single-dispatch-path framing), `:184-185` (retention/idempotency indexes), `:220` (documented but unbuilt `eventbus/` package), `:276-280` (accepted race posture BR-AT-08's design mirrors)
- `specs/backend-go/tdd/services/workflow-service.md:311-316` — per-step deadline convention this solution's outer 2h timeout complements, not duplicates
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:26-28` (mandatory, overridable deadlines), `:30-45` (event conventions, transactional outbox requirement)
- `docs/logic/automation/BL-AT-02-chay-automation.md` — spec
