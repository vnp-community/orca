package domain

import "time"

// OutboxEvent is a pre-built event a usecase asks its repository to
// durably enqueue in the SAME transaction as its domain write — the
// transactional-outbox pattern (05-data-architecture.md). Lives in
// domain/, not common/eventbus.Event, so usecase/ can build one without
// importing anything NATS-specific.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
