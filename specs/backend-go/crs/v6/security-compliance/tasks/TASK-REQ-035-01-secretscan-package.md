# TASK-REQ-035-01: Gói `common/secretscan` (`Scan`, `Redact`, `RedactKinds`) và vector kiểm thử chung

**From Solution:** BE-REQ-SOL-035 (mục E)
**Priority:** P0
**Service:** `backend-go/common`
**File:** `backend-go/common/secretscan/{scan.go,patterns.go,kinds.go,scan_test.go}` (mới), `backend-go/common/secretscan/testdata/vectors.json` (mới)
**Depends on:** None
**Status:** `[x] DONE`

---

## Context

- `mcp-service/internal/domain/secret_redactor.go` có 9 mẫu (`secretPatterns`): PAT GitHub (`gh[pousr]_`, `github_pat_`), `AKIA|ASIA`, `Bearer`, `sk-`, `xox[abprs]-`, JWT, khối khoá riêng PEM, và dòng `password|passwd|secret|token|api_key|authorization = value` (mẫu cuối giữ nhóm 1 để chỉ che giá trị). Chữ ký `Redact(string) (string, bool)`. Nó nằm trong `internal/` của `mcp-service`: **không import chéo**, và CR 035 không sửa `mcp-service`. Hai bản phải không lệch: dùng `testdata/vectors.json` làm hợp đồng (người sở hữu `mcp-service` có thể đọc cùng tệp ở CR sau, Q5).
- Gói mới nằm ở `backend-go/common` (module `common`, đã được mọi service dùng qua `go.work`). Không đặt tên `utils`, `helpers`.
- Điểm dùng: cổng vào `CreateRequest`/webhook/MCP (task 09), trước prompt (BE-REQ-SOL-034, 031, 008, 005, 007, 012), đầu ra AI (CR-008 `analysis_secret_redaction.go`), `ExportRequest` (task 08), `TestGoldenHasNoSecrets` của eval (034-08).
- Cổng vào chỉ thay khi độ tin cậy **cao** (khoá riêng, token có tiền tố biết trước); prompt thay mọi độ tin cậy.

## Việc cần làm

1. `kinds.go`: `type Kind string` và hằng: `KindPrivateKey = "private_key"`, `KindGitHubToken = "github_token"`, `KindAWSAccessKey = "aws_access_key"`, `KindBearer = "bearer_token"`, `KindOpenAIKey = "openai_key"`, `KindAnthropicKey = "anthropic_key"`, `KindSlackToken = "slack_token"`, `KindJWT = "jwt"`, `KindGoogleAPIKey = "google_api_key"`, `KindVaultToken = "vault_token"`, `KindConnectionString = "connection_string"`, `KindDotenvSecret = "dotenv_secret"`, `KindSecretAssignment = "secret_assignment"`; `type Confidence string` (`high`, `medium`); `type Finding struct{ Kind Kind; Confidence Confidence; Start, End int }` (offset byte trong chuỗi gốc, `[Start, End)` là phần **giá trị** cần che).
2. `patterns.go`: bảng `[]pattern{kind, confidence, re *regexp.Regexp, valueGroup int}` đã biên dịch một lần (`regexp.MustCompile` ở `var`). Mẫu gồm 9 mẫu của `mcp-service` (giữ nguyên biểu thức, gán `kind`) cộng: `sk-ant-[A-Za-z0-9_-]{20,}` (đặt **trước** `sk-` để gán đúng `anthropic_key`), `AIza[0-9A-Za-z_-]{35}`, `hvs\.[A-Za-z0-9_-]{24,}`, `\b[a-z][a-z0-9+.-]*://[^\s/:@]+:[^\s/@]+@[^\s/]+` (`connection_string`, che mật khẩu qua nhóm), khoá riêng OpenSSH đã bao bởi `-----BEGIN [A-Z ]*PRIVATE KEY-----` (kiểm có vector), và dòng `.env`: `(?m)^\s*(?:export\s+)?([A-Z][A-Z0-9_]*(?:SECRET|TOKEN|KEY|PASSWORD)[A-Z0-9_]*)\s*=\s*["']?([^\s"']{4,})` (`dotenv_secret`, che nhóm 2). **Không** có mẫu `s\.[A-Za-z0-9]{24}` của Vault cũ (dương tính giả quá nhiều); ghi lý do trong comment ngắn.
3. Độ tin cậy: `high` cho `private_key`, `github_token`, `aws_access_key`, `anthropic_key`, `openai_key`, `slack_token`, `google_api_key`, `vault_token`, `connection_string`; `medium` cho `bearer_token`, `jwt`, `dotenv_secret`, `secret_assignment`.
4. `scan.go`: `const PatternsVersion = "ss/1"`; `func Scan(text string) []Finding` (khử chồng lấp: mẫu cụ thể hơn thắng, sắp theo `Start`); `func Redact(text string) (string, bool)` thay mỗi giá trị bằng `[REDACTED]` (tương thích `mcp-service`), trả `true` nếu có đổi; `func RedactKinds(text string, minConfidence Confidence) (string, []Kind)` thay bằng `[REDACTED:<kind>]` chỉ cho phát hiện đạt `minConfidence` và trả danh sách loại (không bao giờ trả giá trị). Mọi hàm an toàn với chuỗi rỗng và nhiều megabyte (giới hạn quét 1 MiB đầu, phần còn lại được thêm nguyên và hàm trả thêm cờ `Truncated` qua kiểu `Result` nếu cần: chọn `RedactKinds` trả `Result{Text string; Kinds []Kind; Truncated bool}`).
5. Thiết kế API ổn định: không export regexp; không log; không ghi giá trị vào lỗi.
6. `testdata/vectors.json`: mảng `{ "name", "input", "want_redacted", "kinds": [...], "confidence": "high|medium" }` cho từng mẫu dương (ít nhất 2 mẫu mỗi loại, gồm chuỗi nhúng trong văn bản tiếng Việt có dấu, trong JSON, trong URL) và vector **âm** (không được che): UUID, hash SHA-1/SHA-256 hex, câu "password policy" không có `=`, chuỗi `sk-` ngắn 5 ký tự, email, URL không userinfo `https://example.com/a?x=1`, đoạn mã Go có `tokenCount := 5`.
7. Khoảng chống bùng nổ regex: tất cả mẫu dùng `regexp` (RE2, thời gian tuyến tính); không dùng thư viện backtracking.
8. Ghi vào comment đầu gói (một đoạn ngắn): bộ mẫu nhân đôi mẫu của `mcp-service` có chủ ý, vector chung là hợp đồng; thay đổi phải tăng `PatternsVersion`.

