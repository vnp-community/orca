package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ResolveApprovalInput is ResolveApproval's request shape.
type ResolveApprovalInput struct {
	ApprovalID string
	Decision   domain.ApprovalStatus // ApprovalApproved | ApprovalRejected
}

// ResolveApproval is the admin-only decision half of BUG-WF-03's
// lead-requires-admin-approval gate (see PublishTemplate's doc comment).
// Approving atomically flips the approval's status AND applies
// VisibilityCompany to its template, in one transaction
// (ApprovalRepositoryTx.Templates()) — orchestration-service.md §8's
// discipline again: a torn write here could leave an approval marked
// "approved" with the template still private, or vice versa.
type ResolveApproval struct {
	approvals ApprovalRepository
	opa       OPAChecker
}

func NewResolveApproval(approvals ApprovalRepository, opa OPAChecker) *ResolveApproval {
	return &ResolveApproval{approvals: approvals, opa: opa}
}

func (uc *ResolveApproval) Execute(ctx context.Context, in ResolveApprovalInput) (domain.Approval, error) {
	callerUserID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Approval{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}
	if !uc.opa.IsAdmin(ctx, callerUserID) {
		return domain.Approval{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_APPROVAL_ADMIN_ONLY", "only an admin may resolve an approval", nil)
	}
	if in.Decision != domain.ApprovalApproved && in.Decision != domain.ApprovalRejected {
		return domain.Approval{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_APPROVAL_DECISION", "decision must be approved or rejected", nil)
	}

	var result domain.Approval
	err := uc.approvals.WithTx(ctx, func(tx ApprovalRepositoryTx) error {
		approval, gerr := tx.Get(ctx, in.ApprovalID)
		if gerr != nil {
			return gerr
		}

		now := time.Now()
		var terr error
		if in.Decision == domain.ApprovalApproved {
			terr = approval.Approve(callerUserID, now)
		} else {
			terr = approval.Reject(callerUserID, now)
		}
		if terr != nil {
			return terr
		}
		if err := tx.Update(ctx, approval); err != nil {
			return err
		}

		if in.Decision == domain.ApprovalApproved {
			if err := tx.Templates().SetVisibility(ctx, approval.TemplateID, domain.VisibilityCompany); err != nil {
				return err
			}
		}
		result = approval
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrApprovalNotFound):
			return domain.Approval{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_APPROVAL_NOT_FOUND", "approval not found", err)
		case errors.Is(err, domain.ErrApprovalNotPending):
			return domain.Approval{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_APPROVAL_NOT_PENDING", "approval has already been resolved", err)
		case errors.Is(err, domain.ErrTemplateNotFound):
			return domain.Approval{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		default:
			return domain.Approval{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_APPROVAL_RESOLVE_FAILED", "failed to resolve approval", err)
		}
	}
	return result, nil
}
