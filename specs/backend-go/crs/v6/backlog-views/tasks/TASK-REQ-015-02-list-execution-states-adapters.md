# TASK-REQ-015-02: Adapter Postgres và MySQL cho `ExecutionStateReader` kèm integration test

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `task-service`
**File:** `internal/adapter/postgres/execution_states.go` (mới), `internal/adapter/mysql/execution_states.go` (mới), `internal/adapter/postgres/execution_states_test.go` (mới, tag `integration`), `internal/adapter/mysql/execution_states_test.go` (mới), `cmd/server/main.go`
**Depends on:** TASK-REQ-015-01
**Status:** `[ ] TODO`

---

## Context

- Postgres: bảng `task.execution_links`, `task.task_edges`, `task.tasks`; kiểu `UUID`; `pgx` với `r.db.Query(ctx, sql, args...)` (xem `adapter/postgres/execution_links.go`).
- MySQL: bảng không tiền tố (`execution_links`, `task_edges`, `tasks`), `CHAR(36)`, `?` làm placeholder, `database/sql` (xem `adapter/mysql/execution_links.go`).
- Chỉ mục có sẵn: `idx_execution_links_task (task_id, started_at DESC)`; `task_edges_from_idx`.
- MySQL `ROW_NUMBER() OVER` cần ≥ 8.0; cùng nền với yêu cầu `SKIP LOCKED` của CR-DB-002; chưa kiểm chứng TiDB.
- `Repository` ở mỗi adapter đã thoả nhiều cổng (`TaskRepository`, `ExecutionLeaseRepository`...); thêm phương thức cho cổng mới trên cùng `Repository`.

## Việc cần làm

1. Postgres, ba truy vấn một lượt (hoặc một truy vấn gộp):
   - Link gần nhất: `SELECT DISTINCT ON (task_id) task_id, engine, status_mirror, started_at, completed_at FROM task.execution_links WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) ORDER BY task_id, started_at DESC, id DESC`.
   - Số lần lỗi: `SELECT task_id, count(*) FILTER (WHERE status_mirror = 'failed') FROM task.execution_links WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) GROUP BY task_id`.
   - Bị chặn bởi: `SELECT e.from_task_id, e.to_task_id FROM task.task_edges e JOIN task.tasks t ON t.id = e.to_task_id WHERE e.tenant_id = $1 AND e.edge_type = 'depends_on' AND e.from_task_id = ANY($2::uuid[]) AND t.status NOT IN ('done','cancelled')`.
2. MySQL: `IN (?,...)` theo số id; link gần nhất bằng `SELECT * FROM (SELECT task_id, engine, status_mirror, started_at, completed_at, ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY started_at DESC, id DESC) rn FROM execution_links WHERE tenant_id = ? AND task_id IN (...)) x WHERE rn = 1`; số lần lỗi `SUM(status_mirror = 'failed')` nhóm theo `task_id`; bị chặn bởi như Postgres với tên bảng không tiền tố.
3. Gộp kết quả theo `task_id` ở Go; trả `domain.ExecutionState` đủ mọi id (task không link: giá trị rỗng).
4. Nối vào `cmd/server/main.go` (cùng việc ở task 01).
5. Tên file đặt theo khái niệm (`execution_states`), không dùng `helpers`.

## Kiểm thử

Integration hai dialect (`go test -tags=integration`), dữ liệu: task A hai link (cũ `failed`, mới `completed`), task B ba link `failed` (cùng `started_at` để thử tie-break `id DESC`), task C không link, task D `depends_on` task E (`open`) và F (`done`):
- `TestExecutionStates_LastLink_PicksNewest`, `_FailedAttempts_Counts`, `_NoLink_Empty`, `_BlockedBy_ExcludesDone`, `_TenantIsolation`, `_ManyIDs_500`.
- Kết quả hai dialect phải giống nhau (dùng cùng bộ dữ liệu và assertion).
- `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/task-service/internal/adapter/... -run ExecutionStates -v` (cần Docker; chưa chạy).

## Tiêu chí hoàn thành

- [ ] `ListExecutionStates` đúng ở cả hai dialect: link gần nhất, đếm lỗi, `blocked_by`.
- [ ] Tie-break `started_at` bằng `id DESC` ổn định.
- [ ] 500 id chạy dưới ngưỡng hợp lý (ghi thời gian vào PR, chưa có mục tiêu cố định).
- [ ] Mọi truy vấn lọc `tenant_id`; không FK mới.

## Rủi ro và lưu ý

- `EXPLAIN` truy vấn `DISTINCT ON` trên bảng lớn: dựa chỉ mục `(task_id, started_at DESC)`; ghi kế hoạch trong PR, không bắt buộc test.
- MySQL < 8.0 không chạy được; thêm kiểm khả năng khi khởi động (`common/dbcapability`) nếu đã có cơ chế.
- `execution_links` tăng không giới hạn, chưa có dọn dẹp.
