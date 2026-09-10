package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// DeliverMobilePush is HandleIncomingEvent's native-mobile-push sibling:
// WS delivery goes through NotificationBroadcaster (in-process fan-out to
// currently-connected StreamNotifications subscribers); Web Push goes
// through DeliverPush (deliver_push.go, VAPID-signed, CR-NOTIF-001);
// DeliverMobilePush reaches a device APNs/FCM already suspended it, using
// each recipient's ios/android PushSubscription rows. See
// notification-service.md §6.
//
// NAMING NOTE (found while implementing TASK-BE-MOBILE-007): the CR
// gốc/task sketch this usecase was designed from named it "DeliverPush" —
// but CR-NOTIF-001's TASK-BE-NOTIF-011 (implemented earlier the same day,
// independently) already claims that exact name in this same package for
// Web Push/VAPID delivery. The two CRs' authors didn't cross-reference each
// other. Renamed to DeliverMobilePush to avoid a redeclaration — merging
// the two into 1 usecase handling all 3 channels (web/ios/android) was
// considered and rejected: it would require reworking DeliverPush's
// already-implemented, already-tested VAPID JWT construction path
// (TASK-BE-NOTIF-009/010/011) beyond either CR's actual scope, for a
// refactor neither CR asked for. Two clearly-named, independently
// dispatched usecases (see HandleIncomingEvent, TASK-BE-MOBILE-008) is the
// smaller, safer change.
//
// The senders map is keyed by domain.Channel so adding a third mobile push
// channel later means adding one map entry, not a new if/else branch.
type DeliverMobilePush struct {
	subscriptions SubscriptionRepository
	senders       map[domain.Channel]PushSender
	logger        *slog.Logger
}

func NewDeliverMobilePush(subscriptions SubscriptionRepository, senders map[domain.Channel]PushSender, logger *slog.Logger) *DeliverMobilePush {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliverMobilePush{subscriptions: subscriptions, senders: senders, logger: logger}
}

// Execute never returns an error for a per-subscription send failure —
// see HandleIncomingEvent's caller comment (TASK-BE-MOBILE-008): a push
// failure must never cause the WS-delivered half of the same event to be
// NAK'd and redelivered. It DOES return an error for a
// ListByUser/repository failure, since that's this usecase's own
// infrastructure breaking, not a per-recipient delivery problem.
func (uc *DeliverMobilePush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	// Mirrors DeliverPush's own guard (deliver_push.go, CR-NOTIF-001) —
	// event.Channels is the "should this be pushed at all" flag from
	// domain.TranslateEvent's subjectRules table, independent of which
	// PushSubscription.Channel (web/ios/android) a recipient happens to
	// have. Kept here (not in HandleIncomingEvent) so both push usecases
	// share the same "check my own gate" shape — HandleIncomingEvent just
	// calls both unconditionally, same as it already does for DeliverPush.
	if !slices.Contains(event.Channels, domain.ChannelDeliveryPush) {
		return nil
	}
	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			return fmt.Errorf("deliver_mobile_push: listing subscriptions for user %s: %w", userID, err)
		}
		for _, sub := range subs {
			sender, ok := uc.senders[sub.Channel]
			if !ok {
				continue // ChannelWeb (or any channel with no registered sender) has its own delivery path (DeliverPush)
			}
			if err := sender.Send(ctx, sub, event); err != nil {
				if errors.Is(err, ErrDeviceTokenInvalid) {
					if markErr := uc.subscriptions.MarkExpired(ctx, sub.Endpoint); markErr != nil {
						uc.logger.ErrorContext(ctx, "failed to mark expired push subscription",
							slog.String("endpoint", sub.Endpoint), slog.Any("error", markErr))
					}
					continue
				}
				uc.logger.ErrorContext(ctx, "push delivery failed",
					slog.String("channel", string(sub.Channel)), slog.String("event_id", event.ID), slog.Any("error", err))
			}
		}
	}
	return nil
}
