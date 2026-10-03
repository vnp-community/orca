package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type originListerStub struct {
	got usecase.AgentSessionOriginFilter
}

func (s *originListerStub) ListByOrigin(_ context.Context, _ string, f usecase.AgentSessionOriginFilter) ([]domain.AgentSession, error) {
	s.got = f
	return []domain.AgentSession{{ID: "a1", PtyID: "p1", Status: domain.AgentStatusRunning,
		Origin: &domain.SessionOrigin{Type: "mcp", MCPSessionID: "sess-1"}}}, nil
}

func TestListAgentSessions_MapsFilterAndOrigin(t *testing.T) {
	stub := &originListerStub{}
	srv := WithAgentSessionList(nil, usecase.NewListAgentSessionsByOrigin(stub))
	ctx := tenant.WithTenantID(context.Background(), "t1")

	resp, err := srv.ListAgentSessions(ctx, &infrafleetv1.ListAgentSessionsRequest{OriginType: "mcp", OriginSessionId: "sess-1", ActiveOnly: true, Limit: 7})
	if err != nil {
		t.Fatal(err)
	}
	if stub.got.OriginType != "mcp" || stub.got.OriginSessionID != "sess-1" || !stub.got.ActiveOnly || stub.got.Limit != 7 {
		t.Fatalf("filter = %+v", stub.got)
	}
	if len(resp.GetSessions()) != 1 || resp.GetSessions()[0].GetOrigin().GetMcpSessionId() != "sess-1" || resp.GetSessions()[0].GetPtyId() != "p1" {
		t.Fatalf("response = %+v", resp)
	}
	_, err = srv.ListAgentSessions(ctx, &infrafleetv1.ListAgentSessionsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unfiltered listing must be InvalidArgument, got %v", err)
	}
}
