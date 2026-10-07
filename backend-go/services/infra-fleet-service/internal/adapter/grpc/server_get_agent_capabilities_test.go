package grpc

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

type capsFakeAgentClient struct {
	fakeDevServerAgentClient
	isConnected       bool
	lastHandshakeOK   bool
	lastHandshakeInfo usecase.HandshakeInfo
}

func (c *capsFakeAgentClient) IsConnected(devServerID string) bool {
	return c.isConnected
}

func (c *capsFakeAgentClient) LastHandshakeInfo(devServerID string) (usecase.HandshakeInfo, bool) {
	return c.lastHandshakeInfo, c.lastHandshakeOK
}

type capsFakeRepo struct {
	relayFakeDevServerRepository
	getErr error
}

func (r *capsFakeRepo) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	if r.getErr != nil {
		return domain.DevServer{}, r.getErr
	}
	return r.ds, nil
}

func TestServer_GetAgentCapabilities_UnimplementedWithoutWithCodeIntel(t *testing.T) {
	s := &Server{}
	_, err := s.GetAgentCapabilities(context.Background(), &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-1"})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented code, got %v", err)
	}
}

func TestServer_GetAgentCapabilities_TenantContextRequired(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	agent := &capsFakeAgentClient{isConnected: true}
	uc := usecase.NewGetAgentCapabilities(repo, agent)

	s := &Server{}
	s.WithCodeIntel(nil, uc)

	_, err := s.GetAgentCapabilities(context.Background(), &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
	}
}

func TestServer_GetAgentCapabilities_NotFound(t *testing.T) {
	repo := &capsFakeRepo{getErr: errors.New("not found")}
	agent := &capsFakeAgentClient{}
	uc := usecase.NewGetAgentCapabilities(repo, agent)

	s := &Server{}
	s.WithCodeIntel(nil, uc)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	_, err := s.GetAgentCapabilities(ctx, &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}
}

func TestServer_GetAgentCapabilities_OnlineSuccess(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	agent := &capsFakeAgentClient{
		isConnected:     true,
		lastHandshakeOK: true,
		lastHandshakeInfo: usecase.HandshakeInfo{
			Platform:     "darwin",
			Arch:         "arm64",
			NodeVersion:  "v20.0.0",
			AgentVersion: "2.1.0",
			SessionID:    "sess-999",
			Capabilities: []string{"pty", "codeintel"},
			Tools:        []string{"gitnexus", "codegraph"},
		},
	}
	uc := usecase.NewGetAgentCapabilities(repo, agent)

	s := &Server{}
	s.WithCodeIntel(nil, uc)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	resp, err := s.GetAgentCapabilities(ctx, &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Connected {
		t.Errorf("expected connected=true")
	}
	if resp.Platform != "darwin" || resp.Arch != "arm64" || resp.NodeVersion != "v20.0.0" || resp.AgentVersion != "2.1.0" {
		t.Errorf("unexpected platform/agent facts: %+v", resp)
	}
	if resp.SessionId != "sess-999" {
		t.Errorf("expected session_id sess-999, got %s", resp.SessionId)
	}
	if len(resp.Capabilities) != 2 || resp.Capabilities[0] != "pty" || resp.Capabilities[1] != "codeintel" {
		t.Errorf("unexpected capabilities: %v", resp.Capabilities)
	}
	if len(resp.Tools) != 2 || resp.Tools[0] != "gitnexus" || resp.Tools[1] != "codegraph" {
		t.Errorf("unexpected tools: %v", resp.Tools)
	}
}

func TestServer_GetAgentCapabilities_OfflineSuccess(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	agent := &capsFakeAgentClient{
		isConnected: false,
	}
	uc := usecase.NewGetAgentCapabilities(repo, agent)

	s := &Server{}
	s.WithCodeIntel(nil, uc)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	resp, err := s.GetAgentCapabilities(ctx, &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Connected {
		t.Errorf("expected connected=false")
	}
	if resp.Platform != "" || len(resp.Capabilities) != 0 || len(resp.Tools) != 0 {
		t.Errorf("expected empty fields when offline, got %+v", resp)
	}
}

func TestServer_GetAgentCapabilities_RaceConnectedNoHandshakeInfo(t *testing.T) {
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	agent := &capsFakeAgentClient{
		isConnected:     true,
		lastHandshakeOK: false,
	}
	uc := usecase.NewGetAgentCapabilities(repo, agent)

	s := &Server{}
	s.WithCodeIntel(nil, uc)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	resp, err := s.GetAgentCapabilities(ctx, &infrafleetv1.GetAgentCapabilitiesRequest{DevServerId: "ds-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Connected {
		t.Errorf("expected connected=true")
	}
	if resp.Platform != "" || len(resp.Capabilities) != 0 || len(resp.Tools) != 0 {
		t.Errorf("expected empty fields for race condition, got %+v", resp)
	}
}
