# TASK-BE-NOTIF-007: WS read-receipt — real-time đa tab/đa thiết bị

**Solution:** BE-NOTIF-SOL-001 §1 quyết định 2, §2.F | **CR:** CR-NOTIF-001 (mục D, tiêu chí chấp nhận "MarkAsRead trên tab A phản ánh qua WS tới tab B")
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-005
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch — `MarkAsRead`/`MarkAllAsRead` thêm
> tham số `broadcaster NotificationBroadcaster`, broadcast SAU khi persist thành
> công, không broadcast nếu persist lỗi. `cmd/server/main.go` truyền `broadcast`
> (đã có sẵn biến, khai báo dòng 81) vào cả 2 constructor. `impact()` cho
> `MarkAsRead`/`MarkAllAsRead` trả "not found" (index chưa bắt kịp symbol tạo ở
> TASK-005) — xác nhận thủ công bằng grep: đúng 3 nhóm caller (`cmd/server/main.go`,
> `adapter/grpc/server.go`/`server_test.go`, test file riêng của usecase), không
> có gì bất ngờ.
>
> Tái dùng `fakeBroadcaster` đã có sẵn ở `handle_incoming_event_test.go` (cùng
> package `usecase`) cho test — không tạo fake trùng lặp. Test "2 tab cùng nhận
> read-receipt" dùng `broadcaster.Broadcaster` THẬT (không fake), xác nhận channel
> buffer (`channelBufferSize`) đủ để nhận non-blocking mà không cần goroutine đọc
> song song. Thêm `noopBroadcaster` nhỏ trong `adapter/grpc/server_test.go` (khác
> package) để không phải import package `usecase`'s test-only fake xuyên package.
>
> **10/11 test cần cover đã viết đủ (11 test PASS thật)**: `go test
> ./internal/usecase/... -run 'TestMarkAsRead|TestMarkAllAsRead' -v` — **11/11
> PASS**, gồm cả `TestMarkAsRead_TwoSubscribersSameUser_BothReceiveReadReceipt`
> (test trực tiếp cho tiêu chí chấp nhận CR-NOTIF-001). `go build ./...`, `go vet
> ./...`, `gofmt -l` sạch. `go test ./...` toàn service PASS 100%.

## Mục tiêu

Khi `MarkAsRead`/`MarkAllAsRead` chạy thành công, phát 1 "read receipt" qua `NotificationBroadcaster` đã có — mọi tab/thiết bị khác của cùng user thấy unread count giảm ngay, không cần poll `GetUnreadCount` lại. Đây là yêu cầu bắt buộc cho use case SSH/remote (nhiều phiên trình duyệt cùng 1 user, theo AGENTS.md's "SSH Use Case").

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/mark_as_read.go` (MODIFY — thêm tham số `broadcaster`)
2. `backend-go/services/notification-service/internal/usecase/mark_all_as_read.go` (MODIFY — thêm tham số `broadcaster`)
3. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY — truyền `broadcast` vào 2 constructor trên)

## `mark_as_read.go` — thêm broadcast sau khi persist thành công

```go
type MarkAsRead struct {
	repo        NotificationRepository
	broadcaster NotificationBroadcaster
}

func NewMarkAsRead(repo NotificationRepository, broadcaster NotificationBroadcaster) *MarkAsRead {
	return &MarkAsRead{repo: repo, broadcaster: broadcaster}
}

func (uc *MarkAsRead) Execute(ctx context.Context, userID, notificationID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if userID == "" || notificationID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_MISSING_FIELD", "user_id and notification_id are required", nil)
	}
	if err := uc.repo.MarkAsRead(ctx, tenantID, userID, notificationID); err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_MARK_READ_FAILED", "failed to mark notification as read", err)
	}
	// Read receipt: reuses the existing NotificationEvent/Broadcaster
	// pipeline (no new channel/struct) so every WS-connected session of
	// this user sees is_read flip in real time — required for the
	// multi-tab/multi-device SSH remote-workflow case (AGENTS.md).
	uc.broadcaster.Broadcast(ctx, domain.NotificationEvent{
		ID: notificationID, TenantID: tenantID, RecipientUserIDs: []string{userID},
		Type: "notification_read", IsRead: true, CreatedAt: time.Now().UTC(),
	})
	return nil
}
```

`mark_all_as_read.go` — cùng khuôn, nhưng vì không có 1 `notificationID` cụ thể, broadcast 1 sự kiện riêng `Type: "notification_all_read"` (không có `ID`, client hiểu là "reset unread count về 0" thay vì "trừ 1 id cụ thể"):

```go
uc.broadcaster.Broadcast(ctx, domain.NotificationEvent{
	TenantID: tenantID, RecipientUserIDs: []string{userID},
	Type: "notification_all_read", IsRead: true, CreatedAt: time.Now().UTC(),
})
```

**Broadcast chỉ chạy SAU khi `repo.MarkAsRead`/`MarkAllAsRead` trả về không lỗi** — không broadcast "đã đọc" nếu persist thất bại (sẽ làm client hiển thị sai trạng thái không khớp DB).

## `cmd/server/main.go` — wiring

```go
markAsReadUC := usecase.NewMarkAsRead(repo, broadcast)
markAllAsReadUC := usecase.NewMarkAllAsRead(repo, broadcast)
```

## Test cases cần cover

- `TestMarkAsRead_BroadcastsReadReceiptAfterPersist` — fake `NotificationBroadcaster` ghi lại event nhận được; assert `Type == "notification_read"`, `IsRead == true`, `RecipientUserIDs == [userID]`, `ID == notificationID`.
- `TestMarkAsRead_PersistFails_DoesNotBroadcast` — fake repo trả lỗi → fake broadcaster's `Broadcast` KHÔNG được gọi (assert count == 0).
- `TestMarkAsRead_TwoSubscribersSameUser_BothReceiveReadReceipt` — dùng `Broadcaster` THẬT (không fake, mirror `broadcaster_test.go`'s pattern có sẵn): subscribe 2 channel cho cùng `tenantID+userID`, gọi `MarkAsRead`, assert CẢ 2 channel đều nhận được event `notification_read` — đây là test trực tiếp cho tiêu chí chấp nhận "MarkAsRead trên tab A phản ánh tới tab B" của CR-NOTIF-001.
- `TestMarkAllAsRead_BroadcastsAllReadReceipt`.

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/usecase/... -run 'TestMarkAsRead|TestMarkAllAsRead'
gofmt -l internal/usecase/mark_as_read.go internal/usecase/mark_all_as_read.go cmd/server/main.go
```

## gitnexus

`impact({target: "MarkAsRead", direction: "upstream"})`/`impact({target: "MarkAllAsRead", direction: "upstream"})` trước khi sửa chữ ký constructor — 2 symbol mới tạo ở TASK-BE-NOTIF-005, impact ở thời điểm task này chỉ nên gồm `cmd/server/main.go` + `server.go` (TASK-BE-NOTIF-006) + test file; nếu `impact()` trả về caller nào khác ngoài dự kiến, dừng lại và xác nhận trước khi đổi chữ ký.

## Blocking

Không task nào phụ thuộc trực tiếp task này, nhưng TASK-BE-NOTIF-008 (REST) nên hoàn thành SAU task này để `POST /v1/notifications/{id}/read` gọi đúng usecase đã có broadcast (tránh phải sửa lại route sau khi thêm tính năng).
