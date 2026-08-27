package domain

import "time"

// OutboxEvent is a pre-built event EventPublisher asks the postgres adapter
// to durably enqueue — the transactional-outbox pattern from
// specs/backend-go/architecture/05-data-architecture.md, mirroring
// usage-service/internal/domain/outbox.go's identical shape. Lives in
// domain/, not common/eventbus.Event, so usecase/ can build one without
// importing anything NATS-specific — internal/adapter/postgres turns it
// into an outbox_events row; common/outbox.Relay (fed by that table) turns
// the row back into a real eventbus.Event when it publishes.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
