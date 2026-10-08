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

func (s *Server) GetRequestFlow(ctx context.Context, req *requestv1.GetRequestFlowRequest) (*requestv1.GetRequestFlowResponse, error) {
	if s.lifecycle.Flow == nil {
		return nil, status.Error(codes.Unimplemented, "GetRequestFlow is not wired")
	}
	v, err := s.lifecycle.Flow.Execute(ctx, req.GetType(), req.GetSize())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetRequestFlowResponse{
		Type: string(v.Flow.Type), HumanConfirmRequired: v.Flow.HumanConfirmRequired, AnalysisKind: string(v.Flow.AnalysisKind),
		AnalysisGate: string(v.Flow.AnalysisGate), PlanKind: string(v.Flow.PlanKind), HasPhases: v.HasPhases, StartGate: string(v.Flow.StartGate),
		CompletesAfterAnalysis: v.Flow.CompletesAfterAnalysis,
	}
	for _, g := range v.Flow.ExecutionGates {
		out.ExecutionGates = append(out.ExecutionGates, string(g))
	}
	for _, st := range v.StatusPath {
		out.StatusPath = append(out.StatusPath, string(st))
	}
	return out, nil
}

func (s *Server) ReturnToBacklog(ctx context.Context, req *requestv1.ReturnToBacklogRequest) (*requestv1.ReturnToBacklogResponse, error) {
	if s.lifecycle.Return == nil {
		return nil, status.Error(codes.Unimplemented, "ReturnToBacklog is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.lifecycle.Return.Execute(ctx, usecase.ReturnInput{
		RequestID: req.GetRequestId(), Stage: domain.ReturnStage(req.GetStage()), Category: domain.ReturnCategory(req.GetCategory()),
		Reason: req.GetReason(), ExpectedVersion: req.GetExpectedVersion(), ActorID: actor, ActorKind: domain.ActorKindUser,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ReturnToBacklogResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) ReopenRequest(ctx context.Context, req *requestv1.ReopenRequestRequest) (*requestv1.ReopenRequestResponse, error) {
	if s.lifecycle.Reopen == nil {
		return nil, status.Error(codes.Unimplemented, "ReopenRequest is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	r, err := s.lifecycle.Reopen.Execute(ctx, usecase.ReopenInput{RequestID: req.GetRequestId(), Note: req.GetNote(), ExpectedVersion: req.GetExpectedVersion(), ActorID: actor})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ReopenRequestResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) CancelRequest(ctx context.Context, req *requestv1.CancelRequestRequest) (*requestv1.CancelRequestResponse, error) {
	if s.lifecycle.Cancel == nil {
		return nil, status.Error(codes.Unimplemented, "CancelRequest is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.lifecycle.Cancel.Execute(ctx, usecase.CancelInput{RequestID: req.GetRequestId(), Reason: req.GetReason(), ExpectedVersion: req.GetExpectedVersion(), ActorID: actor})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CancelRequestResponse{Request: toProtoRequest(res.Request)}, nil
}

func (s *Server) SpawnChildRequest(ctx context.Context, req *requestv1.SpawnChildRequestRequest) (*requestv1.SpawnChildRequestResponse, error) {
	if s.lifecycle.SpawnChild == nil {
		return nil, status.Error(codes.Unimplemented, "SpawnChildRequest is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.lifecycle.SpawnChild.Execute(ctx, usecase.SpawnInput{
		ParentRequestID: req.GetParentRequestId(), LinkReason: domain.LinkReason(req.GetLinkReason()), Title: req.GetTitle(), Body: req.GetBody(),
		TypeHint: domain.RequestType(req.GetTypeHint()), ClientRequestID: req.GetClientRequestId(), ActorID: actor, Provider: domain.SourceProviderManual,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.SpawnChildRequestResponse{Child: toProtoRequest(res.Child), Created: res.Created}, nil
}

func (s *Server) ListRequestLinks(ctx context.Context, req *requestv1.ListRequestLinksRequest) (*requestv1.ListRequestLinksResponse, error) {
	if s.lifecycle.Links == nil {
		return nil, status.Error(codes.Unimplemented, "ListRequestLinks is not wired")
	}
	v, err := s.lifecycle.Links.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestLinksResponse{}
	for _, l := range v.Parents {
		out.Parents = append(out.Parents, toProtoLink(l))
	}
	for _, l := range v.Children {
		out.Children = append(out.Children, toProtoLink(l))
	}
	return out, nil
}

func toProtoLink(l domain.RequestLink) *requestv1.RequestLink {
	return &requestv1.RequestLink{ParentRequestId: l.ParentRequestID, ChildRequestId: l.ChildRequestID, Reason: string(l.Reason), CreatedAt: timestamppb.New(l.CreatedAt)}
}
