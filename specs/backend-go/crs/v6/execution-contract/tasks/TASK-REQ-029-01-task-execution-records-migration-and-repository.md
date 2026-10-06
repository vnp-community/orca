# TASK-REQ-029-01: Migration `task_execution_records` và repository hai dialect ở `task-service`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.E
**Priority:** P0
**Service/Area:** `task-service` / migration, domain, port, adapter postgres và mysql
**File:** `backend-go/services/task-service/migrations/postgres/NNNN_task_execution_records.{up,down}.sql` (mới), `migrations/mysql/NNNN_task_execution_records.{up,down}.sql` (mới), `internal/domain/execution_record.go` (mới), `internal/usecase/task_execution_record_ports.go` (mới), `internal/adapter/postgres/task_execution_record_repository.go` (mới), `internal/adapter/mysql/task_execution_record_repository.go` (mới), và các `_test.go`
**Depends on:** TASK-REQ-011-01 (migration `0015`/`0016`), migration `task_specs` của CR-REQ-027 (chỉ để chốt số `NNNN`; bảng này không FK sang `task_specs`)
**Status:** [ ] TODO

---

## Context

Đã đọc ngày 2026-10-06:
- `ls backend-go/services/task-service/migrations/postgres | tail` dừng ở `0014_task_sources_site.{up,down}.sql`, bản `mysql` cũng vậy. CR-REQ-011 lấy `0015` (`task_type`) và `0016` (`request_id`); CR-REQ-027 lấy số kế cho `task_specs`. Số `NNNN` của task này là **số lớn nhất đang có cộng một ngay lúc tạo file**; chạy `ls` lại, không đoán.
- Mẫu bảng con có FK cascade và RLS: `migrations/postgres/0012_task_sources.up.sql` (PK `task_id`, `ON DELETE CASCADE`, `ENABLE ROW LEVEL SECURITY`, policy `tenant_isolation` theo `current_setting('app.tenant_id', true)::uuid`); và `0010_execution_links.up.sql` (chỉ mục `idx_execution_links_task (task_id, started_at DESC)`). Bản MySQL tương ứng dùng `CHAR(36)`, `TIMESTAMP(6)`, `CONSTRAINT ... CHECK`, `FOREIGN KEY ... ON DELETE CASCADE`, chỉ mục `DESC` thật (MySQL 8).
- `task-service` không có `set_config('app.tenant_id')` ở repository (RLS chưa chạy thật, xem README feature request-service-foundation): repository **phải** lọc `tenant_id` trong mọi `WHERE`.
- `ExecutionLink` (`internal/domain/execution_link.go`) và `ExecutionLinkRepository` (`usecase/ports.go:463`) là mẫu cổng và kiểu; `postgres.Repository` triển khai nhiều cổng trên một struct, nên tên phương thức phải khác nhau (`CreateExecutionLink`, không phải `Create`). Đặt tên phương thức mới theo cùng cách: `InsertExecutionRecord`, `ListExecutionRecords`.
- Cột theo CR-REQ-029 mục 2.2: `id`, `tenant_id`, `task_id`, `execution_link_id` NULL, `attempt INT`, `spec_digest CHAR(64)`, `packet_digest CHAR(64)`, `template_version VARCHAR(16)`, `parse_status` CHECK `ok|missing|invalid`, `result` JSON NULL, `changes` JSON NULL, `stdout_tail TEXT`, `created_at`; chỉ mục `(tenant_id, task_id, created_at DESC)`. Solution thêm `failure_class` (CHECK năm giá trị, NULL) để `ListExecutionRecords` trả lớp lỗi do `task-service` quyết định.

## Việc cần làm

1. Chạy `ls migrations/postgres migrations/mysql` và chốt `NNNN`; ghi số vào mô tả PR.
2. `NNNN_task_execution_records.up.sql` Postgres:
   ```sql
   CREATE TABLE task.task_execution_records (
       id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       tenant_id         UUID NOT NULL,
       task_id           UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
       execution_link_id UUID NULL,
       attempt           INT  NOT NULL DEFAULT 1,
       spec_digest       CHAR(64) NOT NULL DEFAULT '',
       packet_digest     CHAR(64) NOT NULL DEFAULT '',
       template_version  VARCHAR(16) NOT NULL DEFAULT '',
       parse_status      TEXT NOT NULL CHECK (parse_status IN ('ok','missing','invalid')),
       failure_class     TEXT NULL CHECK (failure_class IN ('retryable','needs_info','spec_defect','env_defect','agent_defect')),
       result            JSONB NULL,
       changes           JSONB NULL,
       stdout_tail       TEXT NOT NULL DEFAULT '',
       created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   CREATE INDEX idx_task_execution_records_task ON task.task_execution_records (tenant_id, task_id, created_at DESC);
   ALTER TABLE task.task_execution_records ENABLE ROW LEVEL SECURITY;
   CREATE POLICY tenant_isolation ON task.task_execution_records USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
   ```
   `execution_link_id` không FK (link có thể bị dọn riêng); `down`: `DROP TABLE IF EXISTS task.task_execution_records;`.
