package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateDepartmentInput mirrors UpdateDepartmentRequest 1:1. CompanyID comes
// from request context (tenant.RequireTenantID), same convention as
// SetUserDepartment — UpdateDepartmentRequest has no company_id field.
type UpdateDepartmentInput struct {
	ID    string
	Patch domain.DepartmentSettingsPatch
}

// UpdateDepartment applies patch and invalidates every user IN THAT
// DEPARTMENT's cached ResolvedProfile — narrower scope than UpdateCompany,
// per tenant-service.md §8's per-mutation invalidation-scope table.
type UpdateDepartment struct {
	departments  DepartmentRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
	opa          OPAClient      // NEW
	audit        AuditPublisher // NEW
}

func NewUpdateDepartment(departments DepartmentRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher, opa OPAClient, audit AuditPublisher) *UpdateDepartment {
	return &UpdateDepartment{departments: departments, profiles: profiles, cache: cache, invalidation: invalidation, opa: opa, audit: audit}
}

func (uc *UpdateDepartment) Execute(ctx context.Context, in UpdateDepartmentInput) (domain.Department, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	// sameDepartment: does the caller's own department match the one being
	// edited? A lead may edit only their own department — see
	// requireDepartmentAccess's doc comment.
	actorID, _ := tenant.UserID(ctx)
	actorProfile, _, err := uc.profiles.Get(ctx, companyID, actorID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up caller profile", err)
	}
	sameDepartment := actorProfile.DepartmentID == in.ID
	if err := requireDepartmentAccess(ctx, uc.opa, sameDepartment); err != nil {
		return domain.Department{}, err
	}

	if in.Patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(in.Patch.SettingsJSON), &settings); err != nil {
			return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_SETTINGS_JSON", "malformed settings_json", err)
		}
		if err := domain.ValidateDepartmentSettings(settings); err != nil {
			return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_DEPARTMENT_SETTINGS", err.Error(), err)
		}
	}

	dept, found, err := uc.departments.Update(ctx, companyID, in.ID, in.Patch)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_DEPARTMENT_FAILED", "failed to update department", err)
	}
	if !found {
		return domain.Department{}, apperrors.New(apperrors.KindNotFound, "TENANT_DEPARTMENT_NOT_FOUND", "department does not exist", nil)
	}

	userIDs, err := uc.profiles.ListUserIDsByDepartment(ctx, companyID, in.ID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_LIST_DEPARTMENT_USERS_FAILED", "failed to resolve invalidation scope", err)
	}
	for _, uid := range userIDs {
		if uc.cache != nil {
			uc.cache.Invalidate(ctx, uid)
		}
		if uc.invalidation != nil {
			_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, uid)
		}
	}

	if uc.audit != nil {
		_ = uc.audit.PublishAuditEvent(ctx, companyID, actorID, "department.profile.updated", in.ID)
	}
	return dept, nil
}
