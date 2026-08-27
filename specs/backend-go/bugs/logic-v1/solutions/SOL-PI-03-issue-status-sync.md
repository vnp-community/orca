# SOL-PI-03: Event-driven issue-status sync (worktree/PR lifecycle → Linear/GitHub status)

**Resolves:** [BUG-PI-03](../BUG-PI-03-issue-status-sync-not-implemented.md)
**Service:** `project-service` (publishes `worktree.*`), `scm-integration-service` (publishes `pr.*`, gains a webhook receiver), new consumer logic in a **new `issue-status-sync` component** — see "Where the consumer lives" below — calling into `issue-tracking-service`/`scm-integration-service`'s existing `UpdateIssue`/`TransitionIssue` usecases
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto` (`Worktree`/`RecordWorktreeCreatedRequest` gain `linked_issue_provider`/`linked_issue_ref`; `Project` gains `issue_status_sync_enabled` — shared with SOL-PI-02)
- `backend-go/services/project-service/internal/adapter/eventbus/publisher.go` (new — first outbox use in this service)
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` (outbox row written in the same transaction as `RecordWorktreeCreated`/`RecordWorktreeRemoved`)
- `backend-go/services/project-service/internal/usecase/record_worktree_created.go`, new `record_worktree_removed.go` outbox event construction
- `backend-go/services/project-service/migrations/000X_outbox_events.up.sql` (new — first outbox table in this service)
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` (webhook receiver RPC, or plain HTTP endpoint — see below)
- `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go`, `merge_pull_request.go` (outbox publish on success)
- `backend-go/services/scm-integration-service/internal/usecase/receive_webhook.go` (new — GitHub/GitLab merge-event ingestion)
- `backend-go/services/scm-integration-service/internal/adapter/eventbus/publisher.go`, `migrations/000X_outbox_events.up.sql` (new — this service's first outbox use)
- `backend-go/services/issue-status-sync/` (new deployable — see rationale) — `cmd/server/main.go`, `internal/usecase/sync_issue_status.go`, `internal/adapter/eventbus/subscriber.go`, `internal/adapter/postgres/processed_events.go`, `migrations/`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### The event mechanism: transactional outbox, per `08-inter-service-communication.md` and `05-data-architecture.md`

BR-PI-09 ("status update không được block main workflow" —
`docs/logic/project-integration/BL-PI-03-update-issue-status.md:38`) is,
almost word for word, the reason `08-inter-service-communication.md`'s
table gives for choosing NATS JetStream over sync gRPC: "Sync gRPC would
couple the publisher's availability to every subscriber's availability;
this is exactly the coupling event-driven architecture avoids"
(`08-inter-service-communication.md:8`). This is not a judgment call this
solution is making — it is the TDD's own stated selection criterion,
applied to exactly the kind of event BUG-PI-03 describes ("domain events
other services react to eventually, not immediately" —
`08-inter-service-communication.md:8`). `05-data-architecture.md`'s outbox
section is equally direct about mechanics: "Service A writes its domain
state change and an outbox row... in the **same Postgres transaction**"
(`05-data-architecture.md:86-88`) — every event this design publishes is
therefore attached to the write that already has to happen for that
lifecycle transition, not a bolt-on publish call.

### Why `project-service`, not `git-gateway-service`, publishes `worktree.created`/`worktree.deleted`

BUG-PI-03's own finding is that `CreateWorktree`
(`create_worktree.go`) "call[s] `project-service`'s
`RecordWorktreeCreated`/deletion RPCs directly and return... neither
publishes an outbox event" (BUG-PI-03:23). The fix is not to add a publish
call to `git-gateway-service` — that service structurally **cannot**
participate in the outbox pattern's core guarantee, because it owns no
Postgres database at all (confirmed by SOL-009's "Same 'owns no data'
shape... no new database, no new migrations", carried into SOL-PI-02's
design above). An outbox row written outside the transaction that commits
the domain state change reintroduces exactly the dual-write inconsistency
the pattern exists to prevent (`05-data-architecture.md:96-98`'s worked
example: "If the publish step fails or is delayed, the task state itself
was already committed correctly — no dual-write inconsistency" — that
guarantee only holds when state-write and outbox-row-write share a
transaction).

`project-service` is the correct publisher because it is already the
transactional writer of the fact this event announces:
"Worktree existence, lineage, activation flag" is `project-service`'s
owned metadata per its own bounded-context table (`project-service.md:36`),
and `RecordWorktreeCreated`/`RecordWorktreeRemoved` are already the
durable Postgres writes that make a worktree's existence real
(`project-service.md:97-98`: "Worktree lifecycle metadata — written by
`git-gateway-service` AFTER the real git op succeeds on the host, never a
trigger for one"). Adding one outbox row to that same transaction is the
textbook case `05-data-architecture.md:95` illustrates almost verbatim
("`task-service` completes a task → outbox row `task.completed`").
`project-service` has no outbox infrastructure yet (its adapter layout at
`project-service.md:289` lists only `eventbus:` as a bare comment,
"`project.created`, `project.rebound`, `member.added`, ..." with no
worktree-lifecycle event named) — this solution is the first thing to
populate that package, following `issue-tracking-service`'s
already-implemented pattern (`link_issue.go`) as the concrete template.

### Why `scm-integration-service` needs its own outbox-only addition for `pr.created`/`pr.merged`

`scm-integration-service.md` §5 is explicit that this service's Postgres
database holds "operational bookkeeping only... explicitly not a copy,
cache, or mirror of provider data" (`scm-integration-service.md:146-150`)
and names exactly two tables, neither of which is a PR/issue row
(`scm-integration-service.md:152-156`). This is structurally the same
situation `issue-tracking-service.LinkIssue` was already in — that
usecase's own doc comment states the resolution plainly: "issue-tracking-
service gained its own (minimal, outbox-only) database in Epic G...
specifically because there's no other domain state to be atomic with"
(`link_issue.go:39-42`). `CreatePullRequest`/`MergePullRequest` are in the
identical shape: the "domain state change" being announced is the
successful provider API call itself, not a local row — the enqueue *is*
the persisted side effect, exactly `link_issue.go`'s framing. This solution
proposes the same fix `issue-tracking-service` already took: extend
`scm-integration-service`'s existing database (it already has one, per §5)
with an `outbox_events` table structurally identical to
`issue-tracking-service`'s, rather than inventing a new mechanism.

This is flagged as a genuine, if precedented, extension: `scm-integration-
service.md` §5's table only lists `rate_limit_cache`/`webhook_delivery_log`
today (`scm-integration-service.md:152-156`) — `outbox_events` is new to
that table, following the exact model `issue-tracking-service.md` §5
already documents for the sibling service (`issuetracking_connections` +
future `issuetracking_webhook_deliveries`, `issue-tracking-service.md:161-190`).

### Why PR-merge detection needs a webhook receiver, and why that's not a new idea for this service

BUG-PI-03 correctly identifies that `MergePullRequest`
(a synchronous, Orca-initiated mutation) only covers merges *through*
Orca — an externally-merged PR (merged directly on github.com) has "no
webhook receiver, polling job, or event that fires" (BUG-PI-03:25).
`scm-integration-service.md` §5 already anticipates exactly this gap: its
data model table names `webhook_delivery_log` — "Append-only record of
inbound webhook deliveries processed... makes delivery idempotent against
provider retries" (`scm-integration-service.md:155`) — a table with no
current writer anywhere in the codebase (per BUG-PI-03's own repo-wide
grep results, §"What's missing"). This solution is the first feature to
give that table a producer: a `ReceiveWebhook` usecase, fed by a new
`api-gateway`-fronted HTTP endpoint (webhooks arrive from GitHub/GitLab's
servers, which cannot speak gRPC or carry a JWT — this must be a plain
HTTP receiver, the one deliberate exception to
`08-inter-service-communication.md`'s "gRPC for sync" default, justified
by the caller being an external, unauthenticated-by-Orca system).
`GetRateLimitStatus`'s per-provider signature-verification posture (§9,
"[t]okens are never logged... resolved per-call") extends naturally to
webhook payload signature verification (GitHub's `X-Hub-Signature-256`,
GitLab's `X-Gitlab-Token`) before any event is trusted.

### Where the consumer lives: a new minimal service, not folded into an existing one

The consumer needs to: receive `worktree.created`/`worktree.deleted`
(from `project-service`) and `pr.created`/`pr.merged` (from
`scm-integration-service`), resolve which issue-tracker (Jira/Linear via
`issue-tracking-service`) or SCM (GitHub/GitLab via `scm-integration-
service`) the linked issue lives in, and call the right `UpdateIssue`/
`TransitionIssue` RPC with BR-PI-08's retry-3-then-give-up policy. No
existing service is a correct home:

- **Not `project-service`** — it would then need outbound clients to both
  `issue-tracking-service` and `scm-integration-service` for a concern
  that isn't project/membership/worktree metadata, violating design
  principle 3 ("a service owns exactly the data it's the system of
  record for" — `02-microservices-decomposition.md:28-32`); it's the
  event's *publisher*, not naturally its consumer too (publish/subscribe
  living in the same service for a two-hop event is not a stated pattern
  anywhere in `08-inter-service-communication.md`).
- **Not `scm-integration-service`** or **`issue-tracking-service`
  individually** — each owns exactly one side of the provider mapping
  (BL-PI-03's own table has both a Linear column and a GitHub column
  reacting to the same four events); putting the consumer in either one
  makes it silently depend on the other as a peer for half its cases,
  which neither service's TDD doc dependency list
  (`issue-tracking-service.md:227-241`, `scm-integration-service.md:194-
  210`) currently includes or should — issue-tracking-service explicitly
  has no outbound dependency on scm-integration-service and vice versa.
- **Not `orchestration-service`** — despite owning "multi-agent
  coordination," its catalog entry is scoped to "messages, dispatch
  contexts, decision gates, coordinator runs"
  (`02-microservices-decomposition.md:51`), none of which is "consume a
  lifecycle event and call a provider-status RPC"; BUG-PI-02 already
  confirmed it has zero `Issue` references today and this isn't the
  place to introduce the first one.

Design principle 4 — "a TS RPC namespace... purely a metadata table with
no business rules of its own... folded into the closest service that owns
related workflow logic" (`02-microservices-decomposition.md:33-36`) — does
**not** apply cleanly here either, because this fan-in consumer has real
business rules of its own (BR-PI-07/08/09, plus the four-row mapping
table) and no single "closest" owner among the four services it touches.
This solution proposes a new, deliberately thin **`issue-status-sync`
service**: no owned business data beyond a `processed_events` idempotency
table (`08-inter-service-communication.md:42-45`'s "dedupe on event ID,
stored in the consuming service's own... table" — the minimal state every
consumer needs regardless), subscribing to both event streams and calling
out to the two existing `UpdateIssue`/`TransitionIssue` usecases
synchronously per BR-PI-08's retry contract. **Flagged explicitly as an
18th service, beyond `02-microservices-decomposition.md`'s "Total: 17
services"** (`02-microservices-decomposition.md:72`) — the alternative
(folding this into one of the four existing touchpoints) was evaluated
above and rejected for the coupling reasons stated, but this is a genuine
scope addition to the TDD's service catalog, not something already
specified, and should be confirmed with whoever owns that catalog before
implementation. If a lighter-weight home is preferred, the second-best
option is a new package inside `api-gateway` that runs as a background
NATS consumer alongside its edge-routing role — rejected here only because
`08-inter-service-communication.md`'s API Gateway responsibilities list
(`08-inter-service-communication.md:47-70`) is scoped entirely to
synchronous edge concerns and a background consumer loop would be the one
exception to that framing, a smaller but still real deviation.

---

## Design — proto additions

### `project.proto` — worktree↔issue link (shared with SOL-PI-02)

```protobuf
message Worktree {
  // ... existing fields 1-15 unchanged ...
  optional string linked_issue_provider = 16;  // "github" | "gitlab" | "jira" | "linear"
  optional string linked_issue_ref = 17;       // provider-native ref: "owner/repo#123" or "ENG-123"
}

