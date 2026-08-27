package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// GetAccessPolicy is an admin-console, read-only operation — always
// returns the LATEST version of a policy, never a specific historical one
// (no such RPC exists yet — see auth-service.md's admin-console surface).
type GetAccessPolicy struct {
	users    UserRepository
	policies AccessPolicyRepository
	opa      OPAClient
}

func NewGetAccessPolicy(users UserRepository, policies AccessPolicyRepository, opa OPAClient) *GetAccessPolicy {
	return &GetAccessPolicy{users: users, policies: policies, opa: opa}
}

func (uc *GetAccessPolicy) Execute(ctx context.Context, id string) (domain.AccessPolicy, error) {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return domain.AccessPolicy{}, err
	}
	if id == "" {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_POLICY_ID", "id is required", nil)
	}

	policy, err := uc.policies.GetLatestPolicy(ctx, id)
	if errors.Is(err, ErrPolicyNotFound) {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindNotFound, "AUTH_POLICY_NOT_FOUND", "access policy not found", err)
	}
	if err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInternal, "AUTH_GET_ACCESS_POLICY_FAILED", "failed to get access policy", err)
	}
	return policy, nil
}
