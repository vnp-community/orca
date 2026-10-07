package devserveragent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCodeIntelEvents_FilterByDevServer(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	ch1, unsub1 := client.SubscribeCodeIntelEvents("ds-1")
	defer unsub1()

	ch2, unsub2 := client.SubscribeCodeIntelEvents("ds-2")
	defer unsub2()

	// route indexChanged on ds-1
	notif := JSONRPCNotification{
		Method: "codeintel.indexChanged",
		Params: json.RawMessage(`{
			"workspaceRoot": "/work/orca",
			"tool": "gitnexus",
			"commit": "sha-c1",
			"indexedAt": "2026-10-06T12:00:00Z",
			"reason": "head_changed",
			"headCommit": "sha-h1",
			"stale": false,
			"indexScope": "repo",
			"mergeBase": "sha-b1",
			"trigger": "watch"
		}`),
	}
	client.routeCodeIntelNotification("ds-1", notif)

	select {
	case ev := <-ch1:
		if ev.Kind != domain.CodeIntelEventKindIndexChanged {
			t.Fatalf("expected index_changed, got %s", ev.Kind)
		}
		if ev.WorkspaceRoot != "/work/orca" || ev.Tool != "gitnexus" || ev.Commit != "sha-c1" {
			t.Errorf("field mismatch on ds-1: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event on ds-1")
	}

	select {
	case ev := <-ch2:
		t.Fatalf("unexpected event on ds-2: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// Expected: ds-2 received nothing
	}
}

func TestCodeIntelEvents_ResyncOnAttach(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	// Subscribe before agent connects (direct-websocket pattern)
	ch, unsub := client.SubscribeCodeIntelEvents("ds-inbound")
	defer unsub()

	// Simulate agent attaching transport
	client.AttachTransport("ds-inbound", "host-1", &stubTransport{}, HandshakeInfo{Platform: "linux"})

	select {
	case ev := <-ch:
		if ev.Kind != domain.CodeIntelEventKindResync {
			t.Fatalf("expected resync event, got %s", ev.Kind)
		}
		if ev.WorkspaceRoot != "" {
			t.Errorf("expected empty workspace_root for resync, got %s", ev.WorkspaceRoot)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resync event")
	}
}

func TestCodeIntelEvents_OverflowOnBufferFullAndNonBlocking(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	ch, unsub := client.SubscribeCodeIntelEvents("ds-overflow")
	defer unsub()

	// Send 200 notifications without reading
	for i := 0; i < 200; i++ {
		notif := JSONRPCNotification{
			Method: "codeintel.indexChanged",
			Params: json.RawMessage(fmt.Sprintf(`{"workspaceRoot": "/work/orca", "tool": "gitnexus", "commit": "c-%d"}`, i)),
		}
		client.routeCodeIntelNotification("ds-overflow", notif)
	}

	// Drain all received events
	var (
		events        []domain.CodeIntelEvent
		overflowCount int
	)

	timeout := time.After(500 * time.Millisecond)
drainLoop:
	for {
		select {
		case ev := <-ch:
			if ev.Kind == domain.CodeIntelEventKindOverflow {
				overflowCount++
			} else {
				events = append(events, ev)
			}
		case <-timeout:
			break drainLoop
		}
	}

	// Buffer should have capped at <= 64 original events
	if len(events) > 64 {
		t.Errorf("expected <= 64 events, got %d", len(events))
	}
	// Exactly one overflow event should have been emitted
	if overflowCount != 1 {
		t.Errorf("expected exactly 1 overflow event, got %d", overflowCount)
	}
}

func TestCodeIntelEvents_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			devServerID := fmt.Sprintf("ds-%d", id%5)
			ch, unsub := client.SubscribeCodeIntelEvents(devServerID)

			// Publish an event concurrently
			client.routeCodeIntelNotification(devServerID, JSONRPCNotification{
				Method: "codeintel.indexChanged",
				Params: json.RawMessage(`{"workspaceRoot": "/work/orca", "tool": "gitnexus"}`),
			})

			// Attempt reading with timeout
			select {
			case <-ch:
			case <-time.After(20 * time.Millisecond):
			}

			// Unsubscribe idempotently
			unsub()
			unsub()
		}(i)
	}

	wg.Wait()
}
