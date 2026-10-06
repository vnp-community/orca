# TASK-REQ-009-03: `ApprovalRepository` cho Postgres và MySQL

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục D
**Priority:** P0
**Service/Area:** `request-service` / adapter
**File:** `backend-go/services/request-service/internal/adapter/postgres/approval_repository.go` (mới), `internal/adapter/mysql/approval_repository.go` (mới), `internal/usecase/ports.go` (sửa, thêm port), `internal/adapter/{postgres,mysql}/approval_repository_integration_test.go` (mới)
**Depends on:** TASK-REQ-009-01, TASK-REQ-009-02
**Status:** [ ] TODO

## Context

- Mẫu hai dialect: `task-service/internal/adapter/postgres/execution_leases.go` và `adapter/mysql/execution_leases.go` (MySQL không có `UPDATE ... RETURNING`, dùng `SELECT ... FOR UPDATE` rồi `UPDATE` trong transaction).
- Mọi truy vấn lọc `tenant_id`; Postgres chạy trong transaction có `set_config('app.tenant_id', ..., true)` như repository của CR-REQ-002.
- Chưa kiểm chứng: kiểu `Tx` chung của `request-service` (do CR-REQ-002 định nghĩa); dùng đúng kiểu đó.

## Việc cần làm

1. Trong `usecase/ports.go` khai báo `ApprovalRepository` đúng chữ ký ở solution mục D (`Insert`, `GetForUpdate`, `FindPendingBySubject`, `UpdateDecision`, `UpdatePendingDigest`, `CancelPendingForRequest`, `List`) và lỗi `ErrPendingExists`, `ErrIdempotencyConflict`, `ErrApprovalNotFound`.
2. Postgres `Insert`: bắt `pgconn.PgError` mã `23505`, so tên ràng buộc `approvals_one_pending` hoặc `approvals_idem` để trả đúng lỗi.
3. MySQL `Insert`: bắt `mysql.MySQLError` số `1062`, so tên khoá trong thông điệp.
4. `GetForUpdate`: `SELECT ... WHERE tenant_id=? AND id=? FOR UPDATE` (hai dialect đều hỗ trợ). Không thấy dòng thì `ErrApprovalNotFound` (chéo tenant cũng vậy).
5. `UpdateDecision(a, expectedVersion)`: `UPDATE approvals SET status=?, decided_by=?, decided_at=?, comment=?, version=version+1, updated_at=? WHERE id=? AND tenant_id=? AND status='pending' AND version=?`; trả `bool` theo số dòng bị ảnh hưởng. Thời gian lấy từ tham số do usecase truyền từ đồng hồ DB (`NowDB`), không `time.Now()`.
6. `UpdatePendingDigest`: chỉ ghi khi còn `pending`.
7. `CancelPendingForRequest`: Postgres `UPDATE ... RETURNING *`; MySQL `SELECT id ... WHERE request_id=? AND status='pending'` rồi `UPDATE ... WHERE id IN (...) AND status='pending'`. Trả danh sách dòng đã đổi để usecase gọi handler.
8. `List`: bộ lọc `request_id`, `subject_type`, `status`; phân trang theo `(created_at, id)` bằng `page_token` mã hoá base64 của cặp đó. Cùng câu SQL logic cho hai dialect.

## Kiểm thử

- Bộ test tích hợp chạy với hai DSN (khuôn `dbcapability`, bỏ qua nếu thiếu DSN): chèn trùng `pending`; chèn trùng `idempotency_key`; hai `UpdateDecision` đồng thời (một trả `true`); `UpdatePendingDigest` sau khi đã `approved` trả `false`; `CancelPendingForRequest` nhiều dòng; phân trang ổn định khi hai dòng cùng `created_at`; chéo tenant không thấy dòng.
- Lệnh: `go test ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run Approval` (kèm biến môi trường DSN theo quy ước repo).

## Tiêu chí hoàn thành

- [ ] Hai adapter qua cùng một bộ test.
- [ ] Lỗi trùng khoá được phân biệt đúng tên ràng buộc ở cả hai DB.
- [ ] Không có `SELECT` thiếu `tenant_id`.
- [ ] Không dùng toán tử riêng của một dialect trong tầng usecase.

## Rủi ro và lưu ý

- Thông điệp lỗi MySQL 1062 phụ thuộc phiên bản (`for key 'approvals.approvals_one_pending'` ở 8.0): test trên 8.0.x thật; khớp bằng `strings.Contains` tên khoá.
- Khóa chết: repository không tự khoá Request; thứ tự khoá do usecase (task 04).
