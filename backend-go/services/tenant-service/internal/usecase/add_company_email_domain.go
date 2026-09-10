package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// AddCompanyEmailDomainInput mirrors AddCompanyEmailDomainRequest 1:1.
type AddCompanyEmailDomainInput struct {
	CompanyID   string
	EmailDomain string
}

// AddCompanyEmailDomain registers an email domain (e.g. "vnpay.vn") as
// belonging to a company — an admin-console operation (caller MUST
// admin-gate, same convention CompanyRepository.List's doc comment already
// establishes for this service; no internal OPA/role check exists in
// tenant-service, see its README). Rejects claiming a domain another
// company already registered — see this method's KindAlreadyExists branch.
type AddCompanyEmailDomain struct {
	companies CompanyRepository
	domains   CompanyEmailDomainRepository
}

func NewAddCompanyEmailDomain(companies CompanyRepository, domains CompanyEmailDomainRepository) *AddCompanyEmailDomain {
	return &AddCompanyEmailDomain{companies: companies, domains: domains}
}

func (uc *AddCompanyEmailDomain) Execute(ctx context.Context, in AddCompanyEmailDomainInput) (domain.CompanyEmailDomain, error) {
	exists, err := uc.companies.Exists(ctx, in.CompanyID)
	if err != nil {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to check company existence", err)
	}
	if !exists {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	cd, err := domain.NewCompanyEmailDomain(in.EmailDomain, in.CompanyID)
	if err != nil {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_EMAIL_DOMAIN", err.Error(), err)
	}

	// Pre-check for a friendly error message — see CompanyEmailDomainRepository.Add's
	// doc comment for why the repository itself doesn't enforce this (a
	// concurrent double-registration race is rare enough to surface as a
	// generic KindInternal error instead, not worth building extra
	// conflict-handling beyond the PRIMARY KEY that already prevents two
	// DIFFERENT companies ever successfully persisting the same domain).
	existingCompanyID, found, err := uc.domains.ResolveCompanyID(ctx, cd.EmailDomain)
	if err != nil {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindInternal, "TENANT_EMAIL_DOMAIN_LOOKUP_FAILED", "failed to check existing email domain registration", err)
	}
	if found && existingCompanyID != cd.CompanyID {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindAlreadyExists, "TENANT_EMAIL_DOMAIN_TAKEN", "this email domain is already registered to a different company", nil)
	}

	if err := uc.domains.Add(ctx, cd.CompanyID, cd.EmailDomain); err != nil {
		return domain.CompanyEmailDomain{}, apperrors.New(apperrors.KindInternal, "TENANT_ADD_EMAIL_DOMAIN_FAILED", "failed to persist email domain", err)
	}
	return cd, nil
}
