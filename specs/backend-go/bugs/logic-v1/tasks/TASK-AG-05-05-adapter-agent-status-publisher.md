# TASK-AG-05-05: `AgentStatusEventPublisher` adapter — direct publish for `statusChanged`, outbox for `rateLimited`

**From Solution:** SOL-AG-05
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go` (new), `backend-go/services/infra-fleet-service/migrations/0009_agent_status_outbox.up.sql` (new)
**Depends on:** TASK-AG-05-03
**Status:** `[ ]` TODO — implements a deliberate, flagged deviation from `08-inter-service-communication.md`'s "publishing always goes through the outbox" rule; needs explicit sign-off (see Context), not silent application

---

## Context

BR-AG-14 targets <500ms detect-to-update for `agent.statusChanged`; the outbox relay's polling cadence (`common/outbox.Config.PollInterval`, see `tenant-service`'s and `usage-service`'s existing relays) eats into that budget for a high-frequency, low-durability-value event — a lost status update just gets re-derived from the next PTY chunk, unlike a business fact like `task.completed`. This task therefore publishes `agent.statusChanged` **directly** (mirroring `tenant-service/internal/adapter/eventbus/publisher.go`'s already-established "best-effort, not outbox-backed" precedent for exactly this kind of event) while still routing `agent:rateLimited` through the outbox (`usage-service`'s pattern) since that one is a real alerting event worth at-least-once delivery. **This is a deliberate exception, not an oversight** — `08-inter-service-communication.md` states outbox-always as a hard rule; get sign-off before merging this task, same as any other documented-rule deviation.

## Changes to make

Create `backend-go/services/infra-fleet-service/migrations/0009_agent_status_outbox.up.sql` (rate-limited events only — `statusChanged` never touches the DB):

```sql
CREATE TABLE infra.agent_rate_limited_outbox_events (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_infra_agent_rate_limited_outbox_unpublished
    ON infra.agent_rate_limited_outbox_events (created_at)
    WHERE published_at IS NULL;
```
(and the matching `.down.sql` dropping the table).

Create `backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go`:

```go
// Package eventbus — agent_status_publisher.go implements
// usecase.AgentStatusEventPublisher. statusChanged publishes DIRECTLY
// (bypassing the outbox — see TASK-AG-05-05's Context for why this is a
// deliberate, signed-off exception to 08-inter-service-communication.md's
// outbox-always rule); rateLimited goes through the outbox, same pattern
// usage-service's RecordUsageSession/Repository.SaveSession already
// establishes for a real alerting event.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	statusChangedSubject = "orca.infra.agent.statusChanged"
	rateLimitedSubject    = "orca.infra.agent.rateLimited"
)

type statusChangedPayload struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

type rateLimitedPayload struct {
	SessionID string `json:"session_id"`
}

// rateLimitedOutboxEnqueuer is the one method this package needs from
// postgres.AgentRateLimitedOutboxStore — a local interface (rather than
// importing internal/adapter/postgres directly) keeps this adapter package
// decoupled from the specific store implementation, same reasoning
// usecase/ports.go's port interfaces already apply one layer up.
type rateLimitedOutboxEnqueuer interface {
	Enqueue(ctx context.Context, rec outbox.Record) error
}

// AgentStatusPublisher implements usecase.AgentStatusEventPublisher.
type AgentStatusPublisher struct {
	pub   *commoneventbus.Publisher
	store rateLimitedOutboxEnqueuer // rateLimited only — statusChanged never touches it
}

func New(pub *commoneventbus.Publisher, store rateLimitedOutboxEnqueuer) *AgentStatusPublisher {
	return &AgentStatusPublisher{pub: pub, store: store}
}

func (p *AgentStatusPublisher) PublishStatusChanged(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus) error {
	payload, err := json.Marshal(statusChangedPayload{SessionID: sessionID, Status: string(status)})
	if err != nil {
		return fmt.Errorf("eventbus: marshal statusChanged payload: %w", err)
	}
	return p.pub.Publish(ctx, statusChangedSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}

// PublishRateLimited enqueues into the outbox rather than publishing
// directly — see this file's package doc comment.
func (p *AgentStatusPublisher) PublishRateLimited(ctx context.Context, tenantID, sessionID string) error {
	payload, err := json.Marshal(rateLimitedPayload{SessionID: sessionID})
	if err != nil {
		return fmt.Errorf("eventbus: marshal rateLimited payload: %w", err)
	}
	return p.store.Enqueue(ctx, outbox.Record{
		ID:      uuid.NewString(),
		Subject: rateLimitedSubject,
		Event: commoneventbus.Event{
			TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
		},
	})
}
```

Add `OutboxStore` (implements `common/outbox.Store`, following
`usage-service`'s `Repository.FetchUnpublished`/`MarkPublished`/enqueue
pattern exactly) in
`internal/adapter/postgres/agent_rate_limited_outbox_repository.go`:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
)

// AgentRateLimitedOutboxStore implements common/outbox.Store against
// infra.agent_rate_limited_outbox_events (TASK-AG-05-05's migration).
type AgentRateLimitedOutboxStore struct {
	pool *pgxpool.Pool
}

func NewAgentRateLimitedOutboxStore(pool *pgxpool.Pool) *AgentRateLimitedOutboxStore {
	return &AgentRateLimitedOutboxStore{pool: pool}
}

func (s *AgentRateLimitedOutboxStore) Enqueue(ctx context.Context, rec outbox.Record) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.agent_rate_limited_outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, rec.ID, rec.Event.TenantID, rec.Subject, rec.Event.OccurredAt, rec.Event.Payload)
	if err != nil {
		return fmt.Errorf("postgres: insert agent rate-limited outbox event: %w", err)
	}
	return nil
}

func (s *AgentRateLimitedOutboxStore) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM infra.agent_rate_limited_outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished agent rate-limited outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan agent rate-limited outbox row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *AgentRateLimitedOutboxStore) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE infra.agent_rate_limited_outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark agent rate-limited outbox events published: %w", err)
	}
	return nil
}
```

Wire `outbox.NewRelay(agentRateLimitedOutboxStore, pub, ...)` (matching
`usage-service`'s `cmd/server/main.go` relay-startup call site) into
`infra-fleet-service`'s `cmd/server/main.go` in TASK-AG-05-06.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/eventbus/... -run TestAgentStatusPublisher -v
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestAgentRateLimitedOutboxStore -v
```
