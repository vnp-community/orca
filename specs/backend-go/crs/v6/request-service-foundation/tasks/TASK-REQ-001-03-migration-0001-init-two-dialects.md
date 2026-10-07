# TASK-REQ-001-03: Migration `0001_init` (outbox_events, processed_events) cho Postgres và MySQL

**From Solution:** BE-REQ-SOL-001
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/postgres/0001_init.up.sql`, `0001_init.down.sql` (mới); `migrations/mysql/0001_init.up.sql`, `0001_init.down.sql` (mới)
**Depends on:** TASK-REQ-001-01 (thư mục service)
**Status:** `[x] DONE`

---

## Context

Số migration: `services/request-service/migrations/` chưa tồn tại, nên `0001` là số đúng; **chạy `ls services/request-service/migrations/postgres` trước khi viết** để chắc không có PR khác đã thêm file. Mẫu: `mcp-service/migrations/postgres/0001_init.up.sql` (schema, RLS `FORCE` với `NULLIF`, hai policy relay), `task-service/migrations/{postgres,mysql}/0005_outbox.up.sql`, `notification-service/migrations/mysql/0002_processed_events.up.sql`.

## Việc cần làm

1. Postgres `0001_init.up.sql`: `CREATE SCHEMA IF NOT EXISTS request;` rồi `request.outbox_events` (`id UUID PK`, `tenant_id UUID NOT NULL`, `subject TEXT NOT NULL`, `occurred_at TIMESTAMPTZ NOT NULL`, `version INT NOT NULL`, `payload JSONB NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`, `published_at TIMESTAMPTZ`, `seq BIGINT GENERATED ALWAYS AS IDENTITY`); chỉ mục riêng phần `idx_request_outbox_unpublished ON request.outbox_events (created_at, seq) WHERE published_at IS NULL`. Cột `seq` và `clock_timestamp()` giữ thứ tự sự kiện ghi trong cùng một giao dịch (khác mẫu `task-service`, xem SOL-001 mục 3, D2).
2. `request.processed_events` (`tenant_id UUID NOT NULL`, `event_id UUID NOT NULL`, `subject TEXT NOT NULL`, `processed_at TIMESTAMPTZ NOT NULL DEFAULT now()`, PK `(tenant_id, event_id)`); chỉ mục `idx_request_processed_events_at (processed_at)`.
3. RLS: khối `DO $$ ... FOREACH t IN ARRAY ARRAY['outbox_events','processed_events']` làm `ENABLE` và `FORCE ROW LEVEL SECURITY` và policy `tenant_isolation` (`USING` và `WITH CHECK` đều `tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid`). Thêm `relay_read` (`FOR SELECT USING (current_setting('app.relay', true) = 'on')`) và `relay_mark_published` (`FOR UPDATE`, cả `USING` và `WITH CHECK`) cho `outbox_events`.
4. Postgres `down`: `DROP SCHEMA IF EXISTS request CASCADE;`.
5. MySQL `0001_init.up.sql`: cùng hai bảng với `CHAR(36)`, `TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`, `JSON NOT NULL`, `ENGINE=InnoDB`; `outbox_events` `seq BIGINT NOT NULL AUTO_INCREMENT` kèm `UNIQUE KEY uq_outbox_seq (seq)` (cột AUTO_INCREMENT bắt buộc có chỉ mục), chỉ mục `(published_at, created_at, seq)`; `processed_events` PK `(tenant_id, event_id)` và chỉ mục `processed_at`. Không schema, không RLS; ghi chú đầu file về lý do.
6. MySQL `down`: `DROP TABLE IF EXISTS processed_events; DROP TABLE IF EXISTS outbox_events;`.
7. Tên bảng là `outbox_events` (không phải `outbox`, README v6 mục 8 điểm 3).

## Kiểm thử

- Integration `-tags=integration` (testcontainers, từng dialect): `TestMigration_0001_UpDownUp` chạy up, down, up bằng `golang-migrate` trên Postgres và MySQL; sau up kiểm tên bảng, cột, kiểu, tính NULL qua `information_schema`.
- Postgres: với role `NOSUPERUSER NOBYPASSRLS` được `GRANT SELECT, INSERT, UPDATE` trên schema, đặt `app.tenant_id=A`, chèn một dòng, đổi sang `B`, `SELECT` không thấy; không đặt `app.tenant_id` thì không thấy dòng nào và không lỗi cast.
- Lệnh: `go test -tags=integration ./services/request-service/migrations/...` (đặt test trong `internal/adapter/postgres` và `internal/adapter/mysql` nếu không muốn package test ở thư mục migrations; ghi vị trí chọn vào PR).

## Tiêu chí hoàn thành

- [x] up/down/up sạch trên Postgres và MySQL.
- [x] RLS cách ly tenant bằng SQL trực tiếp (Postgres, role không superuser).
- [x] Chỉ mục unpublished: Postgres partial, MySQL composite.
- [x] Không còn dòng `outbox_events` khác tên ở tài liệu.

## Rủi ro và lưu ý

- Nếu role kết nối ở dev là superuser thì RLS bị bỏ qua; test tích hợp phải tự tạo role `NOBYPASSRLS` (khuôn `mcp-service/internal/adapter/postgres/*_integration_test.go`).
- CR-REQ-002 sẽ thêm `0002`; không gộp vào file này.
