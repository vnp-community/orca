package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReporterOrAdminAuthorizer is the provisional D6 rule: the reporter, or a tenant admin or lead.
// Designated approvers who are neither get in once CR-REQ-009's approver snapshot is wired here.
type ReporterOrAdminAuthorizer struct{}

var _ SolutionActorAuthorizer = ReporterOrAdminAuthorizer{}

func (ReporterOrAdminAuthorizer) AuthorizeGenerate(ctx context.Context, req domain.Request) error {
	return reporterOrAdmin(ctx, req)
}

func (ReporterOrAdminAuthorizer) AuthorizeChoose(ctx context.Context, req domain.Request) error {
	return reporterOrAdmin(ctx, req)
}

// AuthorizeRevise has no approver rule here; ApproverAwareAuthorizer widens it.
func (ReporterOrAdminAuthorizer) AuthorizeRevise(ctx context.Context, req domain.Request) error {
	return reporterOrAdmin(ctx, req)
}

// AuthorizeRevisePlan has no approver rule here; ApproverAwareAuthorizer widens it.
func (ReporterOrAdminAuthorizer) AuthorizeRevisePlan(ctx context.Context, req domain.Request) error {
	return reporterOrAdmin(ctx, req)
}

func reporterOrAdmin(ctx context.Context, req domain.Request) error {
	if uid, _ := tenant.UserID(ctx); uid != "" && uid == req.ReporterID {
		return nil
	}
	if role, _ := tenant.Role(ctx); role == "admin" || role == "lead" || role == "owner" {
		return nil
	}
	return domain.ErrSolutionForbidden()
}

// analysisGateSubjects are the Approval kinds the Analysis gate opens, one per flow: solution, findings (spike) and
// answer (question). Revising must accept any of them, not only solution.
var analysisGateSubjects = []domain.SubjectType{domain.SubjectSolution, domain.SubjectFindings, domain.SubjectAnswer}

// planGateSubjects are the Approval kinds the Plan gate opens: the plan itself, its task list and the pre-deploy check.
var planGateSubjects = []domain.SubjectType{domain.SubjectPlan, domain.SubjectTaskList, domain.SubjectPreDeploy}

// ApproverAwareAuthorizer lets the people who may decide the pending approval steer it too: choosing an option or
// asking for a revision is part of deciding. Starting the first analysis stays with the reporter and admins. The
// decision check is the engine's own (AuthorizeApprovalDecision), so user, team and role approvers count exactly as they
// do at Approve time.
type ApproverAwareAuthorizer struct {
	Base      ReporterOrAdminAuthorizer
	Approvals ApprovalRepository
	Decider   ApprovalAuthorizer
}

var _ SolutionActorAuthorizer = ApproverAwareAuthorizer{}

// NewApproverAwareAuthorizer refuses to build without its approval lookups: a nil one would panic on the first
// revision by a non-reporter, so startup fails instead.
func NewApproverAwareAuthorizer(approvals ApprovalRepository, decider ApprovalAuthorizer) (ApproverAwareAuthorizer, error) {
	if approvals == nil || decider == nil {
		return ApproverAwareAuthorizer{}, errors.New("approver-aware solution authorizer needs the approval repository and the decision authorizer")
	}
	return ApproverAwareAuthorizer{Approvals: approvals, Decider: decider}, nil
}

func (a ApproverAwareAuthorizer) AuthorizeGenerate(ctx context.Context, req domain.Request) error {
	return a.Base.AuthorizeGenerate(ctx, req)
}

func (a ApproverAwareAuthorizer) AuthorizeChoose(ctx context.Context, req domain.Request) error {
	return a.orPendingApprover(ctx, req, a.Base.AuthorizeChoose(ctx, req), domain.SubjectSolution)
}

// AuthorizeRevise must run before the revision cancels the pending Approval: once it is cancelled the caller no longer
// counts as its approver.
func (a ApproverAwareAuthorizer) AuthorizeRevise(ctx context.Context, req domain.Request) error {
	return a.orPendingApprover(ctx, req, a.Base.AuthorizeRevise(ctx, req), analysisGateSubjects...)
}

// AuthorizeRevisePlan, like AuthorizeRevise, must run before the revision cancels the pending Approval.
func (a ApproverAwareAuthorizer) AuthorizeRevisePlan(ctx context.Context, req domain.Request) error {
	return a.orPendingApprover(ctx, req, a.Base.AuthorizeGenerate(ctx, req), planGateSubjects...)
}

// orPendingApprover passes when the base rule did, or when the caller may decide a pending Approval of one of the
// subjects. A failed lookup is returned as is: an outage must not read as "not allowed".
func (a ApproverAwareAuthorizer) orPendingApprover(ctx context.Context, req domain.Request, baseErr error, subjects ...domain.SubjectType) error {
	if baseErr == nil {
		return nil
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	for _, subject := range subjects {
		pending, _, err := a.Approvals.List(ctx, tenantID, ApprovalListFilter{RequestID: req.ID, SubjectType: subject, Status: domain.ApprovalStatusPending, PageSize: 5})
		if err != nil {
			return err
		}
		for _, ap := range pending {
			decideErr := a.Decider.CanDecide(ctx, req, ap)
			if decideErr == nil {
				return nil
			}
			if !isApproverRefusal(decideErr) {
				return decideErr
			}
		}
	}
	return baseErr
}

// isApproverRefusal tells "this caller may not decide" (permission denied or no signed-in user) from a failure of the
// check itself (directory or database outage), which the caller must see as such.
func isApproverRefusal(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && (ae.Kind == apperrors.KindPermissionDenied || ae.Kind == apperrors.KindUnauthenticated)
}
