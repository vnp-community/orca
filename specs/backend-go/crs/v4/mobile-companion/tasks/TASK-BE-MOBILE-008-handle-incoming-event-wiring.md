# TASK-BE-MOBILE-008: `HandleIncomingEvent` — gọi `DeliverPush` khi event có `ChannelDeliveryPush`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** TASK-BE-MOBILE-007 (`DeliverMobilePush`, đổi tên — xem task đó)
**Status:** ✅ DONE (2026-09-09) — kèm 2 điều chỉnh so với sketch gốc

> **⚠️ Sketch gốc lỗi thời**: viết như thể `HandleIncomingEvent` chưa có field
> `notifications`/`deliverPush` — thực tế cả 2 đã tồn tại (TASK-BE-NOTIF-003,
> TASK-BE-NOTIF-011). Giữ nguyên 2 field đó, chỉ THÊM `deliverMobilePush
> *DeliverMobilePush` (tên đã đổi ở TASK-007) — không ghi đè.
>
> **Quyết định khác sketch**: sketch đặt `if slices.Contains(event.Channels,
> ChannelDeliveryPush)` bọc ngoài lời gọi ở `HandleIncomingEvent`. Thay vào
> đó, đặt check này BÊN TRONG `DeliverMobilePush.Execute` (đã thêm ở
> TASK-007) để nhất quán với `DeliverPush.Execute` (Web Push) đã tự làm y
> hệt vậy — `HandleIncomingEvent` gọi cả 2 usecase vô điều kiện, mỗi usecase
> tự gác cổng cho chính nó, không lặp lại logic gác cổng 2 nơi.
>
> `impact({target: "HandleIncomingEvent"})` trước khi sửa: LOW risk, đúng 1
> caller trực tiếp (`main.go`'s `run`) — khớp CR-MOBILE-001's khảo sát ban đầu.
>
> Thêm 3 test case yêu cầu (đổi tên cho khớp `DeliverMobilePush`):
> `TestHandleIncomingEvent_PushChannelEventCallsDeliverMobilePush`,
> `TestHandleIncomingEvent_WSOnlyEventDoesNotCallDeliverMobilePush`,
> `TestHandleIncomingEvent_DeliverMobilePushErrorDoesNotFailExecute` — cả 3
> PASS, cộng toàn bộ 9 test `HandleIncomingEvent` có sẵn từ trước (không cái
> nào vỡ) và 7 test `DeliverMobilePush` từ TASK-007.
>
> **Build/test thật**: `go vet ./internal/...` sạch (riêng `cmd/server` chưa
> build được — cần TASK-BE-MOBILE-009 wire `main.go`, đang làm tiếp ngay).
> `go test ./internal/usecase/... -run 'TestHandleIncomingEvent|TestDeliverMobilePush' -v`
> — **20/20 PASS**. `gofmt -l` sạch.

---

## Mục tiêu

`HandleIncomingEvent.Execute` hiện chỉ gọi `uc.broadcaster.Broadcast(ctx,
event)`, bỏ hoàn toàn `event.Channels` — thêm nhánh gọi `DeliverPush` khi
event có `ChannelDeliveryPush`, **độc lập lỗi** với nhánh WS (tiêu chí chấp
nhận CR-MOBILE-001: "Lỗi ở nhánh push không được chặn nhánh WS hiện có").

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` (MODIFY)
2. `backend-go/services/notification-service/internal/usecase/handle_incoming_event_test.go` (MODIFY — thêm test case + cập nhật constructor call ở mọi test hiện có)

## Thay đổi cụ thể

```go
type HandleIncomingEvent struct {
	broadcaster     NotificationBroadcaster
	processedEvents ProcessedEventRepository
	deliverPush     *DeliverPush // mới
	logger          *slog.Logger
}

func NewHandleIncomingEvent(broadcaster NotificationBroadcaster, processedEvents ProcessedEventRepository, deliverPush *DeliverPush, logger *slog.Logger) *HandleIncomingEvent {
	if logger == nil {
		logger = slog.Default()
	}
	return &HandleIncomingEvent{broadcaster: broadcaster, processedEvents: processedEvents, deliverPush: deliverPush, logger: logger}
}
```

```go
// Execute — thêm ngay sau uc.broadcaster.Broadcast(ctx, event), trước
// return nil ở cuối hàm.
uc.broadcaster.Broadcast(ctx, event)

if slices.Contains(event.Channels, domain.ChannelDeliveryPush) {
	// Why: a push-delivery failure must never turn into a NAK here — that
	// would redeliver the whole event, including the WS half that just
	// succeeded via Broadcast above. Log and move on; CR-MOBILE-001's
	// acceptance criteria requires the two branches fail independently.
	if err := uc.deliverPush.Execute(ctx, event); err != nil {
		uc.logger.ErrorContext(ctx, "deliver_push failed",
			slog.String("event_id", in.EventID), slog.Any("error", err))
	}
}

return nil
```

Import thêm `"slices"` (stdlib, Go 1.21+ — `go.mod` của
`notification-service` khai `go 1.25.0`, đủ điều kiện).

## Test cases cần cover

- `TestHandleIncomingEvent_PushChannelEventCallsDeliverPush` (subject có
  `ChannelDeliveryPush` trong `subjectRules`, vd.
  `orca.task.task.completed` — fake `DeliverPush`/`PushSender` xác nhận
  được gọi đúng 1 lần với đúng `event`)
- `TestHandleIncomingEvent_WSOnlyEventDoesNotCallDeliverPush` (subject
  `orca.tenant.star_nag.visibility_changed` — WS-only theo `subjectRules`
  hiện có — `DeliverPush` không được gọi)
- `TestHandleIncomingEvent_DeliverPushErrorDoesNotFailExecute` (fake
  `DeliverPush` trả lỗi — `Execute` vẫn trả `nil`, không NAK)
- Cập nhật MỌI test case hiện có gọi `NewHandleIncomingEvent(...)` — thêm
  tham số `deliverPush` thứ 3 (dùng 1 fake no-op nếu test không quan tâm
  nhánh push, tránh nil-pointer panic khi `Execute` gọi
  `uc.deliverPush.Execute`).

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go test ./internal/usecase/... -run TestHandleIncomingEvent -v
gofmt -l internal/usecase/handle_incoming_event.go
```

## gitnexus

`impact({target: "HandleIncomingEvent", direction: "upstream"})` **bắt
buộc trước khi sửa** — CR-MOBILE-001's khảo sát ghi nhận risk LOW/3 impacted
(1 direct, module `Usecase`) tại thời điểm viết CR; chạy lại ngay trước khi
sửa symbol này theo đúng constructor signature mới (thêm tham số) — xác
nhận không phá caller nào khác ngoài `cmd/server/main.go`
(TASK-BE-MOBILE-009) và test file đã liệt kê.

## Blocking

TASK-BE-MOBILE-009 (`cmd/server/main.go` wiring) phụ thuộc cứng — signature
mới của `NewHandleIncomingEvent` cần khớp ở composition root.
