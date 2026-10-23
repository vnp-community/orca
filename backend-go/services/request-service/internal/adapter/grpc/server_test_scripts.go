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

// TestScriptUseCases are the handlers of the test-script lane (docs/integrate/TDD-REQ-TESTSCRIPT-001). Without them those RPCs stay Unimplemented.
type TestScriptUseCases struct {
	Get    *usecase.GetTestScripts
	Start  *usecase.StartTestScripts
	Report *usecase.GetTestScriptReport
	File   *usecase.GetTestScriptFile
}

// WithTestScripts attaches the test-script handlers.
func (s *Server) WithTestScripts(u TestScriptUseCases) *Server { s.testScripts = u; return s }

func (s *Server) GetTestScripts(ctx context.Context, req *requestv1.GetTestScriptsRequest) (*requestv1.GetTestScriptsResponse, error) {
	if s.testScripts.Get == nil {
		return nil, status.Error(codes.Unimplemented, "GetTestScripts is not wired")
	}
	view, err := s.testScripts.Get.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetTestScriptsResponse{RequestId: view.RequestID, Enabled: view.Enabled}
	if view.Current != nil {
		out.Current = toProtoTestScriptRun(*view.Current)
	}
	for _, h := range view.History {
		out.History = append(out.History, toProtoTestScriptRun(h))
	}
	return out, nil
}

func (s *Server) StartTestScripts(ctx context.Context, req *requestv1.StartTestScriptsRequest) (*requestv1.StartTestScriptsResponse, error) {
	if s.testScripts.Start == nil {
		return nil, status.Error(codes.Unimplemented, "StartTestScripts is not wired")
	}
	run, err := s.testScripts.Start.Restart(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.StartTestScriptsResponse{Run: toProtoTestScriptRun(run)}, nil
}

func (s *Server) GetTestScriptReport(ctx context.Context, req *requestv1.GetTestScriptReportRequest) (*requestv1.GetTestScriptReportResponse, error) {
	if s.testScripts.Report == nil {
		return nil, status.Error(codes.Unimplemented, "GetTestScriptReport is not wired")
	}
	text, err := s.testScripts.Report.Execute(ctx, req.GetRequestId(), int(req.GetOffset()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetTestScriptReportResponse{Path: text.Path, Length: int32(text.Total), Text: text.Text}
	if text.Next != nil {
		out.HasNext, out.Next = true, int32(*text.Next)
	}
	return out, nil
}

func (s *Server) GetTestScriptFile(ctx context.Context, req *requestv1.GetTestScriptFileRequest) (*requestv1.GetTestScriptFileResponse, error) {
	if s.testScripts.File == nil {
		return nil, status.Error(codes.Unimplemented, "GetTestScriptFile is not wired")
	}
	text, err := s.testScripts.File.Execute(ctx, req.GetRequestId(), req.GetPath(), int(req.GetOffset()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetTestScriptFileResponse{Path: text.Path, Length: int32(text.Total), Text: text.Text}
	if text.Next != nil {
		out.HasNext, out.Next = true, int32(*text.Next)
	}
	return out, nil
}

func toProtoTestScriptRun(r domain.TestScriptRun) *requestv1.TestScriptRun {
	summary := string(r.Summary)
	if summary == "" {
		summary = "{}"
	}
	out := &requestv1.TestScriptRun{
		Id: r.ID, Attempt: int32(r.Attempt), Status: string(r.State), ReasonCode: r.ReasonCode, ReasonMessage: r.ReasonMessage,
		TestSpecRunId: r.TestSpecRunID, Feature: r.Feature, SpecVersion: int32(r.SpecVersion), TsaJobId: r.TSAJobID, Branch: r.Branch,
		BaseRef: r.BaseRef, WorktreePath: r.WorktreePath, CommitSha: r.CommitSHA, SummaryJson: summary, StartedBy: r.StartedBy,
		StartedAt: timestamppb.New(r.StartedAt), UpdatedAt: timestamppb.New(r.UpdatedAt),
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return out
}
