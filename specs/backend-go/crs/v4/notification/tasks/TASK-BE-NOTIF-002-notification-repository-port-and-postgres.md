# TASK-BE-NOTIF-002: `domain.NotificationEvent` +field, `NotificationRepository` port, Postgres implementation

**Solution:** BE-NOTIF-SOL-001 §1 quyết định 1, §2.B | **CR:** CR-NOTIF-001 (mục A/B, Changes Required)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-001
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `impact()` cho `Repository`(`adapter/postgres`) và
> `NotificationEvent`(`domain`) đều LOW risk trước khi sửa. Thêm
> `IsRead bool`/`ReadAt *time.Time` vào `domain.NotificationEvent` +
> `domain.ErrInvalidCursor` sentinel (không có trong task doc gốc — thêm vì
> `ListByRecipient`'s cursor decode cần trả lỗi rõ ràng, không panic, đúng
> yêu cầu task). `TranslateEvent` KHÔNG đổi, mọi test `TestTranslateEvent_*`
> cũ vẫn pass nguyên.
>
> **Deviation quan trọng so với task doc**: interface method đặt tên
> `SaveNotificationEvent` (không phải `Save` như task doc đề xuất) — code
> thật `*postgres.Repository` đã có sẵn `Save(ctx, domain.PushSubscription)`
> cho `SubscriptionRepository`; Go không hỗ trợ overload theo signature nên
> 2 method cùng tên `Save` trên cùng receiver không build được. Đổi tên ở
> CẢ `usecase/ports.go` (`NotificationRepository.SaveNotificationEvent`) VÀ
> `adapter/postgres/repository.go` — nhất quán 2 phía. TASK-BE-NOTIF-003 (gọi
> `uc.notifications.Save(...)`) sẽ cần đổi thành
> `uc.notifications.SaveNotificationEvent(...)` khi thực thi.
>
> `ListByRecipient` implement đầy đủ (task doc để trống, chỉ ghi chú hình
> dạng) — keyset pagination `(created_at DESC, id DESC)`, cursor
> `base64url("<rfc3339nano>|<id>")`, fetch `limit+1` để biết còn trang kế
> tiếp không.
>
> **Lỗi thiết kế thật phát hiện khi viết test** (đã sửa ở TASK-BE-NOTIF-001,
> xem ghi chú cập nhật ở đó): PK gốc `id` đơn gây duplicate-key thật khi 1
> event có nhiều recipient — sửa thành `PRIMARY KEY (id, recipient_user_id)`.
>
> **Build/test thật đã chạy**: `go build ./...` sạch, `go vet ./...` sạch,
> `gofmt -l` sạch trên 3 file sửa + test file. `go test ./...` (unit, không
> tag integration) — PASS toàn bộ (`domain`, `usecase`, `broadcaster`).
> `go test -tags=integration ./internal/adapter/postgres/... -run
> 'TestRepository_(SaveNotificationEvent|MarkAsRead|MarkAllAsRead|
> CountUnread|ListByRecipient)'` — **9/9 PASS thật** (chạy từng test riêng lẻ
> vì môi trường Docker dùng chung với nhiều agent/container khác — `docker
> ps`/`free -h` cho thấy 36 container đang chạy, 2.5Gi RAM free — gây
> testcontainers-go flake không liên quan tới code
> [`error: failed to open database: pq: the database system is starting up`,
> tái hiện được cả trên 1 test CŨ không hề bị đụng qua `git stash` để xác
> nhận không phải do task này gây ra]; retry tối đa 3 lần/test khi gặp lỗi
> này, không có lần nào retry quá 2 lần). Bao gồm
> `TestRepository_SaveNotificationEvent_OneRowPerRecipient`,
> `TestRepository_MarkAsRead_ScopedByTenantAndUser`,
> `TestRepository_MarkAsRead_WrongTenant_NoOp`,
> `TestRepository_MarkAsRead_Idempotent`,
> `TestRepository_MarkAllAsRead_ReturnsCorrectCount`,
> `TestRepository_CountUnread_MatchesListUnreadOnlyLength`,
> `TestRepository_ListByRecipient_OrderedByCreatedAtDesc`,
> `TestRepository_ListByRecipient_CursorPaginationNoDuplicateNoGap`,
> `TestRepository_ListByRecipient_InvalidCursorReturnsDomainError`. Pre-existing
> tests trong cùng file (`TestRepository_SaveSubscription_UpsertsOnEndpoint` etc.)
> đôi khi cũng lỗi cùng nguyên nhân môi trường — ngoài phạm vi task này,
> không sửa.

