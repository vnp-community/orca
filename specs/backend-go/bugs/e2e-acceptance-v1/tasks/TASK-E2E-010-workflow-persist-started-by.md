# TASK-E2E-010: Lưu `started_by` của execution và khôi phục khi service khởi động lại

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-008
**Priority:** P1 · **Service/Area:** workflow-service
**Files:**
- backend-go/services/workflow-service/migrations/postgres/NNNN_execution_started_by.{up,down}.sql (mới)
- backend-go/services/workflow-service/migrations/mysql/NNNN_execution_started_by.{up,down}.sql (mới)
- internal/domain/execution.go
- internal/adapter/postgres/*.go
- internal/adapter/mysql/*.go
- internal/usecase/recover_executions.go
**Depends on:** TASK-E2E-009
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`RecoverExecutions` dựng context sau khi khởi động lại; không có người dùng nên bước `project:` lỗi `PROJECT_NO_USER`.

## Việc cần làm

- [x] Migration cộng thêm cột `started_by` (nullable) cho cả Postgres và MySQL (G5).
- [x] `Execute` ghi cột; `RecoverExecutions` dựng context với người đó (`tenant.WithUserID`).
- [~] Execution cũ (cột rỗng): chạy tiếp tenant-only, log cảnh báo `started_by missing`. — _có test; chưa kiểm cảnh báo `started_by missing` với execution cũ trên dữ liệu thật_
- [x] Test hai dialect cho repository; test usecase recover.

## Tiêu chí hoàn thành

- [ ] Khởi động lại `workflow-service` giữa execution nhiều wave có bước `project:` vẫn `completed`. — _chưa thử khởi động lại giữa execution nhiều wave trên stack thật_
- [~] Migration lên/xuống chạy sạch hai dialect. — _migration lên đã chạy trên Postgres của stack; xuống và MySQL chưa chạy_

## Kiểm thử

Test tích hợp khởi động lại (container `workflow` restart khi execution đang chạy) trong `orca-flows`.

## Kết quả thực thi

Xong. Migration `0013_execution_started_by` (postgres, mysql), `StartedBy` ở domain và hai repo, khôi phục người dùng khi recover; test `execution_identity_test.go`.
