package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

type fakeToolsBus struct {
	mu       sync.Mutex
	handlers map[string]commoneventbus.Handler
	ready    chan struct{}
}

func (b *fakeToolsBus) SubscribeEphemeral(ctx context.Context, stream, subject string, fn commoneventbus.Handler) error {
	b.mu.Lock()
	if stream != "MCP" {
		b.mu.Unlock()
		return context.Canceled
	}
	b.handlers[subject] = fn
	if len(b.handlers) == len(toolsChangedSubjects) {
		close(b.ready)
	}
	b.mu.Unlock()
	<-ctx.Done()
	return nil
}

type recordingCacheAndNotifier struct {
	mu          sync.Mutex
	invalidated []string
	notified    []string
}

func (r *recordingCacheAndNotifier) InvalidateTenant(t string) {
	r.mu.Lock()
	r.invalidated = append(r.invalidated, t)
	r.mu.Unlock()
}
func (r *recordingCacheAndNotifier) NotifyToolsChanged(t string) {
	r.mu.Lock()
	r.notified = append(r.notified, t)
	r.mu.Unlock()
}

func TestWatchToolsChanged_PolicySettingsAndKillSwitchInvalidateThenNotify(t *testing.T) {
	bus := &fakeToolsBus{handlers: map[string]commoneventbus.Handler{}, ready: make(chan struct{})}
	rec := &recordingCacheAndNotifier{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watchToolsChanged(ctx, bus, rec, rec, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	select {
	case <-bus.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("subscriptions not established")
	}
	for _, subject := range toolsChangedSubjects {
		h := bus.handlers[subject]
		if h == nil {
			t.Fatalf("no subscription for %s", subject)
		}
		if err := h(ctx, commoneventbus.Event{TenantID: "t-" + subject, OccurredAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	h := bus.handlers["orca.mcp.policy.changed"]
	_ = h(ctx, commoneventbus.Event{TenantID: "", OccurredAt: time.Now()})                    // malformed: no tenant
	_ = h(ctx, commoneventbus.Event{TenantID: "old", OccurredAt: time.Now().Add(-time.Hour)}) // history replayed at startup
	if len(rec.notified) != 3 || len(rec.invalidated) != 3 {
		t.Fatalf("notified=%v invalidated=%v", rec.notified, rec.invalidated)
	}
	for i := range rec.notified {
		if rec.notified[i] != rec.invalidated[i] {
			t.Fatal("the catalog cache must be dropped before sessions are told to re-list")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher must stop with its context")
	}
}

func TestWithMCPToolsListChanged_SetsOnlyThatFlag(t *testing.T) {
	var d mcpserver.Deps
	withMCPToolsListChanged()(&d)
	if !d.Config.ToolsListChanged {
		t.Fatal("flag not set")
	}
}
