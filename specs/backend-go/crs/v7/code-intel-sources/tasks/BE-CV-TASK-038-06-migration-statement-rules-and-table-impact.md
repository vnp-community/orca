# BE-CV-TASK-038-06: Phân loại câu lệnh migration (`sql.*`, `migration.*`) hai dialect và `TableImpact`

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/contractdiff/migration_statement_rules.go`, `table_impact.go` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-04; BE-CV-SOL-031-sql-migration-parser (câu lệnh đã tách), BE-CV-SOL-031-erd-model-and-access-scan (`accessedBy`)
**Status:** [x] DONE

---

## Context

Solution 2.D (migration), 2.E. arch/05: `DROP COLUMN`, `NOT NULL` không backfill là phá vỡ; mỗi migration có `down`. Hàm thuần; câu lệnh đã được 031 tách/phân tích (kiểu `Statement`); dialect do thư mục.

## Việc cần làm

1. `migration_statement_rules.go`: `ClassifyStatements(file, dialect, stmts []Statement) []SqlChange`: `sql.drop-table`, `sql.drop-column`, `sql.rename-table`, `sql.rename-column` ⇒ `breaking`; `sql.alter-column-type` (Postgres `ALTER COLUMN … TYPE`; MySQL `MODIFY|CHANGE`): `breaking` khi thu hẹp/đổi họ kiểu, `risky` khi nới (bảng họ kiểu `canonical_type` của 031); `sql.add-column-not-null` (không `DEFAULT`) `breaking`; `sql.add-constraint` `risky`; `sql.drop-policy`/`sql.disable-rls` `breaking` (+ tạo `Finding` `severity:"warning"` với `rule` cùng tên, `kind` theo PQ-06, `confidence:"high"`, `finding_key` theo `NewKey(rule, service::dialect::file::table)`); `sql.create-index` (ghi chú khoá bảng), `sql.add-column-nullable-or-default`, `sql.create-table` ⇒ `compatible`; câu lạ ⇒ `op:"other"`, `unknown`. `CREATE INDEX CONCURRENTLY` không phân tích thêm.
2. Mức độ tệp: `migration.modified-applied` (tệp `up` đã có ở base bị sửa) `risky`; `migration.dialect-parity` (tệp mới chỉ một dialect; **không** áp cho service chỉ Postgres như `mcp-service`: danh sách ngoại lệ lấy từ thư mục `migrations/` có một dialect ở base) `risky`; `migration.number-conflict` `breaking`; `migration.missing-down` `compatible`.
3. `sql.dialect-divergence`: cùng tên tệp hai dialect cho hai mức khác nhau ⇒ `risky` với `details{postgres, mysql}`; `SqlChange.dialect_only` khi chỉ một bên.
4. `table_impact.go`: `BuildTableImpacts(changes []SqlChange, accessors AccessorLookup) []TableImpact`: `AccessorLookup(service, table) []Accessor{Symbol, ColumnsInSQL, Evidence}`; `columns_referenced` = cột bị đổi/xoá mà SQL của accessor nhắc (so khớp **định danh nguyên vẹn**, không chuỗi con: `tenant_id` ≠ `tenant_identity`); `cross_service` khi accessor thuộc service khác; khi `AccessorLookup` không có ⇒ `accessors:[]` và không lỗi.
5. Ánh xạ sang `ContractChange` kind `sql-table|sql-column` (tên `service.table`/`service.table.column`).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/contractdiff/ -run 'Sql|Migration|TableImpact'`: **mỗi dòng bảng `sql.*` có test hai dialect** (Postgres và MySQL cùng ý nghĩa); `ADD COLUMN … NOT NULL` có/không default; sửa migration cũ; trùng số; thiếu một dialect (và `mcp-service` ngoại lệ); divergence; `DROP POLICY`/`DISABLE ROW LEVEL SECURITY` sinh finding; `columns_referenced` không khớp chuỗi con; ổn định hoán vị.
- Test `ma trận dialect`: CI chạy bộ test cho cả hai nhánh (hàm thuần, chạy mọi job).

## Tiêu chí hoàn thành

- [x] Tiêu chí migration và `TableImpact` của §9 đạt.
- [x] Finding `sql.*` có `finding_key` ổn định (dịch dòng không đổi).

## Rủi ro và lưu ý

- Parser nhẹ không phủ cú pháp lạ (`ENGINE=`, extension) ⇒ `other/unknown`; không bao giờ suy `compatible`.
