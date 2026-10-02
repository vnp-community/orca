package domain

import "time"

// OutboxRecord is an event a usecase asks its repository to enqueue in the
// same transaction as the state change (transactional outbox). Kept free of
// NATS types; common/outbox.Relay turns the stored row into a bus event.
type OutboxRecord struct {
	ID          string
	Subject     string // orca.mcp.<entity>.<event>
	OccurredAt  time.Time
	Version     int
	PayloadJSON []byte
}
