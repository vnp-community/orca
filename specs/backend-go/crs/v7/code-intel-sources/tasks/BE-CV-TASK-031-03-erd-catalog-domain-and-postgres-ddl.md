# BE-CV-TASK-031-03: Mô hình `Catalog` và DDL Postgres (CREATE/ALTER/DROP/INDEX/POLICY/COMMENT)

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/erd/catalog.go` (mới), `.../domain/erd/canonical_type.go` (mới), `.../domain/erd/parse_warning.go` (mới), `.../adapter/sqlmigration/ddl_postgres.go` (mới), `.../adapter/sqlmigration/column_definition.go` (mới), `.../adapter/sqlmigration/implicit_names.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-031-02
**Status:** [ ] TODO

---

## Context

SOL-031 mục 2.B–2.D. Postgres trước vì có nhiều tính năng nhất (partial index, biểu thức, RLS, `DROP CONSTRAINT` theo tên ngầm).

## Việc cần làm

1. `catalog.go`: kiểu miền theo SOL-031 2.B; `Catalog.Apply(file, stmts)` thuần, theo thứ tự; ghi `FirstMigration`/`LastMigration` mỗi bảng (tên file); mọi câu lệnh lạ → `UnsupportedStatement` + `Degraded` bảng liên quan nếu xác định được.
2. `canonical_type.go`: ánh xạ `uuid|text|timestamp|json|bool|int|bigint|float|bytes|numeric|date|other` từ kiểu Postgres (`UUID`, `TIMESTAMPTZ`, `JSONB`, `BIGSERIAL`, `DOUBLE PRECISION`, `BYTEA`, `NUMERIC(p,s)`, mảng `[]` → `other`); giữ `type` gốc.
3. `column_definition.go`: phân tích một định nghĩa cột (kiểu kèm tham số, `NOT NULL`, `DEFAULT <biểu thức cân ngoặc/hàm>`, `PRIMARY KEY`, `UNIQUE`, `CHECK (…)`, `REFERENCES`, `GENERATED …`).
4. `ddl_postgres.go`: `CREATE SCHEMA`, `CREATE EXTENSION`, `CREATE TABLE [IF NOT EXISTS]`, `ALTER TABLE` nhiều mệnh đề (`ADD [COLUMN] [IF NOT EXISTS]`, `DROP COLUMN`, `ALTER COLUMN … TYPE|SET|DROP DEFAULT|NOT NULL`, `ADD CONSTRAINT`, `DROP CONSTRAINT [IF EXISTS]`, `ENABLE|FORCE ROW LEVEL SECURITY`, `RENAME`), `CREATE [UNIQUE] INDEX [CONCURRENTLY] [IF NOT EXISTS] … [USING m] (cột|biểu thức) [INCLUDE] [WHERE …]`, `DROP INDEX [IF EXISTS] [schema.]n`, `DROP TABLE`, `CREATE POLICY`, `COMMENT ON`.
5. `implicit_names.go`: sinh tên `<t>_<c>_fkey`, `<t>_<c>_check`, `<t>_pkey`, `<t>_<c>_key` khi ràng buộc không có tên, để `DROP CONSTRAINT terminal_sessions_connection_id_fkey` (infra `0034`) gỡ đúng ràng buộc.
6. Bảng không có tiền tố schema → dùng schema mặc định của service (tham số `Apply`) + cảnh báo `UNQUALIFIED_TABLE`.
7. Không đọc file, không gọi mạng; chỉ nhận tokens/statements.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/erd/... ./services/code-intel-service/internal/adapter/sqlmigration/... -run 'Postgres|Catalog|Implicit'` (chưa chạy).
- Từng mệnh đề: bảng ca đầu vào/đầu ra; ALTER nhiều mệnh đề có comment xen (đã bỏ ở bước tách); `IF NOT EXISTS` khi bảng/cột đã có → bỏ qua; `DROP INDEX task.idx_task_sources_unique` rồi `CREATE UNIQUE INDEX` có `COALESCE(...)::uuid` (task/0014): trạng thái cuối đúng; `0034`: ràng buộc ngầm bị gỡ.
- Golden: `Catalog` JSON của `infra-fleet-service` Postgres từ corpus (đặt ở `testdata/golden/`), cập nhật có chủ đích bằng cờ `-update`.

## Tiêu chí hoàn thành

- [ ] `infra-fleet-service` Postgres cho đủ bảng/cột/ràng buộc kỳ vọng ở SOL-031 mục 6.
- [ ] Mọi câu lệnh lạ không panic và không đổi trạng thái bảng.
- [ ] Không phụ thuộc ngoài thư viện chuẩn.

## Rủi ro và lưu ý

- Biểu thức `CHECK`/`WHERE` giữ chuỗi thô đã chuẩn hoá khoảng trắng (không dựng cây biểu thức).
- Tên enum/kiểu người dùng (`CREATE TYPE`) hiện chưa có trong repo (CR); nếu xuất hiện → cảnh báo.
