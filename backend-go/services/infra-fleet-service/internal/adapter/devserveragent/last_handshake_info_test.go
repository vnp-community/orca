package devserveragent

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// TestLastHandshakeInfo_HandshakedSessionReturnsInfo mirrors
// inbound_test.go's AttachInboundSession setup — a session that has
// completed handshake returns (info, true) matching what attachTransport
// received.
func TestLastHandshakeInfo_HandshakedSessionReturnsInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dialing fake inbound agent: %v", err)
	}

	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-handshake-1", "tenant-1", "unused.invalid", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	attached := HandshakeInfo{Platform: "linux", Arch: "x64", NodeVersion: "v22.3.0", AgentVersion: "5.0.0"}
	client.AttachInboundSession(devServer.ID, devServer.Host, conn, attached)

	got, ok := client.LastHandshakeInfo(devServer.ID)
	if !ok {
		t.Fatal("expected LastHandshakeInfo to report ok=true for a handshaked session")
	}
	want := usecase.HandshakeInfo{Platform: "linux", Arch: "x64", NodeVersion: "v22.3.0", AgentVersion: "5.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// TestLastHandshakeInfo_UnknownDevServerReturnsFalse covers the "no session
// exists at all" case.
func TestLastHandshakeInfo_UnknownDevServerReturnsFalse(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	info, ok := client.LastHandshakeInfo("no-such-dev-server")
	if ok {
		t.Errorf("expected ok=false for an unknown dev server, got info=%+v", info)
	}
	if !reflect.DeepEqual(info, usecase.HandshakeInfo{}) {
		t.Errorf("expected a zero-value HandshakeInfo, got %+v", info)
	}
}
