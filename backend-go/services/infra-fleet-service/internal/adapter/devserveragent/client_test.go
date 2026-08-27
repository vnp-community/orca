package devserveragent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
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
	results         map[string]any   // method -> result to reply with
	streamResults   map[string][]any // method -> ordered sequence of results, each sent as its own response frame replying to the same request id (TASK-PW-03-08's git.execStream shape) — takes priority over results for a matching method
	rejectHandshake bool

	// pushNotifications, if set, are sent (no id, matching a real
	// pty.data/pty.exit/pty.replay push) shortly after the handshake
	// completes — TASK-183/187's StreamPty test support. Fixed field names
	// (id/data/exitCode) mirror ptyNotificationParams's FLAGGED best-effort
	// shape (see session.go).
	pushNotifications []fakeAgentNotification

	// receivedParams records the raw params of the most recent request seen
	// per method — lets a test assert the exact field names/values Client
	// sent over the wire (e.g. WritePty/ResizePty/KillPty's {id,...} params),
	// not just that some pre-registered result came back.
	paramsMu       sync.Mutex
	receivedParams map[string]json.RawMessage
}

// lastParams returns the raw params this fake agent most recently received
// for method, decoded into a map for easy field assertions. Fails the test
// if method was never called.
func (f *fakeAgent) lastParams(t *testing.T, method string) map[string]any {
	t.Helper()
	f.paramsMu.Lock()
	raw, ok := f.receivedParams[method]
	f.paramsMu.Unlock()
	if !ok {
		t.Fatalf("fake agent never received a call to %q", method)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding params for %q: %v", method, err)
	}
	return out
}

type fakeAgentNotification struct {
	method   string
	ptyID    string
	data     string
	exitCode int32
}

func (n fakeAgentNotification) toJSONRPC() JSONRPCNotification {
	params, _ := json.Marshal(map[string]any{"id": n.ptyID, "data": n.data, "exitCode": n.exitCode})
	return JSONRPCNotification{JSONRPC: "2.0", Method: n.method, Params: params}
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

			if len(f.pushNotifications) > 0 {
				go func() {
					time.Sleep(20 * time.Millisecond) // give the client time to subscribe before pushing
					for i, n := range f.pushNotifications {
						frame, err := EncodeJSONRPCFrame(n.toJSONRPC(), uint32(100+i), 0)
						if err != nil {
							continue
						}
						if err := conn.Write(context.Background(), websocket.MessageBinary, frame); err != nil {
							return
						}
					}
				}()
			}
			continue
		}

		f.paramsMu.Lock()
		if f.receivedParams == nil {
			f.receivedParams = make(map[string]json.RawMessage)
		}
		f.receivedParams[req.Method] = req.Params
		f.paramsMu.Unlock()

		if frames, isStream := f.streamResults[req.Method]; isStream {
			for i, frameResult := range frames {
				encoded, _ := json.Marshal(frameResult)
				resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: encoded}
				frame, _ := EncodeJSONRPCFrame(resp, uint32(10+i), decoded.ID)
				if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
					return
				}
			}
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

// TestClientExecTranslatesMethodNotFound is the direct regression test for
// TASK-048/TASK-070's "shipped but honestly inert" contract: a real agent
// that doesn't know a method (e.g. today's agent/ build, which has no
// device.*/host.capabilities handler) returns a real JSON-RPC -32601, and
// Exec must translate that into domain.ErrAgentMethodNotFound rather than a
// bare/opaque error, so usecase.EmulatorRelay/usecase.GetHostCapabilities
// can distinguish it from a transport failure via errors.Is.
func TestClientExecTranslatesMethodNotFound(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-mnf", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "device.list", nil)
	if err == nil {
		t.Fatal("expected an error for a method the fake agent doesn't implement")
	}
	if !errors.Is(err, domain.ErrAgentMethodNotFound) {
		t.Errorf("expected errors.Is(err, domain.ErrAgentMethodNotFound), got %v", err)
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
