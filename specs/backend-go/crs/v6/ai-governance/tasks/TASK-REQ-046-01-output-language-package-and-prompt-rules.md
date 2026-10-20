# TASK-REQ-046-01: Gói `output_language` và chỉ thị ngôn ngữ trong mẫu prompt (JSON và OpenSpec)

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `backend-go/prompts`
**File:** `prompts/output_language/{output_language.go,output_language_test.go}` (mới), `prompts/request_context/_language_rule.tmpl` (mới), `prompts/plan_openspec/_language_rule_openspec.tmpl` (mới), `prompts/analysis/{solution,agent_solution,diagnosis,findings,answer}.tmpl` + `analysis.go` (sửa), `prompts/plan_native/{plan.tmpl,plan_native.go}` (sửa), `prompts/plan_openspec/{plan.tmpl,plan_openspec.go}` (sửa), `prompts/{analysis,plan_native,plan_openspec}/*_test.go` (sửa), `prompts/prompt_size_budget_test.go`, `prompts/README.md` (sửa)
**Depends on:** không (làm đầu tiên)
**Status:** [x] DONE (2026-10-20)

---

## Context

- Không mẫu nào nói viết bằng ngôn ngữ nào (CR-REQ-046 mục 1.1). Chỉ thị phải tới được cả `ai.complete` (Solution, Plan native) lẫn `agent.execPrompt` (chẩn đoán, Findings, Answer, Solution do agent viết, Plan OpenSpec), nên nằm trong mẫu của module `prompts`.
- `Language` rỗng không được phát gì: golden hiện có (158 tệp) phải giữ nguyên từng byte. Đây là điều kiện cứng của task.
- `render.NewSet(funcs, files...)` đòi tên tệp mẫu duy nhất trên mọi `fs.FS` của bộ: partial chung ở `request_context/` tên `_language_rule.tmpl`, partial OpenSpec phải là `_language_rule_openspec.tmpl`.
- Claude giả nhận loại đọc-chỉ bằng chuỗi `"kind":"<loại>"` trước `<request>`: chỉ thị không được chứa chuỗi đó (chỉ `"kind"` đứng riêng).

## Việc cần làm

1. `output_language`: `type Language struct{ Code, Name string }`; `Vietnamese`, `English`; `Supported()` (thứ tự `vi`, `en`); `Parse(raw)` (cắt khoảng trắng, hạ chữ thường, khớp `Code`; rỗng hoặc lạ là lỗi có thông báo liệt kê mã hợp lệ). Zero value = không chỉ thị.
2. `request_context/_language_rule.tmpl`: nội dung ở SOL-046 mục 2.3, bọc `{{if .Language.Code -}} … {{end -}}`, dòng đầu `Output language: {{.Language.Name}}.`, kết thúc bằng xuống dòng.
3. Thêm `Language output_language.Language` vào `analysis.Input`, `plan_native.Input`, `plan_openspec.Input`; gọi `{{template "_language_rule.tmpl" . -}}` ở: `solution.tmpl` giữa dòng `_coverage_rule` và `_body`; `agent_solution.tmpl` giữa `_agent_coverage_rule` và `_body`; `diagnosis.tmpl`, `findings.tmpl`, `answer.tmpl` sau dòng trống và ngay trước `_body`; `plan_native/plan.tmpl` giữa `_type_rules` và `_request_block`.
4. `plan_openspec/_language_rule_openspec.tmpl` (văn xuôi bằng `{{.Language.Name}}`; liệt kê phần giữ tiếng Anh theo CR mục 1.5 và 2.2) và dòng gọi trong `plan.tmpl` ngay trước `_request_block`, trước `result_block.tmpl`.
5. Test mỗi luồng: `Language` rỗng cho đầu ra **bằng đúng** đầu ra cũ (so với chuỗi dựng không có trường); `Vietnamese` và `English` phát đúng một lần dòng `Output language: …`, nằm trước `<request>`, không chứa `"kind":"` kèm loại, chứa danh sách giữ-tiếng-Anh; văn bản thù địch (thẻ đóng trong mọi trường) vẫn nằm trong khối của nó; bộ mẫu phân tích được (không trùng tên tệp).
6. `prompt_size_budget_test.go`: đặt `Language: output_language.Vietnamese` ở fixture của năm luồng phân tích, `plan_native`, `plan_openspec`; đo lại, cập nhật bảng "Measured" và chỉ nâng `maxPromptBytes` nếu vượt (kèm lý do); thêm gói `output_language` và mục "Output language" vào `prompts/README.md` (bảng Layout, bảng ID không đổi).
7. **Cổng golden (làm trước khi viết ca mới):** `backend-go/ci/check-prompt-golden.sh` ở chế độ so sánh phải xanh với các mẫu đã sửa. Không chạy `--update` ở task này.

## Kiểm thử

- `cd backend-go/prompts && go build ./... && go vet ./... && go test ./... -race`.
- `bash backend-go/ci/check-prompt-golden.sh` xanh (không `--update`).
- Cố ý đổi một chữ của partial và thấy test `Language` rỗng/`vi` đỏ đúng chỗ rồi hoàn lại.

## Tiêu chí hoàn thành

