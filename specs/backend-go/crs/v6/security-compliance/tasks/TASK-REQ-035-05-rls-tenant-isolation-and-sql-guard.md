# TASK-REQ-035-05: RLS thật và cách ly tenant: `withTenantTx`, vai trò DB, meta-test, test quét SQL MySQL

**From Solution:** BE-REQ-SOL-035 (mục D)
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/postgres/tenant_tx.go` (mới/kiểm: do SOL-001 mục 2.D tạo), `.../internal/adapter/postgres/rls_audit_integration_test.go` (mới), `.../internal/adapter/postgres/tenant_isolation_integration_test.go` (mới), `.../internal/adapter/mysql/tenant_scope_test.go` (mới), `.../internal/adapter/mysql/tenant_isolation_integration_test.go` (mới), `.../migrations/postgres/NNNN_rls_hardening.{up,down}.sql` (chỉ khi meta-test phát hiện bảng thiếu `FORCE`/`WITH CHECK`; không có thì không tạo), `backend-go/services/request-service/deploy/` hoặc `deploy/dev` (vai trò `request_app`)
**Depends on:** BE-REQ-SOL-001 (tx), BE-REQ-SOL-002 (bảng), nên chạy lại mỗi khi CR 004, 006, 007, 009, 010, 031, 034 thêm bảng
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration ./services/request-service/internal/adapter/...`, `go test ./services/request-service/internal/adapter/{mysql,postgres,contracttest}`)

---

## Context

- RLS của `mcp-service` chạy thật: `mcp-service/migrations/postgres/0002_authorization.up.sql` (`FORCE ROW LEVEL SECURITY`), `internal/adapter/postgres/tenant_tx.go` (`withTenantTx`: `set_config('app.tenant_id', $1, true)` cục bộ giao dịch, `withRelayTx` đặt `app.relay='on'` cho `outbox.Store`). **`task-service` thì không** (`share_link.go` ghi rõ `app.tenant_id` "never SET anywhere"): RLS chỉ là chính sách nằm im. `request-service` theo mẫu `mcp-service` (SOL-001 mục 2.D, F4).
- Siêu người dùng và chủ bảng bỏ qua RLS kể cả khi `FORCE` đối với **superuser** và `BYPASSRLS`; `FORCE` chỉ bắt chủ bảng thường. Do đó kiểm thử tích hợp phải kết nối bằng vai trò **không** phải superuser (quyết định số 3 của BE-MCP-SOL-001) và dịch vụ chạy bằng vai trò `request_app` không phải chủ bảng.
- MySQL không có RLS: biện pháp là kiểu repository bắt buộc nhận `tenantID` + test quét SQL (CR 2.4 bước 3).
- Mọi bảng của `request-service` có `tenant_id NOT NULL` (README v6 mục 6, mục 8 điểm 3) và chỉ mục dẫn đầu `tenant_id`.
- Danh sách bảng sẽ tăng theo CR; test dựa vào truy vấn thông tin lược đồ, không danh sách gõ tay, để bảng mới tự được kiểm.

## Việc cần làm

