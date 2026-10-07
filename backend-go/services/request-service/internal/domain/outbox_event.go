package domain

import "time"

type OutboxEvent struct {
	ID         string
	TenantID   string
	Subject    string
	OccurredAt time.Time
	Version    int
	Payload    []byte
}
