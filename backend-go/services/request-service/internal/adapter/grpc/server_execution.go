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

// ExecutionUseCases are the CR-REQ-013/014/015 use cases behind StartPhase, ReportTaskOutcome, the request checks
// and ListBacklog.
type ExecutionUseCases struct {
	StartPhase    *usecase.StartPhase
	ReportOutcome *usecase.ReportTaskOutcome
	RecordCheck   *usecase.RecordRequestCheck
	ListChecks    *usecase.ListRequestChecks
	Backlog       *usecase.ListBacklog
}

// WithExecution attaches the execution handlers; without it those RPCs stay Unimplemented.
func (s *Server) WithExecution(u ExecutionUseCases) *Server { s.execution = u; return s }

func (s *Server) StartPhase(ctx context.Context, req *requestv1.StartPhaseRequest) (*requestv1.StartPhaseResponse, error) {
	if s.execution.StartPhase == nil {
		return nil, status.Error(codes.Unimplemented, "StartPhase is not wired")
	}
	res, err := s.execution.StartPhase.Execute(ctx, usecase.StartPhaseInput{RequestID: req.GetRequestId(), PhaseTaskID: req.GetPhaseTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.StartPhaseResponse{PhaseTaskId: res.PhaseTaskID, AlreadyStarted: res.AlreadyStarted, DispatchedTaskIds: res.Dispatched}, nil
}

// ReportTaskOutcome is internal: main.go guards it with the shared service token.
func (s *Server) ReportTaskOutcome(ctx context.Context, req *requestv1.ReportTaskOutcomeRequest) (*requestv1.ReportTaskOutcomeResponse, error) {
	if s.execution.ReportOutcome == nil {
		return nil, status.Error(codes.Unimplemented, "ReportTaskOutcome is not wired")
	}
	in := usecase.ReportTaskOutcomeInput{
		EventID: req.GetEventId(), RequestID: req.GetRequestId(), TaskID: req.GetTaskId(), TaskType: req.GetTaskType(),
		PreviousStatus: req.GetPreviousStatus(), NewStatus: req.GetNewStatus(), Cause: req.GetCause(),
		ExecutionLinkID: req.GetExecutionLinkId(), ErrorMessage: req.GetErrorMessage(),
	}
	if t := req.GetOccurredAt(); t != nil {
		in.OccurredAt = t.AsTime()
	}
	if err := s.execution.ReportOutcome.Execute(ctx, in); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ReportTaskOutcomeResponse{}, nil
}

func (s *Server) RecordRequestCheck(ctx context.Context, req *requestv1.RecordRequestCheckRequest) (*requestv1.RecordRequestCheckResponse, error) {
	if s.execution.RecordCheck == nil {
		return nil, status.Error(codes.Unimplemented, "RecordRequestCheck is not wired")
	}
	c, err := s.execution.RecordCheck.Execute(ctx, usecase.RecordRequestCheckInput{
		RequestID: req.GetRequestId(), Kind: req.GetKind(), Status: req.GetStatus(), MetricsJSON: req.GetMetricsJson(),
		Summary: req.GetSummary(), TaskID: req.GetTaskId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.RecordRequestCheckResponse{Check: toProtoRequestCheck(c)}, nil
}

// ListRequestChecks answers oldest first; for each kind the last row is the effective one.
func (s *Server) ListRequestChecks(ctx context.Context, req *requestv1.ListRequestChecksRequest) (*requestv1.ListRequestChecksResponse, error) {
	if s.execution.ListChecks == nil {
		return nil, status.Error(codes.Unimplemented, "ListRequestChecks is not wired")
	}
	checks, err := s.execution.ListChecks.Execute(ctx, req.GetRequestId(), req.GetKind())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestChecksResponse{}
	for _, c := range checks {
		out.Checks = append(out.Checks, toProtoRequestCheck(c))
	}
	return out, nil
}

func toProtoRequestCheck(c domain.RequestCheck) *requestv1.RequestCheck {
	return &requestv1.RequestCheck{
		Id: c.ID, RequestId: c.RequestID, Kind: string(c.Kind), Status: string(c.Status), MetricsJson: string(c.Metrics),
		Summary: c.Summary, Source: string(c.Source), TaskId: c.TaskID, RecordedBy: c.RecordedBy, CreatedAt: timestamppb.New(c.CreatedAt),
	}
}
