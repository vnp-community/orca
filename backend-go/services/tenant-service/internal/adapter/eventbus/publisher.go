// Package eventbus implements tenant-service's cross-replica profile-cache
// invalidation broadcast against NATS JetStream via common/eventbus — see
// docs/execution-plan.md Epic F and this service's README "Known gaps" for
// why this exists: internal/adapter/cache.LRUTTLCache is an in-process
// cache, one instance per replica. A mutating usecase already invalidates
// the entry on the replica that served the write; the remaining gap was
// that every OTHER replica's copy of the same entry stayed correct only by
// waiting out its TTL, not by being told. Publishing this event is
// best-effort, not outbox-backed: a missed publish just means a peer
// replica falls back to today's TTL-bounded staleness, never wrong data —
// this doesn't need the stronger at-least-once guarantee a
// domain-of-record event would.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// Subject is the event subject SetUserDepartment/AddTeamMember publish to
// after invalidating their own replica's cache entry. Every tenant-service
// replica (including the publisher) also subscribes to this subject — see
// consumer.go — so a resolved-profile cache stays correct cluster-wide
// within event-delivery latency, not just within
// usecase.DefaultProfileCacheTTL.
const Subject = "orca.tenant.profile.invalidated"

type invalidatedPayload struct {
	UserID string `json:"user_id"`
}

// Publisher implements usecase.CacheInvalidationPublisher.
type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

func (p *Publisher) PublishProfileInvalidated(ctx context.Context, tenantID, userID string) error {
	payload, err := json.Marshal(invalidatedPayload{UserID: userID})
	if err != nil {
		return fmt.Errorf("eventbus: marshal payload: %w", err)
	}
	return p.pub.Publish(ctx, Subject, commoneventbus.Event{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    payload,
	})
}
