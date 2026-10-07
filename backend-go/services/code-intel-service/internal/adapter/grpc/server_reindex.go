package grpc

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

func (s *CodeIntelServer) RequestReindex(ctx context.Context, req *codeintelv1.RequestReindexRequest) (*codeintelv1.RequestReindexResponse, error) {
	start := time.Now()
	method := "RequestReindex"

	tenantID, _, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	if s.requestReindex == nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_UNIMPLEMENTED", "reindex usecase is not configured", nil))
	}

	job, err := s.requestReindex.Execute(ctx, tenantID, req)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	s.metrics.RecordCall(method, "success")
	return &codeintelv1.RequestReindexResponse{
		Job: ToProtoReindexJob(job),
	}, nil
}

func (s *CodeIntelServer) GetReindexJob(ctx context.Context, req *codeintelv1.GetReindexJobRequest) (*codeintelv1.GetReindexJobResponse, error) {
	start := time.Now()
	method := "GetReindexJob"

	tenantID, _, err := s.resolveTenantAndTarget(ctx, req.GetSelector())
	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	if s.getReindexJob == nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "CODEINTEL_UNIMPLEMENTED", "get reindex job usecase is not configured", nil))
	}

	job, err := s.getReindexJob.Execute(ctx, tenantID, req)
	duration := time.Since(start)
	s.metrics.RecordDuration(method, duration)

	if err != nil {
		s.metrics.RecordCall(method, "error")
		return nil, apperrors.ToGRPCStatus(err)
	}

	s.metrics.RecordCall(method, "success")
	return &codeintelv1.GetReindexJobResponse{
		Job: ToProtoReindexJob(job),
	}, nil
}
