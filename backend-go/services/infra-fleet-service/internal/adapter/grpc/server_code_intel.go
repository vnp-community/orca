package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// WithCodeIntel binds optional CodeIntel event streaming and agent capabilities use cases.
func (s *Server) WithCodeIntel(streamUC *usecase.StreamCodeIntelEvents, capsUC *usecase.GetAgentCapabilities) *Server {
	s.streamCodeIntelEvents = streamUC
	s.getAgentCapabilities = capsUC
	return s
}

// GetAgentCapabilities returns the capabilities and tools advertised by a dev server's agent.
func (s *Server) GetAgentCapabilities(ctx context.Context, req *infrafleetv1.GetAgentCapabilitiesRequest) (*infrafleetv1.GetAgentCapabilitiesResponse, error) {
	if s.getAgentCapabilities == nil {
		return nil, status.Error(codes.Unimplemented, "GetAgentCapabilities is not configured")
	}
	caps, err := s.getAgentCapabilities.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentCapabilities(caps), nil
}

// StreamCodeIntelEvents streams live code intelligence events from the dev server.
func (s *Server) StreamCodeIntelEvents(req *infrafleetv1.StreamCodeIntelEventsRequest, stream infrafleetv1.InfraFleetService_StreamCodeIntelEventsServer) error {
	if s.streamCodeIntelEvents == nil {
		return status.Error(codes.Unimplemented, "StreamCodeIntelEvents is not configured")
	}

	ctx := withTenantFromStreamMetadata(stream.Context())
	events, release, err := s.streamCodeIntelEvents.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	defer release()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(toProtoCodeIntelEvent(ev)); err != nil {
				return err
			}
		}
	}
}

func toProtoAgentCapabilities(caps domain.AgentCapabilities) *infrafleetv1.GetAgentCapabilitiesResponse {
	return &infrafleetv1.GetAgentCapabilitiesResponse{
		Connected:    caps.Connected,
		Platform:     caps.Platform,
		Arch:         caps.Arch,
		NodeVersion:  caps.NodeVersion,
		AgentVersion: caps.AgentVersion,
		Capabilities: caps.Capabilities,
		Tools:        caps.Tools,
		SessionId:    caps.SessionID,
	}
}

func toProtoCodeIntelEvent(ev domain.CodeIntelEvent) *infrafleetv1.CodeIntelEvent {
	var receivedAt *timestamppb.Timestamp
	if !ev.ReceivedAt.IsZero() {
		receivedAt = timestamppb.New(ev.ReceivedAt)
	}
	return &infrafleetv1.CodeIntelEvent{
		Kind:          ev.Kind,
		WorkspaceRoot: ev.WorkspaceRoot,
		Tool:          ev.Tool,
		Commit:        ev.Commit,
		IndexedAt:     ev.IndexedAt,
		JobId:         ev.JobID,
		Stage:         ev.Stage,
		Percent:       ev.Percent,
		Message:       ev.Message,
		ReceivedAt:    receivedAt,
		Reason:        ev.Reason,
		HeadCommit:    ev.HeadCommit,
		Stale:         ev.Stale,
		IndexScope:    ev.IndexScope,
		MergeBase:     ev.MergeBase,
		Trigger:       ev.Trigger,
		Outcome:       ev.Outcome,
		ErrorCode:     ev.ErrorCode,
		RunId:         ev.RunID,
		PayloadJson:   ev.PayloadJSON,
	}
}
