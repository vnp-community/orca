package domain

import "time"

// OutboxEvent is a pre-built event CreatePullRequest/MergePullRequest ask
// their OutboxEnqueuer to durably enqueue — the transactional-outbox
// pattern from specs/backend-go/architecture/05-data-architecture.md,
// mirrored verbatim from issue-tracking-service's own domain.OutboxEvent
// (SOL-PI-03). Lives in domain/, not common/eventbus.Event, so usecase/
// can build one without importing anything NATS-specific.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
