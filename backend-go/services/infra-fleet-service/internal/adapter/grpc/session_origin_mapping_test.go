package grpc

import (
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSessionOriginMapping_RoundTripAndEmpty(t *testing.T) {
	in := &infrafleetv1.SessionOrigin{Type: "mcp", ClientName: "Claude Code", McpSessionId: "s1", UserId: "u1"}
	d := originFromProto(in)
	if d == nil || *d != (domain.SessionOrigin{Type: "mcp", ClientName: "Claude Code", MCPSessionID: "s1", UserID: "u1"}) {
		t.Fatalf("originFromProto = %+v", d)
	}
	out := originToProto(d)
	if out.GetType() != "mcp" || out.GetClientName() != "Claude Code" || out.GetMcpSessionId() != "s1" || out.GetUserId() != "u1" {
		t.Fatalf("originToProto = %+v", out)
	}
	if originFromProto(nil) != nil || originFromProto(&infrafleetv1.SessionOrigin{}) != nil || originToProto(nil) != nil {
		t.Fatal("absent or empty origin must map to nil on both sides")
	}
}

func TestToProtoAgentSession_CarriesOrigin(t *testing.T) {
	got := toProtoAgentSession(domain.AgentSession{ID: "a1", Origin: &domain.SessionOrigin{Type: "mcp", MCPSessionID: "s1"}})
	if got.GetOrigin().GetMcpSessionId() != "s1" {
		t.Fatalf("agent session origin lost: %+v", got)
	}
	if toProtoAgentSession(domain.AgentSession{ID: "a2"}).GetOrigin() != nil {
		t.Fatal("UI-started agent session must not carry an origin")
	}
}
