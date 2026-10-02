package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type memRepo struct {
	rows map[string]domain.TenantSettings
}

func (m *memRepo) GetOrCreateTenantSettings(_ context.Context, d domain.TenantSettings) (domain.TenantSettings, error) {
	if s, ok := m.rows[d.TenantID]; ok {
		return s, nil
	}
	m.rows[d.TenantID] = d
	return d, nil
}

func (m *memRepo) UpdateTenantSettings(_ context.Context, s domain.TenantSettings) (domain.TenantSettings, error) {
	m.rows[s.TenantID] = s
	return s, nil
}

func newServer() *Server {
	repo := &memRepo{rows: map[string]domain.TenantSettings{}}
	return New(usecase.NewGetServerInfo(repo, usecase.Defaults{TenantEnabled: true, MaxTokenDays: 90}))
}

func TestGetServerInfo_MapsSettingsAndScopes(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), "aaaaaaaa-0000-4000-8000-000000000001")
	resp, err := newServer().GetServerInfo(ctx, &mcpv1.GetServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetEnabled() || resp.GetMaxTokenDays() != 90 || resp.GetApprovalTtlSeconds() != 600 || resp.GetKillSwitch().GetActive() {
		t.Fatalf("unexpected response: %v", resp)
	}
	if len(resp.GetScopes()) != 4 || resp.GetScopes()[0].GetId() != "orca:read" || resp.GetScopes()[0].GetRisk() != "read" {
		t.Fatalf("unexpected scopes: %v", resp.GetScopes())
	}
}

func TestGetServerInfo_MissingTenantIsUnauthenticated(t *testing.T) {
	_, err := newServer().GetServerInfo(context.Background(), &mcpv1.GetServerInfoRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("got %v", err)
	}
}
