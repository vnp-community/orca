package wscompat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func wsURLFor(ts *httptest.Server) string {
	return "ws" + ts.URL[len("http"):]
}

// newAcceptingTestServer spins up a plain WS-upgrade-only httptest.Server —
// no auth, no dispatch — and hands each accepted *websocket.Conn back over
// connCh, so a test can drive push_bridge.go's functions directly against a
// real connection (pipePush takes a concrete *websocket.Conn, not an
// interface, so a fake/mock conn isn't an option here).
func newAcceptingTestServer(t *testing.T) (*httptest.Server, <-chan *websocket.Conn) {
	t.Helper()
	connCh := make(chan *websocket.Conn, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		connCh <- conn
		// Keep the handler (and so the underlying HTTP response/conn) alive
		// until the client disconnects, since pipePush's caller owns
		// closing the connection in every test below.
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)
	return ts, connCh
}

func dialTestClient(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURLFor(ts), nil)
	if err != nil {
		t.Fatalf("dialing test server: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func TestPipePush_ForwardsEventsInOrder(t *testing.T) {
	ts, connCh := newAcceptingTestServer(t)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn := <-connCh
	defer serverConn.CloseNow()

	events := make(chan PushEvent)
	done := make(chan struct{})
	var mu sync.Mutex
	go func() {
		pipePush(ctx, serverConn, &mu, events)
		close(done)
	}()

	for i := 0; i < 3; i++ {
		events <- PushEvent{Channel: "test.channel", Args: []any{i}}
	}

	for i := 0; i < 3; i++ {
		var msg PushMessage
		if err := wsjson.Read(ctx, client, &msg); err != nil {
			t.Fatalf("reading push frame %d: %v", i, err)
		}
		if msg.Type != "push" || msg.Channel != "test.channel" {
			t.Fatalf("frame %d: got %+v, want type=push channel=test.channel", i, msg)
		}
		gotIdx, ok := msg.Args[0].(float64) // JSON numbers decode as float64
		if !ok || int(gotIdx) != i {
			t.Fatalf("frame %d: args = %v, want [%d]", i, msg.Args, i)
		}
	}

	close(events)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipePush did not return after events channel closed")
	}
}

func TestPipePush_ReturnsOnContextCancel(t *testing.T) {
	ts, connCh := newAcceptingTestServer(t)
	_ = dialTestClient(t, ts)
	serverConn := <-connCh
	defer serverConn.CloseNow()

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan PushEvent)
	done := make(chan struct{})
	var mu sync.Mutex
	go func() {
		pipePush(ctx, serverConn, &mu, events)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipePush did not return promptly after ctx was cancelled — possible goroutine leak")
	}
}

func TestPipePush_ReturnsOnChannelClose(t *testing.T) {
	ts, connCh := newAcceptingTestServer(t)
	_ = dialTestClient(t, ts)
	serverConn := <-connCh
	defer serverConn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan PushEvent)
	done := make(chan struct{})
	var mu sync.Mutex
	go func() {
		pipePush(ctx, serverConn, &mu, events)
		close(done)
	}()

	close(events)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipePush did not return after events channel was closed")
	}
}
