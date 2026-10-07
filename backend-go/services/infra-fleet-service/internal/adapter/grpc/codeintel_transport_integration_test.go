//go:build integration

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

type staticTokenSource struct {
	token string
}

func (s *staticTokenSource) TokenFor(ctx context.Context, devServer domain.DevServer) (string, error) {
	return s.token, nil
}

type integrationFakeAgent struct {
	mu           sync.Mutex
	conn         *websocket.Conn
	requireToken string
	codeintelErr *devserveragent.JSONRPCError
}

func (f *integrationFakeAgent) handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orca-relay" {
		http.NotFound(w, r)
		return
	}
	if f.requireToken != "" && r.Header.Get("Authorization") != "Bearer "+f.requireToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	ctx := context.Background()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		decoded, err := devserveragent.DecodeFrame(data)
		if err != nil || decoded.Type != devserveragent.MessageTypeRegular {
			continue
		}
		var req devserveragent.JSONRPCRequest
		if err := json.Unmarshal(decoded.Payload, &req); err != nil {
			continue
		}

		if req.Method == "agent.handshake" {
			info := devserveragent.HandshakeInfo{
				Platform:        "darwin",
				Arch:            "arm64",
				NodeVersion:     "v22.0.0",
				AgentVersion:    "5.0.0",
				SessionID:       "sess-integration-1",
				Tools:           []string{"gitnexus", "codegraph"},
				Capabilities:    []string{"codeintel", "symbols"},
				ProtocolVersion: 1,
				BuildVersion:    "1.0.0",
			}
			result, _ := json.Marshal(info)
			resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
			frame, _ := devserveragent.EncodeJSONRPCFrame(resp, 1, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
			continue
		}

		if req.Method == "codeintel.symbol" {
			f.mu.Lock()
			rpcErr := f.codeintelErr
			f.mu.Unlock()
			if rpcErr != nil {
				resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}
				frame, _ := devserveragent.EncodeJSONRPCFrame(resp, 2, decoded.ID)
				_ = conn.Write(ctx, websocket.MessageBinary, frame)
				continue
			}
			resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"symbols":[]}`)}
			frame, _ := devserveragent.EncodeJSONRPCFrame(resp, 2, decoded.ID)
			_ = conn.Write(ctx, websocket.MessageBinary, frame)
			continue
		}

		resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
		frame, _ := devserveragent.EncodeJSONRPCFrame(resp, 2, decoded.ID)
		_ = conn.Write(ctx, websocket.MessageBinary, frame)
	}
}

func (f *integrationFakeAgent) pushNotification(ctx context.Context, notif devserveragent.JSONRPCNotification) error {
	f.mu.Lock()
	conn := f.conn
	f.mu.Unlock()
	if conn == nil {
		return errors.New("no active websocket connection")
	}
	frame, err := devserveragent.EncodeJSONRPCFrame(notif, 10, 0)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageBinary, frame)
}

type integrationDevServerRepository struct {
	ds domain.DevServer
}

func (r *integrationDevServerRepository) Register(ctx context.Context, devServer domain.DevServer) (domain.DevServer, error) {
	return devServer, nil
}
func (r *integrationDevServerRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return r.ds, nil
}
func (r *integrationDevServerRepository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (r *integrationDevServerRepository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (r *integrationDevServerRepository) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerHealthStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	return nil
}
func (r *integrationDevServerRepository) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	return nil, nil
}
func (r *integrationDevServerRepository) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (r *integrationDevServerRepository) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (r *integrationDevServerRepository) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (r *integrationDevServerRepository) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	return nil, nil
}

func TestCodeIntelTransport_Integration(t *testing.T) {
	fakeAgent := &integrationFakeAgent{
		requireToken: "secret-agent-token",
	}
	ts := httptest.NewServer(http.HandlerFunc(fakeAgent.handler))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := devserveragent.DefaultConfig()
	cfg.Port = port
	cfg.DialTimeout = 5 * time.Second
	cfg.HandshakeTimeout = 5 * time.Second

	agentClient := devserveragent.New(
		cfg,
		logger,
		devserveragent.WithAgentTokens(&staticTokenSource{token: "secret-agent-token"}),
	)
	defer agentClient.Close()

	devServers := &integrationDevServerRepository{
		ds: domain.DevServer{
			ID:       "ds-integ",
			TenantID: "tenant-integ",
			Host:     host,
			Mode:     domain.ConnectionModeRelayWebSocket,
		},
	}

	limiter := usecase.NewCodeIntelStreamLimiter(16)
	relayUC := usecase.NewRelayByDevServer(devServers, agentClient)
	streamUC := usecase.NewStreamCodeIntelEvents(devServers, agentClient, limiter)
	capsUC := usecase.NewGetAgentCapabilities(devServers, agentClient)

	server := &Server{
		relayByDevServer:      relayUC,
		streamCodeIntelEvents: streamUC,
		getAgentCapabilities: capsUC,
	}

	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	grpcServer := grpc.NewServer(grpcmw.ChainUnary(logger))
	infrafleetv1.RegisterInfraFleetServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(lis) }()
	defer grpcServer.Stop()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := infrafleetv1.NewInfraFleetServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcmw.MetadataTenantID, "tenant-integ"))

	// Establish session so IsConnected becomes true and LastHandshakeInfo is recorded
	reachable, err := agentClient.Health(ctx, devServers.ds)
	if err != nil || !reachable {
		t.Fatalf("agentClient.Health: reachable=%v, err=%v", reachable, err)
	}

	// 1. GetAgentCapabilities: verifies handshake propagation to gRPC caller
	t.Run("GetAgentCapabilities", func(t *testing.T) {
		resp, err := client.GetAgentCapabilities(callCtx, &infrafleetv1.GetAgentCapabilitiesRequest{
			DevServerId: "ds-integ",
		})
		if err != nil {
			t.Fatalf("GetAgentCapabilities: %v", err)
		}
		if len(resp.Tools) != 2 || resp.Tools[0] != "gitnexus" || resp.Tools[1] != "codegraph" {
			t.Errorf("unexpected tools: %v", resp.Tools)
		}
		if len(resp.Capabilities) != 2 || resp.Capabilities[0] != "codeintel" {
			t.Errorf("unexpected capabilities: %v", resp.Capabilities)
		}
	})

	// 2. RelayByDevServer: verifies agent JSON-RPC error mapping and trailer propagation
	t.Run("RelayByDevServer_ErrorCodeAndTrailer", func(t *testing.T) {
		fakeAgent.mu.Lock()
		fakeAgent.codeintelErr = &devserveragent.JSONRPCError{
			Code:    -32000,
			Message: "index not found on host",
			Data:    json.RawMessage(`{"code":"CODEINTEL_INDEX_MISSING","reason":"unindexed"}`),
		}
		fakeAgent.mu.Unlock()

		var trailer metadata.MD
		_, err := client.RelayByDevServer(
			callCtx,
			&infrafleetv1.RelayByDevServerRequest{
				DevServerId: "ds-integ",
				Method:      "codeintel.symbol",
				ParamsJson:  `{"name":"query"}`,
			},
			grpc.Trailer(&trailer),
		)
		if err == nil {
			t.Fatal("expected error from RelayByDevServer, got nil")
		}
		t.Logf("RelayByDevServer returned error: %v", err)

		trailers := trailer.Get("x-orca-agent-error-data-bin")
		if len(trailers) == 0 {
			t.Fatalf("expected trailer x-orca-agent-error-data-bin, got none")
		}
		var trailerData map[string]any
		if err := json.Unmarshal([]byte(trailers[0]), &trailerData); err != nil {
			t.Fatalf("unmarshaling trailer json: %v", err)
		}
		if trailerData["code"] != "CODEINTEL_INDEX_MISSING" {
			t.Errorf("expected CODEINTEL_INDEX_MISSING in trailer data, got %v", trailerData["code"])
		}
	})

	// 3. StreamCodeIntelEvents: verifies event streaming from agent notification to gRPC client
	t.Run("StreamCodeIntelEvents", func(t *testing.T) {
		stream, err := client.StreamCodeIntelEvents(callCtx, &infrafleetv1.StreamCodeIntelEventsRequest{
			DevServerId: "ds-integ",
		})
		if err != nil {
			t.Fatalf("StreamCodeIntelEvents: %v", err)
		}

		// Push notification via agent websocket
		time.Sleep(50 * time.Millisecond) // allow subscription to register
		notif := devserveragent.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "codeintel.indexChanged",
			Params:  json.RawMessage(`{"workspaceRoot":"/Users/dev/repo","tool":"gitnexus","commit":"commit-123","reason":"head_changed","headCommit":"commit-456","stale":true}`),
		}
		if err := fakeAgent.pushNotification(ctx, notif); err != nil {
			t.Fatalf("pushNotification: %v", err)
		}

		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("stream.Recv: %v", err)
		}
		if ev.Kind != "index_changed" {
			t.Errorf("ev.Kind = %q, want index_changed", ev.Kind)
		}
		if ev.Tool != "gitnexus" || ev.Commit != "commit-123" || ev.HeadCommit != "commit-456" || !ev.Stale {
			t.Errorf("unexpected event payload: %+v", ev)
		}
	})
}
