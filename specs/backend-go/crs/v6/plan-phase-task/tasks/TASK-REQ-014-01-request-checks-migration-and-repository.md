# TASK-REQ-014-01: Migration `request_checks` và repository append-only hai dialect

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/migrations/{postgres,mysql}/NNNN_request_checks.{up,down}.sql` (mới), `internal/domain/request_check.go` (mới), `internal/usecase/ports.go`, `internal/adapter/postgres/request_checks.go`, `internal/adapter/mysql/request_checks.go` (mới), `*_integration_test.go` (mới)
**Depends on:** CR-REQ-001, 002 (module, schema `request`, RLS mẫu); số migration lấy theo quy tắc ở TASK-REQ-013-03
**Status:** `[ ] TODO`

---

## Context

- `request-service` chưa có code ngày 2026-10-06. Số migration chưa cố định (nhiều CR cùng dùng `0002`); `ls migrations/postgres` ngay trước khi tạo, lấy số lớn nhất cộng 1, hai dialect cùng số.
- Mẫu RLS và kiểm `tenant_id` ở mọi `WHERE` (MySQL): `task-service/migrations/postgres/0012_task_sources.up.sql` và `mysql/0012_task_sources.up.sql`.
- Append-only: không có `UPDATE`/`DELETE` ở repository; bản có hiệu lực là bản mới nhất theo `(request_id, kind)`.
- `task-service` không lưu đầu ra có cấu trúc nên số đo phải vào bảng riêng (SOL-014 mục 1).

## Việc cần làm

1. Up Postgres:
   ```sql
   CREATE TABLE request.request_checks (
     id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL,
     kind TEXT NOT NULL CHECK (kind IN ('perf_baseline','perf_after','tests_before','tests_after','security_recheck','ops_result')),
     status TEXT NOT NULL CHECK (status IN ('passed','failed')),
     metrics JSONB NOT NULL DEFAULT '{}'::jsonb, summary TEXT NOT NULL DEFAULT '',
     source TEXT NOT NULL CHECK (source IN ('agent','manual')),
     task_id UUID NULL, recorded_by UUID NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
   CREATE INDEX idx_request_checks_latest ON request.request_checks (tenant_id, request_id, kind, created_at DESC);
   ALTER TABLE request.request_checks ENABLE ROW LEVEL SECURITY;
   CREATE POLICY tenant_isolation ON request.request_checks USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
   ```
2. Up MySQL: `CHAR(36)`, `VARCHAR(30)`/`VARCHAR(10)`, `JSON`, `TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6)`, CHECK cùng giá trị; MySQL `JSON` không có `DEFAULT '{}'` literal ở mọi phiên bản: dùng `DEFAULT (JSON_OBJECT())` (MySQL ≥ 8.0.13) hoặc bắt buộc ứng dụng luôn gửi `{}`; chọn cách sau để tương thích rộng, comment rõ.
3. Down: `DROP TABLE request_checks`.
4. `domain/request_check.go`: `RequestCheck{ID, TenantID, RequestID, Kind CheckKind, Status CheckStatus, Metrics json.RawMessage, Summary, Source, TaskID, RecordedBy string, CreatedAt time.Time}`; hằng `CheckKind` sáu giá trị; `Latest(checks []RequestCheck, kind CheckKind) (RequestCheck, bool)` thuần Go.
5. Cổng: `RequestCheckRepository{ Append(ctx, RequestCheck) error; ListByRequest(ctx, tenantID, requestID string) ([]RequestCheck, error); Latest(ctx, tenantID, requestID string, kind CheckKind) (RequestCheck, bool, error) }`.
6. Adapter Postgres (`pgx`) và MySQL (`database/sql`): `created_at` dùng đồng hồ DB; `Append` sinh `id` ở ứng dụng; `ListByRequest` `ORDER BY created_at ASC`; `Latest` `ORDER BY created_at DESC LIMIT 1`. Không có `Update`/`Delete`.

## Kiểm thử

- Hợp đồng schema (`information_schema`): cột, CHECK, chỉ mục ở hai DB.
- Integration hai dialect: `TestRequestChecks_Append_LatestWins`, `_AppendOnly_NoUpdatePath` (kiểm interface không có hàm sửa), `_TenantIsolation` (Postgres RLS bằng role không phải superuser, MySQL bằng lọc `tenant_id`), `_CheckConstraintRejectsUnknownKind`, `_MetricsRoundTrip`.
- Up, down, up của migration.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... && go test -tags=integration ./services/request-service/internal/adapter/... -run RequestChecks -v` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Migration up/down/up sạch ở hai dialect, cùng số.
- [ ] Mọi CHECK từ chối giá trị sai.
- [ ] Bản mới nhất theo `(request_id, kind)` luôn là bản có hiệu lực; không có đường sửa/xoá.
- [ ] Tenant A không đọc được dòng của tenant B.

## Rủi ro và lưu ý

- Superuser Postgres bỏ qua RLS: test tenant phải chạy bằng role thường (xem BE-MCP-SOL-001 mục 1 điều 3 cho cách làm).
- `JSON DEFAULT` ở MySQL tuỳ phiên bản; nếu tắt default, mọi `INSERT` phải gửi `metrics`.
- Cột `metrics` có thể lớn: đặt trần ở RPC (task 02), không ở DB.
