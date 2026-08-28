package domain

import "time"

// OutboxEvent is a pre-built event a usecase asks its ConnectionRepository
// to durably enqueue in the SAME transaction as the domain write it
// accompanies — the transactional-outbox pattern from
// specs/backend-go/architecture/05-data-architecture.md, mirroring
// usage-service's internal/domain/outbox.go (this service's first outbox
// consumer, TASK-AUTH-05-08's ssh.connect event). Lives in domain/, not
// common/eventbus.Event, so usecase/ can build one without importing
// anything NATS-specific — internal/adapter/postgres turns it into an
// infra.outbox_events row; common/outbox.Relay (fed by that table) turns
// the row back into a real eventbus.Event when it publishes.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
