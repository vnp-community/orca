package domain

import "time"

// OutboxEvent is one durably-committed row awaiting publish to NATS —
// mirrors usage-service's own domain.OutboxEvent shape (this codebase's
// database-per-service rule means each service owns its own copy, not a
// shared package — see common/outbox.Store's doc comment). TenantID is
// carried directly here (unlike usage-service's, where it comes from the
// session being saved alongside it) since this service's first real outbox
// event (dev-server-disconnected) has no other domain object to pull it
// from at the enqueue site.
type OutboxEvent struct {
	ID          string
	TenantID    string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
