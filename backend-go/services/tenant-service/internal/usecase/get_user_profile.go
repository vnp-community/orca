package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// GetUserProfileInput mirrors GetUserProfileRequest 1:1.
type GetUserProfileInput struct {
	UserID string
}

// GetUserProfile is thin: UserProfileRepository.Get already exists and
// already does exactly this lookup (added for SetUserDepartment's internal
// use, never exposed as its own RPC before now).
type GetUserProfile struct {
	profiles UserProfileRepository
}

func NewGetUserProfile(profiles UserProfileRepository) *GetUserProfile {
	return &GetUserProfile{profiles: profiles}
}

func (uc *GetUserProfile) Execute(ctx context.Context, in GetUserProfileInput) (domain.UserProfile, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	profile, found, err := uc.profiles.Get(ctx, companyID, in.UserID)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}
	if !found {
		return domain.UserProfile{}, apperrors.New(apperrors.KindNotFound, "TENANT_PROFILE_NOT_FOUND", "user profile does not exist", nil)
	}
	return profile, nil
}
