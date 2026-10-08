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

type stubTransport struct{}

func (s *stubTransport) ReadFrame(ctx context.Context) (DecodedFrame, error) {
	<-ctx.Done()
	return DecodedFrame{}, ctx.Err()
}

func (s *stubTransport) WriteFrame(_ context.Context, _ []byte) error { return nil }
func (s *stubTransport) Close(_ string) error                         { return nil }

func TestLastHandshakeInfo_CarriesFeaturesAndProtocol(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	attached := HandshakeInfo{
		Platform:        "linux",
		Arch:            "x64",
		NodeVersion:     "v22.3.0",
		AgentVersion:    "5.0.0",
		Capabilities:    []string{"a"},
		Features:        []string{"b"},
		ProtocolVersion: 2,
		BuildVersion:    "test-build",
	}

	client.AttachTransport("ds-1", "host-1", &stubTransport{}, attached)

	got, ok := client.LastHandshakeInfo("ds-1")
	if !ok {
		t.Fatal("expected ok=true")
	}

	want := usecase.HandshakeInfo{
		Platform:        "linux",
		Arch:            "x64",
		NodeVersion:     "v22.3.0",
		AgentVersion:    "5.0.0",
		Capabilities:    []string{"a"},
		Features:        []string{"b"},
		ProtocolVersion: 2,
		BuildVersion:    "test-build",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestLastHandshakeInfo_ToolsAndCapabilitiesImmutability(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	attached := HandshakeInfo{
		Platform:        "linux",
		Arch:            "x64",
		NodeVersion:     "v22.3.0",
		AgentVersion:    "5.0.0",
		SessionID:       "sess-tools-123",
		Capabilities:    []string{"pty", "fs", "git", "codeintel", "codeintel.gitnexus"},
		Tools:           []string{"gitnexus", "codegraph", "git"},
		Features:        []string{"feat_1"},
		ProtocolVersion: 2,
		BuildVersion:    "build-456",
	}

	client.AttachTransport("ds-tools", "host-tools", &stubTransport{}, attached)

	got, ok := client.LastHandshakeInfo("ds-tools")
	if !ok {
		t.Fatal("expected ok=true")
	}

	if got.SessionID != "sess-tools-123" {
		t.Errorf("expected SessionID sess-tools-123, got %s", got.SessionID)
	}
	wantTools := []string{"gitnexus", "codegraph", "git"}
	if !reflect.DeepEqual(got.Tools, wantTools) {
		t.Errorf("tools = %v, want %v", got.Tools, wantTools)
	}
	wantCaps := []string{"pty", "fs", "git", "codeintel", "codeintel.gitnexus"}
	if !reflect.DeepEqual(got.Capabilities, wantCaps) {
		t.Errorf("capabilities = %v, want %v", got.Capabilities, wantCaps)
	}

	// Mutate returned slices to verify immutability (deep copy)
	got.Tools[0] = "mutated-tool"
	got.Capabilities[0] = "mutated-cap"

	got2, ok2 := client.LastHandshakeInfo("ds-tools")
	if !ok2 {
		t.Fatal("expected ok2=true")
	}
	if got2.Tools[0] != "gitnexus" {
		t.Errorf("internal tools slice was mutated: %v", got2.Tools)
	}
	if got2.Capabilities[0] != "pty" {
		t.Errorf("internal capabilities slice was mutated: %v", got2.Capabilities)
	}
}

func TestLastHandshakeInfo_LegacyHandshake(t *testing.T) {
	client := New(DefaultConfig(), slog.Default())
	t.Cleanup(client.Close)

	// legacy handshake lacks ProtocolVersion, BuildVersion, Features
	attached := HandshakeInfo{
		Platform:     "linux",
		Arch:         "x64",
		NodeVersion:  "v22.3.0",
		AgentVersion: "5.0.0",
		Capabilities: []string{"a"},
	}

	client.AttachTransport("ds-legacy", "host-legacy", &stubTransport{}, attached)

	got, ok := client.LastHandshakeInfo("ds-legacy")
	if !ok {
		t.Fatal("expected ok=true")
	}

	if len(got.Features) != 0 {
		t.Errorf("expected empty features")
	}

	if got.EffectiveProtocolVersion() != 1 {
		t.Errorf("expected effective protocol version 1, got %d", got.EffectiveProtocolVersion())
	}
}
