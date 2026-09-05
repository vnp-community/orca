package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ListCompanyEmailDomainsInput mirrors ListCompanyEmailDomainsRequest 1:1.
type ListCompanyEmailDomainsInput struct {
	CompanyID string
}

// ListCompanyEmailDomains returns every email domain registered to a
// company — the admin-console read backing AddCompanyEmailDomain/
// RemoveCompanyEmailDomain's write side. Cross-tenant by nature the same
// way ListCompanies is (any admin could ask for any company's domain
// list): caller MUST admin-gate.
type ListCompanyEmailDomains struct {
	domains CompanyEmailDomainRepository
}

func NewListCompanyEmailDomains(domains CompanyEmailDomainRepository) *ListCompanyEmailDomains {
	return &ListCompanyEmailDomains{domains: domains}
}

func (uc *ListCompanyEmailDomains) Execute(ctx context.Context, in ListCompanyEmailDomainsInput) ([]string, error) {
	domains, err := uc.domains.ListForCompany(ctx, in.CompanyID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_EMAIL_DOMAINS_FAILED", "failed to list email domains", err)
	}
	return domains, nil
}
