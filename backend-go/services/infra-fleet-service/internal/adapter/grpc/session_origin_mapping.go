package grpc

import (
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func originFromProto(o *infrafleetv1.SessionOrigin) *domain.SessionOrigin {
	if o == nil {
		return nil
	}
	d := &domain.SessionOrigin{Type: o.GetType(), ClientName: o.GetClientName(), MCPSessionID: o.GetMcpSessionId(), UserID: o.GetUserId()}
	if d.IsEmpty() {
		return nil
	}
	return d
}

func originToProto(o *domain.SessionOrigin) *infrafleetv1.SessionOrigin {
	if o.IsEmpty() {
		return nil
	}
	return &infrafleetv1.SessionOrigin{Type: o.Type, ClientName: o.ClientName, McpSessionId: o.MCPSessionID, UserId: o.UserID}
}
