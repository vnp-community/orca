package domain

import "time"

// OutboxEvent is a pre-built event RecordWorktreeCreated/RecordWorktreeRemoved
// ask WorktreeRepository to durably enqueue in the SAME transaction as the
// worktrees write — the transactional-outbox pattern from
// specs/backend-go/architecture/05-data-architecture.md (SOL-PI-03).
// Mirrors issue-tracking-service's own domain.OutboxEvent shape. Lives in
// domain/, not common/eventbus.Event, so usecase/ can build one without
// importing anything NATS-specific.
type OutboxEvent struct {
	ID          string
	TenantID    string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
