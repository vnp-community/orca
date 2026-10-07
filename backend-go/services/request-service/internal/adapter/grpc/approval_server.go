package grpc

import (
	"context"

	"github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ApprovalServer struct {
	requestv1.UnimplementedApprovalServiceServer
}

func NewApprovalServer() *ApprovalServer {
	return &ApprovalServer{}
}

func (s *ApprovalServer) RequestApproval(ctx context.Context, req *requestv1.RequestApprovalRequest) (*requestv1.Approval, error) {
	if req.SubjectType != requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PRE_DEPLOY {
		return nil, status.Error(codes.FailedPrecondition, "subject type not allowed for direct request")
	}
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) Approve(ctx context.Context, req *requestv1.ApproveRequest) (*requestv1.DecideApprovalResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) Reject(ctx context.Context, req *requestv1.RejectRequest) (*requestv1.DecideApprovalResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) Cancel(ctx context.Context, req *requestv1.CancelApprovalRequest) (*requestv1.Approval, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) GetApproval(ctx context.Context, req *requestv1.GetApprovalRequest) (*requestv1.Approval, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) ListApprovals(ctx context.Context, req *requestv1.ListApprovalsRequest) (*requestv1.ListApprovalsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *ApprovalServer) ListPendingForUser(ctx context.Context, req *requestv1.ListPendingForUserRequest) (*requestv1.ListPendingForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func mapErrorToGrpc(err error) error {
	if err == nil {
		return nil
	}
	// Stub mapping
	return status.Error(codes.Internal, err.Error())
}
