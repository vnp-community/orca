package devserveragent

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// closeCodeSeenByAgent attaches a server-side wsTransport to a session, runs end(), and
// returns the close code the dialing agent observes.
func closeCodeSeenByAgent(t *testing.T, end func(*session)) websocket.StatusCode {
	t.Helper()
	sessCh := make(chan *session, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		s := newSession("h", Config{}, slog.Default())
		s.transport = newWSTransport(conn, slog.Default())
		sessCh <- s
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	agent, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.CloseNow()
	go end(<-sessCh)

	_, _, err = agent.Read(ctx)
	return websocket.CloseStatus(err)
}

// The agent exits on 1000 and reconnects on anything else, so a service restart must not send 1000.
func TestSessionShutdownAsksAgentToReconnect(t *testing.T) {
	if got := closeCodeSeenByAgent(t, func(s *session) { s.shutdown() }); got != websocket.StatusGoingAway {
		t.Fatalf("shutdown close code = %d, want %d (going away)", got, websocket.StatusGoingAway)
	}
}

func TestSessionCloseStillEndsAgentCleanly(t *testing.T) {
	if got := closeCodeSeenByAgent(t, func(s *session) { s.close() }); got != websocket.StatusNormalClosure {
		t.Fatalf("close code = %d, want %d (normal)", got, websocket.StatusNormalClosure)
	}
}
