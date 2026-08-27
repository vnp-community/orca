package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ValidateTenantInput mirrors ValidateTenantRequest 1:1.
type ValidateTenantInput struct {
	TenantID string
}

// ValidateTenant is the logical-FK check every other service calls to
// confirm a tenant_id it received is real (tenant-service.md §3, §9). It
// has no tenant-context requirement of its own — the tenant_id under test
// IS the input, not the caller's own scope, and this service must not call
// back out to validate the caller (tenant-service.md §7: "zero outbound
// service calls").
type ValidateTenant struct {
	companies CompanyRepository
}

func NewValidateTenant(companies CompanyRepository) *ValidateTenant {
	return &ValidateTenant{companies: companies}
}

func (uc *ValidateTenant) Execute(ctx context.Context, in ValidateTenantInput) (bool, error) {
	if in.TenantID == "" {
		return false, nil
	}
	exists, err := uc.companies.Exists(ctx, in.TenantID)
	if err != nil {
		return false, apperrors.New(apperrors.KindInternal, "TENANT_VALIDATE_FAILED", "failed to check tenant existence", err)
	}
	return exists, nil
}
