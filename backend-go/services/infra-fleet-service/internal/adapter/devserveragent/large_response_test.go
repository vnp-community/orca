package devserveragent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// An agent reply above coder/websocket's 32 KiB default (a big fs.readDir)
// used to close the connection with "message too big" and drop the agent.
func TestClient_AttachInboundSession_LargeAgentReplyKeepsSessionAlive(t *testing.T) {
	big := strings.Repeat("x", 200*1024)
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
			result, _ := json.Marshal(map[string]any{"blob": big})
			frame, _ := EncodeJSONRPCFrame(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, 2, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
		}
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dialing fake agent: %v", err)
	}

	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)
	devServer, err := domain.NewDevServer("ds-large", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	client.AttachInboundSession(devServer.ID, devServer.Host, conn, HandshakeInfo{Platform: "linux", AgentVersion: "5.0.0"})

	for i := 0; i < 2; i++ {
		result, err := client.Exec(context.Background(), devServer, "fs.readDir", nil)
		if err != nil {
			t.Fatalf("Exec #%d: %v", i, err)
		}
		if got, _ := result["blob"].(string); len(got) != len(big) {
			t.Fatalf("Exec #%d: blob length = %d, want %d", i, len(got), len(big))
		}
	}
	if healthy, err := client.Health(context.Background(), devServer); err != nil || !healthy {
		t.Fatalf("session must stay healthy after a large reply: healthy=%v err=%v", healthy, err)
	}
}
