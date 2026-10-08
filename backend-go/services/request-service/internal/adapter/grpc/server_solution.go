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

// SolutionUseCases are the CR-REQ-007/008 use cases behind GenerateSolution, ListSolutions and ChooseSolutionOption.
type SolutionUseCases struct {
	Generate *usecase.GenerateSolution
	List     *usecase.ListSolutions
	Choose   *usecase.ChooseSolutionOption
}

// WithSolution attaches the solution handlers; without it those RPCs stay Unimplemented.
func (s *Server) WithSolution(u SolutionUseCases) *Server { s.solution = u; return s }

func (s *Server) GenerateSolution(ctx context.Context, req *requestv1.GenerateSolutionRequest) (*requestv1.GenerateSolutionResponse, error) {
	if s.solution.Generate == nil {
		return nil, status.Error(codes.Unimplemented, "GenerateSolution is not wired")
	}
	if _, err := actorFromContext(ctx); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if e := req.GetEngineOverride(); e != "" && e != "native" {
		return nil, status.Error(codes.InvalidArgument, "engine_override accepts only \"native\"")
	}
	res, err := s.solution.Generate.Execute(ctx, usecase.GenerateSolutionInput{
		RequestID: req.GetRequestId(), IdempotencyKey: req.GetIdempotencyKey(), Feedback: req.GetFeedback(), Mode: modeFromProto(req.GetAnalysisMode()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GenerateSolutionResponse{SolutionId: res.SolutionID, RunId: res.RunID}, nil
}

func (s *Server) ListSolutions(ctx context.Context, req *requestv1.ListSolutionsRequest) (*requestv1.ListSolutionsResponse, error) {
	if s.solution.List == nil {
		return nil, status.Error(codes.Unimplemented, "ListSolutions is not wired")
	}
	res, err := s.solution.List.Execute(ctx, usecase.ListSolutionsInput{
		RequestID: req.GetRequestId(), Kind: kindFromProto(req.GetKind()), Status: statusFromProto(req.GetStatus()), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListSolutionsResponse{}
	for _, sol := range res.Solutions {
		out.Solutions = append(out.Solutions, toProtoSolution(sol))
	}
	for _, run := range res.Runs {
		out.Runs = append(out.Runs, toProtoAnalysisRun(run))
	}
	return out, nil
}

func (s *Server) ChooseSolutionOption(ctx context.Context, req *requestv1.ChooseSolutionOptionRequest) (*requestv1.ChooseSolutionOptionResponse, error) {
	if s.solution.Choose == nil {
		return nil, status.Error(codes.Unimplemented, "ChooseSolutionOption is not wired")
	}
	if _, err := actorFromContext(ctx); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.solution.Choose.Execute(ctx, usecase.ChooseSolutionOptionInput{
		RequestID: req.GetRequestId(), SolutionID: req.GetSolutionId(), OptionID: req.GetOptionId(), Rationale: req.GetRationale(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ChooseSolutionOptionResponse{
		Solution: toProtoSolution(res.Solution), ApprovalDigest: res.ApprovalDigest,
		DecisionStatus: res.DecisionStatus, RequiresConfirmation: res.RequiresConfirmation,
	}, nil
}

func modeFromProto(m requestv1.AnalysisMode) domain.AnalysisMode {
	switch m {
	case requestv1.AnalysisMode_ANALYSIS_MODE_COMPLETE:
		return domain.AnalysisModeComplete
	case requestv1.AnalysisMode_ANALYSIS_MODE_AGENT_READONLY:
		return domain.AnalysisModeAgentReadonly
	}
	return ""
}

var kindToProto = map[domain.SolutionKind]requestv1.SolutionKind{
	domain.SolutionKindSolution:  requestv1.SolutionKind_SOLUTION_KIND_SOLUTION,
	domain.SolutionKindDiagnosis: requestv1.SolutionKind_SOLUTION_KIND_DIAGNOSIS,
	domain.SolutionKindFindings:  requestv1.SolutionKind_SOLUTION_KIND_FINDINGS,
	domain.SolutionKindAnswer:    requestv1.SolutionKind_SOLUTION_KIND_ANSWER,
}

var statusToProto = map[domain.SolutionStatus]requestv1.SolutionStatus{
	domain.SolutionStatusDraft:      requestv1.SolutionStatus_SOLUTION_STATUS_DRAFT,
	domain.SolutionStatusProposed:   requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED,
	domain.SolutionStatusApproved:   requestv1.SolutionStatus_SOLUTION_STATUS_APPROVED,
	domain.SolutionStatusRejected:   requestv1.SolutionStatus_SOLUTION_STATUS_REJECTED,
	domain.SolutionStatusSuperseded: requestv1.SolutionStatus_SOLUTION_STATUS_SUPERSEDED,
}

func kindFromProto(k requestv1.SolutionKind) domain.SolutionKind {
	for d, p := range kindToProto {
		if p == k {
			return d
		}
	}
	return ""
}

func statusFromProto(s requestv1.SolutionStatus) domain.SolutionStatus {
	for d, p := range statusToProto {
		if p == s {
			return d
		}
	}
	return ""
}

func toProtoSolution(s domain.Solution) *requestv1.Solution {
	chosen := int32(-1)
	if s.ChosenOption != nil {
		chosen = int32(*s.ChosenOption)
	}
	return &requestv1.Solution{
		Id: s.ID, RequestId: s.RequestID, Kind: kindToProto[s.Kind], Status: statusToProto[s.Status], OptionsJson: string(s.OptionsJSON),
		ChosenOption: chosen, ContentRef: s.ContentRef, GenerationRunId: s.GenerationRunID, CreatedAt: timestamppb.New(s.CreatedAt), Version: s.Version,
	}
}

func toProtoAnalysisRun(r domain.AnalysisRun) *requestv1.AnalysisRun {
	out := &requestv1.AnalysisRun{
		Id: r.ID, Kind: kindToProto[domain.SolutionKind(r.Kind)], Status: string(r.Status), StartedAt: timestamppb.New(r.StartedAt),
	}
	if r.ErrorCode != nil {
		out.ErrorCode = *r.ErrorCode
	}
	if r.ErrorMessage != nil {
		out.ErrorMessage = *r.ErrorMessage
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return out
}