## Mục tiêu

Thêm `IsRead`/`ReadAt` vào `domain.NotificationEvent`, định nghĩa port `NotificationRepository`, và implement nó trong `internal/adapter/postgres/repository.go` chống lại bảng `notification.notification_events` (TASK-BE-NOTIF-001).

## Files cần sửa

1. `backend-go/services/notification-service/internal/domain/notification_event.go` (MODIFY — thêm 2 field)
2. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY — thêm interface `NotificationRepository`)
3. `backend-go/services/notification-service/internal/adapter/postgres/repository.go` (MODIFY — implement `NotificationRepository`)

## `domain/notification_event.go` — thêm field

```go
type NotificationEvent struct {
	ID               string
	TenantID         string
	RecipientUserIDs []string
	SourceEventID    string
	SourceSubject    string
	Type             string
	Title            string
	Body             string
	DeepLink         string
	Severity         Severity
	Channels         []DeliveryChannel
	CreatedAt        time.Time
	// IsRead/ReadAt are read-model fields — TranslateEvent never sets them
	// (zero value: false/nil), so every newly translated notification is
	// unread by construction. Populated by NotificationRepository when
	// reading persisted rows back, and set to true (with ReadAt) by
	// MarkAsRead/MarkAllAsRead's read-receipt broadcast. See
	// specs/backend-go/crs/v4/notification/solutions/BE-NOTIF-SOL-001's
	// §1 for why this lives directly on NotificationEvent instead of a
	// separate wrapper struct.
	IsRead  bool
	ReadAt  *time.Time
}
```

**Không sửa `TranslateEvent`** — hàm này tiếp tục không set 2 field mới (giữ nguyên behavior, mọi test `TestTranslateEvent_*` hiện có phải pass không đổi).

## `usecase/ports.go` — thêm `NotificationRepository`

```go
// NotificationRepository is the persistence port for notification_events —
// the audit/unread-state store CR-NOTIF-001 adds. Every method takes
// tenantID + userID explicitly (never trusts a bare notificationID) so a
// caller cannot mark-as-read or read another tenant's/user's row by
// guessing an ID — see architecture/05's tenant-isolation rule.
type NotificationRepository interface {
	// Save persists 1 row per event.RecipientUserIDs entry. Called from
	// HandleIncomingEvent BEFORE Broadcast (TASK-BE-NOTIF-003) so a crash
	// between the two leaves a persisted record, not a lost one.
	Save(ctx context.Context, event domain.NotificationEvent) error
	// ListByRecipient returns cursor-paginated notifications for
	// tenantID+userID, newest first. cursor is opaque (empty string means
	// "first page"); returned nextCursor is empty when there is no next
	// page. unreadOnly filters to IsRead == false.
	ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error)
	// MarkAsRead sets is_read=true, read_at=now() for notificationID
	// scoped to tenantID+userID. Idempotent: marking an already-read row
	// again is a successful no-op, not an error (mirrors
	// DeleteByEndpoint's idempotency rule).
	MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error
	// MarkAllAsRead sets is_read=true, read_at=now() for every unread row
	// of tenantID+userID; returns the number of rows updated (0 is not an
	// error — nothing to mark).
	MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error)
	// CountUnread returns the count of is_read=false rows for
	// tenantID+userID.
	CountUnread(ctx context.Context, tenantID, userID string) (int64, error)
}
```

## `adapter/postgres/repository.go` — implementation

Thêm vào `Repository` struct đã có (không cần struct/file mới — cùng `pool *pgxpool.Pool`).

```go
func (r *Repository) Save(ctx context.Context, event domain.NotificationEvent) error {
	batch := &pgx.Batch{}
	for _, userID := range event.RecipientUserIDs {
		batch.Queue(`
			INSERT INTO notification.notification_events (
				id, tenant_id, recipient_user_id, source_event_id, source_subject,
				type, title, body, deep_link, severity, is_read, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,$11)
		`, event.ID, event.TenantID, userID, event.SourceEventID, event.SourceSubject,
			event.Type, event.Title, event.Body, event.DeepLink, string(event.Severity), event.CreatedAt)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range event.RecipientUserIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("postgres: insert notification_events row: %w", err)
		}
	}
	return nil
}

func (r *Repository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	// Cursor = last row's created_at (RFC3339Nano) + id, keyset pagination
	// (created_at DESC, id DESC tiebreak) — avoids OFFSET's rescan cost on
	// a table that only grows. Decode/encode cursor as "<rfc3339nano>|<id>".
	// ... implement per this shape; see idx_notification_events_recipient_unread
	// (tenant_id, recipient_user_id, is_read, created_at DESC) for the index
	// this query must hit.
}

func (r *Repository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.notification_events
		SET is_read = true, read_at = now()
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND id = $3 AND is_read = false
	`, tenantID, userID, notificationID)
	if err != nil {
		return fmt.Errorf("postgres: mark notification as read: %w", err)
	}
	return nil // 0 rows affected (already read, or wrong id/user/tenant) is not an error — idempotent
}

