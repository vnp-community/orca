package httpgateway

import "sync"

<<<<<<< HEAD
// TraceBroadcast fans one replica's locally-received TRACE stream events
// out to every currently-connected /api/trace-stream SSE client
// (TASK-BE-FFT-009/010) — an in-process pub/sub hub, not itself durable:
// a client connected before a Publish call sees it, a client that connects
// after does not (SSE clients don't need replay, just "what's happening
// live"). Safe for concurrent use.
=======
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
>>>>>>> feat/team-rbac-implementation
type TraceBroadcast struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

<<<<<<< HEAD
// NewTraceBroadcast constructs an empty hub — always safe to construct
// even when NATS/eventbus.Connect fails at startup (see cmd/server/main.go),
// it just never receives a Publish call in that case.
=======
>>>>>>> feat/team-rbac-implementation
func NewTraceBroadcast() *TraceBroadcast {
	return &TraceBroadcast{subs: make(map[chan []byte]struct{})}
}

<<<<<<< HEAD
// Publish fans payload out to every currently-subscribed channel —
// non-blocking: a slow/stuck subscriber is dropped (its channel skipped)
// rather than backing up every other subscriber or the caller (the
// OTel span-processor goroutine in cmd/server/main.go's traceEventHandler).
func (b *TraceBroadcast) Publish(payload []byte) {
=======
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
>>>>>>> feat/team-rbac-implementation
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
<<<<<<< HEAD
		case ch <- payload:
=======
		case ch <- raw:
>>>>>>> feat/team-rbac-implementation
		default:
		}
	}
}
<<<<<<< HEAD

// Subscribe registers a new listener and returns its channel plus an
// unsubscribe func the caller must defer-call exactly once (typically when
// the SSE request's context is done).
func (b *TraceBroadcast) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}
=======
>>>>>>> feat/team-rbac-implementation
