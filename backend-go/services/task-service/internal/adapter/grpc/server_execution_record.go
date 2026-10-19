package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// WithExecutionRecords enables ListExecutionRecords; otherwise it answers Unimplemented.
func (s *Server) WithExecutionRecords(list *usecase.ListExecutionRecords) *Server {
	s.listExecutionRecords = list
	return s
}

// WithTaskExecutionOutput enables GetTaskExecutionOutput; otherwise it answers Unimplemented.
func (s *Server) WithTaskExecutionOutput(get *usecase.GetTaskExecutionOutput) *Server {
	s.getTaskExecutionOutput = get
	return s
}

func (s *Server) GetTaskExecutionOutput(ctx context.Context, req *taskv1.GetTaskExecutionOutputRequest) (*taskv1.GetTaskExecutionOutputResponse, error) {
	if s.getTaskExecutionOutput == nil {
		return s.UnimplementedTaskServiceServer.GetTaskExecutionOutput(ctx, req)
	}
	out, err := s.getTaskExecutionOutput.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GetTaskExecutionOutputResponse{Found: out.Found, Output: out.Output, Redacted: out.Redacted, Truncated: out.Truncated}, nil
}

func toProtoExecutionRecord(r domain.ExecutionRecord) *taskv1.ExecutionRecord {
	return &taskv1.ExecutionRecord{
		Id: r.ID, TaskId: r.TaskID, ExecutionLinkId: r.ExecutionLinkID, Attempt: int32(r.Attempt),
		SpecDigest: r.SpecDigest, PacketDigest: r.PacketDigest, TemplateVersion: r.TemplateVersion,
		ParseStatus: string(r.ParseStatus), FailureClass: string(r.FailureClass),
		ResultJson: string(r.Result), ChangesJson: string(r.Changes), StdoutTail: r.StdoutTail,
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

func (s *Server) ListExecutionRecords(ctx context.Context, req *taskv1.ListExecutionRecordsRequest) (*taskv1.ListExecutionRecordsResponse, error) {
	if s.listExecutionRecords == nil {
		return s.UnimplementedTaskServiceServer.ListExecutionRecords(ctx, req)
	}
	recs, err := s.listExecutionRecords.Execute(ctx, usecase.ListExecutionRecordsInput{
		TaskIDs: req.GetTaskIds(), LatestOnly: req.GetLatestOnly(), Limit: int(req.GetLimit()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.ExecutionRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, toProtoExecutionRecord(r))
	}
	return &taskv1.ListExecutionRecordsResponse{Records: out}, nil
}