message RecordWorktreeCreatedRequest {
  // ... existing fields 1-11 unchanged ...
  optional string linked_issue_provider = 12;
  optional string linked_issue_ref = 13;
}

message Project {
  // ... existing fields ...
  bool issue_status_sync_enabled = N;  // BR-PI-07, default true — see SOL-PI-02
}
```

### `project.proto` — event payloads (published via NATS JetStream, not gRPC — shown here since the schema is this service's to define)

```protobuf
// Subject: orca.project.worktree.created / orca.project.worktree.deleted
// Naming per 08-inter-service-communication.md: orca.<service>.<entity>.<event>
message WorktreeLifecycleEvent {
  string event_id = 1;          // consumer-side dedup key
  string tenant_id = 2;
  string occurred_at = 3;       // RFC3339
  int32 schema_version = 4;     // starts at 1
  string worktree_id = 5;
  string project_id = 6;
  string linked_issue_provider = 7;  // empty = no linked issue, consumer no-ops
  string linked_issue_ref = 8;
  bool had_open_pr = 9;              // worktree.deleted only — BL-PI-03's "no PR" branch of the mapping table
}
```

### `scmintegration.proto` — event payloads + webhook receiver

```protobuf
// Subject: orca.scm.pull_request.created / orca.scm.pull_request.merged
message PullRequestLifecycleEvent {
  string event_id = 1;
  string tenant_id = 2;
  string occurred_at = 3;
  int32 schema_version = 4;
  string provider = 5;
  string repo = 6;
  int32 pr_number = 7;
  string linked_issue_provider = 8;  // resolved from the PR body's closing-keyword reference ("Fixes #123") or Orca's own worktree->issue link, whichever produced the PR
  string linked_issue_ref = 9;
}

