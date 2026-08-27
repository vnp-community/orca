package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// GetResolvedProfile is the uncached base implementation: fetch all four
// layers (company, department, teams, user), then call the pure domain
// merge. Wrapped by CachedGetResolvedProfile (see
// cached_get_resolved_profile.go) for the hot path — this type has zero
// cache awareness itself, per tenant-service.md §6's "profile-resolution
// caching is a usecase/ concern, expressed as a [decorator], not baked into
// adapter/postgres".
type GetResolvedProfile struct {
	companies   CompanyRepository
	departments DepartmentRepository
	profiles    UserProfileRepository
	teams       TeamRepository
}

func NewGetResolvedProfile(companies CompanyRepository, departments DepartmentRepository, profiles UserProfileRepository, teams TeamRepository) *GetResolvedProfile {
	return &GetResolvedProfile{companies: companies, departments: departments, profiles: profiles, teams: teams}
}

// Execute resolves userID's profile. GetResolvedProfileRequest has no
// company_id bound field (tenant.proto), so the scoping company comes
// exclusively from the request context (tenant.RequireTenantID) — this is
// the hot path tenant-service.md §8 calls out for single-digit-ms p99 on
// cache hit, low tens of ms on cache miss (four indexed point-reads).
func (uc *GetResolvedProfile) Execute(ctx context.Context, userID string) (domain.ResolvedProfile, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	company, found, err := uc.companies.Get(ctx, companyID)
	if err != nil {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to look up company", err)
	}
	if !found {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	profile, hasProfile, err := uc.profiles.Get(ctx, companyID, userID)
	if err != nil {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}

	var departmentSettings domain.Settings
	if hasProfile && profile.DepartmentID != "" {
		department, foundDept, err := uc.departments.Get(ctx, companyID, profile.DepartmentID)
		if err != nil {
			return domain.ResolvedProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_DEPARTMENT_LOOKUP_FAILED", "failed to look up department", err)
		}
		if foundDept {
			departmentSettings = department.Settings
		}
	}

	teamLayers, err := uc.teams.ListUserTeamLayers(ctx, companyID, userID)
	if err != nil {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team memberships", err)
	}

	var userSettings domain.Settings
	if hasProfile {
		userSettings = profile.Settings
	}

	resolved, err := domain.ResolveProfile(company.Settings, departmentSettings, teamLayers, userSettings)
	if err != nil {
		return domain.ResolvedProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_RESOLVE_PROFILE_FAILED", "failed to resolve profile", err)
	}
	return resolved, nil
}
