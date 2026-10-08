package domain

import "time"

// PhaseStart is the idempotency record of "start this Phase".
type PhaseStart struct {
	TenantID    string
	PhaseTaskID string
	RequestID   string
	StartedBy   string
	StartedAt   time.Time
}
