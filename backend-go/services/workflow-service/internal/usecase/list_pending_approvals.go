package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ListPendingApprovalsInput is ListPendingApprovals' request shape.
type ListPendingApprovalsInput struct {
	PageToken string
	PageSize  int32
}

// ListPendingApprovalsOutput is ListPendingApprovals' response shape.
type ListPendingApprovalsOutput struct {
	Approvals     []domain.Approval
	NextPageToken string
}

// ListPendingApprovals lists this tenant's pending publish-approval gates —
// the lookup an admin needs before calling usecase.ResolveApproval on any
// of them (that RPC/usecase pair otherwise has no way to discover pending
// approval ids). Admin-only, matching ResolveApproval's own gate.
type ListPendingApprovals struct {
	approvals ApprovalRepository
	opa       OPAChecker
}

func NewListPendingApprovals(approvals ApprovalRepository, opa OPAChecker) *ListPendingApprovals {
	return &ListPendingApprovals{approvals: approvals, opa: opa}
}

func (uc *ListPendingApprovals) Execute(ctx context.Context, in ListPendingApprovalsInput) (ListPendingApprovalsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListPendingApprovalsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	callerUserID, ok := tenant.UserID(ctx)
	if !ok {
		return ListPendingApprovalsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}
	if !uc.opa.IsAdmin(ctx, callerUserID) {
		return ListPendingApprovalsOutput{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_APPROVAL_ADMIN_ONLY", "only an admin may list pending approvals", nil)
	}

	approvals, next, err := uc.approvals.ListPending(ctx, tenantID, in.PageToken, in.PageSize)
	if err != nil {
		return ListPendingApprovalsOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_LIST_APPROVALS_FAILED", "failed to list pending approvals", err)
	}
	return ListPendingApprovalsOutput{Approvals: approvals, NextPageToken: next}, nil
}
