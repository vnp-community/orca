package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// SubscribeInput mirrors the gRPC SubscribeRequest 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the mapping stays traceable. Endpoint+p256dh_key+auth_key are
// Web Push-specific fields (only meaningful/required when Channel is web
// or empty); Channel/DeviceLabel (TASK-BE-MOBILE-001, CR-MOBILE-001) let a
// native (ios/android) client register a device push token through the
// same RPC instead of a Web Push subscription.
type SubscribeInput struct {
	UserID      string
	Endpoint    string
	P256dhKey   string
	AuthKey     string
	Channel     string // empty == "web", preserves every existing caller's behavior unchanged
	DeviceLabel string
}

// Subscribe is notification-service's push-subscription write path.
// TenantID is NOT part of the input — it's pulled from context (see
// common/tenant), never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule. UserID
// comes from the request itself (the proto has no tenant field; tenant
// scoping travels via gRPC metadata per architecture/08).
type Subscribe struct {
	repo SubscriptionRepository
}

func NewSubscribe(repo SubscriptionRepository) *Subscribe {
	return &Subscribe{repo: repo}
}

func (uc *Subscribe) Execute(ctx context.Context, in SubscribeInput) (domain.PushSubscription, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.UserID == "" {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}

	channel := domain.ChannelWeb
	if in.Channel != "" {
		channel = domain.Channel(in.Channel)
	}

	sub, err := domain.NewPushSubscription(
		uuid.NewString(), tenantID, in.UserID, channel, in.Endpoint,
		&in.P256dhKey, &in.AuthKey, in.DeviceLabel, time.Now().UTC(),
	)
	if err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_INVALID_SUBSCRIPTION", err.Error(), err)
	}

	if err := uc.repo.Save(ctx, sub); err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInternal, "NOTIFICATION_SAVE_FAILED", "failed to persist push subscription", err)
	}

	return sub, nil
}
