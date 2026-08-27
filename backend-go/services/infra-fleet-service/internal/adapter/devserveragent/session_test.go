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

// notificationFor builds a JSONRPCNotification with the {id,data,exitCode}
// params shape session.go's routeNotification decodes — mirrors
// fakeAgentNotification.toJSONRPC in client_test.go, but these tests call
// session.routeNotification directly (no network, no fake agent) since
// subscribePty/unsubscribePty/routeNotification only touch in-memory state.
func notificationFor(t *testing.T, method, ptyID, data string, exitCode int32) JSONRPCNotification {
	t.Helper()
	params, err := json.Marshal(map[string]any{"id": ptyID, "data": data, "exitCode": exitCode})
	if err != nil {
		t.Fatalf("marshaling notification params: %v", err)
	}
	return JSONRPCNotification{JSONRPC: "2.0", Method: method, Params: params}
}

// TestSession_RouteNotification_RoutesOnlyToMatchingPtyID is a direct,
// network-free test of the notification demux: two subscribers on the same
// session, keyed by different ptyIds, must each see only their own
// notifications — the core guarantee StreamPty's callers depend on.
func TestSession_RouteNotification_RoutesOnlyToMatchingPtyID(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	chA := sess.subscribePty("pty-a")
	t.Cleanup(func() { sess.unsubscribePty("pty-a", chA) })
	chB := sess.subscribePty("pty-b")
	t.Cleanup(func() { sess.unsubscribePty("pty-b", chB) })

	sess.routeNotification(notificationFor(t, "pty.data", "pty-a", "hello-a", 0))

	select {
	case n := <-chA:
		if n.PtyID != "pty-a" || string(n.Data) != "hello-a" {
			t.Errorf("chA received %+v, want PtyID=pty-a Data=hello-a", n)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for chA to receive its notification")
	}

	select {
	case n := <-chB:
		t.Fatalf("chB (subscribed to a different ptyId) received a notification meant for pty-a: %+v", n)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing was routed to chB
	}
}

// TestSession_UnsubscribePty_ClosesChannelAndStopsRouting checks both halves
// of unsubscribePty's contract: the returned channel is closed (so a caller
// ranging over it terminates), and the session no longer routes further
// notifications for that ptyId anywhere (no leaked entry in s.ptySubs to
// panic or leak on a later routeNotification call).
func TestSession_UnsubscribePty_ClosesChannelAndStopsRouting(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	ch := sess.subscribePty("pty-1")
	sess.unsubscribePty("pty-1", ch)

	if _, ok := <-ch; ok {
		t.Fatal("expected ch to be closed after unsubscribePty")
	}

	sess.ptyMu.Lock()
	_, stillPresent := sess.ptySubs["pty-1"]
	sess.ptyMu.Unlock()
	if stillPresent {
		t.Error("expected ptySubs to have no entry for pty-1 after its only subscriber unsubscribed")
	}

	// A notification arriving after unsubscribe must not panic (send on a
	// closed channel) and must simply be dropped.
	sess.routeNotification(notificationFor(t, "pty.data", "pty-1", "too-late", 0))
}

// TestSession_RouteNotification_NonBlockingWhenSubscriberChannelFull proves
// the read loop's "never block on a slow consumer" contract: filling a
// subscriber's buffered channel to capacity and then routing one more
// notification must return immediately (drop, not block) rather than
// wedging routeNotification (and therefore readLoop) forever.
func TestSession_RouteNotification_NonBlockingWhenSubscriberChannelFull(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	ch := sess.subscribePty("pty-full")
	t.Cleanup(func() { sess.unsubscribePty("pty-full", ch) })

	// subscribePty's channel is buffered 64 (see its doc comment) — fill it
	// without draining, then verify one more route doesn't block.
	for i := 0; i < cap(ch); i++ {
		sess.routeNotification(notificationFor(t, "pty.data", "pty-full", "fill", 0))
	}

	done := make(chan struct{})
	go func() {
		sess.routeNotification(notificationFor(t, "pty.data", "pty-full", "overflow", 0))
		close(done)
	}()

	select {
	case <-done:
		// expected: routeNotification dropped the overflow notification
		// rather than blocking on the full channel.
	case <-time.After(time.Second):
		t.Fatal("routeNotification blocked on a full subscriber channel instead of dropping")
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

	cfg := testConfig(port)
	cfg.ReconnectBaseDelay = 10 * time.Millisecond
	cfg.ReconnectMaxDelay = 50 * time.Millisecond

	client := New(cfg, slog.Default(), WithAgentTokens(fakeStaticTokenSource{token: fakeAgentToken}))
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
