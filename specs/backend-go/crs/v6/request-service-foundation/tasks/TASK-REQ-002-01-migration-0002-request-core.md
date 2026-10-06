# TASK-REQ-002-01: Migration `0002_request_core` (requests, counters, history, solutions, links, idempotency)

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `migrations/postgres/0002_request_core.up.sql`, `0002_request_core.down.sql`, `migrations/mysql/0002_request_core.up.sql`, `0002_request_core.down.sql` (mới)
**Depends on:** TASK-REQ-001-03 (đã có `0001_init`)
**Status:** [ ] TODO

---

## Context

**Xác định số migration:** chạy `ls services/request-service/migrations/postgres`; nếu chỉ có `0001_init.*` thì `0002` là số đúng, nếu đã có file khác (PR song song của CR-REQ-007, 009 dùng cùng thư mục) thì lấy số kế tiếp và đổi tên file, tên test, tham chiếu trong các task sau. Khoá mẫu: `task-service/migrations/postgres/0012_task_sources.up.sql` (RLS, unique) và `mysql/0014_task_sources_site.up.sql` (cột khoá `VARCHAR(255) NOT NULL DEFAULT ''`). RLS thật theo `mcp-service/migrations/postgres/0001_init.up.sql` (không theo `0012`, vì RLS ở đó không chạy). Đặc tả cột đầy đủ: CR-REQ-002 mục 2.1.

## Việc cần làm

1. Postgres `up`: tạo theo thứ tự `request.request_counters`, `request.requests`, `request.request_type_history`, `request.solutions`, `request.request_links`, `request.request_idempotency`. Kiểu: `UUID`, `TIMESTAMPTZ`, `JSONB`, `NUMERIC(4,3)`, `BIGINT`. Mọi `CHECK` ở SOL-002 mục A (`source_provider`, `type`, `type_source`, `size`, `urgency`, `confidence`, `status`, `returned_from_stage`) và `requests_backlog_stage CHECK ((status = 'request_backlog') = (returned_from_stage IS NOT NULL))`.
2. `request_idempotency`: PK `(tenant_id, source_provider, source_site, source_ref)`, `source_site TEXT NOT NULL DEFAULT ''`, `source_ref TEXT NOT NULL CHECK (source_ref <> '')`, `request_id UUID NOT NULL`, `created_at`.
3. `request_links`: PK `(tenant_id, parent_request_id, child_request_id)`, `reason` CHECK 4 giá trị, `CHECK (parent_request_id <> child_request_id)`, chỉ mục `(tenant_id, child_request_id)`.
4. Chỉ mục `requests`: `(tenant_id, status, updated_at DESC)`, `(tenant_id, project_id, status)`, `(tenant_id, plan_task_id)`, `(tenant_id, source_provider, source_site, source_ref)`, `(tenant_id, created_at DESC, id DESC)`; `UNIQUE (tenant_id, number)`. `request_type_history`: `(request_id, at)`. `solutions`: `(tenant_id, request_id, created_at)`.
5. RLS: khối `DO $$` `ENABLE` + `FORCE` + `tenant_isolation` (`NULLIF`) cho cả sáu bảng, y hệt `0001`.
6. Postgres `down`: `DROP TABLE` ngược thứ tự (không FK nên thứ tự tuỳ ý, vẫn giữ ngược).
7. MySQL: cùng sáu bảng, `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `DECIMAL(4,3)`, `VARCHAR(n)` cho `title(500)`, `source_provider(20)`, `source_ref(255)`, `source_site(255)`, `type(20)`, `status(40)`, các `CHECK` tương đương, chỉ mục `DESC` (8.0). Bảng `solutions.options JSON NOT NULL` không `DEFAULT`. Khoá `request_idempotency` ước 2264 byte utf8mb4, trong giới hạn 3072 của InnoDB; kiểm bằng `SHOW CREATE TABLE` sau up.
8. Không thêm `source_hints` (CR-REQ-004, `0003`) và `returned_category` (CR-REQ-006, `0004`).

## Kiểm thử

- `TestMigration_0002_UpDownUp` (từng dialect, `-tags=integration`): up, down, up lại không lỗi; `0001` còn nguyên sau down của `0002`.
- `TestChecks_RejectInvalidValues`: bảng test chèn trực tiếp từng giá trị sai cho mỗi `CHECK` (`type='foo'`, `status='bar'`, `confidence=1.5`, `urgency='x'`, `size='XL'`, `returned_from_stage='z'`, status `request_backlog` không có stage, stage có mà status khác, link cha trùng con, `source_ref=''`).
- `TestSchemaContract_InformationSchema`: so tên bảng, tên cột, `is_nullable` với danh sách mong đợi (CR-REQ-002 mục 2.1); dùng chung với TASK-REQ-002-06.
- MySQL: bỏ qua nhóm kiểm `CHECK` nếu `SELECT VERSION()` dưới 8.0.16 và ghi `t.Skip` có lý do; ghi rõ vào PR.
- Lệnh: `go test -tags=integration ./services/request-service/internal/adapter/... -run Migration -v`.

## Tiêu chí hoàn thành

- [ ] up/down/up sạch trên Postgres và MySQL.
- [ ] Mỗi `CHECK` từ chối giá trị sai.
- [ ] `FORCE RLS` bật cho cả sáu bảng (kiểm `pg_class.relforcerowsecurity`).
- [ ] Danh sách giá trị `CHECK` trùng hằng Go (kiểm ở TASK-REQ-002-06).

## Rủi ro và lưu ý

- Đổi khoá idempotency sau khi `0002` merge là migration phá (Q2 của SOL-002); chốt trước.
- MySQL cũ hơn 8.0.16 bỏ qua `CHECK` âm thầm; ràng buộc phải được `domain` kiểm lại (TASK-REQ-002-02).
- Chỉ mục `DESC` MySQL 8.0 trên TiDB chưa kiểm chứng.
