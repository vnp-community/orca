package domain

import (
	"encoding/json"
	"time"
)

// OutboxEvent is a pre-built event a usecase asks its repository to persist
// in the same transaction as the domain write it describes — see
// usage-service's identical OutboxEvent for the precedent this mirrors.
// UpdateExecution uses this for execution-level terminal events
// (SOL-PW-04/TASK-PW-04-06); UpdateStepExecution reuses the same shape for
// step-level events (BE-SOL-003/TASK-FT-003-03). A zero-value OutboxEvent
// (ID == "") tells the repository there is nothing to enqueue this call.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON json.RawMessage
}
