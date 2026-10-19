package agentwsserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
)

// readsOneMessageAttacher reads the next message on the accepted conn, as an
// attached session would when an agent answers a large fs.readDir.
type readsOneMessageAttacher struct{ got chan error }

func (a *readsOneMessageAttacher) AttachInboundSession(_, _ string, conn *websocket.Conn, _ devserveragent.HandshakeInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	a.got <- err
	conn.CloseNow()
}

// The accepted inbound conn must allow agent messages above coder/websocket's
// 32 KiB default, or one big reply drops the dev server for ~30s.
func TestServer_InboundConn_AcceptsMessagesAboveDefaultReadLimit(t *testing.T) {
	registry := NewRegistry(time.Hour)
	t.Cleanup(registry.Stop)
	registry.Register("tok-big", "ds-1", nil)

	attacher := &readsOneMessageAttacher{got: make(chan error, 1)}
	ts := httptest.NewServer(New(registry, attacher, Config{OrcaVersion: "test"}, nil))
	t.Cleanup(ts.Close)

	conn := dialAndSendHandshake(t, wsURLFor(ts), map[string]any{"agentToken": "tok-big", "platform": "darwin", "agentVersion": "2.0.0"})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err != nil { // handshake response
		t.Fatalf("reading handshake response: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, []byte(strings.Repeat("x", 200*1024))); err != nil {
		t.Fatalf("writing large message: %v", err)
	}
	select {
	case err := <-attacher.got:
		if err != nil {
			t.Fatalf("server failed to read a 200 KiB message: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("attacher never read the message")
	}
}
