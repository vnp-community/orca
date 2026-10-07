package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type AuthorizeApprovalDecision struct {
	ApproverRepo ApprovalApproverRepository
	Auth         domain.ApprovalAuthorization
	// TeamResolver TeamMembershipResolver
}

var _ ApprovalAuthorizer = (*AuthorizeApprovalDecision)(nil)

func (a *AuthorizeApprovalDecision) CanDecide(ctx context.Context, req domain.Request, ap domain.Approval) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)

	actor := domain.DecisionActor{
		UserID:    userID,
		Role:      role,
		IsMachine: false, // Stub
	}

	approvers, err := a.ApproverRepo.ListForApproval(ctx, tenantID, ap.ID)
	if err != nil {
		return err
	}

	return a.Auth.Decide(actor, ap, approvers, req.ReporterID)
}
