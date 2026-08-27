package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// UnregisterPushSubscription removes a Web Push subscription by its
// endpoint — the "unsubscribe" half of Subscribe. Deliberately
// unauthenticated at the api-gateway REST layer (see http-endpoints.md):
// tenant/user scoping comes from the endpoint row's own lookup, not from
// caller identity, so this usecase takes no tenant/user input at all.
type UnregisterPushSubscription struct {
	repo SubscriptionRepository
}

func NewUnregisterPushSubscription(repo SubscriptionRepository) *UnregisterPushSubscription {
	return &UnregisterPushSubscription{repo: repo}
}

func (uc *UnregisterPushSubscription) Execute(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_MISSING_ENDPOINT", "endpoint is required", nil)
	}
	// Idempotent by design — deleting an already-gone subscription is not
	// an error, matching Subscribe's own upsert-not-error-on-duplicate
	// posture.
	if err := uc.repo.DeleteByEndpoint(ctx, endpoint); err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_UNSUBSCRIBE_FAILED", "failed to remove push subscription", err)
	}
	return nil
}
