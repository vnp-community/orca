package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ApprovalPolicyAdminServer is admin-only CRUD of approver policies; ManageApprovalPolicies enforces the admin role.
type ApprovalPolicyAdminServer struct {
	requestv1.UnimplementedApprovalPolicyAdminServiceServer
	manage *usecase.ManageApprovalPolicies
}

func NewApprovalPolicyAdminServer(manage *usecase.ManageApprovalPolicies) *ApprovalPolicyAdminServer {
	return &ApprovalPolicyAdminServer{manage: manage}
}

func (s *ApprovalPolicyAdminServer) ListApprovalPolicies(ctx context.Context, req *requestv1.ListApprovalPoliciesRequest) (*requestv1.ListApprovalPoliciesResponse, error) {
	if s.manage == nil {
		return nil, status.Error(codes.Unimplemented, "approval policy admin is not wired")
	}
	list, next, err := s.manage.List(ctx, usecase.PolicyListFilter{
		ProjectID: req.GetProjectId(), SubjectType: toDomainSubject(req.GetSubjectType()), PageSize: int(req.GetPageSize()), PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListApprovalPoliciesResponse{NextPageToken: next}
	for _, p := range list {
		out.Policies = append(out.Policies, toProtoPolicy(p))
	}
	return out, nil
}

func (s *ApprovalPolicyAdminServer) UpsertApprovalPolicy(ctx context.Context, req *requestv1.UpsertApprovalPolicyRequest) (*requestv1.UpsertApprovalPolicyResponse, error) {
	if s.manage == nil {
		return nil, status.Error(codes.Unimplemented, "approval policy admin is not wired")
	}
	p, err := toDomainPolicy(req.GetPolicy())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	saved, err := s.manage.Upsert(ctx, p, req.GetExpectedVersion())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.UpsertApprovalPolicyResponse{Policy: toProtoPolicy(saved)}, nil
}

func (s *ApprovalPolicyAdminServer) DeleteApprovalPolicy(ctx context.Context, req *requestv1.DeleteApprovalPolicyRequest) (*requestv1.DeleteApprovalPolicyResponse, error) {
	if s.manage == nil {
		return nil, status.Error(codes.Unimplemented, "approval policy admin is not wired")
	}
	if err := s.manage.Delete(ctx, req.GetId(), req.GetExpectedVersion()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.DeleteApprovalPolicyResponse{}, nil
}
