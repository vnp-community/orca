# TASK-REQ-011-01: Migration 0015 (CHECK `task_type` thêm plan/phase) và 0016 (`request_id`, chỉ mục một Plan hoạt động)

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/{postgres,mysql}/0015_task_type_plan_phase.{up,down}.sql` (mới), `.../0016_task_request_id.{up,down}.sql` (mới)
**Depends on:** không (task đầu của feature)
**Status:** `[ ] TODO`

---

## Context

Đã `ls backend-go/services/task-service/migrations/postgres` và `.../mysql` ngày 2026-10-06: cả hai dialect dừng ở `0014_task_sources_site`, nên **0015 là số thật kế tiếp** và 0016 theo sau. Kiểm lại bằng `ls` ngay trước khi tạo file vì CR khác (026 đến 036 đang soạn) có thể chen vào.

- Postgres: CHECK `task_type` là inline ở `0003_task_fields_and_comments.up.sql` dòng 3 (`ADD COLUMN task_type TEXT NOT NULL DEFAULT 'task' CHECK (...)`), Postgres tự đặt tên `tasks_task_type_check`. `0003` đổi `tasks_status_check` theo kiểu DROP rồi ADD (dòng 18 đến 22): làm giống.
- MySQL: `0003_task_fields_and_comments.up.sql` dòng 26 `ADD CONSTRAINT tasks_task_type_check CHECK (...)`; cột `task_type VARCHAR(10)` (dòng 10) đủ cho `feature`, `plan`, `phase`. MySQL chỉ thực thi CHECK từ 8.0.16.
- Mẫu chỉ mục duy nhất với NULL ở MySQL: `mysql/0012_task_sources.up.sql` (generated column `project_key`).
- `task.tasks` đã bật RLS từ `0001`; không thêm policy.
- Migration của task-service là file thuần SQL, áp bằng `migrate` (xem `Makefile` mục `migrate-all`).

## Việc cần làm

1. `postgres/0015_task_type_plan_phase.up.sql`: `ALTER TABLE task.tasks DROP CONSTRAINT tasks_task_type_check;` rồi `ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic','plan','phase'));`.
2. `postgres/0015_...down.sql`: `UPDATE task.tasks SET task_type = 'epic' WHERE task_type IN ('plan','phase');` rồi DROP và ADD lại CHECK bốn giá trị. Comment đầu file: down làm mất phân biệt plan/phase/epic, chỉ dùng khi rollback toàn bộ v6.
3. `mysql/0015_...`: `ALTER TABLE tasks DROP CHECK tasks_task_type_check;` rồi `ADD CONSTRAINT` sáu giá trị; down tương ứng.
4. `postgres/0016_task_request_id.up.sql`:
   ```sql
   ALTER TABLE task.tasks ADD COLUMN request_id UUID;
   CREATE INDEX idx_tasks_request ON task.tasks (tenant_id, request_id) WHERE request_id IS NOT NULL;
   CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON task.tasks (tenant_id, request_id)
     WHERE task_type = 'plan' AND status <> 'cancelled';
   ```
   Down: DROP hai chỉ mục rồi DROP COLUMN.
5. `mysql/0016_task_request_id.up.sql`: `ADD COLUMN request_id CHAR(36) NULL`, `ADD COLUMN active_plan_request_id CHAR(36) GENERATED ALWAYS AS (CASE WHEN task_type = 'plan' AND status <> 'cancelled' THEN request_id END) STORED`; `CREATE INDEX idx_tasks_request ON tasks (tenant_id, request_id)`; `CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON tasks (tenant_id, active_plan_request_id)`. Comment: MySQL coi NULL là khác nhau nên cột sinh chỉ khác NULL với plan chưa huỷ.
6. Không FK, không backfill (hàng cũ `request_id` NULL).
7. Kiểm tên chỉ mục không trùng chỉ mục có sẵn (`grep -rn "idx_tasks_" migrations/`).

## Kiểm thử

- Test hợp đồng schema (file mới `internal/adapter/postgres/schema_plan_phase_test.go` và bản mysql, tag `integration`, theo mẫu `repository_test.go`): `TestMigration0015_AcceptsPlanPhase_RejectsUnknown`, `TestMigration0016_UniqueActivePlanPerRequest` (hai `plan` cùng `request_id` thì lần hai lỗi; sau khi `cancelled` tạo lại được; hai task `task` cùng `request_id` không lỗi).
- Chu trình up, down, up trên Postgres và MySQL:
  `migrate -path services/task-service/migrations/postgres -database "$DATABASE_DSN_TASK" up` rồi `down 2` rồi `up`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/task-service/internal/adapter/postgres/... ./services/task-service/internal/adapter/mysql/...` (cần Docker cho testcontainers; chưa chạy).

## Tiêu chí hoàn thành

- [ ] Bốn cặp file up/down tồn tại, số 0015 và 0016 ở cả hai thư mục.
- [ ] Up/down/up sạch trên Postgres và MySQL ≥ 8.0.16.
- [ ] Chèn `plan`, `phase` thành công; `'xyz'` bị từ chối.
- [ ] Hai plan chưa huỷ cùng `(tenant_id, request_id)` bị chỉ mục duy nhất từ chối ở cả hai DB.
- [ ] Không file nào tên `helpers`, `utils`, `common`, `misc`.

## Rủi ro và lưu ý

- Generated column `STORED` với `CASE` chưa chạy trên MySQL/TiDB mục tiêu; nếu TiDB không hỗ trợ, đổi sang kiểm ở tầng ứng dụng (khoá `SELECT ... FOR UPDATE` trên `request_id`) và ghi vào Câu hỏi mở của solution.
- Down mất thông tin loại container; không dùng ở production sau khi đã có plan thật.
- Số migration có thể va; đổi số chứ không đổi nội dung.
