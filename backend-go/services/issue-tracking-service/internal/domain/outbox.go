package domain

import "time"

// OutboxEvent is a pre-built event LinkIssue asks its Repository to
// durably enqueue — the transactional-outbox pattern from
// specs/backend-go/architecture/05-data-architecture.md, closing the gap
// this service's previous direct-NATS-publish-or-fail-the-RPC approach
// accepted (docs/execution-plan.md Epic G). Lives in domain/, not
// common/eventbus.Event, so usecase/ can build one without importing
// anything NATS-specific — internal/adapter/postgres turns it into an
// outbox_events row; common/outbox.Relay (fed by that table) turns the row
// back into a real eventbus.Event when it publishes.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
