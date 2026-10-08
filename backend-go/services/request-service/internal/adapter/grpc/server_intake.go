package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// actorFromContext takes the acting user from metadata, never from the request body.
func actorFromContext(ctx context.Context) (string, error) {
	id, _ := tenant.UserID(ctx)
	if id == "" {
		return "", domain.ErrRequestReporterRequired()
	}
	return id, nil
}

func (s *Server) CreateRequest(ctx context.Context, req *requestv1.CreateRequestRequest) (*requestv1.CreateRequestResponse, error) {
	if s.intake.Create == nil {
		return nil, status.Error(codes.Unimplemented, "CreateRequest is not wired")
	}
	h := req.GetHints()
	patch, err := usecase.PatchFromJSON("", "", false, false, req.GetAcceptanceCriteriaJson(), req.GetTypeFieldsJson())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	var initialAC []domain.ACInput
	if patch.AcceptanceCriteria != nil {
		initialAC = *patch.AcceptanceCriteria
	}
	res, err := s.intake.Create.Execute(ctx, usecase.CreateRequestInput{
		AcceptanceCriteria: initialAC, TypeFields: patch.TypeFields,
		ProjectID: req.GetProjectId(), Title: req.GetTitle(), Body: req.GetBody(),
		Source:          domain.SourceRef{Provider: domain.SourceProvider(req.GetSource().GetProvider()), Ref: req.GetSource().GetRef(), URL: req.GetSource().GetUrl(), Site: req.GetSource().GetSite()},
		Hints:           domain.SourceHints{IssueType: h.GetIssueType(), Labels: h.GetLabels(), Priority: h.GetPriority()}, // type_hint is for SpawnChildRequest only
		ClientRequestID: req.GetClientRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CreateRequestResponse{Request: toProtoRequest(res.Request), Created: res.Created}, nil
}

func (s *Server) LookupRequestBySource(ctx context.Context, req *requestv1.LookupRequestBySourceRequest) (*requestv1.LookupRequestBySourceResponse, error) {
	if s.intake.Lookup == nil {
		return nil, status.Error(codes.Unimplemented, "LookupRequestBySource is not wired")
	}
	r, found, err := s.intake.Lookup.Execute(ctx, domain.SourceRef{Provider: domain.SourceProvider(req.GetProvider()), Site: req.GetSite(), Ref: req.GetRef()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.LookupRequestBySourceResponse{Found: found, RequestId: r.ID}, nil
}
