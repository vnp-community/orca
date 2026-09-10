package devserveragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// pipeTransport is a real, minimal in-memory Transport — two buffered
// channels standing in for the wire — enough to prove
// getOrProvisionSession's dispatch/reuse logic without a real SSH
// connection (that end-to-end coverage lives in adapter/sshrelay's own
// tests, against a real fake SSH server).
type pipeTransport struct {
	writes chan []byte
	reads  chan DecodedFrame
	closed chan struct{}
}

func newPipeTransport() *pipeTransport {
	return &pipeTransport{
		writes: make(chan []byte, 8),
		reads:  make(chan DecodedFrame, 8),
		closed: make(chan struct{}),
	}
}

func (t *pipeTransport) ReadFrame(ctx context.Context) (DecodedFrame, error) {
	select {
	case f := <-t.reads:
		return f, nil
	case <-t.closed:
		return DecodedFrame{}, errors.New("pipeTransport: closed")
	case <-ctx.Done():
		return DecodedFrame{}, ctx.Err()
	}
}

func (t *pipeTransport) WriteFrame(_ context.Context, frame []byte) error {
	select {
	case t.writes <- frame:
		return nil
	case <-t.closed:
		return errors.New("pipeTransport: closed")
	}
}

func (t *pipeTransport) Close(_ string) error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}
	return nil
}

// respondToNextCall reads one outgoing frame (a JSON-RPC request written by
// session.call) and pushes back a matching success response — lets tests
// drive Exec/Health through a full round trip over pipeTransport. Runs on
// its own goroutine in every caller (session.call blocks synchronously
// waiting for the response), so it reports failures via tb.Errorf rather
// than the Fatal family — calling Fatal/FailNow off the main test goroutine
// is a vet error (it calls runtime.Goexit on the wrong goroutine).
func (t *pipeTransport) respondToNextCall(tb testing.TB, result map[string]any) {
	tb.Helper()
	frame := <-t.writes
	decoded, err := DecodeFrame(frame)
	if err != nil {
		tb.Errorf("decoding frame written by session.call: %v", err)
		return
	}
	var req JSONRPCRequest
	if err := json.Unmarshal(decoded.Payload, &req); err != nil {
		tb.Errorf("unmarshaling request: %v", err)
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		tb.Errorf("marshaling fake result: %v", err)
		return
	}
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	respFrame, err := EncodeJSONRPCFrame(resp, decoded.ID, decoded.ID)
	if err != nil {
		tb.Errorf("encoding fake response: %v", err)
		return
	}
	respDecoded, err := DecodeFrame(respFrame)
	if err != nil {
		tb.Errorf("decoding fake response frame: %v", err)
		return
	}
	t.reads <- respDecoded
}

// fakeSshProvisioner is an in-memory devserveragent.SshProvisioner +
// SshReattacher (sshReattacherAndProvisioner) — WithRelaySSH requires both.
type fakeSshProvisioner struct {
	transport  *pipeTransport
	info       HandshakeInfo
	err        error
	provisions int

	mu              sync.Mutex
	reattachTrans   Transport
	reattachErr     error
	reattaches      int
	reattachSockets []string
}

func (f *fakeSshProvisioner) Provision(context.Context, domain.DevServer) (Transport, HandshakeInfo, error) {
	f.provisions++
	if f.err != nil {
		return nil, HandshakeInfo{}, f.err
	}
	return f.transport, f.info, nil
}

func (f *fakeSshProvisioner) Reattach(_ context.Context, _ domain.DevServer, sockPath string) (Transport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reattaches++
	f.reattachSockets = append(f.reattachSockets, sockPath)
	if f.reattachErr != nil {
		return nil, f.reattachErr
	}
	return f.reattachTrans, nil
}

func (f *fakeSshProvisioner) reattachCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reattaches
}

func relaySSHDevServer(t *testing.T, id string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer(id, "tenant-1", "unused", domain.ConnectionModeRelaySSH, "ssht-1", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	return ds
}

func TestClient_RelaySSH_NotEnabled_ReturnsClearError(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	devServer := relaySSHDevServer(t, "ds-1")

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if healthy {
		t.Error("Health = true, want false when relay-ssh was never enabled via WithRelaySSH")
	}

	if _, err := client.Exec(context.Background(), devServer, "shell.exec", nil); !errors.Is(err, ErrConnectionModeNotImplemented) {
		t.Errorf("Exec error = %v, want ErrConnectionModeNotImplemented", err)
	}
}

func TestClient_RelaySSH_ProvisionsOnceThenReusesLiveSession(t *testing.T) {
	transport := newPipeTransport()
	provisioner := &fakeSshProvisioner{transport: transport, info: HandshakeInfo{Platform: "linux"}}
	client := New(DefaultConfig(), slog.Default(), WithRelaySSH(provisioner))
	t.Cleanup(client.Close)

	devServer := relaySSHDevServer(t, "ds-2")

	go transport.respondToNextCall(t, map[string]any{"branch": "main"})
	result, err := client.Exec(context.Background(), devServer, "git.status", nil)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if result["branch"] != "main" {
		t.Errorf("result = %+v, want branch=main", result)
	}

	// Second call against the same still-live session must NOT re-provision.
	go transport.respondToNextCall(t, map[string]any{"ok": true})
	if _, err := client.Exec(context.Background(), devServer, "preflight.check", nil); err != nil {
		t.Fatalf("second Exec: %v", err)
	}

	if provisioner.provisions != 1 {
		t.Errorf("provisions = %d, want exactly 1 (second call should reuse the live session)", provisioner.provisions)
	}
}

func TestClient_RelaySSH_ProvisionFailurePropagates(t *testing.T) {
	provisioner := &fakeSshProvisioner{err: errors.New("sshrelay: dialing ssh target: connection refused")}
	client := New(DefaultConfig(), slog.Default(), WithRelaySSH(provisioner))
	t.Cleanup(client.Close)

	devServer := relaySSHDevServer(t, "ds-3")

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v, want (false, nil) — a provision failure is the expected common answer, not an error the caller must branch on", err)
	}
	if healthy {
		t.Error("Health = true, want false when provisioning fails")
	}

	if _, err := client.Exec(context.Background(), devServer, "shell.exec", nil); err == nil {
		t.Fatal("expected Exec to propagate the provisioner's error")
	}
}

