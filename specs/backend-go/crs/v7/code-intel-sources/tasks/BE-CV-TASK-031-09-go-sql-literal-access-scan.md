# BE-CV-TASK-031-09: Quét literal SQL trong adapter Go để dựng `accessedBy` (bảng → hàm repository)

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gosqlscan/scan_table_access.go` (mới), `.../gosqlscan/sql_literal_extraction.go` (mới), `.../gosqlscan/scan_table_access_test.go` (mới), `.../gosqlscan/testdata/*.go.txt` (mới)
**Depends on:** BE-CV-TASK-030-05, BE-CV-TASK-031-06
**Status:** [ ] TODO

---

## Context

SOL-031-erd mục 2.D. Dùng `go/parser` (thư viện chuẩn, không dependency mới). Chỉ `internal/adapter/{postgres,mysql}/*.go` không-test (`api-gateway`, `git-gateway-service` không có). Kiểm chứng sơ bộ của CR (grep): `infra-fleet-service` có 19/20 bảng xuất hiện dạng `infra.<bảng>`; `provider_registry_entries` không truy cập.

## Việc cần làm

1. `sql_literal_extraction.go`: `ExtractLiterals(file *ast.File, fset) []SqlLiteral{Text, Pos, EnclosingFunc, FormatVerbs bool}` duyệt `*ast.BasicLit` (chuỗi interpreted/raw) và đối số đầu của `fmt.Sprintf`/`Errorf`; hàm bao `*ast.FuncDecl` (receiver + tên).
2. `scan_table_access.go`: áp mẫu `\b(FROM|JOIN|INTO|UPDATE|DELETE\s+FROM|TRUNCATE(\s+TABLE)?)\s+([a-z_][a-z0-9_]*\.)?([a-z_][a-z0-9_]*)` (không phân biệt hoa/thường) trên mỗi literal, **lọc theo tập bảng Catalog** của service; sinh `TableAccess{Table, Op, Symbol, Confidence}`; `op`: `FROM|JOIN`→`read`; `INSERT INTO|UPDATE|DELETE FROM|TRUNCATE`→`write`; `INSERT … SELECT` hoặc hàm có cả hai trên cùng bảng → `readwrite`.
3. `SymbolRef`: `method:<relPath>:<Receiver>.<Name>` / `function:<relPath>:<Name>`; `startLine/endLine` từ `fset`; dùng hàm dựng khoá chung của BE-CV-SOL-020 nếu đã có (đọc `internal/domain` trước khi viết; nếu chưa có, viết ở `internal/domain/symbol_key.go` theo PQ-20 và báo chủ SOL-020).
4. Độ tin cậy: `0.9` khi có tiền tố schema khớp (Postgres), `0.6` khi chỉ tên trần; literal chứa `%s|%v` ngay sau `FROM|INTO|UPDATE` → `ParseWarning{DYNAMIC_SQL_SUSPECTED, file, line}`.
5. Bỏ `_test.go`, `migration_*_test.go`; không phân tích `usecase`.
6. Đầu vào là nội dung file đã đọc (qua cổng), không tự đọc đĩa.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/gosqlscan/...` (chưa chạy).
- Bảng ca (trong `testdata` đuôi `.go.txt` để không bị build): `Sprintf("SELECT … FROM %s.dev_servers")` (động), raw string nhiều dòng, bí danh `FROM infra.dev_servers ds JOIN infra.terminal_sessions ts`, `INSERT … SELECT`, từ khoá lọt vào chuỗi log (`"failed to update ..."` không khớp bảng → không sinh), literal cùng tên bảng ở hai hàm.
- Với mẫu cắt từ `infra-fleet-service/internal/adapter/postgres/repository.go`: có `read` cho `dev_servers`.
- Hai dialect: chạy cùng bộ ca với bí danh MySQL (tên trần) và Postgres (có schema), kiểm `confidence`.

## Tiêu chí hoàn thành

- [ ] Không dương tính giả với chuỗi không phải SQL trong ca kiểm.
- [ ] `accessedBy` rỗng với bảng không truy cập.
- [ ] Không panic với file không parse được (trả cảnh báo `GO_PARSE_ERROR`).

## Rủi ro và lưu ý

- SQL dựng động/nối chuỗi bị sót; độ chính xác chưa đo (CR §6).
- Quét hai thư mục × ~18 file: chi phí đọc qua RTT SSH đi qua cổng (cache theo oid).
