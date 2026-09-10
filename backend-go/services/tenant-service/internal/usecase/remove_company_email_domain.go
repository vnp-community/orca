package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// RemoveCompanyEmailDomainInput mirrors RemoveCompanyEmailDomainRequest 1:1.
type RemoveCompanyEmailDomainInput struct {
	EmailDomain string
}

// RemoveCompanyEmailDomain unregisters an email domain — an admin-console
// operation (caller MUST admin-gate, same as AddCompanyEmailDomain).
// Idempotent: removing a domain that isn't registered is not an error,
// matching CompanyEmailDomainRepository.Remove's own contract.
type RemoveCompanyEmailDomain struct {
	domains CompanyEmailDomainRepository
}

func NewRemoveCompanyEmailDomain(domains CompanyEmailDomainRepository) *RemoveCompanyEmailDomain {
	return &RemoveCompanyEmailDomain{domains: domains}
}

func (uc *RemoveCompanyEmailDomain) Execute(ctx context.Context, in RemoveCompanyEmailDomainInput) error {
	normalized := domain.NormalizeEmailDomain(in.EmailDomain)
	if normalized == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_EMAIL_DOMAIN", "email_domain is required", nil)
	}
	if err := uc.domains.Remove(ctx, normalized); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_REMOVE_EMAIL_DOMAIN_FAILED", "failed to remove email domain", err)
	}
	return nil
}
