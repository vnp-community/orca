# BE-CV-TASK-038-09: Trích chuỗi SQL từ mã Go bằng `go/parser` (nối `+`, `Sprintf`, `const`, `enclosingFunc`, chú thích cho phép)

**From Solution:** BE-CV-SOL-038-static-tenant-filter-rule
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gosqlscan/{sql_string_extraction.go, enclosing_function.go, allow_comment_directive.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-038-08
**Status:** [ ] TODO

---

## Context

Solution 2.B. Đầu vào là `[]byte` nội dung tệp (do cổng 030 đọc) nên test offline. Chỉ phân tích tĩnh; không thực thi.

## Việc cần làm

1. `sql_string_extraction.go`: `ExtractSQL(content []byte, file string) ([]SQLUse, []Skip, error)`: `parser.ParseFile` với `parser.ParseComments`; duyệt `ast.Inspect`; giải `BasicLit` chuỗi, `BinaryExpr +` giữa literal/`Ident` trỏ tới `const`/`var` **cùng tệp** (đệ quy ≤ 8 cấp), đối số đầu `fmt.Sprintf`; chỉ giữ chuỗi bắt đầu (sau comment `--`/khoảng trắng) bằng `SELECT|UPDATE|DELETE|INSERT|WITH` không phân biệt hoa thường. Gọi `strings.Builder.WriteString`/`bytes.Buffer` dựng SQL ⇒ `Skip{Reason:"dynamic", Func, Line}` (không phát `SQLUse`). Chuẩn hoá: lowercase, gộp khoảng trắng, `$n`/`?` ⇒ `?`; cắt ≤ 4 KiB.
2. `enclosing_function.go`: `EnclosingFunc(file *ast.File, pos token.Pos) string` trả `Recv.Name` hoặc `Name`; closure trong hàm ⇒ tên hàm ngoài cùng; ngoài hàm (`var` cấp gói) ⇒ `"<package>"`.
3. `allow_comment_directive.go`: nhận `// codeintel:allow <rule> <lý do>` ở dòng liền trước câu lệnh hoặc liền trước khai báo hàm; `Allow{Rule, Reason, Valid bool}` với `Valid` khi lý do ≥ 10 ký tự (đếm rune); không hợp lệ ⇒ trả kèm `Finding` gợi ý `sql.allow-without-reason` (`info`) cho bộ phát hiện dùng.
4. Giới hạn: tệp ≤ 1 MiB; lỗi cú pháp ⇒ trả lỗi cho tệp đó (caller gom), không panic.
5. Kết quả sắp theo `(line, col)`; xác định.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/gosqlscan/...` trên `testdata/sqltenant/go`: literal; nối `+` với `const` cùng tệp (đúng ca `… RETURNING `+cols); `Sprintf`; `strings.Builder` ⇒ `Skip dynamic`; CTE `WITH`; comment `-- x` đứng trước `SELECT`; chuỗi không phải SQL (log) bị bỏ; allow hợp lệ/không lý do/9 ký tự/10 ký tự; `EnclosingFunc` cho method, closure, cấp gói; tệp lỗi cú pháp; 100 lần giống nhau.

## Tiêu chí hoàn thành

- [ ] Mọi dạng ở bước 1 có test; mã động được đếm, không phát SQL.
- [ ] `enclosingFunc` ổn định.

## Rủi ro và lưu ý

- `const` khai báo ở tệp khác cùng package không giải được (chỉ cùng tệp, theo CR) ⇒ `Skip{Reason:"cross_file_const"}`.
