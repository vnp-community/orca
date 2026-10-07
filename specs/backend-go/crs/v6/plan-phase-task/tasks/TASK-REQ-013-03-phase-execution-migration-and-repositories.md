# TASK-REQ-013-03: Migration `phase_execution` (`phase_starts`, `task_run_outcomes`) và repository hai dialect

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_phase_execution.{up,down}.sql` (mới), `internal/domain/phase_start.go` (mới), `internal/domain/task_run_outcome.go` (mới), `internal/usecase/ports.go`, `internal/adapter/postgres/phase_starts.go`, `internal/adapter/postgres/task_run_outcomes.go`, `internal/adapter/mysql/phase_starts.go`, `internal/adapter/mysql/task_run_outcomes.go` (mới), `*_integration_test.go` (mới)
**Depends on:** CR-REQ-001, CR-REQ-002 (module, `0002_request_core`, `TxRunner`, outbox, `processed_events`)
**Status:** `[x] DONE`

---

## Context

- `request-service` chưa có thư mục ngày 2026-10-06. Số migration của nó **chưa xác định**: các CR trong v6 cùng đặt `0002` (`request_core` của CR-REQ-002, `processed_events` của CR-REQ-001, `request_sync_state` của CR-REQ-024), `0003_request_source_hints` (CR-REQ-004), `0004_request_return_history` (CR-REQ-006), `0005_outbox` (CR-REQ-001). Quy tắc cho task này: `ls migrations/postgres` ngay trước khi tạo và lấy số lớn nhất cộng 1; báo lệch số cho người điều phối. Số phải **giống nhau ở hai dialect**.
- Mẫu bảng `tenant_id`, RLS Postgres, MySQL kiểm `tenant_id` ở mọi `WHERE`: `task-service/migrations/postgres/0012_task_sources.up.sql` và bản `mysql`.
- `phase_starts` làm khoá idempotency: Postgres `INSERT ... ON CONFLICT DO NOTHING`, MySQL `INSERT IGNORE`.
- Kiểu: Postgres `UUID`, `TIMESTAMPTZ`; MySQL `CHAR(36)`, `TIMESTAMP(6)`.

## Việc cần làm

1. Up Postgres:
   ```sql
   CREATE TABLE request.phase_starts (
     tenant_id UUID NOT NULL, phase_task_id UUID NOT NULL, request_id UUID NOT NULL,
     started_by UUID NOT NULL, started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
     PRIMARY KEY (tenant_id, phase_task_id));
   CREATE TABLE request.task_run_outcomes (
     id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL, task_id UUID NOT NULL,
     container_id UUID NULL,
     outcome TEXT NOT NULL CHECK (outcome IN ('started','succeeded','failed','cancelled','phase_done','plan_done')),
     cause TEXT NOT NULL DEFAULT '', execution_link_id UUID NULL, error_message TEXT NOT NULL DEFAULT '',
     event_id UUID NOT NULL, occurred_at TIMESTAMPTZ NOT NULL);
   CREATE INDEX idx_task_run_outcomes_request ON request.task_run_outcomes (tenant_id, request_id, occurred_at);
   CREATE INDEX idx_task_run_outcomes_task ON request.task_run_outcomes (tenant_id, task_id, outcome);
   ```
   Bật RLS `tenant_isolation` cho cả hai bảng (cùng mẫu `request` schema của CR-REQ-002). Schema Postgres là `request`: đối chiếu với `0001_init` thật của CR-REQ-001.
2. Up MySQL tương đương; `outcome VARCHAR(12)` với CHECK (MySQL ≥ 8.0.16).
3. Down: `DROP TABLE` theo thứ tự ngược.
4. Domain: `PhaseStart`, `TaskRunOutcome`, kiểu `Outcome` (hằng sáu giá trị).
5. Cổng ở `ports.go`: `PhaseStartRepository.TryStart(ctx, PhaseStart) (inserted bool, err error)`; `TaskRunOutcomeRepository.Insert(ctx, TaskRunOutcome) error`, `CountFailed(ctx, tenantID, taskID string) (int, error)`, `LatestFailed(ctx, tenantID string, taskIDs []string) (map[string]TaskRunOutcome, error)` (cho SOL-015 `last_error`), `LastEventAt(ctx, tenantID, requestID string) (time.Time, error)` (cho đối soát).
6. Adapter hai dialect: `TryStart` Postgres `ON CONFLICT (tenant_id, phase_task_id) DO NOTHING` kiểm `RowsAffected`; MySQL `INSERT IGNORE` kiểm `RowsAffected`. `Insert` với `event_id` không unique (một sự kiện có thể sinh một dòng); khử trùng ở `processed_events`, không ở bảng này.
7. `LatestFailed`: Postgres `DISTINCT ON (task_id) ... ORDER BY task_id, occurred_at DESC`; MySQL `ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY occurred_at DESC)`.

## Kiểm thử

- Hợp đồng schema: test đọc `information_schema` hai DB (cột, CHECK, khoá chính, chỉ mục).
- Integration hai dialect: `TestPhaseStarts_TryStart_Race_8Goroutines_OneInserted`, `TestPhaseStarts_TenantIsolation`, `TestTaskRunOutcomes_CountFailed`, `_LatestFailed_PicksNewest`, `_CheckRejectsUnknownOutcome`, `_LastEventAt`.
- Chu trình up, down, up của migration.
- `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/request-service/internal/adapter/... -run 'PhaseStart|TaskRunOutcome' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Migration up/down/up sạch, cùng số ở hai thư mục.
- [x] `TryStart` đua 8 goroutine chỉ một lần chèn thành công.
- [x] Mọi truy vấn lọc `tenant_id`; RLS Postgres chặn chéo tenant.
- [x] Không FK sang `task-service`.
- [x] Không tên file `helpers`/`utils`/`common`/`misc`.

## Rủi ro và lưu ý

- Số migration và tên schema Postgres (`request`) cần đối chiếu CR-REQ-001/002 đã merge.
- `task_run_outcomes` tăng không giới hạn theo số lần chạy; chưa có chính sách dọn (ghi vào backlog).
- MySQL `INSERT IGNORE` nuốt cả lỗi khác (kiểu dữ liệu): kiểm `Warnings` hoặc dùng `ON DUPLICATE KEY UPDATE tenant_id = tenant_id`.
