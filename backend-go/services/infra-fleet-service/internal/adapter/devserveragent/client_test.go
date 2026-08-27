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
	results         map[string]any // method -> result to reply with
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

	// rawParams, if non-nil, overrides the default {id,data,exitCode}
	// pty.* param shape — used by agent.hook tests (TASK-AG-03-03), whose
	// params shape ({worktreeId, providerSession:{key,id}}) is unrelated to
	// pty.* notifications.
	rawParams json.RawMessage
}

func (n fakeAgentNotification) toJSONRPC() JSONRPCNotification {
	if n.rawParams != nil {
		return JSONRPCNotification{JSONRPC: "2.0", Method: n.method, Params: n.rawParams}
	}
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

func testConfig(port int) Config {
	cfg := DefaultConfig()
	cfg.Port = port
	cfg.OrcaVersion = "test"
	cfg.DialTimeout = 5 * time.Second
	cfg.HandshakeTimeout = 5 * time.Second
	cfg.RequestTimeout = 5 * time.Second
	return cfg
}

// fakeStaticTokenSource implements AgentTokenSource, returning the same
// token for every DevServer — TASK-AWS-01-03's per-dial resolution seam,
// stubbed for tests that don't need per-DevServer differentiation. See
// fakeMultiTokenSource below for the regression test that DOES need it.
type fakeStaticTokenSource struct {
	token string
	err   error
}

func (f fakeStaticTokenSource) TokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

// newTestClientWithToken builds a Client with relay-websocket enabled via a
// fixed per-call token — the test-suite replacement for the old
// deployment-wide shared-secret Config field.
func newTestClientWithToken(port int, token string) *Client {
	return New(testConfig(port), slog.Default(), WithAgentTokens(fakeStaticTokenSource{token: token}))
}

func TestClientExecSucceedsAgainstFakeAgent(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"preflight.check": map[string]any{"git": map[string]any{"installed": true}},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
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

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-mnf", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
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

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-2", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
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

	client := newTestClientWithToken(port, "wrong-token")
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-3", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
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
	client := newTestClientWithToken(0, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-4", "tenant-1", "example.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "preflight.check", nil)
	if err == nil {
		t.Fatal("expected an error for direct-websocket mode")
	}
}

// fakeMultiTokenSource maps devServer.ID -> token/error, letting a single
// test drive multiple DevServers with distinct per-DevServer tokens —
// TASK-AWS-01-03's regression coverage against the former shared-token bug.
type fakeMultiTokenSource struct {
	tokens map[string]string
	errs   map[string]error
}

func (f fakeMultiTokenSource) TokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
	if err, ok := f.errs[devServer.ID]; ok {
		return "", err
	}
	return f.tokens[devServer.ID], nil
}

// TestClient_NoRegisteredToken_FailsWithoutDialing covers TASK-AWS-01-03's
// "no registered token" case: TokenFor erroring must fail Exec with a
// clear error, and the fake agent must never even see a connection
// attempt (the token is resolved BEFORE any dial happens).
func TestClient_NoRegisteredToken_FailsWithoutDialing(t *testing.T) {
	agent := &reconnectTestAgent{token: fakeAgentToken}
	server := httptest.NewServer(http.HandlerFunc(agent.handler))
	t.Cleanup(server.Close)
	host, port := hostPortFromURL(t, server.URL)

	tokens := fakeMultiTokenSource{errs: map[string]error{
		"ds-no-token": errors.New("no active agent token registered for this dev server"),
	}}
	client := New(testConfig(port), slog.Default(), WithAgentTokens(tokens))
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-no-token", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "preflight.check", nil)
	if err == nil {
		t.Fatal("expected an error when no token is registered for this dev server")
	}
	if agent.connCount.Load() != 0 {
		t.Errorf("connCount = %d, want 0 — no dial should be attempted when token resolution fails", agent.connCount.Load())
	}
}

