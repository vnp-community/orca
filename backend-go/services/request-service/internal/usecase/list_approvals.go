package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	defaultApprovalPageSize = 50
	maxApprovalPageSize     = 100
)

func clampApprovalPageSize(n int) int {
	if n <= 0 || n > maxApprovalPageSize {
		return defaultApprovalPageSize
	}
	return n
}

type ListApprovals struct {
	Repo ApprovalRepository
}

func (uc *ListApprovals) Execute(ctx context.Context, f ApprovalListFilter) ([]domain.Approval, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}
	f.PageSize = clampApprovalPageSize(f.PageSize)
	return uc.Repo.List(ctx, tenantID, f)
}

// GetApproval reads one approval of the ctx tenant; a foreign tenant's id is indistinguishable from a missing one.
type GetApproval struct {
	Repo ApprovalRepository
}

func (uc *GetApproval) Execute(ctx context.Context, id string) (domain.Approval, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Approval{}, err
	}
	return uc.Repo.Get(ctx, tenantID, id)
}
