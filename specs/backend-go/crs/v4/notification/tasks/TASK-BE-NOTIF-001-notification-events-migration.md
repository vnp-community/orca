# TASK-BE-NOTIF-001: Migration `notification.notification_events`

**Solution:** BE-NOTIF-SOL-001 | **CR:** CR-NOTIF-001
**Service:** `notification-service`
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Tạo `0003_notification_events.up.sql`/`.down.sql` đúng
> nội dung task mô tả, khớp convention RLS của `0001_init.up.sql`/
> `0002_processed_events.up.sql`. Môi trường không có host port cho
> `orca-go-postgres` (không expose 5432 ra host, chỉ nằm trên network
> `orca-go_orca-go-net`) nên KHÔNG chạy được `migrate` CLI trực tiếp từ host
> theo README; verify thay bằng cách `docker cp` 2 file SQL vào container
> `orca-go-postgres` và `psql -U orca -d notification -f` trực tiếp (DB
> `notification` đã tồn tại sẵn trong container này, `schema_migrations`
> đang ở version 2 trước khi áp). Xác nhận thật: `\d
> notification.notification_events` đúng 13 cột, đúng type, đúng
> NOT NULL/DEFAULT; `idx_notification_events_recipient_unread` đúng cột
> `(tenant_id, recipient_user_id, is_read, created_at DESC)`; RLS
> `relrowsecurity = t` + policy `tenant_isolation` đúng biểu thức. INSERT
> thiếu `recipient_user_id` → lỗi NOT NULL thật (`ERROR: null value in
> column "recipient_user_id"`); INSERT `severity='bogus'` → lỗi CHECK thật
> (`ERROR: ... violates check constraint "notification_events_severity_check"`);
> INSERT hợp lệ → thành công. Chạy thử `.down.sql` trên bản test riêng (DROP
> TABLE thành công) rồi re-apply `.up.sql` để service ở trạng thái sẵn sàng
> cho TASK-BE-NOTIF-002, và cập nhật `schema_migrations.version = 3` thủ
> công (bảng này chỉ có 1 row theo đúng cơ chế golang-migrate, không phải
> multi-row log) để phản ánh đúng trạng thái sau khi áp migration 0003.
>
> **Sửa lại sau khi TASK-BE-NOTIF-002 phát hiện lỗi thiết kế thật**: PK gốc
> `id UUID PRIMARY KEY` (task doc ban đầu) SAI khi 1 event có >1
> `RecipientUserIDs` — `Save`/`SaveNotificationEvent` insert CÙNG
> `event.ID` cho mỗi recipient row (đúng như comment "same value as
> domain.NotificationEvent.ID" mô tả), nên row thứ 2 trở đi vi phạm
> UNIQUE ngay lập tức (`ERROR: duplicate key value violates unique
> constraint "notification_events_pkey"` — lỗi thật, bắt được khi chạy
> `TestRepository_Save*OneRowPerRecipient`). Đã sửa PK thành
> `PRIMARY KEY (id, recipient_user_id)` — an toàn vì `MarkAsRead` vốn đã
> luôn lọc thêm `recipient_user_id` trong `WHERE`, không làm yếu tenant/user
> isolation. Đã DROP + re-CREATE lại bảng trên container test
> (`orca-go-postgres`) và xác nhận lại `\d` (PK giờ là composite). File
> `.up.sql` trong repo đã cập nhật đúng nội dung mới này.

## Mục tiêu

Thêm bảng `notification.notification_events` — 1 row / (notification × recipient), lưu lịch sử + trạng thái đã đọc/chưa đọc, theo đúng convention RLS 2 migration trước của service (`0001_init.up.sql`, `0002_processed_events.up.sql`).

## Files cần sửa

1. `backend-go/services/notification-service/migrations/0003_notification_events.up.sql` (MỚI)
2. `backend-go/services/notification-service/migrations/0003_notification_events.down.sql` (MỚI)

## Nội dung `0003_notification_events.up.sql`

```sql
-- One row per (notification, recipient) — NOT one row with an array of
-- recipients — so marking user A's copy read never touches user B's row,
-- same scoping unit push_subscriptions already uses (tenant_id, user_id).
CREATE TABLE notification.notification_events (
    id                 UUID PRIMARY KEY,             -- same value as domain.NotificationEvent.ID (uuid.NewString() in HandleIncomingEvent)
    tenant_id          UUID NOT NULL,
    recipient_user_id  UUID NOT NULL,
    source_event_id    TEXT NOT NULL,
    source_subject     TEXT NOT NULL,
    type               TEXT NOT NULL,
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    deep_link          TEXT,
    severity           TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
    is_read            BOOLEAN NOT NULL DEFAULT false,
    read_at            TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Covers ListNotifications(unread_only=true) and GetUnreadCount's hot path:
-- filter by tenant+recipient(+is_read), order by created_at DESC.
CREATE INDEX idx_notification_events_recipient_unread
    ON notification.notification_events (tenant_id, recipient_user_id, is_read, created_at DESC);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id + recipient_user_id filtering
-- (internal/adapter/postgres) is the primary enforcement; this is the
-- secondary backstop, same pattern as push_subscriptions/vapid_key_metadata.
ALTER TABLE notification.notification_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.notification_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

## Nội dung `0003_notification_events.down.sql`

```sql
DROP TABLE IF EXISTS notification.notification_events;
```

## Test cases cần cover

Không có Go test cho riêng migration (đúng thông lệ 2 migration trước, không có test file `.sql`) — verify bằng cách áp migration thật lên DB test (xem "Verify").

## Verify

```bash
cd backend-go/services/notification-service
# Chạy đúng cơ chế migration thật của service (kiểm tra Makefile/README
# trước khi đoán câu lệnh — ví dụ golang-migrate hoặc script nội bộ)
grep -n "migrate" README.md Makefile 2>/dev/null
# Sau khi xác nhận công cụ, áp thử lên 1 Postgres test và kiểm tra:
#   \d notification.notification_events   (đúng cột, đúng index, RLS bật)
#   INSERT thử 1 row thiếu recipient_user_id -> phải lỗi NOT NULL
#   INSERT severity ngoài enum -> phải lỗi CHECK constraint
```

## gitnexus

Không có symbol Go nào bị sửa ở task này (thuần SQL) — không cần chạy `impact()`. Task tiếp theo (TASK-BE-NOTIF-002) sẽ chạy `impact()` cho `Repository`/`ports.go` trước khi thêm method.

## Blocking

TASK-BE-NOTIF-002 (repository) phụ thuộc bảng này đã tồn tại để viết SQL query thật đúng tên cột.