- [x] `Parse` đúng cho `vi`, `VI `, `en`, `fr`, rỗng; lỗi nêu mã hợp lệ. `TestParse` (`output_language_test.go`): `vi`, `VI `, ` En`, `en`, `fr`, rỗng, khoảng trắng, `vi-VN`, `vietnamese`; lỗi chứa `use one of: vi, en`.
- [x] `Language` rỗng: đầu ra mọi luồng trùng từng byte với trước khi sửa; `check-prompt-golden.sh` xanh. Golden so sánh xanh sau khi thêm partial (158 tệp không đổi); `TestRender_EmptyLanguageAddsNothing` ở ba gói.
- [x] `vi`/`en` phát chỉ thị đúng một lần, trước dữ liệu; mẫu JSON nói giữ tiếng Anh khoá, `kind`, enum, id, nhãn, đường dẫn; mẫu OpenSpec nói giữ tiếng Anh tiêu đề ngữ pháp, `SHALL`/`MUST`, `#### Scenario:`, nhãn dòng task, `modify`/`create`, `ORCA_RESULT_*`. `TestRender_LanguageInstructionOnceBeforeTheData*` (ba gói): một dòng `Output language: <Name>.`, trước `<request>`, không có `"kind":"`, có danh sách giữ tiếng Anh (JSON: khoá, `kind`, enum `a|b|c`, id, nhãn, đường dẫn; OpenSpec: tiêu đề, `SHALL`/`MUST`, `#### Scenario:`, nhãn dòng, `modify`/`create`, `ORCA_RESULT_*`).
- [x] Ngân sách kích thước xanh; README cập nhật. `TestPromptSize_*` xanh; README có mục "Output language", gói `output_language`, `_language_rule` trong bảng Layout và số đo mới.

## Rủi ro và lưu ý

- Vị trí chỉ thị (trước dữ liệu) chưa kiểm bằng mô hình thật; nếu `--real` (task 07) cho kết quả tiếng Anh với thân dài, chuyển dòng gọi xuống sau `_body` (một dòng mỗi mẫu).
- Không viết "Vietnamese" cứng trong mẫu: luôn `{{.Language.Name}}`, để thêm ngôn ngữ chỉ sửa `output_language.go`.

## Ghi chú triển khai

**Tệp (đường dẫn dưới `backend-go/prompts/`):** mới `output_language/{output_language.go,output_language_test.go}`, `request_context/_language_rule.tmpl`, `plan_openspec/_language_rule_openspec.tmpl`, `analysis/language_test.go`, `plan_native/language_test.go`, `plan_openspec/language_test.go`; sửa `analysis/{solution,agent_solution,diagnosis,findings,answer}.tmpl`, `analysis/analysis.go`, `plan_native/{plan.tmpl,plan_native.go}`, `plan_openspec/{plan.tmpl,plan_openspec.go}`, `prompt_size_budget_test.go`, `README.md`.

**Lệnh và kết quả:**
- Trước khi sửa: `bash backend-go/ci/check-prompt-golden.sh` xanh (đường cơ sở). Sau khi thêm hai partial và các dòng gọi (chưa thêm ca mới): vẫn xanh, `git status -- '*/testdata/prompt-golden/*'` rỗng, tức 158 golden giữ nguyên từng byte.
- `cd backend-go/prompts && go build ./... && go vet ./... && go test ./... -race -count=1` xanh.
- Kiểm đột biến: đổi `Never translate:` thành `Do not translate:` trong partial JSON làm đỏ 7 test mới của `analysis` và `plan_native` (ca chỉ thị, vị trí, danh sách giữ tiếng Anh); hoàn lại thì xanh.
- Số đo ngân sách (fixture đặt `Language: output_language.Vietnamese`): `analysis/solution` 174,137 -> 174,719; `agent_solution` 175,563 -> 176,145; `diagnosis` 173,840 -> 174,422; `findings` 173,635 -> 174,217; `answer` 173,498 -> 174,080; `plan_native/plan` 62,971 -> 63,553; `plan_openspec/plan` 66,050 -> 67,109 (byte). Không nâng `maxPromptBytes`.

**Cách chứng minh "Language rỗng = đầu ra cũ":** ngoài golden, mỗi gói có test `TestRender_EmptyLanguageAddsNothing`: bỏ khối chỉ thị khỏi đầu ra `vi` phải cho đúng đầu ra `Language` rỗng, nên chỉ thị là khác biệt duy nhất.

**Lệch so với đặc tả:** test so sánh "với chuỗi dựng không có trường" được thay bằng phép so sánh trên (đầu ra `vi` bỏ khối = đầu ra rỗng) cộng với golden, vì mã cũ không còn để dựng chuỗi đối chiếu. Test vị trí dùng `"<request>\ntitle:"` làm mốc vì chính chỉ thị có chữ `<request>`.

**Blast radius:** chỉ thêm trường `Language` (giá trị zero = không đổi) vào ba `Input`; người gọi ngoài module `prompts` là ba hàm của `request-service/internal/usecase` (task 02).

**Chưa kiểm:** vị trí chỉ thị trước dữ liệu với mô hình thật (task 07 `--real`).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
