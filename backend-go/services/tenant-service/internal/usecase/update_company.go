package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateCompanyInput mirrors UpdateCompanyRequest 1:1.
type UpdateCompanyInput struct {
	ID    string
	Patch domain.CompanySettingsPatch
}

// UpdateCompany applies patch and invalidates every affected user's cached
// ResolvedProfile — the company layer is the base of every merge, so its
// scope is EVERY user in the company (tenant-service.md §8).
type UpdateCompany struct {
	companies    CompanyRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

func NewUpdateCompany(companies CompanyRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *UpdateCompany {
	return &UpdateCompany{companies: companies, profiles: profiles, cache: cache, invalidation: invalidation}
}

func (uc *UpdateCompany) Execute(ctx context.Context, in UpdateCompanyInput) (domain.Company, error) {
	company, found, err := uc.companies.Update(ctx, in.ID, in.Patch)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_COMPANY_FAILED", "failed to update company", err)
	}
	if !found {
		return domain.Company{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	userIDs, err := uc.profiles.ListUserIDsByCompany(ctx, in.ID)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_LIST_COMPANY_USERS_FAILED", "failed to resolve invalidation scope", err)
	}
	for _, uid := range userIDs {
		if uc.cache != nil {
			uc.cache.Invalidate(ctx, uid)
		}
		if uc.invalidation != nil {
			_ = uc.invalidation.PublishProfileInvalidated(ctx, in.ID, uid)
		}
	}
	return company, nil
}
