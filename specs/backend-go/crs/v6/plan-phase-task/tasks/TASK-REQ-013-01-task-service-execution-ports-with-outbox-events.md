# TASK-REQ-013-01: `task-service` đổi cổng `ClaimForExecution`, `CompleteExecution`, `ReleaseExecution` để ghi outbox cùng transaction

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/execution_lease.go` (dòng 113, 238), `internal/usecase/ports.go` (dòng 60), `internal/adapter/postgres/execution_leases.go` (dòng 77, 163), `internal/adapter/postgres/repository.go` (dòng 347), `internal/adapter/mysql/execution_leases.go` (dòng 146, 164), `internal/adapter/mysql/repository.go` (dòng 360), `internal/usecase/fakes_test.go`, `internal/usecase/execution_lease_test.go`, `internal/adapter/grpc/server_test.go`
**Depends on:** TASK-REQ-011-05 (payload `statuschanged` có `cause`, `request_id`, `task_type`)
**Status:** `[ ] TODO`

---

## Context

Ba cổng hiện tại (đã đọc ngày 2026-10-06):
- `TaskExecutionClaimer.ClaimForExecution(ctx, tenantID, taskID string, from domain.Status) (claimed bool, err error)` (`execution_lease.go` dòng 238 đến 241), hiện thực ở `postgres/execution_leases.go` dòng 77 và `mysql/execution_leases.go` dòng 164.
- `TaskExecutionReleaser.ReleaseExecution(ctx, tenantID, taskID, linkID string, to domain.Status) (released bool, err error)` (dòng 113 đến 115), compare-and-set theo link đang hoạt động (test `TestExecutionLeases_ReleaseExecution_IsCompareAndSetOnActiveLink`).
- `TaskRepository.CompleteExecution(ctx, tenantID, id, status string, actualHours float64) error` (`ports.go` dòng 60), hiện thực ở `postgres/repository.go` dòng 347 và `mysql/repository.go` dòng 360.
Không cái nào ghi `task.outbox_events`. Mẫu transaction UPDATE + outbox: `Repository.Update` (postgres dòng 425 đến 465). Test claim đồng thời ở `execution_leases_test.go` dòng 155 phải tiếp tục xanh (đúng một người thắng).
Đổi chữ ký chạm nhiều implementer; chạy `gitnexus_impact` cho ba symbol và grep lại toàn `services/task-service` (chỉ mục có thể cũ).

## Việc cần làm

1. Đổi chữ ký, thêm tham số cuối `events []domain.OutboxEvent`:
   ```go
   ClaimForExecution(ctx, tenantID, taskID string, from domain.Status, events []domain.OutboxEvent) (bool, error)
   ReleaseExecution(ctx, tenantID, taskID, linkID string, to domain.Status, events []domain.OutboxEvent) (bool, error)
   CompleteExecution(ctx, tenantID, id, status string, actualHours float64, events []domain.OutboxEvent) error
   ```
2. Adapter Postgres: mỗi hàm mở transaction (`r.pool.Begin`) nếu `len(events) > 0` (ngược lại giữ đường cũ để không đổi hiệu năng), thực hiện UPDATE hiện có rồi INSERT từng event vào `task.outbox_events` **chỉ khi UPDATE thực sự đổi hàng** (`claimed`/`released` true, hoặc `CompleteExecution` thành công). Claim thua thì không ghi outbox.
3. Adapter MySQL: tương tự với `sql.Tx`, bảng `outbox_events` (xem `mysql/outbox.go`).
4. Nếu `Repository` đang chạy trong `RunInTx` (`db` là tx), dùng tx sẵn có thay vì mở tx mới (xem kiểu `dbtx`, dòng 24 đến 48 của hai `repository.go`).
5. Cập nhật mọi caller hiện có truyền `nil` (hành vi cũ): `execute_task.go` (dòng 229, 385), `report_execution_result.go` (dòng 85, 100), `execution_lease.go` (dòng 201). Việc phát sự kiện thật ở TASK-REQ-013-02.
6. Cập nhật fake: `fakes_test.go`, `execution_lease_test.go`, `adapter/grpc/server_test.go`, và mọi `*_test.go` khác implement các cổng này (grep `ClaimForExecution`, `ReleaseExecution`, `CompleteExecution`).
7. Không đổi `ReleaseUnlinkedInProgress` (hàng loạt, không trả id).

## Kiểm thử

- Unit: các test hiện có xanh sau khi cập nhật chữ ký.
- Integration hai dialect, thêm vào `execution_leases_test.go`: `TestClaimForExecution_WritesOutboxInSameTx`, `TestClaimForExecution_LostRace_NoOutbox`, `TestReleaseExecution_StaleLink_NoOutbox`, `TestCompleteExecution_WritesOutboxInSameTx`, `TestExecutionWrite_OutboxFailure_StatusUnchanged` (chèn lỗi bằng subject vi phạm ràng buộc hoặc đóng tx giữa chừng).
- `cd /opt/repos/orca/backend-go && go build ./services/task-service/... && go test ./services/task-service/... && go test -tags=integration ./services/task-service/internal/adapter/... -run 'Execution' -v`.

## Tiêu chí hoàn thành

- [ ] Ba cổng nhận `events`, hai dialect ghi outbox cùng transaction với UPDATE.
- [ ] Mất CAS (claim thua, link cũ) không ghi dòng outbox nào.
- [ ] Outbox lỗi thì status không đổi (hoặc UPDATE bị rollback).
- [ ] Caller cũ truyền `nil` giữ nguyên hành vi; toàn bộ test hiện có xanh.
- [ ] Không thêm `max-lines` disable.

## Rủi ro và lưu ý

- Mở tx mới trong hàm vốn là một câu UPDATE đơn thêm một round trip khi có events; chỉ áp khi `len(events) > 0`.
- `ClaimForExecution` MySQL phụ thuộc `RowsAffected` (xem test đồng thời); giữ nguyên ngữ nghĩa.
- Thứ tự commit: nếu tách PR, merge task này trước 013-02 để không có trạng thái biên dịch lỗi.
