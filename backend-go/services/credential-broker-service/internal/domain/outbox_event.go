package domain

import "time"

// OutboxEvent is a domain event to be published after the transaction that
// produced it commits. Payload is JSON and must never contain secret material.
type OutboxEvent struct {
	ID         string
	TenantID   string
	Subject    string
	OccurredAt time.Time
	Payload    []byte
}
