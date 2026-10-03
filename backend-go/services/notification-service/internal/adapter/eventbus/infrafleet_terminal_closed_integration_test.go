//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// A terminal.closed event published (as infra-fleet's outbox relay does) while
// notification-service is down is delivered once it starts; a user close is skipped.
func TestInfraFleetTerminalClosed_PublishedWhileConsumerDownIsDeliveredAfterStart(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pub, cons, closeBus, err := commoneventbus.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "INFRAFLEET", []string{"orca.infrafleet.>"}); err != nil {
		t.Fatal(err)
	}

	mk := func(id, reason string) commoneventbus.Event {
		payload, _ := json.Marshal(map[string]any{"tenant_id": "t1", "pty_id": "p-" + reason, "user_id": "u1", "reason": reason,
			"origin": map[string]string{"type": "mcp", "client_name": "Cursor", "mcp_session_id": "s1"}})
		return commoneventbus.Event{ID: id, TenantID: "t1", OccurredAt: time.Now().UTC(), Version: 1, Payload: payload}
	}
	idle := mk("0b6d7d5e-1f1c-5d0a-8a3b-555555555555", "idle")
	for _, ev := range []commoneventbus.Event{mk("0b6d7d5e-1f1c-5d0a-8a3b-666666666666", "user"), idle, idle} {
		if err := pub.PublishDedup(ctx, "orca.infrafleet.terminal.closed", ev); err != nil {
			t.Fatal(err)
		}
	}

	bc := broadcaster.New()
	events, unsubscribe := bc.Subscribe(ctx, "t1", "u1")
	defer unsubscribe()
	uc := usecase.NewHandleIncomingEvent(bc, &memoryProcessedEvents{seen: map[string]bool{}}, nil, nil, nil)
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go New(cons, uc).Run(runCtx, testLogger())

	select {
	case got := <-events:
		if got.Type != "mcp.terminal.idle_stopped" || got.SourceEventID != idle.ID || got.SourceSubject != "orca.infrafleet.terminal.closed" {
			t.Fatalf("unexpected notification %+v", got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("closed event published while the consumer was down was not delivered")
	}
	select {
	case extra := <-events:
		t.Fatalf("unexpected extra notification (user close or duplicate): %+v", extra)
	case <-time.After(1500 * time.Millisecond):
	}
}
