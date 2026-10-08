# TASK-REQ-026-02: Migration `NNNN_solution_engines` và repository `project_engine_settings`, `openspec_changes`

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `migrations/postgres/NNNN_solution_engines.{up,down}.sql`, `migrations/mysql/NNNN_solution_engines.{up,down}.sql`, `internal/usecase/ports.go` (sửa), `internal/adapter/postgres/{project_engine_settings_repository,openspec_change_repository}.go`, `internal/adapter/mysql/{project_engine_settings_repository,openspec_change_repository}.go` và test (tất cả mới trừ `ports.go`)
**Depends on:** TASK-REQ-026-01, TASK-REQ-007-01 (bảng `analysis_runs`), TASK-REQ-002-01 (bảng `requests`), TASK-REQ-001-04 (`TxRunner`, executor trong ctx)
**Status:** [ ] TODO

---

## Context

**Số migration không tự cấp:** thư mục `request-service/migrations` chưa có lúc soạn, và số của CR-REQ-001/002/004/006 đang chồng nhau (README `request-artifact-model`). Bước 1 bắt buộc `ls backend-go/services/request-service/migrations/postgres migrations/mysql` rồi lấy số kế tiếp lớn nhất, **cùng một số cho hai dialect**, ghi `NNNN` ở mô tả PR.

Mẫu để theo: Postgres dùng schema `request.` và RLS `NULLIF(current_setting('app.tenant_id', true), '')::uuid` + `FORCE` như BE-REQ-SOL-002 mục A (sửa lỗi policy inert của `task-service`); MySQL không có schema prefix, `CHAR(36)` cho id, `TIMESTAMP(6)`, mọi truy vấn có `tenant_id`. Đã đọc `task-service/migrations/mysql/0012_task_sources.up.sql` (cách đặt tên ràng buộc `..._check`, `ENGINE=InnoDB`) và `0013_execution_leases.up.sql` (ALTER thêm cột hai dialect).

Chỗ khác solution: nếu migration `NNNN_analysis_runs` của TASK-REQ-007-01 **chưa merge**, gộp hai thay đổi trên `analysis_runs` (cột `engine`, giá trị `agent_proposal` của CHECK `mode`) vào chính file đó thay vì ALTER ở đây; ghi quyết định trong PR.

## Việc cần làm

1. Đọc thư mục migrations, chọn `NNNN`, tạo bốn file `NNNN_solution_engines.{up,down}.sql` cho từng dialect.
2. Postgres `up`: tạo `request.project_engine_settings` (PK `(tenant_id, project_id)`, `solution_engine` CHECK `native|openspec`, `openspec_min_version TEXT NULL`, `updated_by UUID NOT NULL`, `updated_at`, `version BIGINT NOT NULL DEFAULT 1`) và `request.openspec_changes` (cột như SOL-026 mục C, `UNIQUE (tenant_id, request_id)`, chỉ mục `(tenant_id, status, tasks_sync_state)` cho vòng quét thử lại).
   - `ALTER TABLE request.requests ADD COLUMN solution_engine TEXT NULL CHECK (...)`.
   - `ALTER TABLE request.analysis_runs ADD COLUMN engine TEXT NOT NULL DEFAULT 'native'`.
   - đổi CHECK `mode`: lấy tên ràng buộc thật bằng `SELECT conname FROM pg_constraint WHERE conrelid='request.analysis_runs'::regclass AND contype='c'` trên DB dev (ghi tên vào comment), `DROP CONSTRAINT`, `ADD CONSTRAINT analysis_runs_mode_check CHECK (mode IN ('complete','agent_readonly','agent_proposal'))`.
   - Bật `ENABLE` + `FORCE ROW LEVEL SECURITY` và policy `tenant_isolation` cho hai bảng mới.
3. MySQL `up`: cùng cấu trúc;
   - `solution_engine VARCHAR(10)`, `change_id VARCHAR(80)`, `branch VARCHAR(255)`, `status VARCHAR(16)`, `tasks_sync_state VARCHAR(16)`
   - `ALTER ... DROP CHECK <tên>` rồi `ADD CONSTRAINT analysis_runs_mode_check CHECK (...)` (tên lấy từ `information_schema.TABLE_CONSTRAINTS`; ghi chú CHECK chỉ thực thi từ 8.0.16).
4. `down` cả hai dialect: `UPDATE analysis_runs SET mode='complete' WHERE mode='agent_proposal'` trước khi khôi phục CHECK;
   - xoá cột `engine`, `requests.solution_engine`
   - `DROP TABLE openspec_changes, project_engine_settings`.
   - Ghi comment "chỉ dùng khi rollback toàn bộ v6".
