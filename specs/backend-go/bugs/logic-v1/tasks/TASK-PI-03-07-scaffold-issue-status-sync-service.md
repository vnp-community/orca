# TASK-PI-03-07: Scaffold the new `issue-status-sync` service (consumer + retry logic)

**From Solution:** SOL-PI-03
**Priority:** P0 — but see the sign-off note below before starting
**Service:** `issue-status-sync` (NEW — 18th service)
**File:** `backend-go/services/issue-status-sync/cmd/server/main.go` (new), `internal/domain/` (new), `internal/usecase/ports.go` (new), `internal/usecase/sync_issue_status.go` (new), `internal/adapter/eventbus/subscriber.go` (new), `internal/adapter/postgres/processed_events.go` (new), `internal/adapter/grpcclient/` (new), `migrations/0001_processed_events.up.sql` (new), `go.mod` (new)
**Depends on:** TASK-PI-03-03, TASK-PI-03-05, TASK-PI-02-02, TASK-PI-02-06
**Status:** `[ ]` TODO

---

## ⚠️ Needs sign-off before implementation

SOL-PI-03 flags this explicitly: `02-microservices-decomposition.md:72`
states "Total: 17 services" — this is a genuine 18th, beyond what that
catalog names today. The alternative (a background NATS-consumer package
inside `api-gateway`) was evaluated and rejected in the solution's own
rationale (§"Where the consumer lives") as a smaller but still real
deviation from `08-inter-service-communication.md`'s API-Gateway-is-sync-edge-only
framing. **Confirm with whoever owns the service catalog before starting
this task** — the schema, consumer wiring, and retry logic below are written
out in full so implementation can start immediately once approved, not to
imply approval already happened.

## Context

No existing service is a correct home for this fan-in consumer (see
SOL-PI-03's rationale: not `project-service`, not
`scm-integration-service`/`issue-tracking-service` individually, not
`orchestration-service`). It consumes `worktree.created`/`worktree.deleted`
(TASK-PI-03-03) and `pr.created`/`pr.merged` (TASK-PI-03-05), resolves which
issue-tracker or SCM owns the linked issue, and calls the right
`UpdateIssue`/`TransitionIssue` RPC with BR-PI-08's retry-3-then-give-up
policy — never blocking the publishers (BR-PI-09).

## Changes to make

### 1. `migrations/0001_processed_events.up.sql`

```sql
CREATE SCHEMA IF NOT EXISTS issuestatussync;

CREATE TABLE issuestatussync.processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Dedup cache only (08-inter-service-communication.md:42-45), not an audit
-- log — a short-TTL cleanup job (e.g. 7-day retention) should be added as a
-- follow-up, not required for this task.
```

### 2. `internal/domain/events.go` — mirror the two publishers' payload shapes

```go
package domain

type WorktreeLifecycleEvent struct {
	EventID             string
	TenantID            string
	OccurredAt          string
	SchemaVersion       int32
	WorktreeID          string
	ProjectID           string
	LinkedIssueProvider string
	LinkedIssueRef      string
	HadOpenPR           bool
}

type PullRequestLifecycleEvent struct {
	EventID             string
	TenantID            string
	OccurredAt          string
	SchemaVersion       int32
	Provider            string
	Repo                string
	PRNumber            int32
	LinkedIssueProvider string
	LinkedIssueRef      string
}

type TargetState struct {
	TrackerState     string // Jira/Linear transition name
	GitHubLabelPatch string // add/remove label, or "close" for Done
}
```

### 3. `internal/usecase/ports.go`

```go
package usecase

import "context"

type IssueTrackerClient interface {
	TransitionIssue(ctx context.Context, provider, ref string, state string) error
}
type ScmClient interface {
	UpdateIssue(ctx context.Context, provider, ref, labelPatch string) error
	// GetPullRequestForBranch resolves had_open_pr at processing time — see
	// record_worktree_removed.go's doc comment (TASK-PI-03-03) for why the
	// publisher never resolves this itself.
	GetPullRequestForBranch(ctx context.Context, repo, branch string) (found bool, err error)
}
type ProjectSettingsClient interface {
	IsIssueStatusSyncEnabled(ctx context.Context, projectID string) (bool, error)
}
type ProcessedEventStore interface {
	Seen(ctx context.Context, eventID string) (bool, error)
	MarkSeen(ctx context.Context, eventID string) error
}
```

### 4. `internal/usecase/sync_issue_status.go`

