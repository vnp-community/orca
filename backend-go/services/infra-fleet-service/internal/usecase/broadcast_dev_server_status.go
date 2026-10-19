package usecase

import (
	"context"
	"log/slog"
	"sync"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerStatusPublisher sends the snapshot to whoever watches the tenant's dev servers (ephemeral, best effort).
type DevServerStatusPublisher interface {
	PublishDevServerStatus(ctx context.Context, tenantID string, ev domain.DevServerStatusEvent) error
}

// DevServerLiveState is what the broadcaster reads at send time. Implemented by devserveragent.Client.
type DevServerLiveState interface {
	IsConnected(devServerID string) bool
	ActiveExecutions(devServerID string) int
}

// BroadcastDevServerStatus turns "something changed on this dev server" into a status snapshot. Changes arrive in
// bursts (a run starts, another ends), so Notify only marks the dev server and a single worker sends the state as it
// is when the worker gets to it: one worker keeps snapshots in order, and marking coalesces a burst into one send.
type BroadcastDevServerStatus struct {
	tenants DevServerTenantLookup
	live    DevServerLiveState
	pub     DevServerStatusPublisher
	log     *slog.Logger

	mu      sync.Mutex
	pending map[string]struct{}
	wake    chan struct{}
}

func NewBroadcastDevServerStatus(tenants DevServerTenantLookup, live DevServerLiveState, pub DevServerStatusPublisher, log *slog.Logger) *BroadcastDevServerStatus {
	if log == nil {
		log = slog.Default()
	}
	return &BroadcastDevServerStatus{tenants: tenants, live: live, pub: pub, log: log, pending: map[string]struct{}{}, wake: make(chan struct{}, 1)}
}

// Notify never blocks: it is called from the agent session's read path.
func (b *BroadcastDevServerStatus) Notify(devServerID string) {
	b.mu.Lock()
	b.pending[devServerID] = struct{}{}
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Run sends until ctx ends.
func (b *BroadcastDevServerStatus) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.wake:
		}
		b.Flush(ctx)
	}
}

// Flush sends a snapshot for every marked dev server.
func (b *BroadcastDevServerStatus) Flush(ctx context.Context) {
	b.mu.Lock()
	ids := make([]string, 0, len(b.pending))
	for id := range b.pending {
		ids = append(ids, id)
	}
	b.pending = map[string]struct{}{}
	b.mu.Unlock()

	for _, id := range ids {
		tenantID, found, err := b.tenants.TenantIDForDevServer(ctx, id)
		if err != nil || !found {
			if err != nil {
				b.log.WarnContext(ctx, "dev server status not sent: tenant lookup failed", slog.String("dev_server_id", id), slog.Any("error", err))
			}
			continue
		}
		ev := domain.DevServerStatusEvent{DevServerID: id, Connected: b.live.IsConnected(id)}
		if ev.Connected {
			ev.ActiveExecutions = b.live.ActiveExecutions(id)
		}
		if err := b.pub.PublishDevServerStatus(ctx, tenantID, ev); err != nil {
			b.log.WarnContext(ctx, "dev server status not sent", slog.String("dev_server_id", id), slog.Any("error", err))
		}
	}
}
