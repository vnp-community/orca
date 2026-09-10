package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// GetCompany is the missing read half of CreateCompany/UpdateCompany — the
// Admin Console needs to show the caller's own company name before
// offering to rename it.
type GetCompany struct {
	companies CompanyRepository
}

func NewGetCompany(companies CompanyRepository) *GetCompany {
	return &GetCompany{companies: companies}
}

func (uc *GetCompany) Execute(ctx context.Context, id string) (domain.Company, error) {
	company, found, err := uc.companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_GET_COMPANY_FAILED", "failed to load company", err)
	}
	if !found {
		return domain.Company{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company not found", nil)
	}
	return company, nil
}
