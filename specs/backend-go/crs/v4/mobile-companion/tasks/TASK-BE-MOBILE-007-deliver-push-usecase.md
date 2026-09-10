# TASK-BE-MOBILE-007: `DeliverPush` usecase

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** TASK-BE-MOBILE-003 (`MarkExpired`), TASK-BE-MOBILE-005 (`APNsSender`), TASK-BE-MOBILE-006 (`FCMSender`)
**Status:** ✅ DONE (2026-09-09) — kèm 1 quyết định đổi tên bắt buộc

> **⚠️ Xung đột tên phát hiện khi implement: `DeliverPush`/`NewDeliverPush` ĐÃ
> BỊ CHIẾM** bởi [TASK-BE-NOTIF-011](../../notification/tasks/TASK-BE-NOTIF-011-deliver-push-usecase-wiring.md)
> (CR-NOTIF-001, Web Push/VAPID) — cùng package `usecase`, cùng tên struct +
> constructor, viết cùng ngày nhưng độc lập, không tham chiếu chéo 2 CR. Giữ
> nguyên sketch sẽ là lỗi biên dịch (khai báo trùng). **Đổi tên thành
> `DeliverMobilePush`/`NewDeliverMobilePush`**, file `deliver_mobile_push.go`
> (khác `deliver_push.go` đã có). Đã cân nhắc gộp 2 usecase thành 1 xử lý cả
> 3 channel (web/ios/android) — từ chối vì sẽ phải viết lại toàn bộ luồng
> VAPID JWT đã implement+test xong của `DeliverPush`, vượt phạm vi cả 2 CR;
> 2 usecase tên rõ ràng, dispatch riêng từ `HandleIncomingEvent`
> (TASK-BE-MOBILE-008) là thay đổi nhỏ hơn, an toàn hơn.
>
> Phần còn lại đúng sketch. `fakeSubscriptionRepository` (dùng chung,
> `subscribe_test.go`) thiếu hỗ trợ lỗi cho `ListByUser` — thêm field
> `listErr` (không có trong sketch, cần cho `TestDeliverMobilePush_ListByUserErrorReturnsError`).
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/usecase/... -run TestDeliverMobilePush -v` — **6/6 PASS**.
> `go test ./...` PASS 100% toàn `notification-service`.

---

## Mục tiêu

Usecase mới, đúng tên/vị trí design doc chỉ định
(`notification-service.md:218`) — với mỗi recipient của 1
`NotificationEvent`, lấy các subscription `ios`/`android` đang `active`,
gọi `PushSender` tương ứng, đánh dấu hết hạn khi token không còn hợp lệ.
KHÔNG tự gọi từ RPC nào — chỉ được gọi từ `HandleIncomingEvent`
(TASK-BE-MOBILE-008).

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/deliver_push.go` (MỚI)
2. `backend-go/services/notification-service/internal/usecase/deliver_push_test.go` (MỚI)

## `deliver_push.go`

```go
// Package usecase — DeliverPush is HandleIncomingEvent's mobile-push
// sibling: WS delivery goes through NotificationBroadcaster (in-process
// fan-out to currently-connected StreamNotifications subscribers);
// DeliverPush reaches a device APNs/FCM already suspended it, using each
// recipient's ios/android PushSubscription rows. See
// notification-service.md §6.
package usecase

// DeliverPush's senders map is keyed by domain.Channel so adding a third
// push channel later (e.g. a hypothetical "web-native-push") means adding
// one map entry, not a new if/else branch.
type DeliverPush struct {
	subscriptions SubscriptionRepository
	senders       map[domain.Channel]PushSender
	logger        *slog.Logger
}

func NewDeliverPush(subscriptions SubscriptionRepository, senders map[domain.Channel]PushSender, logger *slog.Logger) *DeliverPush {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliverPush{subscriptions: subscriptions, senders: senders, logger: logger}
}

// Execute never returns an error for a per-subscription send failure —
// see HandleIncomingEvent's caller comment (TASK-BE-MOBILE-008): a push
// failure must never cause the WS-delivered half of the same event to be
// NAK'd and redelivered. It DOES return an error for a
// ListByUser/repository failure, since that's this usecase's own
// infrastructure breaking, not a per-recipient delivery problem.
func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			return fmt.Errorf("deliver_push: listing subscriptions for user %s: %w", userID, err)
		}
		for _, sub := range subs {
			sender, ok := uc.senders[sub.Channel]
			if !ok {
				continue // ChannelWeb (or any channel with no registered sender) has its own delivery path
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
```

## Test cases cần cover (fakes cho `SubscriptionRepository`/`PushSender`, mirror `handle_incoming_event_test.go`'s style)

- `TestDeliverPush_SendsToEveryActiveIOSAndAndroidSubscription`
- `TestDeliverPush_SkipsWebChannelSubscriptions` (không có sender đăng ký
  cho `ChannelWeb` trong map — không panic, không gọi nhầm sender khác)
- `TestDeliverPush_DeviceTokenInvalidMarksExpiredAndContinues` (1 trong 2
  subscription lỗi `ErrDeviceTokenInvalid` — subscription còn lại vẫn nhận
  được push, `MarkExpired` được gọi đúng 1 lần với đúng endpoint)
- `TestDeliverPush_GenericSendErrorDoesNotMarkExpired` (lỗi không phải
  `ErrDeviceTokenInvalid` — không gọi `MarkExpired`, không chặn recipient
  khác)
- `TestDeliverPush_ListByUserErrorReturnsError` (khác hẳn lỗi per-send —
  đây LÀ lỗi cần return, không nuốt)
- `TestDeliverPush_MultipleRecipientsEachGetOwnSubscriptions`

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go test ./internal/usecase/... -run TestDeliverPush -v
gofmt -l internal/usecase/deliver_push.go
```

## gitnexus

`impact({target: "DeliverPush", direction: "upstream"})` — symbol mới, sẽ
trả rỗng ở lúc này; chạy lại NGAY SAU khi TASK-BE-MOBILE-008 nối nó vào
`HandleIncomingEvent` để xác nhận đúng 1 caller (không có caller RPC nào
khác gọi nhầm usecase này trực tiếp — đúng ý định "không phải 1 RPC path").

## Blocking

TASK-BE-MOBILE-008 (`HandleIncomingEvent` wiring) phụ thuộc cứng — cần
`DeliverPush`/`NewDeliverPush` tồn tại trước khi gọi được từ
`handle_incoming_event.go`.
