package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GetOnboardingState/SetOnboardingState persist the onboarding wizard's
// per-user progress — see UserProfileRepository.GetOnboardingState's doc
// comment for why this is a dedicated store rather than folded into
// UpdateUserProfile's settings_json. The wire shape (frontend's
// OnboardingState) is opaque JSON as far as this service is concerned —
// api-gateway's onboarding.get/update wscompat channels own decoding it.
//
// Neither request carries company_id (tenant.proto) — same convention as
// SetUserDepartment: the scoping company comes from the request context
// (tenant.RequireTenantID), populated by the gRPC tenant-extraction
// interceptor.

type GetOnboardingState struct {
	profiles UserProfileRepository
}

func NewGetOnboardingState(profiles UserProfileRepository) *GetOnboardingState {
	return &GetOnboardingState{profiles: profiles}
}

// GetOnboardingStateResult's Found distinguishes "never saved" (caller
// should render the wizard-not-started defaults) from a real, possibly
// empty-but-saved state.
type GetOnboardingStateResult struct {
	StateJSON string
	Found     bool
}

func (uc *GetOnboardingState) Execute(ctx context.Context, userID string) (GetOnboardingStateResult, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetOnboardingStateResult{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	stateJSON, found, err := uc.profiles.GetOnboardingState(ctx, companyID, userID)
	if err != nil {
		return GetOnboardingStateResult{}, apperrors.New(apperrors.KindInternal, "TENANT_GET_ONBOARDING_STATE_FAILED", "failed to load onboarding state", err)
	}
	return GetOnboardingStateResult{StateJSON: stateJSON, Found: found}, nil
}

type SetOnboardingState struct {
	profiles UserProfileRepository
}

func NewSetOnboardingState(profiles UserProfileRepository) *SetOnboardingState {
	return &SetOnboardingState{profiles: profiles}
}

type SetOnboardingStateInput struct {
	UserID    string
	StateJSON string
}

func (uc *SetOnboardingState) Execute(ctx context.Context, in SetOnboardingStateInput) error {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.profiles.SetOnboardingState(ctx, companyID, in.UserID, in.StateJSON); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_SET_ONBOARDING_STATE_FAILED", "failed to save onboarding state", err)
	}
	return nil
}
