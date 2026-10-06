# BE-CV-TASK-031-02: Tokenizer và bộ tách câu lệnh SQL (comment, literal, dollar-quote, ngoặc)

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/sqlmigration/tokenizer.go` (mới), `.../statement_splitter.go` (mới), `.../tokenizer_test.go`, `.../statement_splitter_test.go` (mới)
**Depends on:** BE-CV-TASK-031-01
**Status:** [ ] TODO

---

## Context

SOL-031 mục 2.C (đoạn "Tách câu lệnh"). Nền cho mọi phần sau: nếu tách sai (comment chứa `;`, `$$` lồng) thì mọi bảng sai.

## Việc cần làm

1. `Tokenize(src string) ([]Token, []ParseWarning)`: token loại `Ident`, `QuotedIdent` (`"x"`, backtick), `String` (`'…'` với `''`), `DollarString` (`$tag$…$tag$`, tag rỗng được), `Number`, `Punct`, `LineComment`, `BlockComment`, `Newline`; mỗi token có `Line`, `Col`. `Ident` so khớp không phân biệt hoa/thường nhưng giữ nguyên bản gốc.
2. `Split(tokens) []Statement`: bỏ comment **trước**; tách ở `;` khi độ sâu ngoặc = 0 và không trong literal/dollar-quote; mỗi `Statement{StartLine, Tokens}`; câu lệnh rỗng bị bỏ; câu lệnh cuối không có `;` vẫn được giữ.
3. Chịu lỗi: literal/dollar-quote không đóng đến hết file → `ParseWarning{code:"UNTERMINATED_LITERAL"}` và coi phần còn lại là một câu lệnh lạ; không panic, không vòng lặp vô hạn (kiểm bằng test có timeout).
4. Giới hạn: tổng token ≤ 200 000 mỗi file (hằng cấu hình); vượt → cảnh báo `FILE_TOO_COMPLEX` và dừng, trả phần đã tách.
5. Không phụ thuộc gói ngoài thư viện chuẩn.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/sqlmigration/... -run 'Tokenizer|Splitter'` (chưa chạy).
- Bảng ca: `-- a; b` rồi `SELECT 1;`; `/* ; */`; `'it''s;'`; `$$ a; b $$`; `$p$ x $p$` lồng trong `$$ … $$` (tag khác nhau); `DO $$ … END $$;` một câu lệnh; ngoặc lồng `CHECK (a IN (1,2); …)` không tách giữa ngoặc; comment cuối file không có newline.
- Chạy trên toàn bộ corpus (031-01): tổng số câu lệnh khớp số `;` mức 0 đếm độc lập bằng script test.
- `go test -fuzz=FuzzTokenize -fuzztime=30s` (tuỳ chọn): không panic.

## Tiêu chí hoàn thành

- [ ] Mọi ca bảng đạt; fuzz ngắn sạch.
- [ ] Không panic/treo với đầu vào hỏng.
- [ ] Số dòng của câu lệnh đúng (dùng cho `ParseWarning.Line`).

## Rủi ro và lưu ý

- MySQL cho phép `--` phải theo sau bởi khoảng trắng, `#` là comment; nếu migration MySQL dùng `#` thì xử lý (chưa quét; kiểm bằng corpus).
- Backslash escape trong literal MySQL (`'it\'s'`): chưa thấy trong migration; hỗ trợ bằng cờ dialect, mặc định tắt cho Postgres.
