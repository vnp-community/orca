package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// SessionServer adds the session RPCs (BE-MCP-SOL-004) on top of any McpServiceServer.
type SessionServer struct {
	mcpv1.McpServiceServer
	uc *usecase.Sessions
}

func WithSessions(base mcpv1.McpServiceServer, uc *usecase.Sessions) *SessionServer {
	return &SessionServer{McpServiceServer: base, uc: uc}
}

func protoSession(s domain.Session) *mcpv1.McpSession {
	return &mcpv1.McpSession{
		Id: s.ID, TenantId: s.TenantID, UserId: s.UserID, ClientId: s.ClientID, ClientName: s.ClientName, ClientVersion: s.ClientVersion,
		GrantId: s.GrantID, TokenId: s.TokenID, ProtocolVersion: s.ProtocolVersion, CapabilitiesJson: s.CapabilitiesJSON, LogLevel: s.LogLevel,
		State: s.State, ToolCalls: s.ToolCalls, CreatedAt: timestamppb.New(s.CreatedAt), LastSeenAt: timestamppb.New(s.LastSeenAt),
		ActiveStreams: int32(s.ActiveStreams), CloseReason: s.CloseReason,
	}
}

func (s *SessionServer) CreateSession(ctx context.Context, req *mcpv1.CreateSessionRequest) (*mcpv1.McpSession, error) {
	out, err := s.uc.Create(ctx, usecase.CreateSessionInput{
		SecretHash: req.GetSecretHash(), ClientID: req.GetClientId(), ClientName: req.GetClientName(), ClientVersion: req.GetClientVersion(),
		GrantID: req.GetGrantId(), TokenID: req.GetTokenId(), ProtoVer: req.GetProtocolVersion(), CapabilitiesJSON: req.GetCapabilitiesJson(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return protoSession(out), nil
}

func (s *SessionServer) GetSessionBySecret(ctx context.Context, req *mcpv1.GetSessionBySecretRequest) (*mcpv1.McpSession, error) {
	out, err := s.uc.GetBySecret(ctx, req.GetSecretHash())
	if err != nil {
		return nil, toStatus(err)
	}
	return protoSession(out), nil
}

func (s *SessionServer) TouchSession(ctx context.Context, req *mcpv1.TouchSessionRequest) (*mcpv1.TouchSessionResponse, error) {
	st, err := s.uc.Touch(ctx, req.GetSessionId(), req.GetReady(), req.GetToolCallsDelta(), req.GetLogLevel())
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.TouchSessionResponse{State: st}, nil
}

func (s *SessionServer) CloseSession(ctx context.Context, req *mcpv1.CloseSessionRequest) (*mcpv1.CloseSessionResponse, error) {
	id, err := s.uc.Close(ctx, usecase.CloseSessionInput{SessionID: req.GetSessionId(), Hash: req.GetSecretHash(), Reason: req.GetReason()})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.CloseSessionResponse{SessionId: id}, nil
}

func (s *SessionServer) list(ctx context.Context, admin bool) (*mcpv1.ListSessionsResponse, error) {
	rows, err := s.uc.List(ctx, admin)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListSessionsResponse{Sessions: make([]*mcpv1.McpSession, 0, len(rows))}
	for _, r := range rows {
		resp.Sessions = append(resp.Sessions, protoSession(r))
	}
	return resp, nil
}

func (s *SessionServer) ListSessions(ctx context.Context, _ *mcpv1.ListSessionsRequest) (*mcpv1.ListSessionsResponse, error) {
	return s.list(ctx, false)
}

func (s *SessionServer) ListSessionsAdmin(ctx context.Context, _ *mcpv1.ListSessionsRequest) (*mcpv1.ListSessionsResponse, error) {
	return s.list(ctx, true)
}

func (s *SessionServer) OpenStream(ctx context.Context, req *mcpv1.OpenStreamRequest) (*mcpv1.OpenStreamResponse, error) {
	id, err := s.uc.OpenStream(ctx, req.GetSessionId(), req.GetReplicaId(), req.GetKind(), int(req.GetMaxPerUser()), int(req.GetMaxPerTenant()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.OpenStreamResponse{StreamId: id}, nil
}

func (s *SessionServer) HeartbeatStream(ctx context.Context, req *mcpv1.HeartbeatStreamRequest) (*mcpv1.HeartbeatStreamResponse, error) {
	if err := s.uc.Heartbeat(ctx, req.GetStreamId()); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.HeartbeatStreamResponse{}, nil
}

func (s *SessionServer) CloseStream(ctx context.Context, req *mcpv1.CloseStreamRequest) (*mcpv1.CloseStreamResponse, error) {
	if err := s.uc.CloseStream(ctx, req.GetStreamId()); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.CloseStreamResponse{}, nil
}
