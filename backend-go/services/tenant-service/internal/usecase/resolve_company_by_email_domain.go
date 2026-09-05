package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ResolveCompanyByEmailDomainInput mirrors ResolveCompanyByEmailDomainRequest 1:1.
type ResolveCompanyByEmailDomainInput struct {
	EmailDomain string
}

// ResolveCompanyByEmailDomainResult mirrors ResolveCompanyByEmailDomainResponse 1:1.
type ResolveCompanyByEmailDomainResult struct {
	CompanyID string
	Found     bool
}

// ResolveCompanyByEmailDomain answers "which company does this email
// domain belong to" — auth-service's TenantResolver calls this,
// server-to-server, to decide a brand-new SSO signup's tenant (CR-LOGIN-001
// multi-tenant follow-up). Not admin-gated: unlike Add/Remove/List above,
// this is a system-internal read only auth-service calls, never exposed as
// an end-user or admin-console REST route — same trust boundary
// ListCompanies already has for TenantResolver's existing single-tenant
// fallback call (see that type's doc comment in auth-service).
type ResolveCompanyByEmailDomain struct {
	domains CompanyEmailDomainRepository
}

func NewResolveCompanyByEmailDomain(domains CompanyEmailDomainRepository) *ResolveCompanyByEmailDomain {
	return &ResolveCompanyByEmailDomain{domains: domains}
}

func (uc *ResolveCompanyByEmailDomain) Execute(ctx context.Context, in ResolveCompanyByEmailDomainInput) (ResolveCompanyByEmailDomainResult, error) {
	normalized := domain.NormalizeEmailDomain(in.EmailDomain)
	if normalized == "" {
		return ResolveCompanyByEmailDomainResult{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_EMAIL_DOMAIN", "email_domain is required", nil)
	}
	companyID, found, err := uc.domains.ResolveCompanyID(ctx, normalized)
	if err != nil {
		return ResolveCompanyByEmailDomainResult{}, apperrors.New(apperrors.KindInternal, "TENANT_RESOLVE_EMAIL_DOMAIN_FAILED", "failed to resolve company by email domain", err)
	}
	return ResolveCompanyByEmailDomainResult{CompanyID: companyID, Found: found}, nil
}
