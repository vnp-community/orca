package mcpserver

import (
	"sync"
	"time"
)

// NotifyToolsChanged tells the sessions of one tenant held by this replica
// that tools/list changed (policy, kill switch or tenant settings changed).
// Bursts within Config.ToolsListChangedDebounce collapse into one notification.
// Every replica calls it for the same event, so each notifies only its own
// sessions and nothing is delivered twice. No-op unless Config.ToolsListChanged.
func (h *Handler) NotifyToolsChanged(tenantID string) {
	if h == nil || h.host == nil || !h.cfg.ToolsListChanged || tenantID == "" {
		return
	}
	h.toolsDebounce.Trigger(tenantID, func() { h.host.notifyToolsListChanged(tenantID) })
}

// tenantDebouncer runs fn once per quiet period per key (trailing edge).
type tenantDebouncer struct {
	d       time.Duration
	mu      sync.Mutex
	pending map[string]*time.Timer
	stopped bool
}

func newTenantDebouncer(d time.Duration) *tenantDebouncer {
	return &tenantDebouncer{d: d, pending: map[string]*time.Timer{}}
}

// Trigger schedules fn unless one is already waiting for key; the waiting call
// is not extended, so a steady stream of changes still notifies every d.
func (b *tenantDebouncer) Trigger(key string, fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	if _, waiting := b.pending[key]; waiting {
		return
	}
	b.pending[key] = time.AfterFunc(b.d, func() {
		b.mu.Lock()
		delete(b.pending, key)
		stopped := b.stopped
		b.mu.Unlock()
		if !stopped {
			fn()
		}
	})
}

// Stop cancels everything waiting.
func (b *tenantDebouncer) Stop() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	for k, t := range b.pending {
		t.Stop()
		delete(b.pending, k)
	}
}