```go
package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

type SyncIssueStatus struct {
	tracker         IssueTrackerClient
	scm             ScmClient
	projects        ProjectSettingsClient
	processedEvents ProcessedEventStore
	logger          *slog.Logger
}

func NewSyncIssueStatus(tracker IssueTrackerClient, scm ScmClient, projects ProjectSettingsClient, processedEvents ProcessedEventStore, logger *slog.Logger) *SyncIssueStatus {
	return &SyncIssueStatus{tracker: tracker, scm: scm, projects: projects, processedEvents: processedEvents, logger: logger}
}

// HandleWorktreeLifecycle implements BR-PI-08/BR-PI-09. Runs only from the
// async consumer loop (adapter/eventbus/subscriber.go) — never from a
// synchronous RPC path, so a give-up here never propagates back to the
// worktree/PR operation that triggered the event.
func (uc *SyncIssueStatus) HandleWorktreeLifecycle(ctx context.Context, ev domain.WorktreeLifecycleEvent) error {
	if processed, err := uc.processedEvents.Seen(ctx, ev.EventID); err == nil && processed {
		return nil // JetStream at-least-once — idempotent no-op
	}
	if ev.LinkedIssueProvider == "" {
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}
	if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, ev.ProjectID); err != nil || !enabled {
		// BR-PI-07 belt-and-braces re-check: the publisher already gates
		// recording the link when sync is off, but a project's flag can
		// flip off AFTER the link was recorded and BEFORE this event is
		// processed (JetStream delivery is not instantaneous).
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}

	target := mapWorktreeEventToStatus(ev) // BL-PI-03's mapping table

	err := doWithRetry(ctx, 3, func(ctx context.Context) error { // BR-PI-08
		return uc.updateIssueStatus(ctx, ev.LinkedIssueProvider, ev.LinkedIssueRef, target)
	})
	if err != nil {
		uc.logger.ErrorContext(ctx, "gave up syncing issue status after 3 retries", "issue", ev.LinkedIssueRef, "error", err)
		// Give up per BR-PI-08's literal words — no app-level DLQ; JetStream's
		// own redelivery window is the safety net.
	}
	return uc.processedEvents.MarkSeen(ctx, ev.EventID) // mark seen regardless of sync outcome
}

// HandlePullRequestLifecycle mirrors the same shape for pr.created -> "In
// Review" / pr.merged -> "Done".
func (uc *SyncIssueStatus) HandlePullRequestLifecycle(ctx context.Context, ev domain.PullRequestLifecycleEvent) error {
	// identical dedup/opt-out/retry/mark-seen shape as above, target state
	// from mapPullRequestEventToStatus(ev)
	return nil
}

func (uc *SyncIssueStatus) updateIssueStatus(ctx context.Context, provider, ref string, state domain.TargetState) error {
	switch provider {
	case "linear", "jira":
		return uc.tracker.TransitionIssue(ctx, provider, ref, state.TrackerState)
	case "github":
		return uc.scm.UpdateIssue(ctx, provider, ref, state.GitHubLabelPatch)
	default:
		return errUnknownProvider // gitlab issue status sync via labels not yet mapped
	}
}
```

`mapWorktreeEventToStatus`/`mapPullRequestEventToStatus` implement
BL-PI-03's mapping table exactly: `worktree.created`->In Progress,
`worktree.deleted && !had_open_pr`->Cancelled, `pr.created`->In Review,
`pr.merged`->Done. `doWithRetry` is a small local retry helper (exponential
or fixed backoff between attempts, exactly 3 attempts total).

### 5. `internal/adapter/eventbus/subscriber.go`

Subscribe (durable, per `common/eventbus.Consumer.Subscribe`) to both
`orca.project.worktree.created`/`orca.project.worktree.deleted` and
`orca.scm.pull_request.created`/`orca.scm.pull_request.merged`, decoding
each `eventbus.Event.Payload` into the matching `domain.*LifecycleEvent`
and dispatching to `SyncIssueStatus`.

### 6. `internal/adapter/grpcclient/` — thin clients

`IssueTrackerClient`/`ScmClient`/`ProjectSettingsClient` implementations
calling `issue-tracking-service`, `scm-integration-service`,
`project-service` respectively — same `withTenantMetadata`-style pattern
every other `grpcclient` package in this codebase already uses.

### 7. `cmd/server/main.go`

Composition root: DB pool, `NewProcessedEventsStore`, the three grpc
clients, `eventbus.Connect`, `Subscriber.Run` in goroutines for both event
streams, graceful shutdown — follow `issue-tracking-service/cmd/server/main.go`'s
overall shape (minus its own outbox relay, since this service only
consumes).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-status-sync/...
go vet ./services/issue-status-sync/...
```
