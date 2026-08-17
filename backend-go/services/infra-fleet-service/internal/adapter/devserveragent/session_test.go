package devserveragent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// reconnectTestAgent drops the very first connection immediately after a
// successful handshake (simulating a mid-session network drop), then stays
// open on every later connection — enough to prove session.backgroundReconnect
// (not the lazy-redial fallback in getOrCreateSession) is what recovers it,
// since the test below never calls Exec/Health again after the initial one.
type reconnectTestAgent struct {
	token     string
	connCount atomic.Int32
}

func (a *reconnectTestAgent) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orca-relay" {
		http.NotFound(w, r)
		return
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+a.token {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	n := a.connCount.Add(1)

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := context.Background()

	_, data, err := conn.Read(ctx)
	if err != nil {
		return
	}
	decoded, err := DecodeFrame(data)
	if err != nil || decoded.Type != MessageTypeRegular {
		return
	}
	var req JSONRPCRequest
	if err := json.Unmarshal(decoded.Payload, &req); err != nil || req.Method != "agent.handshake" {
		return
	}
	info := HandshakeInfo{Platform: "linux"}
	result, _ := json.Marshal(info)
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	frame, _ := EncodeJSONRPCFrame(resp, 1, decoded.ID)
	_ = conn.Write(ctx, websocket.MessageBinary, frame)

	if n == 1 {
		return // drop right after handshake — triggers readLoop -> handleDisconnect
	}
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

func TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry(t *testing.T) {
	agent := &reconnectTestAgent{token: fakeAgentToken}
	server := httptest.NewServer(http.HandlerFunc(agent.handler))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("splitting host:port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port: %v", err)
	}

	cfg := testConfig(port, fakeAgentToken)
	cfg.ReconnectBaseDelay = 10 * time.Millisecond
	cfg.ReconnectMaxDelay = 50 * time.Millisecond

	client := New(cfg, slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-reconnect", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	// Establish the session — this is connection #1, which the fake agent
	// drops right after handshake.
	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Fatal("expected the initial connection to handshake successfully")
	}

	// Deliberately no further Exec/Health call — if the session comes back
	// at all, it must be backgroundReconnect, not the lazy-redial fallback.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client.mu.Lock()
		sess, ok := client.sessions[devServer.ID]
		client.mu.Unlock()
		if ok && sess.isHandshaked() && agent.connCount.Load() >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session did not reconnect in the background after a drop (connCount=%d)", agent.connCount.Load())
}
