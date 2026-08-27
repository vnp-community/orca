package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type CreateAccessPolicyInput struct {
	Name         string
	Kind         string
	DocumentJSON string
}

// CreateAccessPolicy is an admin-console operation — every new policy
// starts at version 1 (auth-service.md:150).
type CreateAccessPolicy struct {
	users    UserRepository
	policies AccessPolicyRepository
	clock    Clock
	opa      OPAClient
}

func NewCreateAccessPolicy(users UserRepository, policies AccessPolicyRepository, clock Clock, opa OPAClient) *CreateAccessPolicy {
	return &CreateAccessPolicy{users: users, policies: policies, clock: clock, opa: opa}
}

func (uc *CreateAccessPolicy) Execute(ctx context.Context, in CreateAccessPolicyInput) (domain.AccessPolicy, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.AccessPolicy{}, err
	}

	now := uc.clock.Now()
	policy, err := domain.NewAccessPolicy(uuid.NewString(), in.Name, in.Kind, in.DocumentJSON, 1, actor.ID, now)
	if err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_ACCESS_POLICY", err.Error(), err)
	}

	if err := uc.policies.InsertPolicyVersion(ctx, policy); err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInternal, "AUTH_CREATE_ACCESS_POLICY_FAILED", "failed to create access policy", err)
	}
	return policy, nil
}
