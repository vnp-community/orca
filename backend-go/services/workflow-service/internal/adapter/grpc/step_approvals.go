package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// WithStepApprovals enables the step approval RPCs (docs/integrate/TDD-WF-AGENT-001 §7.5).
func (s *Server) WithStepApprovals(uc *usecase.StepApprovals) *Server {
	s.stepApprovals = uc
	return s
}

func (s *Server) stepApprovalsEnabled() error {
	if s.stepApprovals == nil {
		return status.Error(codes.Unimplemented, "step approvals are not enabled on this workflow-service")
	}
	return nil
}

func (s *Server) ListStepApprovals(ctx context.Context, req *workflowv1.ListStepApprovalsRequest) (*workflowv1.ListStepApprovalsResponse, error) {
	if err := s.stepApprovalsEnabled(); err != nil {
		return nil, err
	}
	rows, err := s.stepApprovals.List(ctx, req.GetProjectId(), req.GetExecutionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*workflowv1.StepApproval, 0, len(rows))
	for _, a := range rows {
		out = append(out, toProtoStepApproval(a))
	}
	return &workflowv1.ListStepApprovalsResponse{Approvals: out}, nil
}

func (s *Server) GetStepApproval(ctx context.Context, req *workflowv1.GetStepApprovalRequest) (*workflowv1.GetStepApprovalResponse, error) {
	if err := s.stepApprovalsEnabled(); err != nil {
		return nil, err
	}
	a, canDecide, err := s.stepApprovals.Get(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.GetStepApprovalResponse{Approval: toProtoStepApproval(a), CanDecide: canDecide}, nil
}

func (s *Server) ResolveStepApproval(ctx context.Context, req *workflowv1.ResolveStepApprovalRequest) (*workflowv1.ResolveStepApprovalResponse, error) {
	if err := s.stepApprovalsEnabled(); err != nil {
		return nil, err
	}
	var approve bool
	switch req.GetDecision() {
	case "approved":
		approve = true
	case "rejected":
	default:
		return nil, status.Error(codes.InvalidArgument, `decision must be "approved" or "rejected"`)
	}
	a, err := s.stepApprovals.Resolve(ctx, req.GetId(), approve, req.GetDecisionHash(), req.GetRationale())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.ResolveStepApprovalResponse{Approval: toProtoStepApproval(a)}, nil
}

func toProtoStepApproval(a domain.StepApproval) *workflowv1.StepApproval {
	out := &workflowv1.StepApproval{
		Id: a.ID, ExecutionId: a.ExecutionID, StepId: a.StepID, ProjectId: a.ProjectID, Kind: a.Kind,
		SubjectJson: string(a.Subject), DecisionJson: string(a.Decision), DecisionHash: a.DecisionHash,
		Status: string(a.Status), ResolvedBy: a.ResolvedBy, Rationale: a.Rationale, CreatedAt: timestamppb.New(a.CreatedAt),
	}
	if a.ResolvedAt != nil {
		out.ResolvedAt = timestamppb.New(*a.ResolvedAt)
	}
	return out
}