// Webhook ingestion — plain HTTP, fronted by api-gateway at
// /v1/scm/webhooks/{provider}, forwarded to this RPC. Not a gRPC-first
// design because the caller is GitHub/GitLab's servers, which cannot be
// given Orca credentials — see rationale above.
rpc ReceiveWebhook(ReceiveWebhookRequest) returns (ReceiveWebhookResponse);

message ReceiveWebhookRequest {
  string provider = 1;
  bytes raw_body = 2;         // signature verified against this exact byte sequence
  string signature_header = 3;
  string delivery_id_header = 4;  // GitHub X-GitHub-Delivery / GitLab X-Gitlab-Event-UUID — dedup key for webhook_delivery_log
}
message ReceiveWebhookResponse {
  bool accepted = 1;
  bool duplicate = 2;  // true if delivery_id_header already in webhook_delivery_log
}
```

---

## Design — `project-service` outbox wiring

```go
// internal/usecase/record_worktree_created.go — extended
func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
    wt := domain.Worktree{ /* ... existing fields ..., */
        LinkedIssueProvider: in.LinkedIssueProvider, LinkedIssueRef: in.LinkedIssueRef,
    }
    event := lifecycleEvent(wt, "orca.project.worktree.created", tenantID)
    // Single Postgres transaction: INSERT worktrees row + INSERT outbox_events row.
    // See adapter/postgres/worktree_repository.go — same pattern as
    // usage-service's SaveSession(ctx, session, event) call, which passes
    // both the domain write and the event into one repository method
    // specifically so they share one *sql.Tx (record_usage_session.go:95).
    if err := uc.repo.CreateWorktreeWithEvent(ctx, wt, event); err != nil {
        return domain.Worktree{}, err
    }
    return wt, nil
}
```

`record_worktree_removed.go` (new, since BUG-PI-03 confirms no removal
usecase publishes anything today) follows the identical shape, additionally
resolving `had_open_pr` via a synchronous check — **not** an extra
`scm-integration-service` call from inside this transaction (that would
reach across a service boundary from inside a DB transaction, which
`05-data-architecture.md` never sanctions); instead `had_open_pr` is
resolved by the **consumer** at processing time, from the SCM's live PR
list for that branch (`GetPullRequestForBranch`, already a real RPC per
BUG-PI-01's references) — `project-service` always publishes
`had_open_pr: false` (the field is consumer-populated context, not
publisher-authoritative) — clarified in the message field's comment above.

The `outbox_events` table and its `common/outbox.Relay` wiring in
`project-service`'s `cmd/server/main.go` are a direct copy of
`issue-tracking-service`'s existing setup (`outbox.go:81-105`,
`NewRelay(store, pub, cfg, logger)`), reused verbatim — no new relay
mechanism, just a second table this service's `main.go` now feeds it from.

---

## Design — `scm-integration-service` outbox + webhook wiring

```go
// internal/usecase/create_pull_request.go — extended
func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (domain.PullRequest, error) {
    pr, err := uc.providers.For(in.Provider).CreatePullRequest(ctx, cred, repo, input)
    if err != nil {
        return domain.PullRequest{}, err
    }
    // Best-effort: PR creation already succeeded provider-side; a failed
    // enqueue here must not turn a successful CreatePullRequest into an
    // error response (BR-PI-09's non-blocking posture applies even to the
    // publisher side, not just the consumer side).
    if err := uc.outbox.Enqueue(ctx, in.TenantID, prCreatedEvent(pr, in.LinkedIssueRef)); err != nil {
        uc.logger.WarnContext(ctx, "failed to enqueue pr.created event", "error", err, "pr", pr.ID)
    }
    return pr, nil
}
```

`MergePullRequest` gets the identical addition for the Orca-initiated
merge path. `ReceiveWebhook` covers the externally-merged path:

```go
// internal/usecase/receive_webhook.go
func (uc *ReceiveWebhook) Execute(ctx context.Context, in ReceiveWebhookInput) (ReceiveWebhookOutput, error) {
    if !uc.verifier.Verify(in.Provider, in.RawBody, in.SignatureHeader) {
        return ReceiveWebhookOutput{}, apperrors.New(apperrors.KindPermissionDenied, "SCM_WEBHOOK_BAD_SIGNATURE", "signature verification failed", nil)
    }
    if seen, err := uc.deliveries.Exists(ctx, in.Provider, in.DeliveryIDHeader); err == nil && seen {
        return ReceiveWebhookOutput{Accepted: true, Duplicate: true}, nil // idempotent per webhook_delivery_log's own purpose
    }
    parsed, ok := parseMergeEvent(in.Provider, in.RawBody) // only "PR/MR merged" events are relevant to this flow; others recorded but not published
    if err := uc.deliveries.Record(ctx, in.Provider, in.DeliveryIDHeader, "processed"); err != nil {
        return ReceiveWebhookOutput{}, err
    }
    if ok {
        if err := uc.outbox.Enqueue(ctx, parsed.TenantID, prMergedEvent(parsed)); err != nil {
            uc.logger.WarnContext(ctx, "failed to enqueue webhook-sourced pr.merged event", "error", err)
        }
    }
    return ReceiveWebhookOutput{Accepted: true}, nil
}
```

`webhook_delivery_log`'s existing schema (`scm-integration-service.md:155`)
already has the `(provider, external_event_id)` uniqueness this dedup
needs — no schema change to that table, only its first writer.

---

## Design — `issue-status-sync` service (new)

```
issue-status-sync/
├── cmd/server/main.go
├── internal/
│   ├── domain/          # WorktreeLifecycleEvent, PullRequestLifecycleEvent, ProjectMapping
│   ├── usecase/
│   │   ├── ports.go     # IssueTrackerClient, ScmClient, ProjectSettingsClient, ProcessedEventStore
│   │   └── sync_issue_status.go
│   ├── adapter/
│   │   ├── eventbus/    # NATS JetStream durable subscriber, both subjects
│   │   ├── postgres/    # processed_events table only
│   │   └── grpcclient/  # issue-tracking-service, scm-integration-service, project-service clients
│   └── config/
├── migrations/           # 0001_processed_events.up.sql
└── go.mod
```

```go
// internal/usecase/sync_issue_status.go
// BR-PI-08: retry up to 3 times on API failure before giving up.
// BR-PI-09: never block the main workflow — this whole usecase runs from
// an async consumer loop, never from a synchronous RPC path; a give-up
// here logs and moves on, it never propagates back to the worktree/PR
// operation that triggered the event.
func (uc *SyncIssueStatus) HandleWorktreeLifecycle(ctx context.Context, ev domain.WorktreeLifecycleEvent) error {
    if processed, err := uc.processedEvents.Seen(ctx, ev.EventID); err == nil && processed {
        return nil // JetStream at-least-once — 08-inter-service-communication.md's idempotency rule
    }
    if ev.LinkedIssueProvider == "" {
        return uc.processedEvents.MarkSeen(ctx, ev.EventID) // no linked issue, nothing to sync
    }
    if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, ev.ProjectID); err != nil || !enabled {
        // BR-PI-07 — belt-and-braces re-check: the publisher (project-service,
        // SOL-PI-02) already gates recording the link at all when sync is
        // off, but re-checking here covers the case where a project's flag
        // is flipped off AFTER the link was recorded but BEFORE this event
        // is processed (JetStream delivery is not instantaneous).
        return uc.processedEvents.MarkSeen(ctx, ev.EventID)
    }

    targetState := mapWorktreeEventToStatus(ev) // BL-PI-03's mapping table: created->InProgress, deleted&&!hadOpenPR->Cancelled

    err := retry.Do(ctx, 3, func(ctx context.Context) error { // BR-PI-08
        return uc.updateIssueStatus(ctx, ev.LinkedIssueProvider, ev.LinkedIssueRef, targetState)
    })
    if err != nil {
        uc.logger.ErrorContext(ctx, "gave up syncing issue status after 3 retries", "issue", ev.LinkedIssueRef, "error", err)
        // Give up, per BR-PI-08's literal words — no dead-letter queue
        // specified by the business rule; JetStream's own redelivery
        // window is the safety net, not an app-level DLQ this solution invents.
    }
    return uc.processedEvents.MarkSeen(ctx, ev.EventID) // mark seen regardless of sync outcome — retried the API call, not the event
}