## Kiểm thử

- `scan_test.go`:
  - `TestVectors` đọc `testdata/vectors.json`, chạy `Redact` và `RedactKinds`, so `want_redacted`, `kinds`, `confidence`.
  - `TestNegativeVectorsUntouched`.
  - `TestAnthropicBeforeOpenAI` (`sk-ant-...` gán `anthropic_key`, không `openai_key`).
  - `TestPrivateKeyBlockWholeBlock` (khối nhiều dòng, cả khi thiếu dòng `END`: che đến hết chuỗi như mẫu gốc).
  - `TestRedactKindsMinConfidenceHigh` (`Bearer abc12345` không bị che, `ghp_...` bị che).
  - `TestLargeInputLinear` (2 MiB, thời gian dưới 1 giây; đo thô, không `Benchmark` chặn CI).
  - `TestNoValueInFindings` (struct `Finding` không chứa giá trị).
  - `TestIdempotent` (`Redact(Redact(x)) == Redact(x)`).
  - `FuzzRedact` (Go fuzz, `-run` mặc định chỉ chạy seed corpus): không panic, đầu ra không dài hơn đầu vào + 64 byte × số phát hiện.
- Lệnh: `cd backend-go/common && go test ./secretscan/... && go test -run=^$ -fuzz=FuzzRedact -fuzztime=10s ./secretscan/` (fuzz chạy tay).

## Tiêu chí hoàn thành

- [x] Mọi vector dương được che, mọi vector âm giữ nguyên (gồm tiếng Việt có dấu).
- [x] `RedactKinds(..., high)` không che mức `medium`.
- [x] Không có giá trị bí mật trong `Finding`, lỗi, log.
- [x] Không regex backtracking; 2 MiB đầu vào xử lý dưới 1 giây.
- [x] `PatternsVersion` có và có test nhắc cập nhật khi đổi bảng mẫu (so băm bảng với hằng).

## Ví dụ tham khảo

Hai vector mẫu cho `testdata/vectors.json`:

```json
[{"name": "github_pat_in_vietnamese_text", "input": "Token của tôi là ghp_abcdefghijklmnopqrstuvwx rồi nhé", 
  "want_redacted": "Token của tôi là [REDACTED] rồi nhé", "kinds": ["github_token"], "confidence": "high"},
 {"name": "uuid_not_secret", "input": "id=3f2504e0-4f89-11d3-9a0c-0305e82c3301", "want_redacted": "id=3f2504e0-4f89-11d3-9a0c-0305e82c3301", "kinds": []}]
```

Cách gọi ở các điểm dùng: cổng vào `secretscan.RedactKinds(body, secretscan.High)`; trước prompt `secretscan.Redact(text)`; xuất dữ liệu `secretscan.Redact` trên từng chuỗi của gói JSON.

## Thứ tự làm gợi ý

1. Chép 9 mẫu của `mcp-service` thành bảng có `kind`, đặt vector dương cho từng mẫu trước khi viết hàm.
2. Thêm mẫu mới (Anthropic, Google, Vault, chuỗi kết nối, `.env`) kèm vector âm.
3. `Scan`, `Redact`, rồi `RedactKinds` với `minConfidence`.
4. Fuzz và kiểm hiệu năng cuối cùng.

## Rủi ro và lưu ý

- Regex bỏ sót nhiều dạng bí mật (khoá nhà cung cấp tự đặt tên, bí mật nhiều dòng không có tiêu đề); độ phủ chưa kiểm chứng với dữ liệu khách (CR mục 5).
- Dương tính giả với mẫu `dotenv_secret` trong tài liệu hướng dẫn; chấp nhận, vì đường prompt ưu tiên an toàn.
- Hai bản mẫu (`mcp-service` và gói này) có thể lệch theo thời gian; vector chung là biện pháp duy nhất ở v1.
- Gói thuộc module `common` dùng chung 16 service: chạy `gitnexus_impact` không cần (gói mới), nhưng CI toàn `go.work` phải xanh (`go build ./...`).
