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
