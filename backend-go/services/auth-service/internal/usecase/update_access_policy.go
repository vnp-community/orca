package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// UpdateAccessPolicy inserts a NEW version row — never mutates a row in
// place, per auth-service.md:150. expectedVersion is an optimistic-
// concurrency guard: a caller must have read the current version before
// calling this, and a mismatch means someone else updated the policy
// concurrently (KindFailedPrecondition — refetch and retry, not a silent
// overwrite).
type UpdateAccessPolicy struct {
	users     UserRepository
	policies  AccessPolicyRepository
	publisher PolicyDataPublisher
	clock     Clock
	opa       OPAClient
}

func NewUpdateAccessPolicy(users UserRepository, policies AccessPolicyRepository, publisher PolicyDataPublisher, clock Clock, opa OPAClient) *UpdateAccessPolicy {
	return &UpdateAccessPolicy{users: users, policies: policies, publisher: publisher, clock: clock, opa: opa}
}

func (uc *UpdateAccessPolicy) Execute(ctx context.Context, id, documentJSON string, expectedVersion int32) (domain.AccessPolicy, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.AccessPolicy{}, err
	}

	current, err := uc.policies.GetLatestPolicy(ctx, id)
	if errors.Is(err, ErrPolicyNotFound) {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindNotFound, "AUTH_POLICY_NOT_FOUND", "access policy not found", err)
	}
	if err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInternal, "AUTH_GET_ACCESS_POLICY_FAILED", "failed to look up access policy", err)
	}
	if current.Version != expectedVersion {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindFailedPrecondition, "AUTH_POLICY_VERSION_CONFLICT", "policy was updated concurrently, refetch and retry", nil)
	}

	now := uc.clock.Now()
	next, err := domain.NewAccessPolicy(current.ID, current.Name, current.Kind, documentJSON, current.Version+1, actor.ID, now)
	if err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_ACCESS_POLICY", err.Error(), err)
	}

	if err := uc.policies.InsertPolicyVersion(ctx, next); err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInternal, "AUTH_UPDATE_ACCESS_POLICY_FAILED", "failed to update access policy", err)
	}

	// Publish to the OPA bundle registry — see usecase.PolicyDataPublisher's
	// doc comment for why this is a stub (policypublisher.NoopPublisher) in
	// this scaffold rather than a real bundle-registry push. A publish
	// failure surfaces as an error to the caller (matching the RPC's
	// synchronous contract) but does NOT roll back the just-inserted
	// version row — the version history stays the source of truth; a
	// retry re-reads the now-current version and republishes.
	if err := uc.publisher.PublishPolicyChange(ctx, next); err != nil {
		return domain.AccessPolicy{}, apperrors.New(apperrors.KindInternal, "AUTH_POLICY_PUBLISH_FAILED", "access policy was updated but failed to publish to the policy registry", err)
	}

	return next, nil
}
