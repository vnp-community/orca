package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// AgentSessionListServer layers ListAgentSessions over any InfraFleetServiceServer
// (a wrapper rather than another positional argument of New, which is already
// unwieldy).
type AgentSessionListServer struct {
	infrafleetv1.InfraFleetServiceServer
	list *usecase.ListAgentSessionsByOrigin
}

func WithAgentSessionList(inner infrafleetv1.InfraFleetServiceServer, list *usecase.ListAgentSessionsByOrigin) *AgentSessionListServer {
	return &AgentSessionListServer{InfraFleetServiceServer: inner, list: list}
}

func (s *AgentSessionListServer) ListAgentSessions(ctx context.Context, req *infrafleetv1.ListAgentSessionsRequest) (*infrafleetv1.ListAgentSessionsResponse, error) {
	sessions, err := s.list.Execute(ctx, usecase.AgentSessionOriginFilter{
		OriginType: req.GetOriginType(), OriginSessionID: req.GetOriginSessionId(),
		ActiveOnly: req.GetActiveOnly(), Limit: int(req.GetLimit()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.ListAgentSessionsResponse{Sessions: make([]*infrafleetv1.AgentSession, 0, len(sessions))}
	for _, sess := range sessions {
		resp.Sessions = append(resp.Sessions, toProtoAgentSession(sess))
	}
	return resp, nil
}
