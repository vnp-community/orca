package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ApprovalRequestLocator finds the Request of an approval for the access check. Not found and another
// tenant's approval give the same error because the repository read is tenant scoped.
type ApprovalRequestLocator struct{ approvals ApprovalRepository }

func NewApprovalRequestLocator(approvals ApprovalRepository) *ApprovalRequestLocator {
	return &ApprovalRequestLocator{approvals: approvals}
}

func (l *ApprovalRequestLocator) RequestIDOf(ctx context.Context, approvalID string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", domain.ErrRequestTenantRequired()
	}
	a, err := l.approvals.Get(ctx, tenantID, approvalID)
	if err != nil {
		return "", err
	}
	return a.RequestID, nil
}