5. `ports.go` (sửa): thêm `EngineSettingsRepository{Get(ctx, projectID string) (domain.ProjectEngineSettings, bool, error); Upsert(ctx, s domain.ProjectEngineSettings, expectedVersion int64) (domain.ProjectEngineSettings, error)}` và `OpenSpecChangeRepository{GetByRequest(ctx, requestID string) (domain.OpenSpecChange, bool, error); Upsert(ctx, c domain.OpenSpecChange) (domain.OpenSpecChange, error); UpdateSync(ctx, id string, state domain.SyncState, digest string, at *time.Time, expectedVersion int64) error; ListPendingSync(ctx, limit int) ([]domain.OpenSpecChange, error)}`.
6. Hai adapter mỗi dialect, dùng executor trong ctx (BE-REQ-SOL-001 mục D): `Upsert` settings là `INSERT ... ON CONFLICT (tenant_id, project_id) DO UPDATE ... WHERE version = $n` (Postgres) và `INSERT ... ON DUPLICATE KEY UPDATE` kèm kiểm `version` bằng `UPDATE ... WHERE version=?` rồi đọc lại (MySQL);
   - lệch version trả `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT`.
   - `OpenSpecChange.Upsert` theo `UNIQUE (tenant_id, request_id)`: tạo lần đầu `preparing`, gọi lặp trả bản hiện có (không đổi `change_id` và `branch`).
7. Thêm trường `SolutionEngine *EngineName` vào `domain.Request` (cột nullable, NULL ánh xạ thành nil) và đọc nó ở `Get`, `GetByNumber`, `List`.
   - Thêm vào cổng `RequestRepository` phương thức riêng `UpdateSolutionEngine(ctx context.Context, id string, name domain.EngineName, expectedVersion int64) error` (CAS theo `version`).
   - `Update` chung **bỏ qua** cột này để không ghi đè giá trị đã ghim.
   - Chỉ `PinSolutionEngine` (task 026-05) và `engine_override` được gọi `UpdateSolutionEngine`.
   - Ghi chú trong comment và thêm test chốt chặn (`go/parser`) nếu cần.

## Kiểm thử

- Hợp đồng schema (mở rộng `schema_contract_test.go` của TASK-REQ-002-06): đọc `information_schema` hai DB, so tên bảng, tên cột, tính NULL, kiểu của ba thay đổi mới.
- `TestEngineSettingsRepository_Contract` (một bộ kịch bản cho hai adapter): upsert lần đầu, upsert đúng version, upsert sai version, tenant A không đọc được dòng B, project khác không lẫn.
- `TestOpenSpecChangeRepository_Contract`: Upsert idempotent (`change_id` không đổi)
- `UNIQUE (tenant_id, request_id)` chặn 2 dòng, `UpdateSync` CAS, `ListPendingSync` chỉ trả `pending|failed`, giới hạn `limit`.
- `TestAnalysisRunsMode_AcceptsAgentProposal` và `_RejectsUnknownMode` ở cả hai DB.
- `TestMigration_UpDownUp` (lên, xuống, lên) hai dialect; sau `down` các dòng `agent_proposal` đã thành `complete`.
- Postgres RLS: role `NOSUPERUSER NOBYPASSRLS`, SQL trực tiếp không thấy dòng tenant khác (hai bảng mới).
- Lệnh: `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/request-service/internal/adapter/... -run 'EngineSettings|OpenSpecChange|AnalysisRunsMode|Migration'` (cần Postgres 14+ và MySQL 8.0.16+; biến kết nối theo TASK-REQ-002-06).

## Tiêu chí hoàn thành

- [ ] Hai dialect cùng số `NNNN`, `up` và `down` chạy sạch; PR ghi số đã chọn và lý do (gộp hoặc tách với `analysis_runs`).
- [ ] CHECK `mode` nhận `agent_proposal` và từ chối giá trị khác ở cả hai DB.
- [ ] Hai bảng mới có `tenant_id` trong mọi truy vấn; Postgres có RLS thật (test role không bypass).
- [ ] Bộ test hợp đồng chạy xanh cho cả hai adapter bằng cùng một kịch bản.
- [ ] `requests.solution_engine` đọc ghi qua `RequestRepository` và không đổi kết quả test cũ của TASK-REQ-002-06.

## Rủi ro và lưu ý

- `DROP CHECK` MySQL cần đúng tên; tên sai làm migration lỗi giữa chừng và MySQL không có DDL transaction, nên chuẩn bị `down` thủ công và thử trên DB trống trước.
- MySQL cũ hơn 8.0.16 phân tích rồi bỏ qua CHECK: `agent_proposal` khi đó không bị chặn (xem rủi ro SOL-002).
- `branch` và `change_id` có thể chứa tiếng Việt đã bỏ dấu, nhưng cột MySQL nên dùng `utf8mb4` mặc định của dự án; kiểm trong `0001_init`.
- Hai CR khác cùng sửa `analysis_runs` (SOL-007, 008); đọc bản mới nhất của file migration của chúng trước khi tạo ràng buộc để không đè tên.
- Các cột `tenant_id` của hai bảng mới phải nằm trong mọi chỉ mục tra cứu theo Request, để hai tenant không tranh khoá.
