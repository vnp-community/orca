// Package portevents implements usecase.PortForwardEventPublisher as an
// in-process, per-connectionId fan-out — the smallest change that closes
// BR-SSH-15's "live push, no polling" requirement today. Wiring the full
// NATS JetStream outbox path (infra-fleet-service.md §7) across
// infra-fleet-service -> notification-service -> api-gateway is a larger
// cross-service initiative tracked separately; see
// specs/backend-go/bugs/logic-v1/tasks/TASK-SSH-04-08-port-forward-push-notifications.md.
package portevents

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// PortEvent is one dev_server.port_opened/port_closed occurrence.
type PortEvent struct {
	Kind    string // "opened" | "closed"
	Forward domain.PortForward
}

// Broadcaster fans out PortForward lifecycle events to subscribers keyed by
// connectionID — mirrors devserveragent/session.go's routeNotification/
// subscribePty subs-map pattern (same "drop on full, never block the
// publisher" discipline).
type Broadcaster struct {
	mu   sync.Mutex
	subs map[string][]chan PortEvent
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[string][]chan PortEvent)}
}

// Publish implements usecase.PortForwardEventPublisher.
func (b *Broadcaster) Publish(_ context.Context, event string, pf domain.PortForward) {
	kind := "opened"
	if event == "dev_server.port_closed" {
		kind = "closed"
	}
	b.mu.Lock()
	subs := append([]chan PortEvent(nil), b.subs[pf.ConnectionID]...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- PortEvent{Kind: kind, Forward: pf}:
		default: // slow/gone consumer — drop rather than block PollWorkspacePorts
		}
	}
}

// Subscribe registers a new listener for connectionID's events. The
// returned unsubscribe func MUST be called exactly once when done.
func (b *Broadcaster) Subscribe(connectionID string) (<-chan PortEvent, func()) {
	ch := make(chan PortEvent, 16)
	b.mu.Lock()
	b.subs[connectionID] = append(b.subs[connectionID], ch)
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		subs := b.subs[connectionID]
		for i, c := range subs {
			if c == ch {
				b.subs[connectionID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		close(ch)
	}
}
