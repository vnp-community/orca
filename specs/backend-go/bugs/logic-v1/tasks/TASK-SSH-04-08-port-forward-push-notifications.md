# TASK-SSH-04-08: `workspacePorts.opened`/`closed` WS push (BR-SSH-15's notification)

**From Solution:** SOL-SSH-04
**Priority:** P2
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/portevents/broadcaster.go` (new)
**Depends on:** TASK-SSH-04-06
**Status:** `[x] DONE — portevents.Broadcaster + StreamPortForwardEvents RPC + wscompat workspacePorts.subscribe channel added (found complete/verified in the shared worktree — not originally in this session's assigned list, but built, tested, and left uncommitted, so verified and committed here rather than left dangling); go build/vet/test clean across infra-fleet-service and api-gateway`

---

## Context

`PollWorkspacePorts.Publish` (TASK-SSH-04-06) needs a real
`PortForwardEventPublisher` implementation that reaches the browser as a
"Port 3001 → remote:3000 (node)" notification, without polling.
`infra-fleet-service.md` §7 describes this as a NATS JetStream
outbox-published event `notification-service` and others subscribe to;
this task implements the simpler in-process equivalent — a per-connectionId
fan-out broadcaster feeding a new server-streaming gRPC endpoint, the same
"open a stream, forward each item" shape `wscompat`'s existing
`registerNotificationStreamChannel` already uses for `StreamNotifications`
(`channels_push.go:44-63`). Wiring the full NATS/outbox path end-to-end
across `infra-fleet-service` → `notification-service` → `api-gateway` is a
larger cross-service initiative tracked separately from this bug-fix scope;
this task closes BR-SSH-15's user-visible requirement (a live push, no
polling) with the smallest change that satisfies it today.

## Changes to make

`backend-go/services/infra-fleet-service/internal/adapter/portevents/broadcaster.go` (new):

```go
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
```

Add a server-streaming RPC to `infrafleet.proto`:

```protobuf
  rpc StreamPortForwardEvents(StreamPortForwardEventsRequest) returns (stream PortForwardEvent);
```

```protobuf
message StreamPortForwardEventsRequest {
  string connection_id = 1;
}
message PortForwardEvent {
  string kind = 1; // "opened" | "closed"
  PortForward forward = 2;
}
```

Add the gRPC server handler streaming from `Broadcaster.Subscribe`
(`main.go` wires one shared `*portevents.Broadcaster`, passed to both
`PollWorkspacePorts` — as its `PortForwardEventPublisher` — and the new gRPC
handler).

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go` —
add a `workspacePorts.subscribe` stream channel, mirroring
`registerNotificationStreamChannel`:

```go
func registerWorkspacePortsStreamChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStream("workspacePorts.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		type subArgs struct {
			ConnectionID string `json:"connectionId"`
		}
		in, err := decodeArg[subArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		stream, err := client.StreamPortForwardEvents(ctx, &infrafleetv1.StreamPortForwardEventsRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		out := make(chan PushEvent)
		go func() {
			defer close(out)
			for {
				item, err := stream.Recv()
				if err != nil {
					return
				}
				channel := "workspacePorts.opened"
				if item.GetKind() == "closed" {
					channel = "workspacePorts.closed"
				}
				select {
				case out <- PushEvent{Channel: channel, Args: []any{toWorkspacePortForwardResult(item.GetForward())}}:
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, nil
	})
}
```

Register it from `RegisterPushChannels` alongside the existing
`registerNotificationStreamChannel`/`registerClientEventsChannel` calls.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./...
go test ./services/infra-fleet-service/... ./services/api-gateway/... -v
```

Expected: a `Broadcaster.Publish("dev_server.port_opened", pf)` call
delivers to every `Subscribe(pf.ConnectionID)` listener and none of any
other connectionId's; the `workspacePorts.subscribe` wscompat channel
end-to-end test (fake `InfraFleetServiceClient` stream) asserts a
`port_opened`/`port_closed` event arrives as `workspacePorts.opened`/
`workspacePorts.closed` push frames.