3. Bản MySQL: `id CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID())`, `result JSON NULL`, `changes JSON NULL` (**không DEFAULT** cho JSON), `stdout_tail MEDIUMTEXT NOT NULL` (TEXT 64 KB đủ cho 16 KB nhưng nhiều byte UTF-8; dùng `MEDIUMTEXT` để không cắt giữa ký tự do giới hạn cột), `parse_status VARCHAR(10)` + `CONSTRAINT task_execution_records_parse_status_check CHECK (...)`, tương tự `failure_class VARCHAR(16) NULL` + CHECK, `created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`, `CONSTRAINT fk_task_execution_records_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE`, `CREATE INDEX idx_task_execution_records_task ON task_execution_records (tenant_id, task_id, created_at DESC)`.
4. `internal/domain/execution_record.go`: `ExecutionRecord{ID, TenantID, TaskID, ExecutionLinkID string; Attempt int; SpecDigest, PacketDigest, TemplateVersion string; ParseStatus ParseStatus; FailureClass FailureClass; Result, Changes []byte; StdoutTail string; CreatedAt time.Time}`:
   - `ParseStatus` (`ParseStatusOK|Missing|Invalid`) và `FailureClass` (`FailureRetryable|NeedsInfo|SpecDefect|EnvDefect|AgentDefect`, `Valid()`)
   - hằng `MaxStdoutTailBytes = 16 * 1024` và hàm thuần `TailUTF8(s string, max int) string` cắt giữ phần cuối **tại ranh giới rune** (không sinh UTF-8 hỏng).
5. Port `internal/usecase/task_execution_record_ports.go`: `TaskExecutionRecordRepository` với `InsertExecutionRecord(ctx, rec domain.ExecutionRecord) (domain.ExecutionRecord, error)` và `ListExecutionRecords(ctx, tenantID string, taskIDs []string, latestOnly bool, limit int) ([]domain.ExecutionRecord, error)` (`latestOnly` trả đúng một dòng mới nhất cho mỗi task; `limit` mặc định 50, trần 200; `taskIDs` rỗng thì lỗi `ErrInvalidArgument`).
6. Cài Postgres: `InsertExecutionRecord` chèn `result`/`changes` bằng `$n::jsonb` (nil thì `NULL`); `ListExecutionRecords` với `latestOnly` dùng `SELECT DISTINCT ON (task_id) ... ORDER BY task_id, created_at DESC`, ngược lại `ORDER BY created_at DESC LIMIT $n`; luôn `WHERE tenant_id = $1 AND task_id = ANY($2)`.
7. Cài MySQL: `InsertExecutionRecord` (UUID sinh bằng `uuid.NewString()` ở Go vì `DEFAULT (UUID())` không trả lại id khi chèn):
   - `latestOnly` bằng truy vấn con `WHERE (task_id, created_at) IN (SELECT task_id, MAX(created_at) ... GROUP BY task_id)` hoặc `ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY created_at DESC) = 1` (MySQL 8.0 có)
   - `task_id IN (?,...)` dựng động, chặn `len==0`.
8. `postgres.Repository` và `mysql.Repository` thoả `usecase.TaskExecutionRecordRepository` (thêm dòng kiểm tra biên dịch `var _ usecase.TaskExecutionRecordRepository = (*Repository)(nil)`).

## Kiểm thử

- `TestTailUTF8_CutsOnRuneBoundary` (tiếng Việt, emoji), `TestTailUTF8_ShortStringUnchanged`, `TestFailureClass_Valid`, `TestParseStatus_Valid`.
- Integration (`-tags=integration`, mẫu `repository_test.go` hiện có): `TestTaskExecutionRecordRepository_InsertList`, `_LatestOnlyPerTask`, `_CascadeOnTaskDelete`, `_TenantIsolation` (tenant B không thấy bản ghi của A, kể cả khi biết `task_id`), `_JSONRoundTripUnicode`, `_RejectsBadParseStatus` (CHECK), `_EmptyTaskIDsInvalid`, `TestMigrationNNNN_UpDownUp`.
- Hai dialect chạy cùng một bộ test bảng nếu repo có khung dùng chung; nếu không, viết hai bản đối xứng.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/domain/... ./services/task-service/internal/usecase/...` và `go test -tags=integration ./services/task-service/internal/adapter/postgres/... ./services/task-service/internal/adapter/mysql/...`.

## Tiêu chí hoàn thành

- [ ] Migration lên, xuống, lên sạch trên Postgres 14+ và MySQL 8.0.16+; CHECK từ chối `parse_status` và `failure_class` lạ.
- [ ] Xoá task xoá mọi bản ghi của nó (cascade).
- [ ] Không có truy vấn nào thiếu `tenant_id`.
- [ ] `TailUTF8` không bao giờ trả chuỗi không hợp lệ UTF-8.
- [ ] Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable.

## Rủi ro và lưu ý

- `stdout_tail` 16 KB cho mỗi lần chạy của mọi task có spec: với Plan 100 task và 3 lần thử là khoảng 5 MB; chưa có chính sách dọn (đề xuất dọn theo Request đã `completed` quá N ngày ở CR-REQ-035; ghi vào câu hỏi mở, không làm ở task này).
- `DISTINCT ON` chỉ có ở Postgres; đừng dùng chung câu SQL cho hai dialect.
- Nếu CR-REQ-011 đổi số `0015`/`0016` thì `NNNN` đổi theo; không hard-code số trong test, dùng `ls` hoặc tên tệp.
