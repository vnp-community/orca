# BE-CV-TASK-010-05: Migration `0001_init` (`outbox_events`, `processed_events`) cho Postgres và MySQL

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/migrations/postgres/0001_init.up.sql`, `0001_init.down.sql` (mới); `migrations/mysql/0001_init.up.sql`, `0001_init.down.sql` (mới)
**Depends on:** BE-CV-TASK-010-03 (thư mục service)
**Status:** [ ] TODO

---

## Context

Hợp đồng §4.2 T0 là nguồn cột (`outbox_events`, `processed_events`), §4.1 là quy ước hai dialect và RLS. Mẫu: `mcp-service/migrations/postgres/0001_init.up.sql` (RLS `FORCE` + `NULLIF`, hai chính sách relay; đã đọc), `task-service/migrations/{postgres,mysql}/0005_outbox.up.sql`, `notification-service/migrations/*/0002_processed_events.up.sql`. **Chạy `ls services/code-intel-service/migrations/postgres` trước khi viết** để chắc số `0001` còn trống.

## Việc cần làm

1. Postgres `up`: `CREATE SCHEMA IF NOT EXISTS codeintel;` rồi `codeintel.outbox_events` (`id UUID PRIMARY KEY`, `tenant_id UUID NOT NULL`, `subject TEXT NOT NULL`, `occurred_at TIMESTAMPTZ NOT NULL`, `version INT NOT NULL`, `payload JSONB NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `published_at TIMESTAMPTZ`); `CREATE INDEX idx_codeintel_outbox_unpublished ON codeintel.outbox_events (created_at) WHERE published_at IS NULL;`.
2. `codeintel.processed_events` (`tenant_id UUID NOT NULL`, `event_id UUID NOT NULL`, `subject TEXT NOT NULL`, `processed_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `PRIMARY KEY (tenant_id, event_id)`); chỉ mục `idx_codeintel_processed_events_at (processed_at)`.
3. RLS: khối `DO $$ … FOREACH t IN ARRAY ARRAY['outbox_events','processed_events']` làm `ENABLE` + `FORCE ROW LEVEL SECURITY` và chính sách `tenant_isolation` (`USING` và `WITH CHECK` đều `tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid`). Thêm `relay_read` (`FOR SELECT USING (current_setting('app.relay', true) = 'on')`) và `relay_mark_published` (`FOR UPDATE`, cả `USING` và `WITH CHECK`) cho `outbox_events`. Chính sách `app.maintenance` **không** ở đây (thêm ở `0002`, SOL-011).
4. Postgres `down`: `DROP SCHEMA IF EXISTS codeintel CASCADE;`.
5. MySQL `up`: cùng hai bảng với `CHAR(36)`, `TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`, `JSON NOT NULL`, `ENGINE=InnoDB`, `utf8mb4`; `outbox_events` chỉ mục `(published_at, created_at)`; `processed_events` PK `(tenant_id, event_id)` + chỉ mục `processed_at`. Không schema (database `codeintel`), không RLS; chú thích đầu file lý do.
6. MySQL `down`: `DROP TABLE IF EXISTS processed_events; DROP TABLE IF EXISTS outbox_events;`.
7. Tên bảng đúng `outbox_events` (không `outbox`).

## Kiểm thử

- Integration (`-tags=integration`, từng dialect): `TestMigration_0001_UpDownUp` bằng `golang-migrate` (cùng cách workflow `task-service`: `go install -tags 'postgres'|'mysql' …/migrate@latest`); kiểm tên bảng, cột, kiểu, tính NULL qua `information_schema` ở cả hai DB.
- Postgres, role `NOSUPERUSER NOBYPASSRLS` được cấp `SELECT, INSERT, UPDATE` trên schema: `app.tenant_id=A` chèn một dòng; đổi sang `B` thì `SELECT` không thấy; không đặt `app.tenant_id` → không thấy và không lỗi cast; `app.relay='on'` thấy chéo tenant nhưng `INSERT` bị từ chối.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... ./services/code-intel-service/internal/adapter/mysql/...` (đặt test ở adapter, ghi vị trí vào PR).

## Tiêu chí hoàn thành

- [ ] up/down/up sạch trên Postgres và MySQL.
- [ ] RLS cách ly tenant bằng SQL trực tiếp (role không superuser).
- [ ] Chỉ mục unpublished: Postgres từng phần, MySQL composite.

## Rủi ro và lưu ý

- Role dev `orca` là superuser (`POSTGRES_USER: orca`): test phải tự tạo role `NOBYPASSRLS` (khuôn `mcp-service/internal/adapter/postgres/*_integration_test.go`).
- `0002` (SOL-011) bổ sung chính sách bảo trì cho hai bảng này; không gộp vào file này.
- Hai sự kiện cùng transaction có cùng `created_at` (SOL-010 mục 6, Q2).
