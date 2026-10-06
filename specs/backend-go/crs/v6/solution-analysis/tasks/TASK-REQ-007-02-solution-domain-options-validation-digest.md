# TASK-REQ-007-02: Domain `Solution`, `SolutionOptions.Validate`, bộ trích JSON và digest

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục B
**Priority:** P0
**Service/Area:** `request-service` / domain, usecase (hàm thuần)
**File:** `internal/domain/solution.go` (mới), `solution_options.go` (mới), `canonical_json_digest.go` (mới), `internal/usecase/solution_output_extraction.go` (mới), và `_test.go`
**Depends on:** TASK-REQ-007-01 (hằng `AnalysisMode`); CR-REQ-002 (kiểu `Solution` nếu đã định nghĩa, nếu có thì mở rộng thay vì tạo lại)
**Status:** [ ] TODO

## Context

- Kiểm tra trước: `ls internal/domain/solution*.go`. CR-REQ-002 có thể đã tạo struct `Solution`; khi đó chỉ thêm phương thức ở file này.
- Digest phải tính trên JSON chuẩn tắc, không trên văn bản đã qua `JSONB`/`JSON` (khoá bị đổi thứ tự). Mẫu: `mcp-service/internal/domain/params_hash.go` (`parseStrictJSON` từ chối khoá trùng, `writeCanonical`). Sao chép ý tưởng, không import (`internal` của service khác).
- Mẫu parse AI hiện tại: `parseSubtaskProposalsJSON` trong `task-service/internal/usecase/ai_decompose.go` (không chịu code fence): đây là điều cần làm tốt hơn.

## Việc cần làm

1. `solution.go`: `SolutionKind`, `SolutionStatus`, struct `Solution` (có `ChosenOption *int`, `Version`), phương thức `Propose(opts []byte)`, `Choose(idx int)`, `Supersede()`, `Approve()`, `Reject()`, lỗi `ErrSolutionNotProposed`, `ErrOptionIndexOutOfRange`.
2. `solution_options.go`: `SolutionOptions`, `Option`, `Risk`, `Effort`, `AffectedArea`, `Recommendation`; `ParseSolutionOptions(raw)`; `Validate(minOptions int) error` theo CR 2.3: số phương án trong `[minOptions, 4]`, `id` duy nhất dạng `opt-N`, đúng một `recommended=true` trùng `recommendation.option_id`, `effort.size` bắt buộc, `hours_estimate >= 0`, độ dài chuỗi (title 1..120, summary 1..600, approach 1..4000), tổng 64 KB.
3. `canonical_json_digest.go`: `DigestOptions(options []byte, chosen *int) (string, error)` = sha256 hex của JSON chuẩn tắc (khoá sắp xếp, không khoảng trắng, từ chối khoá trùng và độ sâu > 32) nối `|chosen=<n|none>`.
4. `solution_output_extraction.go`: `ExtractJSONObject(text string) ([]byte, error)`: bỏ code fence, lấy khối `{...}` ngoài cùng có cân bằng ngoặc (tôn trọng chuỗi và escape), lỗi `ErrNoJSONObject`.
5. Hằng `MaxOptionsBytes = 64*1024`.

## Kiểm thử

- `solution_options_test.go`: bảng đúng/sai (1 phương án khi `min=2`; 0 hoặc 2 phương án `recommended`; `id` trùng; `id` sai dạng; `recommendation` lạ; chuỗi quá dài; 5 phương án).
- `canonical_json_digest_test.go`: cùng nội dung khác thứ tự khoá và khoảng trắng cho cùng digest; khoá trùng bị từ chối; `chosen` đổi thì digest đổi; Unicode tiếng Việt ổn định.
- `solution_output_extraction_test.go`: có code fence, văn bản thừa trước và sau, JSON lồng, dấu ngoặc trong chuỗi, không có JSON.
- Lệnh: `go test ./internal/domain/... ./internal/usecase/... -run "Solution|Digest|Extract"`.

## Tiêu chí hoàn thành

- [ ] `Validate` bao phủ mọi luật 2.3 của CR, mỗi luật có ít nhất một test sai.
- [ ] Digest bằng nhau trước và sau khi đi qua Postgres `JSONB` và MySQL `JSON` (kiểm ở task 03).
- [ ] Domain chỉ stdlib.

## Rủi ro và lưu ý

- `min_options` của `refactor` chưa chốt (câu hỏi mở 1); nhận từ registry, không hằng trong domain.
