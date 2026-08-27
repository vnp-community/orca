package agentwsserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
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
