// Package grpc implements mcpv1.McpServiceServer: wire <-> usecase
// translation only, no business logic.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type Server struct {
	mcpv1.UnimplementedMcpServiceServer

	getServerInfo *usecase.GetServerInfo
}

func New(getServerInfo *usecase.GetServerInfo) *Server {
	return &Server{getServerInfo: getServerInfo}
}

func (s *Server) GetServerInfo(ctx context.Context, _ *mcpv1.GetServerInfoRequest) (*mcpv1.GetServerInfoResponse, error) {
	info, err := s.getServerInfo.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoServerInfo(info), nil
}

func toProtoServerInfo(info usecase.ServerInfo) *mcpv1.GetServerInfoResponse {
	st := info.Settings
	ks := &mcpv1.KillSwitch{Active: st.KillSwitch.Active, Reason: st.KillSwitch.Reason}
	if st.KillSwitch.At != nil {
		ks.At = timestamppb.New(*st.KillSwitch.At)
	}
	scopes := make([]*mcpv1.ScopeDescriptor, 0, len(info.Scopes))
	for _, sc := range info.Scopes {
		scopes = append(scopes, toProtoScope(sc))
	}
	return &mcpv1.GetServerInfoResponse{
		Enabled:            st.Enabled,
		DcrEnabled:         st.DCREnabled,
		MaxTokenDays:       int32(st.MaxTokenDays),
		ApprovalTtlSeconds: int32(st.ApprovalTTLSeconds),
		KillSwitch:         ks,
		Scopes:             scopes,
	}
}

func toProtoScope(sc domain.ScopeDescriptor) *mcpv1.ScopeDescriptor {
	return &mcpv1.ScopeDescriptor{Id: sc.ID, Label: sc.Label, Description: sc.Description, Risk: string(sc.Risk)}
}
