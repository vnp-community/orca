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
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const fakeAgentToken = "test-token-123"

// fakeAgent is a minimal stand-in for agent-connection-relay.ts's
// WebSocketServer — enough to drive Client through a real dial, a real
// Stack B handshake exchange, and a real subsequent JSON-RPC call, all
// over an actual TCP loopback connection (httptest.Server), without a real
// Dev Server Agent binary. See client.go's package doc comment: this is the
// substitute for the "no real agent to test against live" gap noted when
// Epic A started.
type fakeAgent struct {
	t               *testing.T
	requireToken    string
	results         map[string]any // method -> result to reply with
	rejectHandshake bool
}

func (f *fakeAgent) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orca-relay" {
		http.NotFound(w, r)
		return
	}
	if f.requireToken != "" {
		got := r.Header.Get("Authorization")
		if got != "Bearer "+f.requireToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
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

		if req.Method == "agent.handshake" {
			if f.rejectHandshake {
				resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCError{Code: -33101, Message: "invalid token"}}
				frame, _ := EncodeJSONRPCFrame(resp, 1, decoded.ID)
				_ = conn.Write(ctx, websocket.MessageBinary, frame)
				continue
			}
			info := HandshakeInfo{Platform: "linux", Arch: "x64", NodeVersion: "v22.0.0", AgentVersion: "5.0.0", SessionID: "sess-test"}
			result, _ := json.Marshal(info)
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
			frame, _ := EncodeJSONRPCFrame(resp, 1, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
			continue
		}

		result, known := f.results[req.Method]
		if !known {
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method}}
			frame, _ := EncodeJSONRPCFrame(resp, 2, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
			continue
		}
		encodedResult, _ := json.Marshal(result)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: encodedResult}
		frame, _ := EncodeJSONRPCFrame(resp, 2, decoded.ID)
		_ = conn.Write(ctx, websocket.MessageBinary, frame)
	}
}

func startFakeAgent(t *testing.T, agent *fakeAgent) (host string, port int) {
	t.Helper()
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
	port, err = strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port: %v", err)
	}
	return host, port
}

func testConfig(port int, token string) Config {
	cfg := DefaultConfig()
	cfg.Port = port
	cfg.Token = token
	cfg.OrcaVersion = "test"
	cfg.DialTimeout = 5 * time.Second
	cfg.HandshakeTimeout = 5 * time.Second
	cfg.RequestTimeout = 5 * time.Second
	return cfg
}

func TestClientExecSucceedsAgainstFakeAgent(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"preflight.check": map[string]any{"git": map[string]any{"installed": true}},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	result, err := client.Exec(context.Background(), devServer, "preflight.check", nil)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	git, ok := result["git"].(map[string]any)
	if !ok || git["installed"] != true {
		t.Errorf("result = %+v, want git.installed=true", result)
	}
}

func TestClientHealthReflectsHandshake(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-2", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Error("Health = false, want true against a live fake agent")
	}
}

func TestClientHealthFalseOnAuthFailure(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, "wrong-token"), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-3", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if healthy {
		t.Error("Health = true, want false when the bearer token is wrong")
	}
}

func TestClientExecReturnsClearErrorForUnimplementedMode(t *testing.T) {
	client := New(testConfig(0, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-4", "tenant-1", "example.invalid", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "preflight.check", nil)
	if err == nil {
		t.Fatal("expected an error for direct-websocket mode")
	}
}
