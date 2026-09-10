package httpgateway

import "sync"

// TraceBroadcast fans one replica's locally-received TRACE stream events
// out to every currently-connected /api/trace-stream SSE client
// (TASK-BE-FFT-009/010) — an in-process pub/sub hub, not itself durable:
// a client connected before a Publish call sees it, a client that connects
// after does not (SSE clients don't need replay, just "what's happening
// live"). Safe for concurrent use.
type TraceBroadcast struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// NewTraceBroadcast constructs an empty hub — always safe to construct
// even when NATS/eventbus.Connect fails at startup (see cmd/server/main.go),
// it just never receives a Publish call in that case.
func NewTraceBroadcast() *TraceBroadcast {
	return &TraceBroadcast{subs: make(map[chan []byte]struct{})}
}

// Publish fans payload out to every currently-subscribed channel —
// non-blocking: a slow/stuck subscriber is dropped (its channel skipped)
// rather than backing up every other subscriber or the caller (the
// OTel span-processor goroutine in cmd/server/main.go's traceEventHandler).
func (b *TraceBroadcast) Publish(payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- payload:
		default:
		}
	}
}

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