func (r *Repository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification.notification_events
		SET is_read = true, read_at = now()
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND is_read = false
	`, tenantID, userID)
	if err != nil {
		return 0, fmt.Errorf("postgres: mark all notifications as read: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM notification.notification_events
		WHERE tenant_id = $1 AND recipient_user_id = $2 AND is_read = false
	`, tenantID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count unread notifications: %w", err)
	}
	return count, nil
}
```

`Save` dùng `pgx.Batch` (1 row/recipient trong 1 round-trip) vì `event.RecipientUserIDs` có thể có nhiều phần tử (ví dụ automation run thông báo nhiều người) — không loop `Exec` riêng lẻ (N round-trip không cần thiết).

**`ListByRecipient`'s cursor encode/decode**: viết 2 helper riêng (`encodeCursor`/`decodeCursor`) trong cùng file, unit test riêng cho 2 hàm này (không phụ thuộc DB) — cursor sai định dạng phải trả lỗi rõ ràng (`apperrors.KindInvalidArgument` ở tầng usecase, không phải panic ở tầng repository).

## Test cases cần cover

- `TestRepository_Save_OneRowPerRecipient` — event với 3 `RecipientUserIDs` tạo đúng 3 row.
- `TestRepository_MarkAsRead_ScopedByTenantAndUser` — 2 user cùng tenant, mark-as-read của user A không ảnh hưởng row của user B dù cùng notification gốc (khác `recipient_user_id`).
- `TestRepository_MarkAsRead_WrongTenant_NoOp` — `notificationID` đúng nhưng `tenantID` sai → 0 row ảnh hưởng, không lỗi, không đọc được dữ liệu tenant khác.
- `TestRepository_MarkAsRead_Idempotent` — gọi 2 lần liên tiếp, lần 2 không lỗi.
- `TestRepository_MarkAllAsRead_ReturnsCorrectCount`.
- `TestRepository_CountUnread_MatchesListUnreadOnlyLength` — `CountUnread` khớp `len(ListByRecipient(..., unreadOnly=true))`'s tổng số qua hết các trang.
- `TestRepository_ListByRecipient_OrderedByCreatedAtDesc`.
- `TestRepository_ListByRecipient_CursorPaginationNoDuplicateNoGap` — 2 trang liên tiếp không trùng, không thiếu phần tử.

Dùng `testcontainers-go` (đã có trong `go.mod` làm indirect dep — kiểm tra cách 2 migration trước được test, nếu service dùng testcontainers cho Postgres test thì mirror đúng khuôn đó, không tạo cách test mới).

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/adapter/postgres/... -run TestRepository
gofmt -l internal/domain/notification_event.go internal/usecase/ports.go internal/adapter/postgres/repository.go
```

## gitnexus

`impact({target: "Repository", direction: "upstream"})` (package `adapter/postgres`) trước khi sửa — xác nhận không phá caller nào của `postgres.New(...)` khi thêm method (thêm method vào struct hiện có không đổi constructor, rủi ro thấp, nhưng vẫn xác nhận theo quy tắc bắt buộc). `impact({target: "NotificationEvent", direction: "upstream"})` trước khi thêm field — theo `docs/crs/v4/notification/README.md`'s khảo sát trước, symbol này có 5 incoming refs (4 test file + `TranslateEvent`); thêm field không đổi chữ ký `TranslateEvent` nên các test đó không cần sửa, nhưng xác nhận lại số liệu mới nhất trước khi sửa.

## Blocking

TASK-BE-NOTIF-003 (HandleIncomingEvent), TASK-BE-NOTIF-005 (4 usecase mới) đều phụ thuộc `NotificationRepository` đã tồn tại và implement xong ở đây.
