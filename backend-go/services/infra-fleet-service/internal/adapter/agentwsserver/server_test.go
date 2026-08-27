package agentwsserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
)

type attachedCall struct {
	devServerID string
	host        string
	info        devserveragent.HandshakeInfo
}

type fakeAttacher struct {
	attached chan attachedCall
}

func newFakeAttacher() *fakeAttacher {
	return &fakeAttacher{attached: make(chan attachedCall, 1)}
}

func (f *fakeAttacher) AttachInboundSession(devServerID, host string, conn *websocket.Conn, info devserveragent.HandshakeInfo) {
	f.attached <- attachedCall{devServerID: devServerID, host: host, info: info}
	conn.Close(websocket.StatusNormalClosure, "test done")
}

// dialAndSendHandshake dials wsURL and writes an agent.handshake frame
// with the given params, mirroring what a real agent's first message
// looks like (see agent-wire-protocol.ts's AgentHandshakeParams).
func dialAndSendHandshake(t *testing.T, wsURL string, params any) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dialing test server: %v", err)
	}

	paramsRaw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	req := devserveragent.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "agent.handshake", Params: paramsRaw}
	frame, err := devserveragent.EncodeJSONRPCFrame(req, 1, 0)
	if err != nil {
		t.Fatalf("encode handshake frame: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatalf("writing handshake frame: %v", err)
	}
	return conn
}

func wsURLFor(ts *httptest.Server) string {
	return "ws" + ts.URL[len("http"):]
}

// TestServer_ValidToken_AttachesInboundSession covers the happy path:
// registered token + correct first message → success response + the
// connection is handed to AttachInboundSession, and the token is consumed
// (single-use).
func TestServer_ValidToken_AttachesInboundSession(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-good", "ds-1", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{
		"agentToken":   "tok-good",
		"platform":     "darwin",
		"agentVersion": "2.0.0",
	})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading handshake response: %v", err)
	}
	frame, err := devserveragent.DecodeFrame(data)
	if err != nil {
		t.Fatalf("decoding response frame: %v", err)
	}
	resp, ok, err := devserveragent.ParseJSONRPCResponse(frame.Payload)
	if err != nil || !ok {
		t.Fatalf("parsing response: ok=%v err=%v", ok, err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}
	var result handshakeOKResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.OK || result.OrcaVersion != "test-version" || result.SessionID == "" {
		t.Errorf("result = %+v, want ok=true orcaVersion=test-version and a non-empty sessionId", result)
	}

	select {
	case call := <-attacher.attached:
		if call.devServerID != "ds-1" {
			t.Errorf("devServerID = %q, want ds-1", call.devServerID)
		}
		if call.info.Platform != "darwin" || call.info.AgentVersion != "2.0.0" {
			t.Errorf("info = %+v, want platform=darwin agentVersion=2.0.0", call.info)
		}
		if call.info.SessionID != result.SessionID {
			t.Errorf("info.SessionID = %q, want it to match the handshake response's sessionId %q", call.info.SessionID, result.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachInboundSession was never called")
	}

	if registry.Has("tok-good") {
		t.Error("token should have been consumed (single-use) after a successful handshake")
	}
}

// TestServer_ValidToken_DefaultsMissingHandshakeFields covers
// runOrcaReceiverHandshake's `params?.platform ?? 'linux'`-style defaults.
func TestServer_ValidToken_DefaultsMissingHandshakeFields(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-bare", "ds-2", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "tok-bare"})
	defer conn.CloseNow()

	select {
	case call := <-attacher.attached:
		if call.info.Platform != "linux" {
			t.Errorf("Platform = %q, want default linux", call.info.Platform)
		}
		if call.info.Arch != "x64" {
			t.Errorf("Arch = %q, want default x64", call.info.Arch)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachInboundSession was never called")
	}
}

// TestServer_InvalidToken_RejectsAndDoesNotAttach covers the auth-failure
// path: JSON-RPC error frame with code -33101, then close 1008, and
// AttachInboundSession must never be called.
func TestServer_InvalidToken_RejectsAndDoesNotAttach(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "bogus-unregistered"})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading error response: %v", err)
	}
	frame, err := devserveragent.DecodeFrame(data)
	if err != nil {
		t.Fatalf("decoding response frame: %v", err)
	}
	resp, ok, err := devserveragent.ParseJSONRPCResponse(frame.Payload)
	if err != nil || !ok {
		t.Fatalf("parsing response: ok=%v err=%v", ok, err)
	}
	if resp.Error == nil || resp.Error.Code != -33101 {
		t.Fatalf("Error = %+v, want code -33101", resp.Error)
	}

	select {
	case <-attacher.attached:
		t.Fatal("AttachInboundSession must not be called for an invalid token")
	case <-time.After(200 * time.Millisecond):
	}

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}
}

