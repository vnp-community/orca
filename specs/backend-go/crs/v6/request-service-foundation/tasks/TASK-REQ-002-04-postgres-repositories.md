# TASK-REQ-002-04: Repository Postgres (pgx) cho Request, lịch sử loại, Solution, liên kết, idempotency

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/postgres/{request_repository.go,request_counter.go,request_type_history_repository.go,solution_repository.go,request_link_repository.go,request_idempotency_repository.go,request_scan.go}` (mới)
**Depends on:** TASK-REQ-002-01, TASK-REQ-002-02, TASK-REQ-002-03
**Status:** [ ] TODO

---

## Context

`Repository` và `exec(ctx)` (executor lấy từ ctx, tự bọc giao dịch ngắn có `set_config('app.tenant_id')` khi ngoài giao dịch) đã có từ TASK-REQ-001-04. Mẫu SQL viết tay bằng pgx: `task-service/internal/adapter/postgres/task_sources.go`, `repository.go` (không dùng sqlc). Postgres schema `request`, mọi câu SQL ghi `request.<bảng>`. Mọi truy vấn ghi `tenant_id = $n` ở `WHERE` dù có RLS (phòng thủ chiều sâu).

## Việc cần làm

1. `request_scan.go`: hàm quét một hàng `requests` vào `domain.Request` (NULL sang rỗng cho `type`, `type_source`, `size`, `project_id`, `plan_task_id`, `returned_from_stage`; `confidence` sang `*float64`); danh sách cột `requestColumns` dùng chung các truy vấn.
2. `request_repository.go`:
   - `Create`: `INSERT INTO request.requests (...) VALUES (...)`; lỗi vi phạm unique `(tenant_id, number)` trả lỗi nội bộ (không xảy ra vì `NextNumber` trong cùng giao dịch).
   - `Get(ctx, id)`: `WHERE id=$1 AND tenant_id=$2`; không thấy trả `ErrRequestNotFound`.
   - `GetByNumber`.
   - `Update(ctx, r, expectedVersion)`: `UPDATE ... SET <mọi cột đổi được>, version = version + 1, updated_at = now() WHERE id=$1 AND tenant_id=$2 AND version=$3 RETURNING <requestColumns>`; không có hàng thì `SELECT 1 ... WHERE id AND tenant_id` để chọn `ErrRequestNotFound` hay `ErrRequestVersionConflict`.
   - `List`: dựng `WHERE tenant_id=$1` + điều kiện tùy chọn (`project_id`, `reporter_id`, `source_*`, `status = ANY($n)`, `type = ANY($n)`), keyset `AND (created_at, id) < ($a, $b)`, `ORDER BY created_at DESC, id DESC LIMIT pageSize+1`; dư một hàng thì tạo `NextPageToken` từ hàng cuối trang.
3. `request_counter.go`: `NextNumber`: `INSERT INTO request.request_counters (tenant_id, next_number) VALUES ($1, 1) ON CONFLICT (tenant_id) DO UPDATE SET next_number = request.request_counters.next_number + 1 RETURNING next_number`. Phải gọi trong giao dịch của use case (ctx có `txKey`); ngoài giao dịch trả lỗi lập trình `usecase: NextNumber requires InTx` để khỏi cấp số rồi mất.
4. `request_type_history_repository.go`: `Append` (chỉ INSERT), `List(ctx, requestID)` `ORDER BY at, id`.
5. `solution_repository.go`: `Insert`, `Get`, `ListByRequest` (`ORDER BY created_at`), `Update` CAS giống `Request` (`SOLUTION_NOT_FOUND`, `SOLUTION_VERSION_CONFLICT`); `options` là `[]byte` vào cột `jsonb` (`$n::jsonb`).
6. `request_link_repository.go`: `Insert` (trùng khoá chính là lỗi nội bộ; bắt vi phạm CHECK cha trùng con trả `REQUEST_LINK_SELF`), `ListChildren`, `ListParents`.
7. `request_idempotency_repository.go`: `Claim`: `INSERT ... ON CONFLICT DO NOTHING`, kiểm `RowsAffected()`; 0 thì `SELECT request_id` và trả `(id, false, nil)`; 1 thì `(“”, true, nil)`. `Find`: `SELECT request_id`.
8. Assert `var _ usecase.RequestRepository = (*Repository)(nil)` cho cả năm cổng.

## Kiểm thử

Test tích hợp riêng Postgres (`-tags=integration`) gọi bộ kịch bản dùng chung ở TASK-REQ-002-06; ngoài ra các test đặc thù:
- `TestPostgres_RLS_DirectSQLCannotSeeOtherTenant` (role `NOBYPASSRLS`).
- `TestPostgres_NextNumber_OutsideTxRejected`.
- `TestPostgres_Update_ReturnsBumpedVersion`.
Lệnh: `go test -tags=integration ./services/request-service/internal/adapter/postgres/... -v`.

## Tiêu chí hoàn thành

- [ ] Năm cổng được cài đặt, assert biên dịch.
- [ ] Mọi truy vấn có `tenant_id` ở `WHERE`.
- [ ] CAS phân biệt `NOT_FOUND` với `VERSION_CONFLICT`.
- [ ] Bộ kịch bản dùng chung xanh với Postgres.

## Rủi ro và lưu ý

- `status = ANY($n)` với `[]string`: pgx cần kiểu `[]string` tường minh, không phải `[]RequestStatus`; chuyển trước khi truyền.
- `(created_at, id) < ($a,$b)` cần chỉ mục `(tenant_id, created_at DESC, id DESC)` đã có ở `0002`; kiểm `EXPLAIN` trên dữ liệu thử (chưa kiểm chứng).
