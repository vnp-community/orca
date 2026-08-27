package domain

import (
	"encoding/json"
	"time"
)

// OutboxEvent is a pre-built event UpdateTask asks its repository to
// persist in the same transaction as the task row it describes — see
// usage-service's identical OutboxEvent for the precedent this mirrors.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON json.RawMessage
}
