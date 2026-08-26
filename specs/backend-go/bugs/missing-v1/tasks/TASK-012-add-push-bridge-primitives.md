# TASK-012: Add `StreamHandler`/`PushEvent`/`pipePush` primitives

**From Solution:** SOL-035
**Priority:** P0 — everything else in this solution depends on these types
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/push_bridge.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — `StreamHandler`/`PushEvent`/`pipePush` exist in `push_bridge.go` (with `PushEvent.Args` as `[]any` to match `rpc-client.ts`'s spread semantics, per envelope.go's `PushMessage`, a deliberate refinement over this doc's single-value sketch); builds clean.

---

## Changes to make

```go
package wscompat

import (
    "context"
    "sync"
    "time"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
)

// StreamHandler opens a subscription for one push-capable channel and
// returns a channel of events to forward as push frames until ctx is
// cancelled. Registered in Registry.StreamHandlers, parallel to the
// existing invoke/send ChannelHandler map — this does not replace or
// modify that map.
type StreamHandler func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error)

// PushEvent is one server→client push frame's payload.
type PushEvent struct {
    Channel string
    Args    any
}

// PushMessage is the wire shape for a "push" frame — matches
// docs/execution-plan.md §0's {type:"push",channel,args} description.
type PushMessage struct {
    Type    string `json:"type"`
    Channel string `json:"channel"`
    Args    any    `json:"args"`
}

// pipePush reads from a subscription's event channel until ctx is
// cancelled or the channel closes, writing each event as a push frame —
// serialized through the SAME writeMu handleInvoke already uses
// (handler.go), so push frames never interleave-corrupt a concurrent
// invoke response on the same connection.
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
            writeCtx, cancel := context.WithTimeout(context.Background(), writeTimeout)
            _ = wsjson.Write(writeCtx, conn, PushMessage{Type: "push", Channel: ev.Channel, Args: ev.Args})
            cancel()
            writeMu.Unlock()
        }
    }
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
```