func (uc *SyncIssueStatus) updateIssueStatus(ctx context.Context, provider, ref string, state domain.TargetState) error {
    switch provider {
    case "linear", "jira":
        return uc.tracker.TransitionIssue(ctx, provider, ref, state.TrackerState) // issue-tracking-service.TransitionIssue, scmintegration-service.md:80
    case "github":
        return uc.scm.UpdateIssue(ctx, provider, ref, state.GitHubLabelPatch) // add/remove "in-progress" label, or CloseIssue for Done
    default:
        return apperrors.New(apperrors.KindInvalidArgument, "ISSUE_STATUS_SYNC_UNKNOWN_PROVIDER", "gitlab issue status sync via labels not yet mapped", nil)
    }
}
```

`HandlePullRequestLifecycle` mirrors the same shape for `pr.created` →
"In Review" / linked PR, `pr.merged` → "Done" / closed.

### `processed_events` table

```sql
CREATE TABLE processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- short-TTL cleanup job (e.g. 7-day retention) — this is a dedup cache,
-- not an audit log; 08-inter-service-communication.md:42-45 only
-- requires "a short-TTL cache," not permanent retention.
```

---

## Test plan

- `project-service`: `CreateWorktreeWithEvent` integration test (real
  `testcontainers-go` Postgres, per `05-data-architecture.md`'s migration
  CI convention) — asserts the worktree row and the outbox row commit or
  roll back together (kill the transaction mid-way, assert neither row
  persists).
- `record_worktree_created_test.go` / `record_worktree_removed_test.go`:
  event payload has the right subject, `linked_issue_ref` empty when the
  request didn't carry one.
- `scm-integration-service`: `create_pull_request_test.go` — outbox
  enqueue failure does not fail `Execute`'s return (BR-PI-09 on the
  publisher side); assert `pr` is still returned successfully.
- `receive_webhook_test.go`: bad signature rejected before any dedup
  check; duplicate `delivery_id_header` returns `Duplicate: true` without
  a second outbox enqueue; a non-merge event (e.g. `pull_request.opened`)
  is recorded to `webhook_delivery_log` but does not enqueue any event.
- `issue-status-sync/sync_issue_status_test.go`:
  - duplicate `event_id` (already in `processed_events`) is a no-op,
    asserted via zero calls to `tracker`/`scm` fakes.
  - `LinkedIssueProvider == ""` marks seen without calling any provider.
  - `IsIssueStatusSyncEnabled == false` marks seen without calling any
    provider — regression guard for the "flag flipped mid-flight" race
    this design explicitly re-checks for.
  - retry: fake provider client fails twice then succeeds — assert 3
    total attempts, success recorded.
  - give-up: fake provider client always fails — assert exactly 3
    attempts (not 4, not unbounded), event still marked seen, no error
    propagated out of `HandleWorktreeLifecycle` (BR-PI-09).
  - all four mapping-table rows (`worktree.created`→In Progress,
    `pr.created`→In Review, `pr.merged`→Done, `worktree.deleted && !had_open_pr`→Cancelled)
    resolve to the correct target state — table-driven test mirroring
    BL-PI-03's own table verbatim.
- End-to-end (docker-compose or `testcontainers-go` NATS): publish a real
  `worktree.created` event from `project-service`'s outbox relay, assert
  `issue-status-sync` consumes it and calls the fake Linear/GitHub client
  within the test's timeout — validates the whole JetStream wiring, not
  just each service's unit boundary.

## Agent (`agent/`) impact

**None.** Every event source in this design (`RecordWorktreeCreated`/
`RecordWorktreeRemoved`, `CreatePullRequest`/`MergePullRequest`, and the
new webhook receiver) is already backend-go-internal or externally sourced
from GitHub/GitLab's own servers — none of it touches the Dev Server Agent
or its relay protocol.

## References

- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:5-9` (channel-selection table, BR-PI-09's exact justification for NATS over gRPC), `:30-45` (event conventions: subject naming, outbox-only publishing, idempotent consumers), `:47-70` (API Gateway responsibilities — webhook HTTP entry point's home)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98` (transactional outbox pattern, worked example this design mirrors), `:100-112` (synchronous saga — explicitly the pattern *not* used here)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:28-36` (design principles 3 & 4, applied to rule out folding the consumer into an existing service), `:72` ("Total: 17 services" — this solution's flagged 18th)
- `specs/backend-go/tdd/services/project-service.md:36` (worktree metadata ownership), `:97-98` (`RecordWorktreeCreated`/`Removed` semantics), `:289` (existing bare `eventbus:` package-layout placeholder this solution fills in)
- `specs/backend-go/tdd/services/scm-integration-service.md:146-160` (§5 data model, `webhook_delivery_log`'s existing-but-unused schema), `:194-210` (§7 dependencies — no `issue-tracking-service` edge, supporting the "consumer doesn't belong here" argument)
- `specs/backend-go/tdd/services/issue-tracking-service.md:227-241` (§7 dependencies, same argument for the other side), `:236-238` (`orca.issuetracking.link.created` outbox precedent)
- `backend-go/services/issue-tracking-service/internal/usecase/link_issue.go:1-85` — the concrete outbox-usecase template this design's `pr.created`/`pr.merged` publishing follows, including its "outbox-only database" precedent (comment, lines 39-46)
- `backend-go/common/outbox/outbox.go` — `Relay`/`Store`/`Config` reused verbatim, no new relay mechanism
- `backend-go/services/usage-service/internal/usecase/record_usage_session.go:89-95` — the "domain write + event share one `*sql.Tx`" precedent `CreateWorktreeWithEvent` follows
- `docs/logic/project-integration/BL-PI-03-update-issue-status.md:21-38` — event→status mapping table and BR-PI-07/08/09 verbatim
- `specs/backend-go/bugs/logic-v1/BUG-PI-03-issue-status-sync-not-implemented.md` — problem statement and all "what's missing" findings this solution addresses
- `specs/backend-go/bugs/logic-v1/BUG-PI-02-worktree-from-issue-not-implemented.md:20` — confirms `LinkIssue` links to `task_id`, not `worktree_id`; this solution's `linked_issue_ref` on `Worktree` is the field that closes that specific gap
