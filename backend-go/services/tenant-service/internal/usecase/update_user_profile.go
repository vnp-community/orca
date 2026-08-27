package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateUserProfileInput mirrors UpdateUserProfileRequest 1:1 — see that
// message's doc comment (TASK-127) for the clear_department/department_id
// no-change-vs-clear contract.
type UpdateUserProfileInput struct {
	UserID          string
	DepartmentID    string
	ClearDepartment bool
	Settings        domain.Settings
	SetSettings     bool // false = settings_json was empty ("" = no change)
}

// UpdateUserProfile is the "expose Upsert directly" RPC —
// UserProfileRepository.Upsert already existed for SetUserDepartment's
// internal use; this is the case the port's own former doc comment
// ("no UpdateUserProfile RPC yet") flagged as missing. Invalidation scope
// is just the one user being updated — no extra lookup needed.
type UpdateUserProfile struct {
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

func NewUpdateUserProfile(profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *UpdateUserProfile {
	return &UpdateUserProfile{profiles: profiles, cache: cache, invalidation: invalidation}
}

func (uc *UpdateUserProfile) Execute(ctx context.Context, in UpdateUserProfileInput) (domain.UserProfile, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	existing, _, err := uc.profiles.Get(ctx, companyID, in.UserID)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}

	departmentID := existing.DepartmentID
	if in.ClearDepartment {
		departmentID = ""
	} else if in.DepartmentID != "" {
		departmentID = in.DepartmentID
	}
	settings := existing.Settings
	if in.SetSettings {
		settings = in.Settings
	}

	profile, err := domain.NewUserProfile(in.UserID, companyID, departmentID, settings)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_PROFILE", err.Error(), err)
	}
	if err := uc.profiles.Upsert(ctx, profile); err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_PROFILE_FAILED", "failed to persist user profile", err)
	}

	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	if uc.invalidation != nil {
		_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, in.UserID)
	}
	return profile, nil
}
