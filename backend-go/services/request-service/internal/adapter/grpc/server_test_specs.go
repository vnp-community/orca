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

// TestSpecUseCases are the handlers of the test-spec lane (docs/integrate/TDD-REQ-TESTSPEC-001). Without them those RPCs stay Unimplemented.
type TestSpecUseCases struct {
	Get      *usecase.GetTestSpecs
	Start    *usecase.StartTestSpecs
	Document *usecase.GetTestSpecDocument
}

// WithTestSpecs attaches the test-spec handlers.
func (s *Server) WithTestSpecs(u TestSpecUseCases) *Server { s.testSpecs = u; return s }

func (s *Server) GetTestSpecs(ctx context.Context, req *requestv1.GetTestSpecsRequest) (*requestv1.GetTestSpecsResponse, error) {
	if s.testSpecs.Get == nil {
		return nil, status.Error(codes.Unimplemented, "GetTestSpecs is not wired")
	}
	view, err := s.testSpecs.Get.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetTestSpecsResponse{RequestId: view.RequestID, Enabled: view.Enabled}
	if view.Current != nil {
		out.Current = toProtoTestSpecRun(*view.Current)
	}
	for _, h := range view.History {
		out.History = append(out.History, toProtoTestSpecRun(h))
	}
	if view.Join != nil {
		out.Gate, out.Join, out.JoinWarn, out.JoinCause = string(view.Join.Gate), string(view.Join.Verdict), view.Join.Warn, view.Join.Cause
	}
	return out, nil
}

func (s *Server) StartTestSpecs(ctx context.Context, req *requestv1.StartTestSpecsRequest) (*requestv1.StartTestSpecsResponse, error) {
	if s.testSpecs.Start == nil {
		return nil, status.Error(codes.Unimplemented, "StartTestSpecs is not wired")
	}
	run, err := s.testSpecs.Start.Restart(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.StartTestSpecsResponse{Run: toProtoTestSpecRun(run)}, nil
}

func (s *Server) GetTestSpecDocument(ctx context.Context, req *requestv1.GetTestSpecDocumentRequest) (*requestv1.GetTestSpecDocumentResponse, error) {
	if s.testSpecs.Document == nil {
		return nil, status.Error(codes.Unimplemented, "GetTestSpecDocument is not wired")
	}
	text, err := s.testSpecs.Document.Execute(ctx, req.GetRequestId(), req.GetDocument(), int(req.GetOffset()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetTestSpecDocumentResponse{
		Document: text.Document, Version: int32(text.Version), Status: text.Status, Length: int32(text.Length), Text: text.Text,
	}
	if text.Next != nil {
		out.HasNext, out.Next = true, int32(*text.Next)
	}
	return out, nil
}

func toProtoTestSpecRun(r domain.TestSpecRun) *requestv1.TestSpecRun {
	summary := string(r.Summary)
	if summary == "" {
		summary = "{}"
	}
	out := &requestv1.TestSpecRun{
		Id: r.ID, Attempt: int32(r.Attempt), Status: string(r.State), ReasonCode: r.ReasonCode, ReasonMessage: r.ReasonMessage,
		ContextMode: r.ContextMode, ContextRef: r.ContextRef, ContextSha256: r.ContextSHA256, TddRunId: r.TDDRunID, Feature: r.Feature,
		SummaryJson: summary, StartedBy: r.StartedBy, StartedAt: timestamppb.New(r.StartedAt), UpdatedAt: timestamppb.New(r.UpdatedAt),
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return out
}
