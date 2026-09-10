# TASK-BE-NOTIF-003: `HandleIncomingEvent` lưu trước khi broadcast

**Solution:** BE-NOTIF-SOL-001 §2.C | **CR:** CR-NOTIF-001 (mục B)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-002
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `impact({target:"HandleIncomingEvent", direction:
> "upstream"})` chạy lại ngay trước khi sửa — vẫn LOW risk, 3 impacted (1
> direct, module `Usecase`), không đổi so với khảo sát cũ ở
> TASK-BE-NOTIF-002. Xác nhận `NewHandleIncomingEvent` chỉ có 1 production
> call site (`cmd/server/main.go`) trước khi sửa chữ ký.
>
> Thêm field `notifications NotificationRepository`, tham số thứ 3 của
> `NewHandleIncomingEvent`, và bước persist giữa `TranslateEvent`/
> `Broadcast` trong `Execute` — đúng thứ tự task doc yêu cầu. **Deviation
> so với task doc**: gọi `uc.notifications.SaveNotificationEvent(ctx,
> event)` (không phải `.Save(...)`) — khớp tên method thật đã đổi ở
> TASK-BE-NOTIF-002 (xem ghi chú deviation ở đó). `cmd/server/main.go`
> sửa đúng 1 dòng: `usecase.NewHandleIncomingEvent(broadcast, repo, repo,
> logger)` — `repo` truyền 2 lần (implement cả `ProcessedEventRepository`
> và `NotificationRepository`), không cần biến mới.
>
> `handle_incoming_event_test.go`: thêm `fakeNotificationRepository` (đủ 5
> method của `NotificationRepository`, `saveErr`/`saved`/`order` để test
> assert lỗi và thứ tự gọi) + `orderedFakeBroadcaster` (wrap
> `fakeBroadcaster`, ghi thêm vào `order` dùng chung) để
> `TestHandleIncomingEvent_SavesBeforeBroadcast` assert đúng thứ tự
> `["save","broadcast"]`. Sửa cả 5 call site `NewHandleIncomingEvent(...)`
> cũ (thêm `&fakeNotificationRepository{}`) — không đổi hành vi test cũ.
> Thêm đủ 3 test mới task doc yêu cầu:
> `TestHandleIncomingEvent_SavesBeforeBroadcast`,
> `TestHandleIncomingEvent_SaveFails_ReturnsErrorNoBroadcast`,
> `TestHandleIncomingEvent_NoRecipients_SkipsSaveAndBroadcast`.
>
> **Build/test thật đã chạy**: `go build ./...` sạch, `go vet ./...` sạch,
> `gofmt -l` sạch trên 3 file sửa. `go test ./internal/usecase/... -run
> TestHandleIncomingEvent -v` — **8/8 PASS thật** (5 test cũ + 3 test mới).
> `go test ./...` toàn service — PASS hết (không có test nào regress).

## Mục tiêu

Thêm bước `notificationRepo.Save(ctx, event)` vào `HandleIncomingEvent.Execute`, giữa `TranslateEvent` và `Broadcast` — để một replica crash giữa 2 bước vẫn để lại record persist.

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` (MODIFY)
2. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY — truyền `repo` làm `NotificationRepository` vào `NewHandleIncomingEvent`)

## Nội dung sửa `handle_incoming_event.go`

`HandleIncomingEvent` struct hiện có field `processedEvents ProcessedEventRepository` — thêm 1 field mới `notifications NotificationRepository`:

```go
type HandleIncomingEvent struct {
	broadcaster     NotificationBroadcaster
	processedEvents ProcessedEventRepository
	notifications   NotificationRepository
	logger          *slog.Logger
}

