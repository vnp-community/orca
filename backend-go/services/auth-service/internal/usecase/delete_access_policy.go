package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// DeleteAccessPolicy is an admin-console operation — removes every version
// row for a policy id.
type DeleteAccessPolicy struct {
	users    UserRepository
	policies AccessPolicyRepository
	opa      OPAClient
}

func NewDeleteAccessPolicy(users UserRepository, policies AccessPolicyRepository, opa OPAClient) *DeleteAccessPolicy {
	return &DeleteAccessPolicy{users: users, policies: policies, opa: opa}
}

func (uc *DeleteAccessPolicy) Execute(ctx context.Context, id string) error {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return err
	}
	if id == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_POLICY_ID", "id is required", nil)
	}

	if err := uc.policies.DeletePolicy(ctx, id); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_DELETE_ACCESS_POLICY_FAILED", "failed to delete access policy", err)
	}
	return nil
}