// TestClient_TwoDevServers_TwoTokens_ProduceTwoAuthHeaders is the direct
// regression guard for the shared-token bug this task fixes: two different
// DevServers, each with its own token, must each present their OWN bearer
// token on the wire — never the other's.
func TestClient_TwoDevServers_TwoTokens_ProduceTwoAuthHeaders(t *testing.T) {
	agent := &recordingAuthAgent{}
	server := httptest.NewServer(http.HandlerFunc(agent.handler))
	t.Cleanup(server.Close)
	host, port := hostPortFromURL(t, server.URL)

	tokens := fakeMultiTokenSource{tokens: map[string]string{
		"ds-a": "token-for-a",
		"ds-b": "token-for-b",
	}}
	client := New(testConfig(port), slog.Default(), WithAgentTokens(tokens))
	t.Cleanup(client.Close)

	devA, err := domain.NewDevServer("ds-a", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer(ds-a): %v", err)
	}
	devB, err := domain.NewDevServer("ds-b", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer(ds-b): %v", err)
	}

	if _, err := client.Health(context.Background(), devA); err != nil {
		t.Fatalf("Health(ds-a): %v", err)
	}
	if _, err := client.Health(context.Background(), devB); err != nil {
		t.Fatalf("Health(ds-b): %v", err)
	}

	headers := agent.authHeaders()
	if len(headers) != 2 {
		t.Fatalf("expected 2 recorded Authorization headers, got %d: %v", len(headers), headers)
	}
	if headers[0] == headers[1] {
		t.Errorf("expected two different Authorization headers, got the same value twice: %q", headers[0])
	}
	wantA, wantB := "Bearer token-for-a", "Bearer token-for-b"
	if !(contains(headers, wantA) && contains(headers, wantB)) {
		t.Errorf("headers = %v, want them to contain both %q and %q", headers, wantA, wantB)
	}
}

// TestClient_RevokedToken_NextDialAttemptFailsClosed covers TASK-AWS-01-03's
// "resolve fresh on every dial" guarantee: once the token source starts
// erroring (simulating a revoke), the very next dial attempt — triggered
// here by the agent dropping the connection right after the first
// handshake — fails closed with no further successful reconnect, and with
// no process restart involved.
func TestClient_RevokedToken_NextDialAttemptFailsClosed(t *testing.T) {
	agent := &reconnectTestAgent{token: fakeAgentToken}
	server := httptest.NewServer(http.HandlerFunc(agent.handler))
	t.Cleanup(server.Close)
	host, port := hostPortFromURL(t, server.URL)

	tokens := &fakeRevocableTokenSource{token: fakeAgentToken}
	cfg := testConfig(port)
	cfg.ReconnectBaseDelay = 10 * time.Millisecond
	cfg.ReconnectMaxDelay = 20 * time.Millisecond
	client := New(cfg, slog.Default(), WithAgentTokens(tokens))
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-revoke", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil || !healthy {
		t.Fatalf("expected the initial connection to succeed, got healthy=%v err=%v", healthy, err)
	}

	tokens.revoke() // simulate RevokeAgentToken taking effect — no process restart

	// The fake agent drops the connection right after its first successful
	// handshake (reconnectTestAgent's n==1 branch), which triggers
	// handleDisconnect -> backgroundReconnect. With the token now revoked,
	// every reconnect attempt must fail at TokenFor, before ever dialing
	// again.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h, _ := client.Health(context.Background(), devServer); !h {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	healthy, _ = client.Health(context.Background(), devServer)
	if healthy {
		t.Fatal("expected the session to be unhealthy once its token was revoked")
	}
	if got := agent.connCount.Load(); got != 1 {
		t.Errorf("connCount = %d, want exactly 1 — a revoked token must never reconnect successfully again", got)
	}
}

// hostPortFromURL splits an httptest.Server's URL into a dialable
// host/port pair, mirroring startFakeAgent's own parsing.
func hostPortFromURL(t *testing.T, rawURL string) (host string, port int) {
	t.Helper()
	u, err := url.Parse(rawURL)
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

func contains(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

// recordingAuthAgent completes a full agent.handshake against any request,
// recording each connection's Authorization header rather than rejecting
// on mismatch — used to assert two different DevServers present two
// different bearer tokens on the wire (TASK-AWS-01-03's shared-token
// regression guard).
type recordingAuthAgent struct {
	mu      sync.Mutex
	headers []string
}

func (a *recordingAuthAgent) authHeaders() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.headers))
	copy(out, a.headers)
	return out
}

func (a *recordingAuthAgent) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orca-relay" {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	a.headers = append(a.headers, r.Header.Get("Authorization"))
	a.mu.Unlock()

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

	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

// fakeRevocableTokenSource starts by returning a fixed token and switches
// to always-erroring once revoke() is called — simulating
// RevokeAgentToken's immediate, no-restart-required effect
// (TASK-AWS-01-03/SOL-AWS-01).
type fakeRevocableTokenSource struct {
	mu      sync.Mutex
	token   string
	revoked bool
}

func (f *fakeRevocableTokenSource) revoke() {
	f.mu.Lock()
	f.revoked = true
	f.mu.Unlock()
}

func (f *fakeRevocableTokenSource) TokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revoked {
		return "", errors.New("devserveragent: agent token revoked")
	}
	return f.token, nil
}