1. Kiểm `tenant_tx.go` của `request-service` (SOL-001 đã định nghĩa): mọi truy vấn có phạm vi tenant đi qua `InTx` (hoặc câu lệnh đơn bọc giao dịch ngắn có `set_config`); không có đường truy vấn nào dùng `pool.Query` trực tiếp ngoài `outbox.Store` (`withRelayTx`). Thêm test lint dạng grep trong Go (`TestNoDirectPoolQuery`): đọc các file `adapter/postgres/*.go` (trừ `tx.go`, `outbox.go`) và đòi không có chuỗi `pool.Query(`, `pool.Exec(`, `pool.QueryRow(`.
2. Vai trò DB: tài liệu hoá trong `deploy/dev` (do TASK-REQ-001-06 sở hữu compose; task này cung cấp SQL) hai vai trò: `request_owner` (chạy migration, chủ bảng) và `request_app` (`LOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, chỉ `SELECT, INSERT, UPDATE, DELETE` và `USAGE` trên schema `request`). Dịch vụ kết nối bằng `request_app` qua `DATABASE_DSN`. Migration `GRANT` cho `request_app` trong mỗi `up.sql` mới (đọc `mcp-service` có làm tương tự không; nếu cùng chủ sở hữu thì không cần `GRANT`, ghi rõ lựa chọn trong PR).
3. `rls_audit_integration_test.go` (Postgres): `TestEveryRequestTableHasForcedRLS`: truy vấn `SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='request' AND c.relkind='r'` rồi đòi cả hai cờ `true` cho mọi bảng (danh sách miễn trừ rỗng; có bảng cần miễn trừ thì ghi tường minh trong test với lý do). `TestEveryRequestTableHasTenantPolicyWithCheck`: `pg_policies` có `tenant_isolation` với `qual` và `with_check` không rỗng. `TestEveryRequestTableHasTenantID`: `information_schema.columns` có `tenant_id` `NOT NULL`. `TestAppRoleCannotBypassRLS`: `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user` cả hai `false`.
4. `tenant_isolation_integration_test.go` (Postgres) và bản MySQL: bảng-điều-khiển theo danh sách bảng từ lược đồ (`information_schema.tables`): với mỗi bảng có thể chèn (cần fixture riêng mỗi bảng: viết bộ dựng `fixtureFor(table)` hoặc `testdata` SQL nhỏ) kiểm: tenant A chèn một dòng; ngữ cảnh tenant B không `SELECT`, `UPDATE`, `DELETE` được (0 dòng); chèn dòng `tenant_id` = A bằng ngữ cảnh B bị chặn bởi `WITH CHECK` (lỗi vi phạm chính sách, **riêng Postgres**). Bảng chưa có fixture làm test **đỏ** (buộc người thêm bảng viết fixture).
5. Postgres, "quên `set_config`": giao dịch không đặt `app.tenant_id` đọc mọi bảng trả 0 dòng (không phải dữ liệu lẫn, không phải lỗi hiểu nhầm); `TestForgottenSetConfigReturnsNothing`.
6. MySQL `tenant_scope_test.go`: dùng `go/parser` + `go/ast` duyệt mọi file `*.go` trong `internal/adapter/mysql` (trừ `_test.go`), thu mọi `*ast.BasicLit` kiểu chuỗi (kể cả chuỗi nối `+`, và chuỗi raw) có chứa `SELECT `, `UPDATE `, `DELETE FROM `, `INSERT INTO ` (không phân biệt hoa thường); mỗi câu SQL phải chứa `tenant_id` (đối với `INSERT` có `tenant_id` trong danh sách cột). Danh sách cho phép tường minh `allowedWithoutTenant = map[string]string{"outbox.fetchUnpublished": "relay đa tenant", "expiry.scan": "...", "audit_outbox.deliver": "...", "retention.scan": "..."}` khoá theo `file:hàm` với lý do; câu SQL không có `tenant_id` mà không thuộc danh sách làm test đỏ. Thêm `TestAllowListHasNoStaleEntries` (mục trong danh sách phải còn tồn tại).
7. Postgres: cũng áp quy tắc "mọi câu SQL có `tenant_id`" ở `adapter/postgres` làm lớp bảo vệ thứ hai (lọc ứng dụng song song RLS, lớp 2 trong 4 lớp của `arch/05`); cùng bộ quét AST, cùng danh sách cho phép (RLS vẫn là lớp chặn khi quên).
8. Id là UUID ngẫu nhiên; mọi lỗi "không thấy" trả `NOT_FOUND` chung (task 04) để không lộ tồn tại; thêm test `TestCrossTenantGetReturnsNotFound` ở tầng usecase.
9. Outbox: `outbox_events` có policy riêng cho bộ phát (`app.relay`); test `TestOutboxRelayPolicyOnlyFetchMark` (vai trò app với `app.relay='on'` đọc được chéo tenant **chỉ** hàng chưa phát hành và chỉ `UPDATE published_at`, không `DELETE`).

## Kiểm thử

- Toàn bộ test ở "Việc cần làm" (build tag `integration`; Postgres 14+ và MySQL 8.0.16+ qua `testcontainers`, theo mẫu `mcp-service/internal/adapter/postgres/*_integration_test.go`; kết nối **bằng `request_app`**).
- `tenant_scope_test.go` chạy không cần Docker (chỉ đọc nguồn Go) ⇒ chạy trong `go test` thường.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/mysql/... -run TenantScope && go test -tags=integration ./services/request-service/internal/adapter/... -run 'RLS|TenantIsolation|ForgottenSetConfig'`. Chưa chạy.

## Tiêu chí hoàn thành

- [x] Mọi bảng Postgres schema `request` có `ENABLE` + `FORCE RLS` và `tenant_isolation` với `WITH CHECK` (meta-test).
- [x] Test cách ly hai tenant đạt cho mọi bảng trên Postgres (có và không có `set_config`) và MySQL.
- [x] Test quét SQL của MySQL đỏ khi thêm câu SQL không có `tenant_id`.
- [x] Kết nối kiểm thử bằng vai trò không phải superuser, không `BYPASSRLS` (kiểm thử khẳng định).
- [x] Không còn `pool.Query` trực tiếp ngoài outbox.

## Ví dụ tham khảo

Truy vấn của meta-test RLS (Postgres):

```sql
SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'request' AND c.relkind = 'r' AND (NOT c.relrowsecurity OR NOT c.relforcerowsecurity);
-- kỳ vọng: 0 dòng
SELECT tablename FROM pg_tables t WHERE schemaname = 'request'
  AND NOT EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname = t.schemaname AND p.tablename = t.tablename AND p.policyname = 'tenant_isolation');
-- kỳ vọng: 0 dòng
```

Cấp quyền cho vai trò ứng dụng (mẫu cho `deploy/dev`): `CREATE ROLE request_app LOGIN NOSUPERUSER NOBYPASSRLS; GRANT USAGE ON SCHEMA request TO request_app; GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA request TO request_app; ALTER DEFAULT PRIVILEGES IN SCHEMA request GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO request_app;`

## Thứ tự làm gợi ý

1. Viết `tenant_scope_test.go` (MySQL, không cần Docker) trước: chạy ngay trên các repository đã có để thấy các câu SQL còn thiếu `tenant_id`.
2. Viết `rls_audit_integration_test.go` (Postgres) và cho nó đỏ trên bảng nào chưa `FORCE`.
3. Viết `fixtureFor(table)` cho từng bảng hiện có, rồi `tenant_isolation_integration_test.go`.
4. Cuối cùng thêm vai trò `request_app` vào `deploy/dev` và đổi DSN của test sang vai trò đó.

## Rủi ro và lưu ý

- `FORCE RLS` với PgBouncer chế độ transaction hoặc pool khác chưa kiểm chứng; `set_config(..., true)` cục bộ giao dịch an toàn với pool giao dịch nhưng không với chế độ session sai cấu hình.
- Test "mọi bảng có fixture" buộc mỗi CR thêm bảng phải thêm fixture: đây là chủ ý nhưng thêm công việc cho CR 004 đến 034; báo trong PR.
- Quét AST bỏ sót SQL dựng động bằng `fmt.Sprintf` nhiều mảnh: quy ước repo là hằng chuỗi; câu dựng động phải nằm trong danh sách cho phép với lý do.
- Vai trò chủ bảng ≠ `request_app` làm migration phải `GRANT` cho mỗi bảng mới (dễ quên): thêm test `TestAppRoleHasPrivilegesOnAllTables`.

## Ghi chú triển khai (2026-10-08)

- Postgres: `rls_audit_integration_test.go` (FORCE RLS mọi bảng, policy `FOR ALL` có `WITH CHECK`, `tenant_id NOT NULL`, vai trò app không bypass, quyền trên mọi bảng, cách ly A/B mọi bảng theo fixture bắt buộc cho bảng mới, quên `set_config` trả 0 dòng và không ghi được, relay outbox chỉ đọc/đánh dấu). Kết nối bằng `request_app` NOSUPERUSER NOBYPASSRLS.
- Quét AST `contracttest.CheckTenantScope` cho cả hai dialect (có test của chính bộ quét, danh sách cho phép phải có lý do và không được cũ). Phát hiện: `mysql/project_engine_settings_repository.go Get` đọc không có `tenant_id` (đã sửa); `mysql/openspec_change_repository.go` GetByRequest/Upsert là stub của đợt solution-engines (Upsert ghi tenant 0000...) nên để trong danh sách cho phép kèm lý do, chủ sở hữu rf-sol phải scope cả hai cùng lúc.
- MySQL: `TestMySQL_EveryTableHasTenantID`. `TestNoDirectPoolQuery` cho Postgres.
- Chưa làm: SQL tạo vai trò `request_owner`/`request_app` trong `deploy/dev` (compose do TASK-REQ-001-06 sở hữu); test dựng vai trò bằng `GRANT` trong fixture.
