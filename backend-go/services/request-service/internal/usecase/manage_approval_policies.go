package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ManageApprovalPolicies is the admin CRUD for approver policies. Callers are authorized here, not at the gateway.
type ManageApprovalPolicies struct {
	Repo ApprovalPolicyRepository
}

func requireAdmin(ctx context.Context) (tenantID, userID string, err error) {
	tenantID, err = tenant.RequireTenantID(ctx)
	if err != nil {
		return "", "", err
	}
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		return "", "", domain.ErrAgentForbidden
	}
	if role, _ := tenant.Role(ctx); role != "admin" {
		return "", "", domain.ErrApprovalForbidden
	}
	userID, _ = tenant.UserID(ctx)
	return tenantID, userID, nil
}

func (uc *ManageApprovalPolicies) List(ctx context.Context, f PolicyListFilter) ([]domain.ApprovalPolicy, string, error) {
	tenantID, _, err := requireAdmin(ctx)
	if err != nil {
		return nil, "", err
	}
	f.PageSize = clampApprovalPageSize(f.PageSize)
	return uc.Repo.List(ctx, tenantID, f)
}

// Upsert creates a policy when p.ID is empty, otherwise updates it with a version check.
func (uc *ManageApprovalPolicies) Upsert(ctx context.Context, p domain.ApprovalPolicy, expectedVersion int64) (domain.ApprovalPolicy, error) {
	tenantID, userID, err := requireAdmin(ctx)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	if err := p.Validate(); err != nil {
		return domain.ApprovalPolicy{}, err
	}
	p.TenantID = tenantID
	if p.ID == "" {
		p.ID = uuid.NewString()
		expectedVersion = 0
		p.CreatedBy = userID
	} else if expectedVersion == 0 {
		// An update without a version would silently overwrite a concurrent edit.
		return domain.ApprovalPolicy{}, domain.ErrApprovalVersionConflict
	}
	return uc.Repo.Upsert(ctx, p, expectedVersion)
}

func (uc *ManageApprovalPolicies) Delete(ctx context.Context, id string, expectedVersion int64) error {
	tenantID, _, err := requireAdmin(ctx)
	if err != nil {
		return err
	}
	return uc.Repo.Delete(ctx, tenantID, id, expectedVersion)
}
