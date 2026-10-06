# TASK-REQ-024-03: Migration `0002_request_sync_state` và repository hai dialect

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `issue-status-sync`
**File:** `backend-go/services/issue-status-sync/migrations/postgres/0002_request_sync_state.up.sql`, `.down.sql` (mới), `backend-go/services/issue-status-sync/migrations/mysql/0002_request_sync_state.up.sql`, `.down.sql` (mới), `.../internal/adapter/postgres/request_sync_state.go`, `.../internal/adapter/mysql/request_sync_state.go` (mới), tests tích hợp, `.../internal/usecase/ports.go`
**Depends on:** None
**Status:** `[ ] TODO`

---

## Context

- `migrations/postgres/0001_processed_events.up.sql`: `CREATE SCHEMA IF NOT EXISTS issuestatussync; CREATE TABLE issuestatussync.processed_events (...)`. MySQL: không schema, `VARCHAR(255)`, `TIMESTAMP(6)`. `0001` là migration duy nhất; `0002` đúng số tiếp theo.
- Adapter hiện có: `adapter/postgres/processed_events.go` (pgx, `ON CONFLICT DO NOTHING`), `adapter/mysql/processed_events.go` (`database/sql`) và `processed_events_test.go` (tích hợp, tag `integration`).
- Workflow `backend-go-issue-status-sync.yml` đã chạy ma trận `postgres`, `mysql` với `-tags=integration`.
- Thiết kế cột: SOL-024 mục 2.4.

## Việc cần làm

1. Postgres `0002`: `CREATE TABLE issuestatussync.request_sync_state (tenant_id TEXT NOT NULL, request_id TEXT NOT NULL, last_version BIGINT NOT NULL, last_target TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (tenant_id, request_id));` `down`: `DROP TABLE IF EXISTS issuestatussync.request_sync_state;`. Không RLS (như `processed_events`, dịch vụ sở hữu DB riêng).
2. MySQL `0002`: cùng bảng, `VARCHAR(255)`, `VARCHAR(64)` cho `last_target`, `TIMESTAMP(6)`. `down` tương ứng.
3. `ports.go`: `RequestSyncStateStore interface { Advance(ctx, tenantID, requestID string, version int64, target string) (applied bool, err error) }`.
4. Postgres: `INSERT ... ON CONFLICT (tenant_id, request_id) DO UPDATE SET ... WHERE issuestatussync.request_sync_state.last_version < EXCLUDED.last_version RETURNING 1`; `applied = (một dòng trả về)`.
5. MySQL: một transaction: `SELECT last_version ... FOR UPDATE`; không có dòng thì `INSERT`; có dòng và `last_version < ?` thì `UPDATE`; ngược lại không làm gì; `applied` theo nhánh. (Cách này tránh phụ thuộc `ROW_COUNT()`; xem SOL-024 mục 6.)
6. Cập nhật `cmd/server/main.go`: dựng repository theo dialect (cùng khối `switch caps.Dialect` hiện có) và truyền vào usecase ở task 04.

## Kiểm thử

- Tích hợp (tag `integration`, testcontainers) cho cả hai adapter: lần đầu `applied=true`; version lớn hơn `applied=true`; bằng hoặc nhỏ hơn `applied=false`; hai goroutine cùng `Advance(v=5)` thì đúng một `applied=true`.
- Migration up, down, up trên cả hai dialect.
- Lệnh: `go test -tags=integration ./internal/adapter/postgres/... -v` và `./internal/adapter/mysql/...`.

## Tiêu chí hoàn thành

- [ ] Hai migration up/down chạy; `go build ./...` xanh.
- [ ] Test `Advance` xanh trên Postgres và MySQL.

## Rủi ro và lưu ý

- Bảng không có dọn dẹp (câu hỏi mở Q5); ghi `TODO` kèm số issue.
- `tenant_id` `VARCHAR(255)` trong khoá chính MySQL: độ dài khoá tối đa với utf8mb4 là 255*4*2 = 2040 byte, trong giới hạn 3072 của InnoDB; kiểm khi chạy.