// TestSession_RelaySSHReconnect_ReattachesWithoutRedeploy mirrors
// TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry
// (session_test.go) for relay-ssh's managedModeRelaySSHReattach path:
// dropping the transport must call Reattach, never Provision again.
func TestSession_RelaySSHReconnect_ReattachesWithoutRedeploy(t *testing.T) {
	firstTransport := newPipeTransport()
	secondTransport := newPipeTransport()
	provisioner := &fakeSshProvisioner{
		transport:     firstTransport,
		info:          HandshakeInfo{Platform: "linux", SockPath: "/tmp/orca-relay/relay.sock"},
		reattachTrans: secondTransport,
	}

	cfg := DefaultConfig()
	cfg.ReconnectBaseDelay = 10 * time.Millisecond
	cfg.ReconnectMaxDelay = 50 * time.Millisecond
	client := New(cfg, slog.Default(), WithRelaySSH(provisioner))
	t.Cleanup(client.Close)

	devServer := relaySSHDevServer(t, "ds-reattach-1")

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Fatal("expected the initial provision to handshake successfully")
	}
	if provisioner.provisions != 1 {
		t.Fatalf("provisions = %d, want 1", provisioner.provisions)
	}

	// Simulate an SSH-drop: close the first transport out from under the
	// session, same as readLoop observing a real ReadFrame error.
	_ = firstTransport.Close("simulated ssh drop")

	deadline := time.After(2 * time.Second)
	for provisioner.reattachCount() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for relaySSHReconnect to call Reattach")
		case <-time.After(5 * time.Millisecond):
		}
	}

	if provisioner.provisions != 1 {
		t.Errorf("provisions = %d, want still exactly 1 (reconnect must reattach, not re-provision)", provisioner.provisions)
	}
	if got := provisioner.reattachSockets[0]; got != "/tmp/orca-relay/relay.sock" {
		t.Errorf("Reattach's sockPath = %q, want the cached HandshakeInfo.SockPath", got)
	}

	// The session should be live again on the SECOND transport without any
	// caller having to retry — same "recovers on its own" assertion
	// TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry
	// makes for relay-websocket.
	go secondTransport.respondToNextCall(t, map[string]any{"ok": true})
	if _, err := client.Exec(context.Background(), devServer, "preflight.check", nil); err != nil {
		t.Fatalf("Exec after reattach: %v", err)
	}
}

// TestSession_RelaySSHReconnect_DetachedProcessGoneStopsLooping asserts the
// ErrRelayDetachedProcessGone path returns without looping further, leaving
// the session non-live for the next caller's full re-Provision.
func TestSession_RelaySSHReconnect_DetachedProcessGoneStopsLooping(t *testing.T) {
	firstTransport := newPipeTransport()
	provisioner := &fakeSshProvisioner{
		transport:   firstTransport,
		info:        HandshakeInfo{Platform: "linux", SockPath: "/tmp/orca-relay/relay.sock"},
		reattachErr: fmt.Errorf("sshrelay: reattach: %w", ErrRelayDetachedProcessGone),
	}

	cfg := DefaultConfig()
	cfg.ReconnectBaseDelay = 10 * time.Millisecond
	cfg.ReconnectMaxDelay = 20 * time.Millisecond
	client := New(cfg, slog.Default(), WithRelaySSH(provisioner))
	t.Cleanup(client.Close)

	devServer := relaySSHDevServer(t, "ds-reattach-gone")

	healthy, err := client.Health(context.Background(), devServer)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Fatal("expected the initial provision to handshake successfully")
	}

	_ = firstTransport.Close("simulated ssh drop")

	deadline := time.After(2 * time.Second)
	for provisioner.reattachCount() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for relaySSHReconnect's first Reattach attempt")
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Give the loop a chance to retry (it must NOT — it should have returned
	// after the first ErrRelayDetachedProcessGone).
	time.Sleep(100 * time.Millisecond)
	if got := provisioner.reattachCount(); got != 1 {
		t.Errorf("reattachCount = %d, want exactly 1 (loop must stop on ErrRelayDetachedProcessGone, not keep retrying)", got)
	}
}
