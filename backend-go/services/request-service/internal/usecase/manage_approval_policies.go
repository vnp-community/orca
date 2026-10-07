package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ManageApprovalPolicies struct {
	Repo ApprovalPolicyRepository
}

func (uc *ManageApprovalPolicies) List(ctx context.Context) ([]domain.ApprovalPolicy, error) {
	return nil, nil // Stub
}

func (uc *ManageApprovalPolicies) Upsert(ctx context.Context, p domain.ApprovalPolicy) error {
	role, _ := tenant.Role(ctx)
	if role != "admin" {
		return errors.New("PermissionDenied")
	}

	if len(p.Approvers) > 20 {
		return errors.New("REQUEST_APPROVAL_POLICY_INVALID")
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	p.TenantID = tenantID

	return uc.Repo.Upsert(ctx, p)
}

func (uc *ManageApprovalPolicies) Delete(ctx context.Context, id string) error {
	role, _ := tenant.Role(ctx)
	if role != "admin" {
		return errors.New("PermissionDenied")
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}

	return uc.Repo.Delete(ctx, tenantID, id)
}
