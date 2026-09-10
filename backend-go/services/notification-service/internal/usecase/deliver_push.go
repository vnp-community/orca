package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"slices"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type pushPayload struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link,omitempty"`
}

// DeliverPush sends event as a VAPID-signed Web Push message to every
// active `web`-channel subscription of event.RecipientUserIDs, when
// event.Channels contains ChannelDeliveryPush. Mobile (ios/android)
// channels are out of scope — see specs/backend-go/crs/v4/notification/
// solutions/BE-NOTIF-SOL-002.md's "Không thuộc phạm vi". One
// subscription's failure (expired endpoint, signing error, transient
// network error) must not abort delivery to other subscriptions/recipients
// — every failure is logged and delivery continues.
type DeliverPush struct {
	subscriptions   SubscriptionRepository
	vapidKeys       VapidKeyRepository
	signer          VaultSigner
	sender          WebPushSender
	vapidContactURI string
	logger          *slog.Logger
}

func NewDeliverPush(subscriptions SubscriptionRepository, vapidKeys VapidKeyRepository, signer VaultSigner, sender WebPushSender, vapidContactURI string, logger *slog.Logger) *DeliverPush {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliverPush{
		subscriptions:   subscriptions,
		vapidKeys:       vapidKeys,
		signer:          signer,
		sender:          sender,
		vapidContactURI: vapidContactURI,
		logger:          logger,
	}
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	if !slices.Contains(event.Channels, domain.ChannelDeliveryPush) {
		return nil
	}

	key, err := uc.vapidKeys.GetPublicKey(ctx, event.TenantID)
	if err != nil {
		// No active VAPID key for this tenant — nothing can be signed;
		// this is not a caller error (WS already delivered via
		// Broadcast), log and no-op rather than failing the whole event.
		uc.logger.WarnContext(ctx, "no active vapid key, skipping push delivery", slog.String("tenant_id", event.TenantID), slog.Any("error", err))
		return nil
	}

	payload, err := json.Marshal(pushPayload{ID: event.ID, Type: event.Type, Title: event.Title, Body: event.Body, DeepLink: event.DeepLink})
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_PUSH_PAYLOAD_MARSHAL_FAILED", "failed to marshal push payload", err)
	}

	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			uc.logger.ErrorContext(ctx, "failed to list push subscriptions", slog.String("user_id", userID), slog.Any("error", err))
			continue // 1 recipient's lookup failure must not abort the others
		}
		for _, sub := range subs {
			if sub.Channel != domain.ChannelWeb {
				continue // mobile (ios/android) channels are out of scope for this usecase
			}
			uc.deliverOne(ctx, event.TenantID, key.PublicKey, sub, payload)
		}
	}
	return nil
}

func (uc *DeliverPush) deliverOne(ctx context.Context, tenantID, publicKey string, sub domain.PushSubscription, payload []byte) {
	origin, err := originOf(sub.Endpoint)
	if err != nil {
		uc.logger.ErrorContext(ctx, "invalid subscription endpoint, cannot build vapid aud claim", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
		return
	}
	authHeader, err := BuildVapidAuthHeader(ctx, uc.signer, tenantID, origin, uc.vapidContactURI, publicKey)
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to build vapid auth header", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
		return
	}
	expired, err := uc.sender.Send(ctx, sub, authHeader, payload)
	if expired {
		if markErr := uc.subscriptions.MarkExpired(ctx, sub.Endpoint); markErr != nil {
			uc.logger.ErrorContext(ctx, "failed to mark subscription expired", slog.String("endpoint", sub.Endpoint), slog.Any("error", markErr))
		}
		return
	}
	if err != nil {
		uc.logger.ErrorContext(ctx, "web push delivery failed", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
	}
}

// originOf returns endpoint's scheme+host — the RFC 8292 `aud` claim a
// VAPID JWT must carry (the push service's origin, not the full endpoint
// path, which is unique per subscription and would make every JWT
// audience-scoped to exactly 1 endpoint instead of the whole push service).
func originOf(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", &url.Error{Op: "originOf", URL: endpoint, Err: errNoOrigin}
	}
	return u.Scheme + "://" + u.Host, nil
}

var errNoOrigin = apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_ENDPOINT_NO_ORIGIN", "endpoint has no scheme/host to derive a vapid aud claim from", nil)
