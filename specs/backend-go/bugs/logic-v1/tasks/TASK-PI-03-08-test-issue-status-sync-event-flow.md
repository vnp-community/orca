# TASK-PI-03-08: Tests for outbox publishing, webhook ingestion, and issue-status-sync

**From Solution:** SOL-PI-03
**Priority:** P1
**Service:** `project-service`, `scm-integration-service`, `issue-status-sync`
**File:** `services/project-service/internal/adapter/postgres/worktree_repository_test.go`, `services/project-service/internal/usecase/record_worktree_created_test.go`, `record_worktree_removed_test.go` (new), `services/scm-integration-service/internal/usecase/create_pull_request_test.go`, `receive_webhook_test.go` (new), `services/issue-status-sync/internal/usecase/sync_issue_status_test.go` (new)
**Depends on:** TASK-PI-03-03, TASK-PI-03-05, TASK-PI-03-06, TASK-PI-03-07
**Status:** `[x] DONE (unit-test scope) — record_worktree_created/removed_test.go assert outbox payload+had_open_pr; sync_issue_status_test.go covers dedup/empty-link/sync-disabled/retry/give-up/mapping-table. receive_webhook_test.go now added (TASK-PI-03-06 completed this batch — bad-signature-before-dedup, duplicate-no-second-enqueue, non-merge-recorded-not-enqueued, plus merge/record-failure/enqueue-failure cases). STILL NOT done: create_pull_request outbox-failure-non-fatal test and the optional testcontainers-NATS E2E — both remain out of this batch's scope.`

---

## Tests to add

### `project-service`

- `worktree_repository_test.go`: `CreateWorktreeWithEvent` integration test
  (real `testcontainers-go` Postgres, per `05-data-architecture.md`'s
  migration CI convention) — kill the transaction mid-way, assert neither
  the `worktrees` row nor the `outbox_events` row persists (both-or-neither).
- `record_worktree_created_test.go` / `record_worktree_removed_test.go`:
  event payload has the right subject and `linked_issue_ref`; empty when the
  request didn't carry one; `record_worktree_removed_test.go` asserts
  `had_open_pr` is always published `false`.

### `scm-integration-service`

- `create_pull_request_test.go`: outbox enqueue failure does not fail
  `Execute`'s return (BR-PI-09 on the publisher side) — assert `pr` is
  still returned successfully.
- `receive_webhook_test.go`: bad signature rejected before any dedup check;
  duplicate `delivery_id_header` returns `Duplicate: true` without a second
  outbox enqueue; a non-merge event (e.g. `pull_request.opened`) is recorded
  to `webhook_delivery_log` but does not enqueue any event.

### `issue-status-sync`

`sync_issue_status_test.go`:

- Duplicate `event_id` (already in `processed_events`) is a no-op —
  asserted via zero calls to `tracker`/`scm` fakes.
- `LinkedIssueProvider == ""` marks seen without calling any provider.
- `IsIssueStatusSyncEnabled == false` marks seen without calling any
  provider — regression guard for the "flag flipped mid-flight" race this
  design explicitly re-checks for.
- Retry: fake provider client fails twice then succeeds — assert 3 total
  attempts, success recorded.
- Give-up: fake provider client always fails — assert exactly 3 attempts
  (not 4, not unbounded), event still marked seen, no error propagated out
  of `HandleWorktreeLifecycle` (BR-PI-09).
- All four mapping-table rows (`worktree.created`->In Progress,
  `pr.created`->In Review, `pr.merged`->Done,
  `worktree.deleted && !had_open_pr`->Cancelled) resolve to the correct
  target state — table-driven test mirroring BL-PI-03's own table verbatim.

### End-to-end (optional, `testcontainers-go` NATS)

Publish a real `worktree.created` event from `project-service`'s outbox
relay, assert `issue-status-sync` consumes it and calls the fake
Linear/GitHub client within the test's timeout — validates the whole
JetStream wiring, not just each service's unit boundary.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/project-service/internal/adapter/postgres/... -run TestWorktreeRepository -v
go test ./services/project-service/internal/usecase/... -run "RecordWorktreeCreated|RecordWorktreeRemoved" -v
go test ./services/scm-integration-service/internal/usecase/... -run "CreatePullRequest|ReceiveWebhook" -v
go test ./services/issue-status-sync/internal/usecase/... -v
go build ./... && go vet ./...
```
