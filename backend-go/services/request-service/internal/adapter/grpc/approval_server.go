package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ApprovalUseCases are the CR-REQ-009/010 use cases behind ApprovalService. Tenant and caller always come from ctx
// (grpcmw), never from the request body; each use case authorizes for itself because the gateway does not.
type ApprovalUseCases struct {
	Request *usecase.RequestApprovalFromAPI
	Decide  *usecase.DecideApproval
	Cancel  *usecase.CancelApproval
	Get     *usecase.GetApproval
	List    *usecase.ListApprovals
	Pending *usecase.ListPendingApprovalsForUser
	Extend  *usecase.ExtendApproval
}

type ApprovalServer struct {
	requestv1.UnimplementedApprovalServiceServer
	uc ApprovalUseCases
}

func NewApprovalServer(uc ApprovalUseCases) *ApprovalServer { return &ApprovalServer{uc: uc} }

func (s *ApprovalServer) RequestApproval(ctx context.Context, req *requestv1.RequestApprovalRequest) (*requestv1.RequestApprovalResponse, error) {
	if s.uc.Request == nil {
		return nil, status.Error(codes.Unimplemented, "RequestApproval is not wired")
	}
	var key *string
	if k := req.GetIdempotencyKey(); k != "" {
		key = &k
	}
	a, err := s.uc.Request.Execute(ctx, usecase.OpenApprovalInput{
		RequestID: req.GetRequestId(), SubjectType: toDomainSubject(req.GetSubjectType()), SubjectID: req.GetSubjectId(), IdempotencyKey: key,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.RequestApprovalResponse{Approval: toProtoApproval(*a)}, nil
}

func (s *ApprovalServer) decide(ctx context.Context, decision, id, comment, digest string, version int64) (*requestv1.Approval, string, error) {
	if s.uc.Decide == nil {
		return nil, "", status.Error(codes.Unimplemented, "approval decisions are not wired")
	}
	res, err := s.uc.Decide.Execute(ctx, usecase.DecideApprovalInput{ID: id, Decision: decision, Comment: comment, ExpectedDigest: digest, ExpectedVersion: version})
	if err != nil {
		return nil, "", apperrors.ToGRPCStatus(err)
	}
	return toProtoApproval(res.Approval), string(res.RequestStatus), nil
}

func (s *ApprovalServer) Approve(ctx context.Context, req *requestv1.ApproveRequest) (*requestv1.ApproveResponse, error) {
	a, st, err := s.decide(ctx, usecase.DecisionApprove, req.GetId(), req.GetComment(), req.GetExpectedDigest(), req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	return &requestv1.ApproveResponse{Approval: a, RequestStatus: st}, nil
}

func (s *ApprovalServer) Reject(ctx context.Context, req *requestv1.RejectRequest) (*requestv1.RejectResponse, error) {
	a, st, err := s.decide(ctx, usecase.DecisionReject, req.GetId(), req.GetComment(), req.GetExpectedDigest(), req.GetExpectedVersion())
	if err != nil {
		return nil, err
	}
	return &requestv1.RejectResponse{Approval: a, RequestStatus: st}, nil
}

func (s *ApprovalServer) Cancel(ctx context.Context, req *requestv1.ApprovalServiceCancelRequest) (*requestv1.ApprovalServiceCancelResponse, error) {
	if s.uc.Cancel == nil {
		return nil, status.Error(codes.Unimplemented, "Cancel is not wired")
	}
	a, err := s.uc.Cancel.Execute(ctx, req.GetId(), req.GetReason())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ApprovalServiceCancelResponse{Approval: toProtoApproval(*a)}, nil
}

func (s *ApprovalServer) GetApproval(ctx context.Context, req *requestv1.GetApprovalRequest) (*requestv1.GetApprovalResponse, error) {
	if s.uc.Get == nil {
		return nil, status.Error(codes.Unimplemented, "GetApproval is not wired")
	}
	a, err := s.uc.Get.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetApprovalResponse{Approval: toProtoApproval(a)}, nil
}

func (s *ApprovalServer) ListApprovals(ctx context.Context, req *requestv1.ListApprovalsRequest) (*requestv1.ListApprovalsResponse, error) {
	if s.uc.List == nil {
		return nil, status.Error(codes.Unimplemented, "ListApprovals is not wired")
	}
	list, next, err := s.uc.List.Execute(ctx, usecase.ApprovalListFilter{
		RequestID: req.GetRequestId(), SubjectType: toDomainSubject(req.GetSubjectType()), Status: toDomainStatus(req.GetStatus()),
		PageSize: int(req.GetPageSize()), PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListApprovalsResponse{NextPageToken: next}
	for _, a := range list {
		out.Approvals = append(out.Approvals, toProtoApproval(a))
	}
	return out, nil
}

func (s *ApprovalServer) ListPendingForUser(ctx context.Context, req *requestv1.ListPendingForUserRequest) (*requestv1.ListPendingForUserResponse, error) {
	if s.uc.Pending == nil {
		return nil, status.Error(codes.Unimplemented, "ListPendingForUser is not wired")
	}
	list, next, err := s.uc.Pending.Execute(ctx, toDomainSubject(req.GetSubjectType()), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListPendingForUserResponse{NextPageToken: next}
	for _, a := range list {
		out.Approvals = append(out.Approvals, toProtoPending(a))
	}
	return out, nil
}

func (s *ApprovalServer) ExtendApproval(ctx context.Context, req *requestv1.ExtendApprovalRequest) (*requestv1.ExtendApprovalResponse, error) {
	if s.uc.Extend == nil {
		return nil, status.Error(codes.Unimplemented, "ExtendApproval is not wired")
	}
	a, err := s.uc.Extend.Execute(ctx, usecase.ExtendApprovalInput{
		ID: req.GetId(), ExtendSeconds: int(req.GetExtendSeconds()), Reason: req.GetReason(), ExpectedVersion: req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ExtendApprovalResponse{Approval: toProtoApproval(*a)}, nil
}
