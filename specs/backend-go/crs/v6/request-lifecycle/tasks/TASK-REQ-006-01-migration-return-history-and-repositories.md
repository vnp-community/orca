# TASK-REQ-006-01: Migration `request_return_history` + `returned_category`, domain, repository

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `request-service`
**File:** `migrations/postgres/0005_request_return_history.{up,down}.sql`, `migrations/mysql/0005_request_return_history.{up,down}.sql` (mới); `internal/domain/{request_return_category.go,request_return_history.go,request_return_stage.go,request_return_errors.go}` và `*_test.go` (mới); `internal/domain/request.go` (sửa); `internal/usecase/ports.go` (sửa: `ReturnHistoryRepository`); `internal/adapter/{postgres,mysql}/{return_history_repository.go,request_repository.go,request_scan.go}` (mới/sửa)
**Depends on:** TASK-REQ-005-01 (migration `0004`), TASK-REQ-002-04, TASK-REQ-002-05
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: go test ./internal/domain ./internal/usecase; go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql (Postgres 16 và MySQL 8.0 thật) -run "Migration|Return")

---

## Context

CR-REQ-006 mục 2.1 ghi `0004_request_return_history`, nhưng SOL-005 đã dùng `0004` cho `classification_attempts` và SOL-004 dùng `0003`; **chạy `ls services/request-service/migrations/postgres` để lấy số kế tiếp thật** (kỳ vọng `0005`; nếu CR-REQ-007 hay 009 đã thêm migration thì số lớn hơn) rồi đổi tên file và tham chiếu. CHECK `(status='request_backlog') = (returned_category IS NOT NULL)` sẽ vi phạm bởi mọi dòng backlog sẵn có: phải `UPDATE ... SET returned_category='other'` trước `ADD CONSTRAINT` (SOL-006 Correction C2). MySQL CHECK cần 8.0.16.

## Việc cần làm

1. Postgres up: thêm cột `returned_category TEXT CHECK (... 5 giá trị)`; `UPDATE request.requests SET returned_category='other' WHERE status='request_backlog'`; `ADD CONSTRAINT requests_backlog_category CHECK (...)`; tạo `request.request_return_history` và chỉ mục `(tenant_id, request_id, at)`; khối RLS `ENABLE` + `FORCE` + `tenant_isolation` (`NULLIF`) cho bảng mới. Down: bỏ ràng buộc, bảng, cột.
2. MySQL: cùng nội dung (`returned_category VARCHAR(30) NULL`; `UPDATE` rồi `ADD CONSTRAINT requests_backlog_category CHECK (...)`; bảng `CHAR(36)`, `TIMESTAMP(6)`); down tương ứng.
3. `domain`: `ReturnCategory` (`missing_info`, `infeasible`, `blocked_dependency`, `rejected`, `other`) với `ParseReturnCategory` (`REQUEST_RETURN_CATEGORY_INVALID`); `ReturnAction` (`returned`, `reopened`, `cancelled`); `ReturnHistoryEntry{ID, RequestID string; Action; Stage ReturnStage; Category ReturnCategory; Reason, ActorID string; ActorKind ActorKind; At time.Time}`; `ActorKind` thêm hằng `system`; `Request.ReturnedCategory ReturnCategory`. `request_return_stage.go`: `StageForStatus(status RequestStatus, flow FlowDefinition, size RequestSize) ([]ReturnStage, error)` trả tập stage hợp lệ (bảng SOL-006 mục B).
4. `ports.go`: `ReturnHistoryRepository{ Append(ctx, ReturnHistoryEntry) error; List(ctx, requestID string) ([]ReturnHistoryEntry, error) }`.
5. Hai adapter: `Append` chỉ INSERT (id sinh ở ứng dụng, `at` mặc định đồng hồ DB), `List` `ORDER BY at, id`; `request_repository.go`, `request_scan.go` ghi và đọc `returned_category` (NULL ↔ rỗng).
6. `contracttest.ExpectedColumns()` thêm cột và bảng mới.

## Kiểm thử

- `TestMigration_Return_UpDownUp` hai dialect, kèm kịch bản: chèn một dòng `request_backlog` trước migration (bằng migrate tới bản trước, chèn, rồi up) và kiểm backfill `other`.
- `TestChecks_ReturnCategoryAndBacklogPairing` (backlog không category và category không backlog đều bị từ chối).
- `TestReturnHistory_AppendListOrder`, `TestReturnHistory_TenantIsolation`.
- `TestStageForStatus` (bảng 7 trạng thái; `executing` + loại không có Phase chỉ trả `task`).
- Lệnh: `go test ./services/request-service/internal/domain/...`; `go test -tags=integration ./services/request-service/internal/adapter/... -run "Return|Migration"`.

## Tiêu chí hoàn thành

- [x] up/down/up sạch hai dialect, có backfill.
- [x] CHECK ghép cặp giữ ở mọi trường hợp.
- [x] `StageForStatus` đúng bảng, `phase` chỉ khi loại có Phase.
- [x] Số migration xác nhận bằng `ls` và ghi trong PR.

## Rủi ro và lưu ý

- Task này chỉ merge cùng đợt với TASK-REQ-006-02: giữa hai task, mọi `TransitionRequest` sang backlog sẽ vi phạm CHECK. Cờ `request_flow_enabled` tắt nên không có dữ liệu thật, nhưng test của SOL-003 sẽ đỏ nếu thiếu `Category`.
- Số migration có thể bị chiếm: không giữ `0005` cứng.
