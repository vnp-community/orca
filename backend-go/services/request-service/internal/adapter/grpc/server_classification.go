package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ClassifyRequest returns at once with the run id; the AI call runs in the background and its
// result is read back through GetRequest and the classified event.
func (s *Server) ClassifyRequest(ctx context.Context, req *requestv1.ClassifyRequestRequest) (*requestv1.ClassifyRequestResponse, error) {
	if s.classification.Runner == nil {
		return nil, status.Error(codes.Unimplemented, "ClassifyRequest is not wired")
	}
	if _, err := actorFromContext(ctx); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.classification.Runner.Enqueue(ctx, usecase.EnqueueClassificationInput{RequestID: req.GetRequestId(), Trigger: "manual", Manual: true})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.getRequest.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ClassifyRequestResponse{Request: toProtoRequest(r), RunId: res.Run.ID}, nil
}

func (s *Server) ConfirmRequestType(ctx context.Context, req *requestv1.ConfirmRequestTypeRequest) (*requestv1.ConfirmRequestTypeResponse, error) {
	if s.classification.Confirm == nil {
		return nil, status.Error(codes.Unimplemented, "ConfirmRequestType is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.classification.Confirm.Execute(ctx, usecase.ConfirmInput{
		RequestID: req.GetRequestId(), Type: req.GetType(), Size: req.GetSize(), Urgency: req.GetUrgency(), Reason: req.GetReason(),
		ExpectedVersion: req.GetExpectedVersion(), ActorID: actor, ActorKind: domain.ActorKindUser,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ConfirmRequestTypeResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) ChangeRequestType(ctx context.Context, req *requestv1.ChangeRequestTypeRequest) (*requestv1.ChangeRequestTypeResponse, error) {
	if s.classification.Change == nil {
		return nil, status.Error(codes.Unimplemented, "ChangeRequestType is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.classification.Change.Execute(ctx, usecase.ChangeInput{
		RequestID: req.GetRequestId(), NewType: req.GetNewType(), Size: req.GetSize(), Urgency: req.GetUrgency(), Reason: req.GetReason(),
		ExpectedVersion: req.GetExpectedVersion(), ActorID: actor, ActorKind: domain.ActorKindUser,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ChangeRequestTypeResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) ListRequestTypeHistory(ctx context.Context, req *requestv1.ListRequestTypeHistoryRequest) (*requestv1.ListRequestTypeHistoryResponse, error) {
	if s.classification.History == nil {
		return nil, status.Error(codes.Unimplemented, "ListRequestTypeHistory is not wired")
	}
	rows, err := s.classification.History.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestTypeHistoryResponse{}
	for _, h := range rows {
		out.Changes = append(out.Changes, toProtoTypeChange(h))
	}
	return out, nil
}

// toProtoTypeChange maps the stored actor kind `agent` to the wire value `ai` the proto documents.
func toProtoTypeChange(h domain.RequestTypeChange) *requestv1.RequestTypeChange {
	kind := string(h.ActorKind)
	if h.ActorKind == domain.ActorKindAgent {
		kind = "ai"
	}
	return &requestv1.RequestTypeChange{
		FromType: string(h.FromType), ToType: string(h.ToType), ActorId: h.ActorID, ActorKind: kind,
		Reason: h.Reason, At: timestamppb.New(h.At),
	}
}
