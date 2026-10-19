package devserveragent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// The agent holds each agent.execPrompt answer until released, so the count can be read while runs are in flight.
func TestClient_ActiveExecutions_CountsRunsInFlightOnly(t *testing.T) {
	release := make(chan struct{})
	arrived := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := context.Background()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			decoded, err := DecodeFrame(data)
			if err != nil || decoded.Type != MessageTypeRegular {
				continue
			}
			var req JSONRPCRequest
			if json.Unmarshal(decoded.Payload, &req) != nil {
				continue
			}
			go func() {
				arrived <- req.Method
				if req.Method == "agent.execPrompt" {
					<-release
				}
				result, _ := json.Marshal(map[string]any{"ok": true})
				frame, _ := EncodeJSONRPCFrame(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, 2, decoded.ID)
				_ = conn.Write(ctx, websocket.MessageBinary, frame)
			}()
		}
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatal(err)
	}
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)
	ds, _ := domain.NewDevServer("ds-busy", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	client.AttachInboundSession(ds.ID, ds.Host, conn, HandshakeInfo{Platform: "linux"})

	if n := client.ActiveExecutions(ds.ID); n != 0 {
		t.Fatalf("idle dev server: %d", n)
	}
	if n := client.ActiveExecutions("unknown"); n != 0 {
		t.Fatalf("unknown dev server: %d", n)
	}

	done := make(chan error, 3)
	for range 2 {
		go func() {
			_, err := client.Exec(context.Background(), ds, "agent.execPrompt", map[string]any{"prompt": "x"})
			done <- err
		}()
	}
	for range 2 {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("agent never received the runs")
		}
	}
	if n := client.ActiveExecutions(ds.ID); n != 2 {
		t.Fatalf("two runs in flight, counted %d", n)
	}

	// A short probe is not a run: it must not change the count, in flight or after.
	if _, err := client.Exec(context.Background(), ds, "ports.scan", nil); err != nil {
		t.Fatal(err)
	}
	if n := client.ActiveExecutions(ds.ID); n != 2 {
		t.Fatalf("a probe changed the count: %d", n)
	}

	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if n := client.ActiveExecutions(ds.ID); n != 0 {
		t.Fatalf("runs finished, still counted %d", n)
	}
}

// A run that fails (timeout, dropped session) must not leave the dev server busy forever.
func TestClient_ActiveExecutions_FailedRunIsNotCountedAfterwards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			if _, _, err := conn.Read(context.Background()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatal(err)
	}
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)
	ds, _ := domain.NewDevServer("ds-fail", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	client.AttachInboundSession(ds.ID, ds.Host, conn, HandshakeInfo{})

	ctx, stop := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer stop()
	if _, err := client.Exec(ctx, ds, "ai.complete", map[string]any{}); err == nil {
		t.Fatal("the silent agent never answers: expected a timeout")
	}
	if n := client.ActiveExecutions(ds.ID); n != 0 {
		t.Fatalf("a timed-out run is still counted: %d", n)
	}
}

// The activity callback is how the status event is triggered: attach, a run starting, a run ending and a drop all fire it.
func TestClient_OnActivity_FiresOnAttachRunAndDrop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := context.Background()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			decoded, err := DecodeFrame(data)
			if err != nil || decoded.Type != MessageTypeRegular {
				continue
			}
			var req JSONRPCRequest
			if json.Unmarshal(decoded.Payload, &req) != nil {
				continue
			}
			result, _ := json.Marshal(map[string]any{})
			frame, _ := EncodeJSONRPCFrame(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, 2, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
		}
	}))
	t.Cleanup(server.Close)
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var seen []string
	client := New(DefaultConfig(), slog.Default(), WithOnActivity(func(id string) { mu.Lock(); seen = append(seen, id); mu.Unlock() }))
	t.Cleanup(client.Close)
	ds, _ := domain.NewDevServer("ds-activity", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(seen) }

	client.AttachInboundSession(ds.ID, ds.Host, conn, HandshakeInfo{})
	if count() != 1 {
		t.Fatalf("attach fires once: %v", seen)
	}
	if _, err := client.Exec(context.Background(), ds, "agent.execPrompt", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if count() != 3 {
		t.Fatalf("a run fires on start and on end: %v", seen)
	}
	if _, err := client.Exec(context.Background(), ds, "ports.scan", nil); err != nil {
		t.Fatal(err)
	}
	if count() != 3 {
		t.Fatalf("a short probe is not activity: %v", seen)
	}
	for _, id := range seen {
		if id != ds.ID {
			t.Fatalf("unexpected dev server %q", id)
		}
	}
}
