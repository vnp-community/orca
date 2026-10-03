//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"io"
	"log/slog"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

type memoryProcessedEvents struct{ seen map[string]bool }

func (m *memoryProcessedEvents) MarkProcessed(_ context.Context, id, _ string) (bool, error) {
	if m.seen[id] {
		return true, nil
	}
	m.seen[id] = true
	return false, nil
}

// An idle-stop event published (twice, same id) while notification-service is
// down must reach the owner's stream once the consumer starts.
func TestIdleStopped_PublishedWhileConsumerDownIsDeliveredAfterStart(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pub, cons, closeBus, err := commoneventbus.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "MCP", []string{"orca.mcp.>"}); err != nil {
		t.Fatal(err)
	}

	// Gateway side: same AsyncPublisher + payload shape as api-gateway's idleStoppedNotifier.
	payload, _ := json.Marshal(map[string]string{"session_id": "s1", "user_id": "u1", "pty_id": "p1", "reason": "idle", "client_name": "Cursor"})
	ev := commoneventbus.Event{ID: "0b6d7d5e-1f1c-5d0a-8a3b-222222222222", TenantID: "t1", OccurredAt: time.Now().UTC(), Version: 1, Payload: payload}
	async := commoneventbus.NewAsyncPublisher(pub, commoneventbus.AsyncPublishConfig{}, nil)
	async.Publish("orca.mcp.terminal.idlestopped", ev)
	async.Publish("orca.mcp.terminal.idlestopped", ev) // retry of the same event: stream must dedupe
	if err := async.Wait(ctx); err != nil {
		t.Fatal(err)
	}

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, err := js.Stream(ctx, "MCP")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := stream.Info(ctx); err != nil || info.State.Msgs != 1 {
		t.Fatalf("Nats-Msg-Id dedupe: stream holds %v messages (%v), want 1", info.State.Msgs, err)
	}

	// Consumer side starts only now.
	bc := broadcaster.New()
	events, unsubscribe := bc.Subscribe(ctx, "t1", "u1")
	defer unsubscribe()
	uc := usecase.NewHandleIncomingEvent(bc, &memoryProcessedEvents{seen: map[string]bool{}}, nil, nil, nil)
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go New(cons, uc).Run(runCtx, testLogger())

	select {
	case got := <-events:
		if got.Type != "mcp.terminal.idle_stopped" || got.DeepLink != "/?section=mcp&tab=connect" || got.SourceEventID != ev.ID {
			t.Fatalf("unexpected notification %+v", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("event published while the consumer was down was not delivered")
	}
	select {
	case extra := <-events:
		t.Fatalf("duplicate delivery: %+v", extra)
	case <-time.After(1500 * time.Millisecond):
	}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
