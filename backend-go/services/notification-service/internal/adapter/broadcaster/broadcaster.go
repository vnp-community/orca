// Package broadcaster implements usecase.NotificationBroadcaster as a
// real, working in-process fan-out registry keyed by tenant+user — the WS
// delivery mechanism api-gateway's StreamNotifications gRPC call fans out
// to (notification-service.md §7: "api-gateway is the gRPC client... this
// service never dials api-gateway; it only serves the stream api-gateway
// opened").
//
// Known scaling limitation: this registry lives entirely in one process's
// memory. It fans out correctly to every StreamNotifications subscriber
// connected to THIS replica, but a subscriber connected to a different
// replica of a horizontally-scaled notification-service never sees the
// event. Solving that needs either sticky routing at api-gateway (same
// user's stream always lands on the same replica) or a distributed
// fan-out mechanism (e.g. republishing to a NATS subject this registry
// also subscribes to) — tracked in this service's README, not solved here.
package broadcaster

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// subscriberKey scopes a subscriber registry entry to one tenant's one
// user — never just user_id, since user IDs are only unique within a
// tenant (see architecture/05-data-architecture.md's tenant-isolation rule).
type subscriberKey struct {
	tenantID string
	userID   string
}

// channelBufferSize is generous enough that a StreamNotifications reader
// briefly falling behind the sender (a slow network write to the browser,
// not a hung consumer) doesn't cause Broadcast to drop its notification.
const channelBufferSize = 16

// Broadcaster is a real, working channel-based fan-out registry — not a
// stub. It is safe for concurrent use.
type Broadcaster struct {
	mu   sync.Mutex
	subs map[subscriberKey]map[chan domain.NotificationEvent]struct{}
}

// New returns an empty Broadcaster.
func New() *Broadcaster {
	return &Broadcaster{subs: make(map[subscriberKey]map[chan domain.NotificationEvent]struct{})}
}

// Subscribe registers a new channel for tenantID+userID. The returned
// unsubscribe func removes and closes the channel; callers (the gRPC
// StreamNotifications handler) must invoke it exactly once, typically via
// defer, when the stream ends.
func (b *Broadcaster) Subscribe(ctx context.Context, tenantID, userID string) (<-chan domain.NotificationEvent, func()) {
	ch := make(chan domain.NotificationEvent, channelBufferSize)
	key := subscriberKey{tenantID: tenantID, userID: userID}

	b.mu.Lock()
	if b.subs[key] == nil {
		b.subs[key] = make(map[chan domain.NotificationEvent]struct{})
	}
	b.subs[key][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if set, ok := b.subs[key]; ok {
				delete(set, ch)
				if len(set) == 0 {
					delete(b.subs, key)
				}
			}
			close(ch)
		})
	}
	return ch, unsubscribe
}

// Broadcast delivers event to every channel currently subscribed for each
// of event.RecipientUserIDs, scoped to event.TenantID. A recipient with no
// active subscription on this replica simply doesn't receive it — no
// offline WS replay queue (§2). A full (slow-reader) channel has its send
// dropped rather than blocking the caller, which in production runs on the
// shared event-consumer goroutine (see internal/adapter/eventbus) and must
// not stall behind one slow subscriber.
func (b *Broadcaster) Broadcast(ctx context.Context, event domain.NotificationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, userID := range event.RecipientUserIDs {
		key := subscriberKey{tenantID: event.TenantID, userID: userID}
		for ch := range b.subs[key] {
			select {
			case ch <- event:
			default:
			}
		}
	}
}
