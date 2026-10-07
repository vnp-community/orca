package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListApprovals struct {
	Repo ApprovalRepository
}

func (uc *ListApprovals) Execute(ctx context.Context, f ApprovalListFilter) ([]domain.Approval, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 50
	}
	return uc.Repo.List(ctx, tenantID, f)
}
