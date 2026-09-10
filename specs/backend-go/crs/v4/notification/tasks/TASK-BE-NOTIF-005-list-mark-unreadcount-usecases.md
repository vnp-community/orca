# TASK-BE-NOTIF-005: Usecase `ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount` (CRUD, chưa WS)

**Solution:** BE-NOTIF-SOL-001 §2.D | **CR:** CR-NOTIF-001 (mục C)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-002
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** 4 usecase implement đúng theo sketch (`ListNotifications`
> gọi `repo.ListByRecipient`, `MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount` gọi
> đúng `repo.MarkAsRead`/`MarkAllAsRead`/`CountUnread` — tên method thật ở
> `NotificationRepository` là `CountUnread`, không phải "GetUnreadCount" như
> task doc gọi tắt, usecase struct vẫn tên `GetUnreadCount` cho khớp API
> public). `fakeNotificationRepository` dùng chung với `handle_incoming_event_test.go`
> (đã có sẵn từ TASK-BE-NOTIF-003) — không tạo fake mới trùng lặp. 15/15 test
> PASS thật (`go test ./internal/usecase/... -run 'TestListNotifications|TestMarkAsRead|TestMarkAllAsRead|TestGetUnreadCount' -v`),
> `go build ./...` sạch, `gofmt -l` sạch. `impact({target: "NotificationRepository", direction: "upstream"})`
> trả về "not found" — index gitnexus chưa bắt kịp symbol mới tạo trong phiên này (giới hạn
> đã ghi nhận nhiều lần ở các task khác). Xác nhận thủ công bằng `grep -rn "NotificationRepository" internal/`:
> đúng 5 caller — 4 usecase mới (`ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount`)
> + `HandleIncomingEvent` (TASK-BE-NOTIF-003), không có caller ngoài ý muốn.

## Mục tiêu

4 usecase mới, mỗi file 1 usecase (đúng pattern `subscribe.go`/`get_vapid_public_key.go` hiện có), gọi thẳng `NotificationRepository` — **không có logic WS ở task này** (WS read-receipt tách riêng, xem TASK-BE-NOTIF-007, để giữ CRUD path và real-time propagation path test độc lập nhau).

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/list_notifications.go` (MỚI)
2. `backend-go/services/notification-service/internal/usecase/mark_as_read.go` (MỚI)
3. `backend-go/services/notification-service/internal/usecase/mark_all_as_read.go` (MỚI)
4. `backend-go/services/notification-service/internal/usecase/get_unread_count.go` (MỚI)

## `list_notifications.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type ListNotificationsInput struct {
	UserID     string
	Cursor     string
	Limit      int32
	UnreadOnly bool
}

// ListNotifications is the Notification Center's read path — tenantID
// comes from context (never trusted from input), matching every other
// usecase in this package.
type ListNotifications struct {
	repo NotificationRepository
}

func NewListNotifications(repo NotificationRepository) *ListNotifications {
	return &ListNotifications{repo: repo}
}

func (uc *ListNotifications) Execute(ctx context.Context, in ListNotificationsInput) ([]domain.NotificationEvent, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.UserID == "" {
		return nil, "", apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // default/cap — mirror the repo's index-friendly page size
	}
	events, next, err := uc.repo.ListByRecipient(ctx, tenantID, in.UserID, in.Cursor, limit, in.UnreadOnly)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindInternal, "NOTIFICATION_LIST_FAILED", "failed to list notifications", err)
	}
	return events, next, nil
}
```

## `mark_as_read.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type MarkAsRead struct {
	repo NotificationRepository
}

func NewMarkAsRead(repo NotificationRepository) *MarkAsRead {
	return &MarkAsRead{repo: repo}
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
	return nil
}
```

## `mark_all_as_read.go` / `get_unread_count.go`

Cùng khuôn ngắn gọn — `MarkAllAsRead.Execute(ctx, userID string) (int64, error)` trả số row đã update; `GetUnreadCount.Execute(ctx, userID string) (int64, error)`. Không lặp lại code ở đây — mirror chính xác structure của `mark_as_read.go`/`list_notifications.go` ở trên (validate `tenant.RequireTenantID` → validate input → gọi repo → map lỗi qua `apperrors`).

## Test cases cần cover

- `TestListNotifications_RequiresTenantContext`
- `TestListNotifications_RequiresUserID`
- `TestListNotifications_DefaultsLimitWhenZeroOrOutOfRange`
- `TestListNotifications_UnreadOnlyPassedThrough` — fake repo assert `unreadOnly` truyền đúng
- `TestMarkAsRead_RequiresTenantAndFields`
- `TestMarkAsRead_CallsRepoWithCorrectTenantAndUser` — fake repo assert đúng 3 tham số (`tenantID`, `userID`, `notificationID`) không lẫn lộn thứ tự
- `TestMarkAllAsRead_ReturnsCountFromRepo`
- `TestGetUnreadCount_ReturnsCountFromRepo`
- Mỗi usecase: fake `NotificationRepository` trả lỗi → usecase map đúng `apperrors.KindInternal`

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/usecase/... -run 'TestListNotifications|TestMarkAsRead|TestMarkAllAsRead|TestGetUnreadCount'
gofmt -l internal/usecase/list_notifications.go internal/usecase/mark_as_read.go internal/usecase/mark_all_as_read.go internal/usecase/get_unread_count.go
```

## gitnexus

4 symbol này là MỚI — không có blast radius để đo trước khi tạo. Sau khi tạo, chạy `impact({target: "NotificationRepository", direction: "upstream"})` để xác nhận đúng 4 usecase mới (cộng `HandleIncomingEvent` từ TASK-BE-NOTIF-003) là toàn bộ caller — không có caller ngoài dự kiến nào khác trước khi merge.

## Blocking

TASK-BE-NOTIF-006 (gRPC handler), TASK-BE-NOTIF-007 (WS read-receipt, mở rộng `mark_as_read.go`/`mark_all_as_read.go`) đều phụ thuộc 4 usecase này đã tồn tại.
