package httpgateway

import "sync"

// TraceBroadcast fans a stream of already-JSON-encoded F40 TraceEvent
// bytes out to every locally-connected SSE client on THIS api-gateway
// replica. Simpler than wscompat.ClientEventBus (channels_push.go:80-116)
// because trace data has no per-user routing — every authorized SSE
// client receives every event, matching the old backend's
// registerTraceSink fan-out (trace-sse-routes.ts) and this endpoint's own
// "intentionally low-security... diagnostic, not sensitive" stance
// (see trace_routes.go's doc comment).
//
// Exported (unlike a package-private bus) so cmd/server/main.go
// (TASK-BE-FFT-011) can construct one, feed it from its NATS
// SubscribeEphemeral loop via Publish, and hand the same instance to
// Deps.TraceBroadcast for mountTraceRoutes to Subscribe from.
type TraceBroadcast struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func NewTraceBroadcast() *TraceBroadcast {
	return &TraceBroadcast{subs: make(map[chan []byte]struct{})}
}

// Subscribe registers a new SSE client, returning a receive-only channel
// and an unsubscribe func the caller MUST invoke exactly once (typically
// via defer) when the client disconnects — otherwise the channel and its
// registry entry leak.
func (b *TraceBroadcast) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 16) // buffer matches wscompat.ClientEventBus's convention
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

// Publish is best-effort — mirrors ClientEventBus.Publish and
// notification-service's Broadcaster.Broadcast: a slow/stalled SSE client
// is dropped for that one event, never allowed to block delivery to
// other clients.
func (b *TraceBroadcast) Publish(raw []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- raw:
		default:
		}
	}
}
