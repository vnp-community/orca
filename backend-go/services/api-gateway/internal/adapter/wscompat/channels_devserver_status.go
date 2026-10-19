package wscompat

import (
	"context"
	"encoding/json"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// registerDevServerStatusSubscribeChannel streams dev server status snapshots ({devServerId, connected,
// activeExecutions, busy}) of the caller's tenant as "devServer.statusChanged" pushes. Snapshots are idempotent and
// ephemeral, so a client that misses some or subscribes late is made right by the next one or by devServer.list.
// bus is nil when NATS was unreachable at startup: the channel then closes at once, as agent.subscribeStatus does.
func registerDevServerStatusSubscribeChannel(r *Registry, bus *commoneventbus.Consumer) {
	r.RegisterStream("devServer.subscribeStatus", func(ctx context.Context, id Identity, _ []json.RawMessage) (<-chan PushEvent, error) {
		sink := newPushFanInSink()
		if bus == nil {
			sink.close()
			return sink.out, nil
		}
		go func() {
			defer sink.close()
			_ = bus.SubscribeEphemeral(ctx, "INFRA", "orca.infra.devserver.statusChanged", func(ctx context.Context, ev commoneventbus.Event) error {
				if ev.TenantID != id.TenantID {
					return nil // tenant isolation
				}
				sink.send(ctx, PushEvent{Channel: "devServer.statusChanged", Args: []any{ev.Payload}})
				return nil
			})
		}()
		return sink.out, nil
	})
}
