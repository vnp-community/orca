package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type ListAccessPoliciesInput struct {
	PageToken string
	PageSize  int32
}

type ListAccessPoliciesOutput struct {
	Policies      []domain.AccessPolicy
	NextPageToken string
}

// ListAccessPolicies is an admin-console, read-only operation — returns
// one row per policy id, its LATEST version only.
type ListAccessPolicies struct {
	users    UserRepository
	policies AccessPolicyRepository
	opa      OPAClient
}

func NewListAccessPolicies(users UserRepository, policies AccessPolicyRepository, opa OPAClient) *ListAccessPolicies {
	return &ListAccessPolicies{users: users, policies: policies, opa: opa}
}

func (uc *ListAccessPolicies) Execute(ctx context.Context, in ListAccessPoliciesInput) (ListAccessPoliciesOutput, error) {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return ListAccessPoliciesOutput{}, err
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	policies, next, err := uc.policies.ListLatestPolicies(ctx, in.PageToken, pageSize)
	if err != nil {
		return ListAccessPoliciesOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_ACCESS_POLICIES_FAILED", "failed to list access policies", err)
	}
	return ListAccessPoliciesOutput{Policies: policies, NextPageToken: next}, nil
}
