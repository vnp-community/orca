package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

type relayFakeDevServerRepository struct {
	ds domain.DevServer
}

func (r *relayFakeDevServerRepository) Register(ctx context.Context, devServer domain.DevServer) (domain.DevServer, error) {
	return devServer, nil
}
func (r *relayFakeDevServerRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return r.ds, nil
}
func (r *relayFakeDevServerRepository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (r *relayFakeDevServerRepository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (r *relayFakeDevServerRepository) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerHealthStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	return nil
}
func (r *relayFakeDevServerRepository) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	return nil, nil
}
func (r *relayFakeDevServerRepository) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (r *relayFakeDevServerRepository) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (r *relayFakeDevServerRepository) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (r *relayFakeDevServerRepository) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	return nil, nil
}

type relayFakeDevServerAgentClient struct {
	fakeDevServerAgentClient
	execErr error
}

func (f *relayFakeDevServerAgentClient) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	return nil, f.execErr
}

func (f *relayFakeDevServerAgentClient) IsConnected(devServerID string) bool {
	return true
}

func TestServer_RelayByDevServer_SetsTrailerOnAgentRPCError(t *testing.T) {
	ds := domain.DevServer{
		ID:       "ds-1",
		TenantID: "t-1",
	}
	devServers := &relayFakeDevServerRepository{ds: ds}
	agent := &relayFakeDevServerAgentClient{
		execErr: &domain.AgentRPCError{
			Code:    -32000,
			Message: "ambiguous symbol resolution",
			Data:    json.RawMessage(`{"code":"CODEINTEL_AMBIGUOUS_SYMBOL","count":3}`),
		},
	}

	relayUC := usecase.NewRelayByDevServer(devServers, agent)
	server := &Server{
		relayByDevServer: relayUC,
	}

	var capturedMD metadata.MD
	ctx := metadata.NewIncomingContext(
		tenant.WithTenantID(context.Background(), "t-1"),
		metadata.MD{},
	)
	stream := &mockServerTransportStream{
		setTrailerFunc: func(md metadata.MD) {
			capturedMD = metadata.Join(capturedMD, md)
		},
	}
	ctx = grpc.NewContextWithServerTransportStream(ctx, stream)

	req := &infrafleetv1.RelayByDevServerRequest{
		DevServerId: "ds-1",
		Method:      "codeintel.symbol",
		ParamsJson:  `{"name":"calc"}`,
	}

	_, err := server.RelayByDevServer(ctx, req)
	if err == nil {
		t.Fatal("expected error from RelayByDevServer, got nil")
	}

	trailers := capturedMD.Get("x-orca-agent-error-data-bin")
	if len(trailers) == 0 {
		t.Fatalf("expected trailer x-orca-agent-error-data-bin to be set, got empty")
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(trailers[0]), &data); err != nil {
		t.Fatalf("trailer value is not valid JSON: %v", err)
	}
	if data["code"] != "CODEINTEL_AMBIGUOUS_SYMBOL" {
		t.Errorf("expected trailer code CODEINTEL_AMBIGUOUS_SYMBOL, got %v", data["code"])
	}
}

type mockServerTransportStream struct {
	setTrailerFunc func(md metadata.MD)
}

func (m *mockServerTransportStream) Method() string                  { return "" }
func (m *mockServerTransportStream) SetHeader(md metadata.MD) error  { return nil }
func (m *mockServerTransportStream) SendHeader(md metadata.MD) error { return nil }
func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error {
	if m.setTrailerFunc != nil {
		m.setTrailerFunc(md)
	}
	return nil
}
