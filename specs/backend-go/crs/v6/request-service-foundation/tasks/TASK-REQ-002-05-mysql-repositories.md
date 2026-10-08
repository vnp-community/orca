# TASK-REQ-002-05: Repository MySQL (database/sql) tương ứng

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/mysql/{request_repository.go,request_counter.go,request_type_history_repository.go,solution_repository.go,request_link_repository.go,request_idempotency_repository.go,request_scan.go}` (mới)
**Depends on:** TASK-REQ-002-01, TASK-REQ-002-02, TASK-REQ-002-03 (song song được với TASK-REQ-002-04)
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: go test -tags integration ./internal/adapter/mysql/... (PASS, MySQL 8.0))

---

## Context

Mẫu: `task-service/internal/adapter/mysql/{repository.go,task_sources.go,outbox.go}`: `database/sql`, placeholder `?`, `ExecContext`/`QueryContext`, không `RETURNING`, không RLS (`dbcapability` ghi `SupportsRLS=false`, `SupportsReturning=false`). Executor lấy từ ctx (`*sql.Tx` hoặc `*sql.DB`) từ TASK-REQ-001-04. Isolation level mặc định InnoDB là `REPEATABLE READ`: các lần đọc lại sau CAS thất bại phải nằm trong giao dịch mới của lần thử sau, không tái dùng snapshot cũ.

## Việc cần làm

1. `request_scan.go`: quét hàng, `sql.NullString`, `sql.NullFloat64`, `sql.NullTime`; thời gian đọc với `parseTime=true` (DSN từ TASK-REQ-001-05); chuyển `[]byte` JSON sang `[]byte` thô.
2. `request_repository.go`: `Create`, `Get`, `GetByNumber` như bản Postgres nhưng `?`, `tenant_id = ?` ở mọi `WHERE`. `Update` CAS: `UPDATE ... SET ..., version = version + 1, updated_at = NOW(6) WHERE id=? AND tenant_id=? AND version=?`; sau `ExecContext` kiểm `RowsAffected()`; 0 thì `SELECT` phân biệt; thành công thì `SELECT` đọc lại bản mới (không có `RETURNING`). `List`: keyset `AND (created_at < ? OR (created_at = ? AND id < ?))`; `IN (?,...)` dựng động cho `Statuses`, `Types` (chặn mảng rỗng: bỏ điều kiện).
3. `request_counter.go`: `NextNumber`: `INSERT INTO request_counters (tenant_id, next_number) VALUES (?, 1) ON DUPLICATE KEY UPDATE next_number = next_number + 1;` rồi `SELECT next_number FROM request_counters WHERE tenant_id = ?` trong cùng giao dịch (hàng bị khoá tới commit). Ngoài giao dịch trả lỗi lập trình như bản Postgres.
4. `request_type_history_repository.go`, `solution_repository.go`, `request_link_repository.go`: tương ứng; lỗi CHECK cha trùng con (MySQL mã 3819, chỉ khi bản 8.0.16 trở lên) ánh xạ sang `REQUEST_LINK_SELF`; thêm kiểm `parent == child` ở `NewRequestLink` (đã có ở domain) làm hàng rào chính.
5. `request_idempotency_repository.go`: `Claim` dùng `INSERT IGNORE INTO request_idempotency (...)`, kiểm `RowsAffected()`; 0 thì `SELECT request_id` của bên thắng. `Find` như Postgres.
6. Id sinh bằng `uuid.NewString()` ở ứng dụng; không `UUID()` của MySQL.
7. Assert `var _ usecase.RequestRepository = (*Repository)(nil)` cho cả năm cổng.

## Kiểm thử

Chạy cùng bộ kịch bản ở TASK-REQ-002-06 trên MySQL; test đặc thù:
- `TestMySQL_QueriesAlwaysFilterTenant`: tạo hai tenant, mỗi repo method được gọi bằng ctx của tenant B không thấy, không cập nhật, không xóa dữ liệu của A.
- `TestMySQL_NextNumber_Concurrent20`: 20 goroutine `InTx` + `NextNumber` + `Create`, tập số là `1..20` không trùng, không lỗ hổng; rollback một giao dịch không để lỗ hổng.
- `TestMySQL_Update_CAS_RowsAffected`: hai `Update` cùng `expectedVersion`, đúng một thắng.
- `TestMySQL_JSONRoundTrip` (`options` `[]` và có dữ liệu).
Lệnh: `go test -tags=integration ./services/request-service/internal/adapter/mysql/... -v`.

## Tiêu chí hoàn thành

- [x] Năm cổng được cài đặt, assert biên dịch.
- [x] 20 `Create` đồng thời cho 20 số liên tiếp.
- [x] Hai `Claim` đồng thời: đúng một bên `claimed=true`.
- [x] Bộ kịch bản dùng chung xanh với MySQL.

## Rủi ro và lưu ý

- Không có RLS: một câu thiếu `tenant_id` là rò rỉ dữ liệu; review từng truy vấn và để test `QueriesAlwaysFilterTenant` bao phủ mọi phương thức.
- `RowsAffected` mặc định tính hàng thật sự đổi; vì `version` luôn tăng nên CAS an toàn mà không cần `clientFoundRows=true`.
- `INSERT IGNORE` nuốt cả lỗi khác (truncate dữ liệu); kiểm cảnh báo hoặc dùng `ON DUPLICATE KEY UPDATE request_id = request_id` làm phương án, ghi lựa chọn vào PR.

## Ghi chú triển khai

Cùng lệch về struct riêng như 002-04. `NextNumber` dùng upsert rồi đọc lại trong cùng giao dịch (giữ khoá hàng đếm); `Claim` dùng `ON DUPLICATE KEY UPDATE` no-op (không dùng `INSERT IGNORE` vì nuốt cả lỗi CHECK). Test đặc thù phủ bởi bộ chung: `TenantIsolation*`, `NumberingConcurrent20`, `UpdateCASConflict`, `SolutionsCAS` (JSON `[]` và có dữ liệu), `ClaimConcurrent`. Chạy trên MySQL 8.0 thật.
