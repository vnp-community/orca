# TASK-PI-03-04: `scm-integration-service`'s outbox table + relay wiring

**From Solution:** SOL-PI-03
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/migrations/0003_outbox_events.up.sql` (new), `backend-go/services/scm-integration-service/internal/adapter/eventbus/publisher.go` (new), `backend-go/services/scm-integration-service/internal/adapter/postgres/outbox_repository.go` (new), `backend-go/services/scm-integration-service/cmd/server/main.go`
**Depends on:** TASK-PI-01-04 (this task's migration number follows `0002_issue_list_cache`)
**Status:** `[x] DONE — migrations/0003_outbox_events, domain.OutboxEvent, OutboxRepository, OutboxEnqueuer port, relay wired in main.go.`

---

## Context

`scm-integration-service.md` §5 already anticipates a `webhook_delivery_log`
table with no current writer. This task adds `outbox_events` alongside it —
`CreatePullRequest`/`MergePullRequest`/`ReceiveWebhook`'s "domain state
change" is the successful provider API call itself, not a local row; the
enqueue *is* the persisted side effect, the identical shape
`issue-tracking-service.LinkIssue` already established (`link_issue.go:39-42`).
Same relay mechanism as TASK-PI-03-02, copied verbatim — no new pattern.

## Changes to make

`migrations/0003_outbox_events.up.sql` (new):

```sql
CREATE TABLE scm.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_scm_outbox_events_unpublished
    ON scm.outbox_events (created_at)
    WHERE published_at IS NULL;

ALTER TABLE scm.outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

`internal/adapter/postgres/outbox_repository.go` (new) — identical
`Enqueue`/`FetchUnpublished`/`MarkPublished` shape as
`issue-tracking-service/internal/adapter/postgres/repository.go`, retargeted
at `scm.outbox_events`. Add `OutboxEnqueuer`/`domain.OutboxEvent` to
`scm-integration-service`'s `usecase`/`domain` packages, mirroring
`issue-tracking-service`'s `OutboxEnqueuer` port and `domain.OutboxEvent`
type exactly (this service's `Enqueue` is a plain single-INSERT call, same
as `issue-tracking-service`'s — `CreatePullRequest`/`MergePullRequest` have
no local domain-state write of their own to share a transaction with; see
that service's `link_issue.go` doc comment for why a bare
`Enqueue(ctx, tenantID, event)` is still correct here).

`cmd/server/main.go` — wire `eventbus.Connect`, `EnsureStream` for
`orca.scm.pull_request.created`/`orca.scm.pull_request.merged`, and
`outbox.NewRelay(...)` + `relay.Run(ctx)` in a goroutine, following
`issue-tracking-service/cmd/server/main.go`'s existing wiring.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
