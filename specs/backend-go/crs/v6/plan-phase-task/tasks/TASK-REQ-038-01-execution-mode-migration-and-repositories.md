# TASK-REQ-038-01: Migration `0098` và repository cho `execution_mode`, `phase_starts.trigger`, chỉ mục nhật ký

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/0098_execution_mode_and_activity_indexes.{up,down}.sql` (mới), `internal/adapter/{postgres,mysql}/{project_engine_settings_repository,request_repository,phase_starts,task_run_outcomes}.go` (sửa), `internal/adapter/contracttest/*` (sửa)
**Depends on:** không
**Status:** [x] DONE

---

## Context

- Migration mới nhất của `request-service` là `0097_openspec_change_sync_progress` ở cả hai dialect (đọc 2026-10-16). **Chạy `ls migrations/{postgres,mysql}` lại trước khi tạo file**; nếu CR khác đã lấy `0098` thì dùng số lớn nhất cộng 1, hai dialect cùng số.
- `project_engine_settings` đã có `engine`, `min_version`, `solution_agent` (migration `0095`); thêm `execution_mode` cùng kiểu.
- Mặc định giữ nguyên hành vi: `step_by_step`, `trigger='manual'`.
- Không FK chéo; mọi bảng có `tenant_id`; Postgres giữ RLS hiện có (bảng không đổi chính sách).

## Việc cần làm

1. Migration `up`:
   - `request.project_engine_settings` thêm `execution_mode TEXT NOT NULL DEFAULT 'step_by_step'` với CHECK IN (`step_by_step`,`continuous`) (MySQL: `VARCHAR(20)` và CHECK).
   - `request.requests` thêm `execution_mode TEXT NULL` (CHECK như trên khi không NULL). Không thêm `execution_paused`.
   - `request.phase_starts` thêm `trigger TEXT NOT NULL DEFAULT 'manual'` với CHECK IN (`manual`,`auto`).
   - Chỉ mục phục vụ nhật ký (kiểm lại tên cột thời gian từng bảng và bỏ chỉ mục đã có): `task_run_outcomes (tenant_id, request_id, occurred_at DESC)`, `request_return_history (tenant_id, request_id, at DESC)`, `approvals (tenant_id, request_id, updated_at DESC)`, `clarifications (tenant_id, request_id, created_at DESC)`, `phase_starts (tenant_id, request_id, started_at DESC)`.
2. Migration `down`: gỡ chỉ mục và cột theo thứ tự ngược; migration thuận chiều, `down` chỉ dùng ở môi trường thử.
3. Repository:
   - `ProjectEngineSettingsRepository`: đọc/ghi `execution_mode` (`Upsert` giữ `version` tăng); giá trị rỗng từ dữ liệu cũ đọc thành `step_by_step`.
   - `RequestRepository`: thêm `PinExecutionMode(ctx, requestID, mode)` ghi một lần (`WHERE execution_mode IS NULL`), trả `pinned bool`; đọc cột vào `domain.Request.ExecutionMode` (task 02 thêm trường).
   - `PhaseStartRepository.Claim` nhận `trigger` và ghi cột.
   - `TaskRunOutcomeRepository`: thêm `ListByRequest(ctx, requestID, limit)` và `CountStartedByTask(ctx, requestID) map[taskID]int`.
4. Cập nhật bộ contract test repository để chạy trên cả hai dialect.

## Kiểm thử

- Integration hai dialect: migration xuôi trên DB đã có dữ liệu (dòng cũ có `execution_mode`/`trigger` mặc định); `PinExecutionMode` chỉ ghi một lần; `Claim` ghi `trigger`; `CountStartedByTask` đúng khi một task có 1 và 2 lần `started`.
- `EXPLAIN` truy vấn nhật ký dùng chỉ mục mới (ghi kết quả vào PR, không phải test bắt buộc).
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/... -tags integration -run 'ExecutionMode|PhaseStart|TaskRunOutcome' -v` (cần Docker; chưa chạy).

## Tiêu chí hoàn thành

- [x] Migration chạy xuôi và ngược trên Postgres và MySQL, cùng số.
- [x] Dự án và Request cũ đọc được; mặc định `step_by_step`, `trigger=manual`.
- [x] `PinExecutionMode` ghi đúng một lần.
- [x] Chỉ mục đã có không bị tạo trùng.

## Rủi ro và lưu ý

- Xung đột số migration với CR khác (xem README backend mục 4, điểm 1).
- Tên cột thời gian khác nhau giữa bảng (`at`, `occurred_at`, `updated_at`); đọc lại file migration gốc trước khi tạo chỉ mục.
- MySQL không hỗ trợ `DESC` trong chỉ mục ở phiên bản cũ; xác nhận phiên bản và bỏ `DESC` nếu cần.

## Ghi chú triển khai

Hoàn thành 2026-10-16 (kiểm chứng bằng integration test thật trên Postgres 16 và MySQL 8, testcontainers).

- Migration `0098_execution_mode.{up,down}.sql` ở cả hai dialect (số lớn nhất hiện có cộng 1, đã `ls` lại trước khi tạo): `project_engine_settings.execution_mode` (`NOT NULL DEFAULT 'step_by_step'`, CHECK), `requests.execution_mode` (NULL cho tới khi ghim), `phase_starts.start_trigger` (`manual|auto`). Chạy xuôi, ngược và xuôi lại thành công.
- Lệch so với bản soạn: **không thêm chỉ mục mới**, các chỉ mục sẵn có đã phủ truy vấn nhật ký/tóm tắt (đối chiếu từng nhánh UNION); vì vậy không có rủi ro chỉ mục trùng.
- Repository: `PinExecutionModeIfUnset` (ghi một lần, trả giá trị đã ghim), `Claim` ghi `start_trigger`, `TaskRunOutcomeRepository.ListByRequest`; cột mới có trong `expected_columns_execution_mode.go` và danh sách miễn xoá cột text (`erasable_columns.go`).
- Contract test hai dialect chạy cùng một bộ kịch bản (`engine_settings_contract.go`, `execution_repository_contract.go`).
