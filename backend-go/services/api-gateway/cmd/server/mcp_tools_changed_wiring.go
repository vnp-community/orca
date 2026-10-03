package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// toolsChangedSubjects are the mcp-service events after which a tenant's
// tools/list can differ: tool policy and tenant settings (enable, rollout)
// emit policy.changed, the kill switch feeds the policy inputs.
var toolsChangedSubjects = []string{"orca.mcp.policy.changed", "orca.mcp.settings.changed", "orca.mcp.killswitch.changed"}

// toolsChangedReplayGrace: an ephemeral consumer starts at the beginning of
// the stream, and old events are history, not news.
const toolsChangedReplayGrace = 10 * time.Second

// withMCPToolsListChanged advertises tools.listChanged. Apply it only when
// watchToolsChanged runs, otherwise the capability would promise a signal
// nothing sends.
func withMCPToolsListChanged() func(*mcpserver.Deps) {
	return func(d *mcpserver.Deps) { d.Config.ToolsListChanged = true }
}

type toolsChangeBus interface {
	SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error
}

type toolCacheInvalidator interface{ InvalidateTenant(tenantID string) }

type toolsChangeNotifier interface{ NotifyToolsChanged(tenantID string) }

// watchToolsChanged turns each policy/settings/kill-switch event into a
// per-tenant catalog invalidation and a tools/list_changed notification to the
// tenant's sessions on this replica. Every replica consumes every event (its
// own ephemeral consumer), so sessions on other replicas are told by them.
// Blocks until ctx ends; run it in its own goroutine.
func watchToolsChanged(ctx context.Context, bus toolsChangeBus, cache toolCacheInvalidator, notify toolsChangeNotifier, logger *slog.Logger) {
	startedAt := time.Now()
	handle := func(_ context.Context, ev commoneventbus.Event) error {
		if ev.TenantID == "" || ev.OccurredAt.Before(startedAt.Add(-toolsChangedReplayGrace)) {
			return nil
		}
		if cache != nil {
			cache.InvalidateTenant(ev.TenantID)
		}
		notify.NotifyToolsChanged(ev.TenantID)
		return nil
	}
	var wg sync.WaitGroup
	for _, subject := range toolsChangedSubjects {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := bus.SubscribeEphemeral(ctx, "MCP", subject, handle); err != nil && ctx.Err() == nil {
				logger.WarnContext(ctx, "mcp tools-changed subscription ended", slog.String("subject", subject), slog.Any("error", err))
			}
		}()
	}
	wg.Wait()
}
