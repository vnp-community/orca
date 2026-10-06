# BE-CV-TASK-038-10: Phân tích câu lệnh SQL nhẹ và điều kiện báo thiếu `tenant_id` (hai dialect)

**From Solution:** BE-CV-SOL-038-static-tenant-filter-rule
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/tenantfilter/{sql_statement_analysis.go, tenant_filter_rule.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-09; BE-CV-SOL-031-sql-migration-parser (tokenizer dùng lại nếu xuất được)
**Status:** [ ] TODO

---

## Context

Solution 2.C. Hàm thuần. Mục tiêu là **không** làm parser SQL đầy đủ: chỉ cần loại câu lệnh, danh sách bảng, có/không định danh `tenant_id`, danh sách cột `INSERT`.

## Việc cần làm

1. `sql_statement_analysis.go`: `Analyze(normalized string, dialect Dialect) Statement{Kind, Tables []TableRef, HasTenantIdent bool, InsertColumns []string, FilterColumns []string}`: `Kind ∈ select|update|delete|insert|with`; `Tables` từ `FROM|JOIN|UPDATE|DELETE FROM|INSERT INTO`, bỏ tiền tố schema Postgres (`schema.table`→`table`), MySQL backtick; quét CTE (`WITH x AS (…)`) và subquery theo độ sâu ngoặc; `HasTenantIdent` theo **ranh giới định danh** (không khớp `tenant_identity`, `x_tenant_id`; có thể kèm bí danh `t.tenant_id`); `FilterColumns` = cột ở `WHERE`/`ON conflict target` (để phân loại khoá toàn cục); `INSERT … SELECT`: `InsertColumns` từ danh sách cột, ghi `HasTenantIdent` cả ở select-list. Câu lệnh không phân tích được ⇒ `Kind:"unknown"` (không báo).
2. `tenant_filter_rule.go`: `Evaluate(st Statement, ctx RuleContext) (Candidate, bool)`: `RuleContext{TenantTables map[string]bool, IsAllowlisted func(table string) bool}`; `sql.missing-tenant-filter` khi `select|update|delete|with` có bảng ∈ `TenantTables`, không bị allowlist, và `!HasTenantIdent`; `sql.insert-missing-tenant` khi `insert` vào bảng tenant mà `InsertColumns` không có `tenant_id` (và không phải select-list có `tenant_id`). Allowlist cố định: `outbox_events`, `processed_events`.
3. Hai dialect: placeholder `$n`/`?` đã chuẩn hoá ở TASK-038-09; khác biệt tiền tố schema/backtick xử lý ở bước 1; test chạy **cả hai** nhánh.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/tenantfilter/ -run 'Analyze|Evaluate'`: bảng ≥ 50 ca hai dialect: `SELECT … WHERE id = ?` trên bảng tenant ⇒ ứng viên; có `tenant_id = ?` ⇒ không; `tenant_identity` ⇒ ứng viên; JOIN hai bảng chỉ một bảng tenant; CTE; subquery có `tenant_id` ở trong nhưng ngoài thiếu (quyết định: toàn văn có định danh ⇒ không báo, theo CR; ghi test để chủ bộ biết giới hạn); `INSERT` có/thiếu cột; `INSERT … SELECT`; `ON CONFLICT`/`ON DUPLICATE KEY`; `outbox_events` không báo; câu lạ ⇒ `unknown`; tên bảng có schema Postgres; MySQL backtick; không panic với chuỗi rác.

## Tiêu chí hoàn thành

- [ ] Hai dialect đều có test; kết quả khớp bảng điều kiện solution 2.C.
- [ ] Giới hạn "toàn văn có `tenant_id` thì không báo" có test và ghi trong comment (nguồn bỏ sót đã biết).

## Rủi ro và lưu ý

- Quy tắc "toàn văn" bỏ sót truy vấn có `tenant_id` ở subquery nhưng thiếu ở truy vấn chính; chấp nhận theo CR (giảm ồn), ghi ở §7 nếu bộ vàng cho thấy recall thấp.