func NewHandleIncomingEvent(broadcaster NotificationBroadcaster, processedEvents ProcessedEventRepository, notifications NotificationRepository, logger *slog.Logger) *HandleIncomingEvent {
	if logger == nil {
		logger = slog.Default()
	}
	return &HandleIncomingEvent{broadcaster: broadcaster, processedEvents: processedEvents, notifications: notifications, logger: logger}
}
```

`Execute` — chèn `Save` ngay sau `TranslateEvent` thành công, TRƯỚC `Broadcast`:

```go
	event, err := domain.TranslateEvent(uuid.NewString(), in.EventID, in.Subject, in.TenantID, payload, in.OccurredAt)
	if err != nil {
		if errors.Is(err, domain.ErrNoRecipients) {
			uc.logger.InfoContext(ctx, "skipping event with no recipient",
				slog.String("subject", in.Subject), slog.String("event_id", in.EventID))
			return nil
		}
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_TRANSLATE_FAILED", "failed to translate event", err)
	}

	if err := uc.notifications.Save(ctx, event); err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_PERSIST_FAILED", "failed to persist notification event", err)
	}

	uc.broadcaster.Broadcast(ctx, event)
	return nil
```

**Lỗi `Save` PHẢI làm `Execute` trả lỗi** (JetStream NAK → redelivery) — không nuốt lỗi rồi vẫn `Broadcast`, đúng nguyên tắc CR: "lưu trước broadcast để... mất live-push, không mất lịch sử", nghĩa là chiều ngược lại (broadcast thành công nhưng persist thất bại) không được coi là thành công.

## `cmd/server/main.go` — wiring

```go
handleIncomingEventUC := usecase.NewHandleIncomingEvent(broadcast, repo, repo, logger)
```

(`repo` — biến `*notificationpostgres.Repository` đã tồn tại — implement CẢ `ProcessedEventRepository` VÀ `NotificationRepository` sau TASK-BE-NOTIF-002, nên truyền cùng 1 instance 2 lần là đúng, không cần biến mới.)

## Test cases cần cover

- `TestHandleIncomingEvent_SavesBeforeBroadcast` — dùng 2 fake (`fakeNotificationRepository`, `fakeBroadcaster`) ghi lại thứ tự gọi (ví dụ append vào 1 slice `[]string{"save","broadcast"}`) — assert thứ tự đúng `save` trước `broadcast`.
- `TestHandleIncomingEvent_SaveFails_ReturnsErrorNoBroadcast` — fake `NotificationRepository.Save` trả lỗi → `Execute` trả lỗi, VÀ `fakeBroadcaster.Broadcast` KHÔNG được gọi (assert count == 0).
- `TestHandleIncomingEvent_NoRecipients_SkipsSaveAndBroadcast` — case `ErrNoRecipients` đã có từ trước, xác nhận vẫn không gọi `Save`/`Broadcast` sau khi thêm field mới (regression guard).
- Cập nhật mọi test hiện có của `handle_incoming_event_test.go` gọi `NewHandleIncomingEvent(...)` — thêm tham số `notifications` (dùng 1 fake no-op mặc định nếu test đó không quan tâm tới persist).

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/usecase/... -run TestHandleIncomingEvent
gofmt -l internal/usecase/handle_incoming_event.go cmd/server/main.go
```

## gitnexus

`impact({target: "HandleIncomingEvent", direction: "upstream"})` trước khi sửa — theo `docs/crs/v4/notification/README.md`'s khảo sát trước, LOW risk, 3 impacted (1 direct, module `Usecase`). Chạy lại ngay trước khi sửa để lấy số liệu mới nhất (khảo sát cũ không tính symbol mới thêm ở TASK-BE-NOTIF-002). **Lưu ý riêng cho task này**: `NewHandleIncomingEvent`'s chữ ký đổi (thêm tham số) — xác nhận KHÔNG còn call site nào khác ngoài `cmd/server/main.go` trước khi merge (test file tự gọi lại constructor, không tính là "production caller" nhưng vẫn phải sửa).

## Blocking

TASK-BE-NOTIF-011 (Track 2, `DeliverPush` wiring) cũng sửa `Execute` — PHẢI merge task này trước, theo [tasks/README.md](./README.md)'s thứ tự khuyến nghị, để tránh 2 PR cùng chạm 1 hàm.
