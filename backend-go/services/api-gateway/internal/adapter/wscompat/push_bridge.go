package wscompat

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// StreamHandler opens a subscription for one push-capable channel and
// returns a channel of events to forward as push frames until ctx is
// cancelled. Registered in Registry.streamHandlers, parallel to the
// existing invoke/send ChannelHandler map (registry.go) — this does not
// replace or modify that map; a channel is either request/response
// (ChannelHandler) or stream-registering (StreamHandler), never both.
type StreamHandler func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error)

// PushEvent is one server->client push frame's payload. Args is a slice —
// NOT a single value — because rpc-client.ts's push handling
// (`handlers.forEach(h => h(...args))`) spreads Args as positional
// arguments to the registered `on(channel, handler)` callback; see
// envelope.go's PushMessage, whose wire shape this becomes.
type PushEvent struct {
	Channel string
	Args    []any
}

// pipePush reads from a subscription's event channel until ctx is
// cancelled or the channel closes, writing each event as a PushMessage
// frame (envelope.go) — serialized through the SAME writeMu
// handleInvoke/handleSubscribe already use (handler.go), so push frames
// never interleave-corrupt a concurrent invoke response on the same
// connection. Writes use ctx directly (the connection-lifetime context from
// ServeHTTP), matching handleInvoke's own write call — there is no separate
// write-timeout budget in this scaffold yet.
func pipePush(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, events <-chan PushEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			writeMu.Lock()
			_ = wsjson.Write(ctx, conn, PushMessage{Type: "push", Channel: ev.Channel, Args: ev.Args})
			writeMu.Unlock()
		}
	}
}

// pipePushForDialect is pipePush's dialect-aware counterpart (BUG-005
// Phase 2, specs/backend-go/bugs/api-v1/solutions/SOL-005). The native
// dialect delegates straight to pipePush, unchanged. The session-client
// dialect cannot reuse PushMessage's channel-keyed shape at all —
// WebSessionClient has no `on(channel, handler)` concept, only per-request-id
// correlation (see SessionClientResultMessage's doc comment) — so every
// event is re-encoded as a Streaming:true SessionClientResultMessage
// carrying requestID (the original subscribe/invoke call's id, NOT
// ev.Channel), and the loop sends one final {"type":"end"} frame when the
// event channel closes so WebSessionClient's onClose fires instead of the
// subscription silently going quiet forever.
func pipePushForDialect(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, events <-chan PushEvent, d dialect, requestID string) {
	if d != dialectSessionClient {
		pipePush(ctx, conn, writeMu, events)
		return
	}
	for {
		select {
		case <-ctx.Done():
			// Connection is going away — no point writing an end frame the
			// client will never receive, same reasoning pipePush's own
			// ctx.Done() branch already applies.
			return
		case ev, ok := <-events:
			runtimeID := sessionClientRuntimeID
			if !ok {
				writeMu.Lock()
				_ = wsjson.Write(ctx, conn, SessionClientResultMessage{
					ID: requestID, OK: true, Result: sessionClientStreamEnd,
					Meta: sessionClientMeta{RuntimeID: &runtimeID},
				})
				writeMu.Unlock()
				return
			}
			writeMu.Lock()
			_ = wsjson.Write(ctx, conn, SessionClientResultMessage{
				ID: requestID, OK: true, Result: pushEventResult(ev),
				Meta: sessionClientMeta{RuntimeID: &runtimeID}, Streaming: true,
			})
			writeMu.Unlock()
		}
	}
}

// pushEventResult collapses a PushEvent's Args (a slice, designed for
// rpc-client.ts's `handlers.forEach(h => h(...args))` spread — see
// PushEvent's doc comment) into the single Result value
// SessionClientResultMessage carries: every current StreamHandler
// (registerNotificationStreamChannel, channels_push.go) emits exactly one
// arg per event, so the common case unwraps to that value directly rather
// than forcing every session-client consumer to index into a one-element
// array. A future multi-arg emitter still degrades safely to the raw slice.
func pushEventResult(ev PushEvent) any {
	switch len(ev.Args) {
	case 1:
		return ev.Args[0]
	default:
		return ev.Args
	}
}
