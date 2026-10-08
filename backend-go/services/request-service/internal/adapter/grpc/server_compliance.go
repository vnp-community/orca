package grpc

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ComplianceUseCases are the CR-REQ-035 erase and export use cases (admin only, enforced by the interceptor and again inside).
type ComplianceUseCases struct {
	Erase     *usecase.EraseRequest
	Export    *usecase.ExportRequest
	ExportAll *usecase.ExportTenantRequests
}

func (s *Server) WithCompliance(u ComplianceUseCases) *Server { s.compliance = u; return s }

func (s *Server) EraseRequest(ctx context.Context, req *requestv1.EraseRequestRequest) (*requestv1.EraseRequestResponse, error) {
	if s.compliance.Erase == nil {
		return nil, status.Error(codes.Unimplemented, "EraseRequest is not wired")
	}
	res, err := s.compliance.Erase.Execute(ctx, req.GetRequestId(), req.GetReason())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.EraseRequestResponse{ErasedAt: timestamppb.New(res.ErasedAt), NotCoveredNote: res.NotCoveredNote}
	for _, e := range res.External {
		out.ExternalErasure = append(out.ExternalErasure, &requestv1.ExternalErasureResult{System: e.System, Status: e.Status})
	}
	return out, nil
}

func (s *Server) ExportRequest(ctx context.Context, req *requestv1.ExportRequestRequest) (*requestv1.ExportRequestResponse, error) {
	if s.compliance.Export == nil {
		return nil, status.Error(codes.Unimplemented, "ExportRequest is not wired")
	}
	b, err := s.compliance.Export.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ExportRequestResponse{BundleJson: b.JSON, Bytes: b.Bytes}, nil
}

func (s *Server) ExportTenantRequests(req *requestv1.ExportTenantRequestsRequest, stream requestv1.RequestService_ExportTenantRequestsServer) error {
	if s.compliance.ExportAll == nil {
		return status.Error(codes.Unimplemented, "ExportTenantRequests is not wired")
	}
	_, err := s.compliance.ExportAll.Run(stream.Context(), req.GetProjectId(), func(line string, index int64) error {
		return stream.Send(&requestv1.ExportTenantRequestsResponse{NdjsonLine: line, Index: index})
	})
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	return nil
}
