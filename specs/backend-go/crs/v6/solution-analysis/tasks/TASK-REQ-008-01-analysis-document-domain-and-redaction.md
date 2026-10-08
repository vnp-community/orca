# TASK-REQ-008-01: Tài liệu `diagnosis`, `findings`, `answer`: domain, kiểm schema, che bí mật

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục E, D bước 8
**Priority:** P1
**Service/Area:** `request-service` / domain, usecase (hàm thuần)
**File:** `internal/domain/diagnosis_document.go` (mới), `findings_document.go` (mới), `answer_document.go` (mới), `internal/usecase/analysis_document_validation.go` (mới), `analysis_secret_redaction.go` (mới), và `_test.go`; `testdata/analysis_documents/*.json` (mới)
**Depends on:** TASK-REQ-007-02 (`ExtractJSONObject`, `MaxOptionsBytes`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/domain/... ./internal/usecase/... -run "Diagnosis|Findings|Answer|Redact|ValidateByKind"`)

## Context

- Schema JSON nằm ở CR-REQ-008 mục 2.4 (`schema_version: 1`; tối đa 64 KB; `confidence` thuộc `[0,1]`; `evidence` tối đa 20; `excerpt` tối đa 600).
- Ba tài liệu lưu vào cột `options` của `solutions` với `chosen_option` NULL và `content_ref` rỗng.
- Mẫu che bí mật hiện có để tham khảo: `mcp-service/internal/domain/secret_redactor.go`. Sao chép ý tưởng, không import chéo service. Độ phủ mẫu chưa kiểm chứng.

## Việc cần làm

1. `DiagnosisDocument` (summary, `root_cause{statement, confidence, evidence[]}`, `reproduction`, `impact{severity, scope, affected_components, user_facing, data_risk}`, `fix_directions[]`, `suggested_size`, `suggest_escalate_to_change_request`, `escalation_reason`, `measurements[]`, `open_questions`) và `Validate()`: `root_cause.statement` bắt buộc; `severity` thuộc `low|medium|high|critical`; `fix_directions.id` duy nhất; `suggested_size` thuộc `S|M|L`.
2. `FindingsDocument` + `Validate()`: ít nhất một finding; mỗi finding có `evidence` hoặc được nêu trong `unknowns`; `follow_ups[].suggested_type` thuộc `change_request|task`.
3. `AnswerDocument` + `Validate()`: `1 <= len(answer_markdown) <= 8000` tính theo rune; `citations` hợp lệ.
4. `ValidateByKind(kind SolutionKind, raw []byte) error` chọn validator, kiểm `schema_version==1`, `kind` trong JSON khớp, tổng kích thước.
5. `RedactSecrets(s string) (string, int)` thay bằng `[REDACTED]`: khối PEM `-----BEGIN ... PRIVATE KEY-----...-----END ...-----`, `ghp_` + 36 ký tự, `AKIA` + 16 ký tự, JWT ba đoạn base64url bắt đầu `eyJ`, `password\s*[=:]\s*\S+` (không phân biệt hoa thường); trả số lần thay.
6. `RedactDocument(kind, raw)` áp lên các trường `excerpt`, `answer_markdown`, `statement`, `summary` của tài liệu đã parse rồi in lại; đồng thời `RedactSecrets` cho `raw_output`.

## Kiểm thử

- Bảng đúng/sai cho từng `Validate` (thiếu trường, `confidence` 1.2, `evidence` 21 mục, `answer_markdown` 8001 rune, tiếng Việt có dấu đếm theo rune).
- `RedactSecrets`: mẫu dương cho từng loại; mẫu âm (chuỗi `AKIA` ngắn, `password` không có giá trị, văn bản tiếng Việt bình thường); không phá UTF-8.
- Golden JSON hợp lệ cho cả ba `kind`.
- Lệnh: `go test ./internal/domain/... ./internal/usecase/... -run "Diagnosis|Findings|Answer|Redact"`.

## Tiêu chí hoàn thành

- [x] Mỗi luật schema có ít nhất một test sai.
- [x] `[REDACTED]` xuất hiện ở `excerpt` và `raw_output` khi có khoá PEM hoặc `ghp_...` (tiêu chí 8 của CR).
- [x] Không import ngoài stdlib trong domain.

## Rủi ro và lưu ý

- Che bí mật là giảm thiểu, không bảo đảm; ghi rõ trong doc comment của hàm.
- Regex phải tránh backtracking tệ (Go RE2 an toàn, vẫn giới hạn độ dài đầu vào 256 KB).

## Ghi chú triển khai (2026-10-08)

- Ba tài liệu ở `internal/domain/{diagnosis,findings,answer}_document.go` (+ `analysis_evidence.go` dùng chung kiểu `SupportingRef`; tên `Evidence` đã bị bảng bằng chứng của CR-REQ-027 chiếm). Golden hợp lệ/sai ở `internal/domain/testdata/analysis_documents/`.
- `RedactSecrets` thật dùng `common/secretscan` (PEM, `ghp_`, `AKIA`, JWT, `password=`, cả chuỗi kết nối và token khác); `RedactDocument` duyệt mọi giá trị chuỗi của tài liệu (không chỉ bốn trường được nêu) rồi in lại; `RedactRaw` cắt 256 KB trước khi quét vì `secretscan` chỉ quét một cửa sổ. Hàm `RedactApplicationSecrets` và `domain.RedactSecrets` giả (trả nguyên đầu vào) đã xoá, không có người gọi.
- Che bí mật là giảm thiểu, không bảo đảm; độ phủ mẫu chưa đo trên dữ liệu thật.
