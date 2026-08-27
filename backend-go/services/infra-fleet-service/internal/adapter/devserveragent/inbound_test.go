package devserveragent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestClient_DirectWebSocket_NoInboundSession_ReturnsClearError covers
// getInboundSession's core contract: Orca must never attempt to dial an
// agent that only ever dials in.
func TestClient_DirectWebSocket_NoInboundSession_ReturnsClearError(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-direct", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if healthy {
		t.Error("Health = true, want false with no inbound connection ever attached")
	}

	_, err = client.Exec(context.Background(), devServer, "preflight.check", nil)
	if err == nil {
		t.Fatal("expected Exec to error with no inbound connection ever attached")
	}
}

// TestClient_AttachInboundSession_MakesExecAndHealthWork exercises the
// direct-websocket happy path end to end: dial a raw WS connection (as if
// adapter/agentwsserver had already validated the token and completed the
// handshake — this test constructs the HandshakeInfo directly rather than
// re-running that handshake, since that's agentwsserver's own contract to
// test), attach it, then confirm Health/Exec work against it exactly like
// a relay-websocket session would.
func TestClient_AttachInboundSession_MakesExecAndHealthWork(t *testing.T) {
	results := map[string]any{"branch": "main"}
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
			if err := json.Unmarshal(decoded.Payload, &req); err != nil {
				continue
			}
			result, _ := json.Marshal(results)
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
			frame, _ := EncodeJSONRPCFrame(resp, 2, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
		}
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dialing fake inbound agent: %v", err)
	}

	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-direct-2", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	client.AttachInboundSession(devServer.ID, devServer.Host, conn, HandshakeInfo{Platform: "linux", AgentVersion: "5.0.0"})

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Fatal("Health = false, want true right after AttachInboundSession")
	}

	result, err := client.Exec(context.Background(), devServer, "git.status", nil)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if result["branch"] != "main" {
		t.Errorf("result = %+v, want branch=main", result)
	}
}

// TestClient_CloseSessionsForDevServerToken_ClosesActiveSession covers
// TASK-AWS-03-06's usecase.LiveSessionCloser implementation: closing an
// active direct-websocket session reports closed=1, and the session is no
// longer healthy afterward.
func TestClient_CloseSessionsForDevServerToken_ClosesActiveSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := context.Background()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dialing fake inbound agent: %v", err)
	}

	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-close", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	client.AttachInboundSession(devServer.ID, devServer.Host, conn, HandshakeInfo{Platform: "linux"})

	if healthy, _ := client.Health(context.Background(), devServer); !healthy {
		t.Fatal("expected the session to be healthy before closing it")
	}

	closed, err := client.CloseSessionsForDevServerToken(context.Background(), devServer.ID, "tok-whatever")
	if err != nil {
		t.Fatalf("CloseSessionsForDevServerToken: %v", err)
	}
	if closed != 1 {
		t.Errorf("closed = %d, want 1", closed)
	}

	if healthy, _ := client.Health(context.Background(), devServer); healthy {
		t.Error("expected the session to be unhealthy after CloseSessionsForDevServerToken")
	}
}

// TestClient_CloseSessionsForDevServerToken_NoLiveSession_ReturnsZero
// covers the no-session case: a devServer with no live session returns
// (0, nil), not an error.
func TestClient_CloseSessionsForDevServerToken_NoLiveSession_ReturnsZero(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	closed, err := client.CloseSessionsForDevServerToken(context.Background(), "ds-never-connected", "tok-whatever")
	if err != nil {
		t.Fatalf("CloseSessionsForDevServerToken: %v", err)
	}
	if closed != 0 {
		t.Errorf("closed = %d, want 0", closed)
	}
}