// TestServer_WrongFirstMessage_ClosesConnection covers the "first message
// must be agent.handshake" protocol-violation path.
func TestServer_WrongFirstMessage_ClosesConnection(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURLFor(ts), nil)
	if err != nil {
		t.Fatalf("dialing test server: %v", err)
	}
	defer conn.CloseNow()

	req := devserveragent.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "not.a.handshake"}
	frame, err := devserveragent.EncodeJSONRPCFrame(req, 1, 0)
	if err != nil {
		t.Fatalf("encode frame: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatalf("writing frame: %v", err)
	}

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}

	select {
	case <-attacher.attached:
		t.Fatal("AttachInboundSession must not be called for a protocol violation")
	default:
	}
}

// TestServer_BelowMinVersion_RejectsWithHandshakeFailed covers
// TASK-AWS-02-02: an agentVersion below Cfg.MinAgentVersion is rejected
// with code -33100 (handshakeFailedCode) and close 1008, and
// AttachInboundSession is never called.
func TestServer_BelowMinVersion_RejectsWithHandshakeFailed(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-old", "ds-old", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version", MinAgentVersion: "1.0.0"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{
		"agentToken":   "tok-old",
		"agentVersion": "0.9.0",
	})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading error response: %v", err)
	}
	frame, err := devserveragent.DecodeFrame(data)
	if err != nil {
		t.Fatalf("decoding response frame: %v", err)
	}
	resp, ok, err := devserveragent.ParseJSONRPCResponse(frame.Payload)
	if err != nil || !ok {
		t.Fatalf("parsing response: ok=%v err=%v", ok, err)
	}
	if resp.Error == nil || resp.Error.Code != handshakeFailedCode {
		t.Fatalf("Error = %+v, want code %d", resp.Error, handshakeFailedCode)
	}
	if !containsBothVersions(resp.Error.Message, "0.9.0", "1.0.0") {
		t.Errorf("Error.Message = %q, want it to contain both versions", resp.Error.Message)
	}

	select {
	case <-attacher.attached:
		t.Fatal("AttachInboundSession must not be called for a below-minimum version")
	case <-time.After(200 * time.Millisecond):
	}

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}
}

// TestServer_NoAgentVersion_NotRejected covers the fail-open path: a
// handshake with no agentVersion field is never rejected on version
// grounds, even with MinAgentVersion configured.
func TestServer_NoAgentVersion_NotRejected(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-noversion", "ds-noversion", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version", MinAgentVersion: "1.0.0"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "tok-noversion"})
	defer conn.CloseNow()

	select {
	case call := <-attacher.attached:
		if call.devServerID != "ds-noversion" {
			t.Errorf("devServerID = %q, want ds-noversion", call.devServerID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachInboundSession was never called — a missing agentVersion must fail open, not be rejected")
	}
}

// TestServer_AtOrAboveMinVersion_Succeeds covers the boundary: a version
// equal to or newer than MinAgentVersion succeeds unchanged.
func TestServer_AtOrAboveMinVersion_Succeeds(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-current", "ds-current", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version", MinAgentVersion: "1.0.0"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{
		"agentToken":   "tok-current",
		"agentVersion": "1.0.0",
	})
	defer conn.CloseNow()

	select {
	case call := <-attacher.attached:
		if call.devServerID != "ds-current" {
			t.Errorf("devServerID = %q, want ds-current", call.devServerID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachInboundSession was never called for a version at the minimum")
	}
}

func containsBothVersions(msg, a, b string) bool {
	return len(msg) > 0 && strings.Contains(msg, a) && strings.Contains(msg, b)
}

// fakeTokenValidator is an in-memory TokenValidator for TASK-AWS-03-06's
// persistent-token handshake fallback tests.
type fakeTokenValidator struct {
	byHash    map[string]struct{ devServerID, tokenID string }
	revoked   map[string]bool
	touched   []string
	findErr   error
}

func newFakeTokenValidator() *fakeTokenValidator {
	return &fakeTokenValidator{
		byHash:  make(map[string]struct{ devServerID, tokenID string }),
		revoked: make(map[string]bool),
	}
}

func (f *fakeTokenValidator) register(token, devServerID, tokenID string) {
	f.byHash[hashAgentToken(token)] = struct{ devServerID, tokenID string }{devServerID, tokenID}
}

func (f *fakeTokenValidator) revoke(token string) {
	f.revoked[hashAgentToken(token)] = true
}

func (f *fakeTokenValidator) FindActiveByHash(ctx context.Context, hash string) (string, string, bool, error) {
	if f.findErr != nil {
		return "", "", false, f.findErr
	}
	if f.revoked[hash] {
		return "", "", false, nil
	}
	v, ok := f.byHash[hash]
	if !ok {
		return "", "", false, nil
	}
	return v.devServerID, v.tokenID, true, nil
}

func (f *fakeTokenValidator) TouchLastUsed(ctx context.Context, tokenID string) {
	f.touched = append(f.touched, tokenID)
}

// TestServer_PersistentToken_SucceedsAndIsNonSingleUse covers
// TASK-AWS-03-06: a handshake succeeds against a persistent (non-Registry)
// token, and succeeds again on a second handshake with the same token
// (proves it is non-single-use, unlike Registry's Consume).
func TestServer_PersistentToken_SucceedsAndIsNonSingleUse(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)

	tokens := newFakeTokenValidator()
	tokens.register("persistent-tok", "ds-persistent", "tok-id-1")

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	srv.Tokens = tokens
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	for i := 0; i < 2; i++ {
		conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "persistent-tok"})
		select {
		case call := <-attacher.attached:
			if call.devServerID != "ds-persistent" {
				t.Errorf("attempt %d: devServerID = %q, want ds-persistent", i, call.devServerID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("attempt %d: AttachInboundSession was never called", i)
		}
		conn.CloseNow()
	}
	if len(tokens.touched) != 2 {
		t.Errorf("expected TouchLastUsed to be called twice, got %d", len(tokens.touched))
	}
}

// TestServer_RevokedPersistentToken_Rejected covers TASK-AWS-03-06: a
// revoked persistent token's handshake is rejected exactly like an
// unregistered one.
func TestServer_RevokedPersistentToken_Rejected(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)

	tokens := newFakeTokenValidator()
	tokens.register("revoked-tok", "ds-revoked", "tok-id-2")
	tokens.revoke("revoked-tok")

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	srv.Tokens = tokens
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "revoked-tok"})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("reading error response: %v", err)
	}
	_, _, err := conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}
	select {
	case <-attacher.attached:
		t.Fatal("AttachInboundSession must not be called for a revoked token")
	default:
	}
}

