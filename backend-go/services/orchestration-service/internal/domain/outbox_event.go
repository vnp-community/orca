package domain

import "time"

// OutboxEvent is a pre-built event a usecase asks its repository to
// durably enqueue in the SAME transaction as the domain write it
// accompanies — see TASK-FT-003-01's Context for why this follows
// usage-service's SaveSession(ctx, session, event) pattern rather than
// exposing a pgx.Tx to the usecase layer. A zero-value OutboxEvent (ID
// == "") tells the repository there is nothing to enqueue this call.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
