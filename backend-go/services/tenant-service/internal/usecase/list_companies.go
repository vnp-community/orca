package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListCompanies is the missing "browse every company" read — without it, a
// company created via CreateCompany becomes unreachable the moment the
// creating session ends: GetCompany only ever answers for one known id, and
// nothing else enumerates tenant.companies. Cross-tenant by nature (see
// CompanyRepository.List's doc comment); the wscompat channel calling this
// MUST admin-gate the same way profile.createCompany/createDept already do.
type ListCompanies struct {
	companies CompanyRepository
}

func NewListCompanies(companies CompanyRepository) *ListCompanies {
	return &ListCompanies{companies: companies}
}

func (uc *ListCompanies) Execute(ctx context.Context) ([]domain.Company, error) {
	companies, err := uc.companies.List(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_COMPANIES_FAILED", "failed to list companies", err)
	}
	return companies, nil
}