// fakeSessionCounter is a stubbed SessionCounter for TASK-AWS-02-03's
// capacity tests.
type fakeSessionCounter struct {
	count int
}

func (f *fakeSessionCounter) LiveSessionCount() int { return f.count }

// TestServer_AtCapacity_RejectsBeforeHandshake covers TASK-AWS-02-03: a
// SessionCounter stubbed at the configured cap rejects new connections
// with 1008 before the handshake read even starts — Registry.Consume must
// never be reached (the reject is pre-auth), asserted here by the token
// remaining unconsumed.
func TestServer_AtCapacity_RejectsBeforeHandshake(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-cap", "ds-cap", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version", MaxConcurrentSessions: 2}, nil)
	srv.Sessions = &fakeSessionCounter{count: 2}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURLFor(ts), nil)
	if err != nil {
		t.Fatalf("dialing test server: %v", err)
	}
	defer conn.CloseNow()

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}

	if !registry.Has("tok-cap") {
		t.Error("token should NOT have been consumed — a capacity reject must happen before Registry.Consume is ever reached")
	}
	select {
	case <-attacher.attached:
		t.Fatal("AttachInboundSession must not be called when at capacity")
	default:
	}
}

// TestServer_MaxConcurrentSessionsDisabled_NoRegression covers
// MaxConcurrentSessions <= 0 leaving the check fully disabled (existing
// behavior, no regression), even with a SessionCounter reporting a huge
// count.
func TestServer_MaxConcurrentSessionsDisabled_NoRegression(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-nolimit", "ds-nolimit", nil)

	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version", MaxConcurrentSessions: 0}, nil)
	srv.Sessions = &fakeSessionCounter{count: 999999}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "tok-nolimit"})
	defer conn.CloseNow()

	select {
	case call := <-attacher.attached:
		if call.devServerID != "ds-nolimit" {
			t.Errorf("devServerID = %q, want ds-nolimit", call.devServerID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AttachInboundSession was never called — MaxConcurrentSessions<=0 must disable the check entirely")
	}
}

// TestServer_MalformedFirstFrame_ClosesConnection covers a first message
// that isn't even a decodable frame at all (e.g. too short / garbage).
func TestServer_MalformedFirstFrame_ClosesConnection(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	attacher := newFakeAttacher()
	srv := New(registry, attacher, Config{OrcaVersion: "test-version"}, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURLFor(ts), nil)
	if err != nil {
		t.Fatalf("dialing test server: %v", err)
	}
	defer conn.CloseNow()

	if err := conn.Write(ctx, websocket.MessageBinary, []byte("not a frame")); err != nil {
		t.Fatalf("writing garbage: %v", err)
	}

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v (err=%v), want %v", status, err, websocket.StatusPolicyViolation)
	}
}
