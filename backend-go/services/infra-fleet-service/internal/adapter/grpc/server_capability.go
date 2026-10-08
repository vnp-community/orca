package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// WithDevServerCapabilities binds the capability-profile use case; added as
// an option rather than a New() parameter to keep the positional list stable.
func (s *Server) WithDevServerCapabilities(uc *usecase.GetDevServerCapabilities) *Server {
	s.getDevServerCapabilities = uc
	return s
}

// GetDevServerCapabilities returns the stored capability profile of a dev
// server, refreshing it from the agent when connected and stale.
func (s *Server) GetDevServerCapabilities(ctx context.Context, req *infrafleetv1.GetDevServerCapabilitiesRequest) (*infrafleetv1.DevServerCapabilityProfile, error) {
	if s.getDevServerCapabilities == nil {
		return nil, status.Error(codes.Unimplemented, "GetDevServerCapabilities is not configured")
	}
	res, err := s.getDevServerCapabilities.Execute(ctx, usecase.GetCapabilitiesInput{
		ConnectionID: req.GetConnectionId(),
		DevServerID:  req.GetDevServerId(),
		Refresh:      req.GetRefresh(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	p := res.Profile
	features := p.Features
	if features == nil {
		features = []string{}
	}
	return &infrafleetv1.DevServerCapabilityProfile{
		DevServerId:       p.DevServerID,
		Source:            string(p.Source),
		AgentBuildVersion: p.AgentBuildVersion,
		ProtocolVersion:   int32(p.ProtocolVersion),
		Features:          features,
		ProfileJson:       string(p.ProfileJSON),
		Degraded:          p.Degraded(),
		ProbedAt:          timestamppb.New(p.ProbedAt),
		Connected:         res.Connected,
	}, nil
}
