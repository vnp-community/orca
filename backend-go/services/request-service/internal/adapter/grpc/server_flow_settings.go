package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WithFlowSettings attaches GetRequestFlowSettings and SetRequestFlowSettings; without it they stay Unimplemented.
func (s *Server) WithFlowSettings(u *usecase.FlowSettings) *Server { s.flowSettings = u; return s }

func (s *Server) GetRequestFlowSettings(ctx context.Context, _ *requestv1.GetRequestFlowSettingsRequest) (*requestv1.GetRequestFlowSettingsResponse, error) {
	if s.flowSettings == nil {
		return nil, status.Error(codes.Unimplemented, "GetRequestFlowSettings is not wired")
	}
	on, err := s.flowSettings.Get(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetRequestFlowSettingsResponse{Enabled: on}, nil
}

func (s *Server) SetRequestFlowSettings(ctx context.Context, req *requestv1.SetRequestFlowSettingsRequest) (*requestv1.SetRequestFlowSettingsResponse, error) {
	if s.flowSettings == nil {
		return nil, status.Error(codes.Unimplemented, "SetRequestFlowSettings is not wired")
	}
	on, err := s.flowSettings.Set(ctx, req.GetEnabled())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.SetRequestFlowSettingsResponse{Enabled: on}, nil
}
