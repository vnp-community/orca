package usecase

import (
	"context"

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

func reporterOrAdmin(ctx context.Context, req domain.Request) error {
	if uid, _ := tenant.UserID(ctx); uid != "" && uid == req.ReporterID {
		return nil
	}
	if role, _ := tenant.Role(ctx); role == "admin" || role == "lead" || role == "owner" {
		return nil
	}
	return domain.ErrSolutionForbidden()
}

// ApproverAwareAuthorizer lets the people who may decide the pending approval steer it too: choosing an option is
// part of deciding. Generating stays with the reporter and admins. The decision check is the engine's own
// (AuthorizeApprovalDecision), so team and role approvers count exactly as they do at Approve time.
type ApproverAwareAuthorizer struct {
	Base      ReporterOrAdminAuthorizer
	Approvals ApprovalRepository
	Decider   ApprovalAuthorizer
}

var _ SolutionActorAuthorizer = ApproverAwareAuthorizer{}

func (a ApproverAwareAuthorizer) AuthorizeGenerate(ctx context.Context, req domain.Request) error {
	return a.Base.AuthorizeGenerate(ctx, req)
}

func (a ApproverAwareAuthorizer) AuthorizeChoose(ctx context.Context, req domain.Request) error {
	baseErr := a.Base.AuthorizeChoose(ctx, req)
	if baseErr == nil {
		return nil
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	pending, _, err := a.Approvals.List(ctx, tenantID, ApprovalListFilter{RequestID: req.ID, SubjectType: domain.SubjectSolution, Status: domain.ApprovalStatusPending, PageSize: 5})
	if err != nil {
		return err
	}
	for _, ap := range pending {
		if a.Decider.CanDecide(ctx, req, ap) == nil {
			return nil
		}
	}
	return baseErr
}
