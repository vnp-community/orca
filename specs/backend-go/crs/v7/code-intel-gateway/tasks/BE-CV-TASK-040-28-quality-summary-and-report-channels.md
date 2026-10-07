# BE-CV-TASK-040-28: Kênh `codeIntel.quality.summary` (24 s) và `codeIntel.quality.report`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_ai.go` (mới), `channels_codeintel_quality_ai_test.go` (mới)
**Depends on:** TASK-040-23, TASK-040-08 (bất biến 24 s); stub `GenerateReviewSummary` (BE-CV-SOL-093-ai-review-summary), `ExportReviewReport` (BE-CV-SOL-090-review-report-model)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `summary` (sel, `base?`, `profileName?`, `level` metadata|diff, `dryRun?`, `forceRefresh?`, `locale?`) => `{summary: AiReviewSummary|null, manifest, cache, labels}`, quyền `quality_read` ∧ `read_source`; cột T/o ghi 120 s nhưng chú thích: gateway không thể vượt 25 s (invokeTimeout) nên `dryRun:false` trả `CODEINTEL_TIMEOUT | {"inProgress":true,"retryAfterMs":3000}` sau ≤ 24 s và hoàn tất nền (cache 24 h); client thử lại (PQ-13). `report` (sel, `base?`, `profileName?`, `turnKey?`, `sections?`, `maxFindings ≤ 50` (10), `maxReadingSteps ≤ 50` (15), `includePeople?` (false), `includeWaiverReasons?` (false)) => `{model: ReviewReportModel, generatedFor, warnings}` 20 s. Lỗi: `AI_REVIEW_DISABLED`, `AI_NO_RELAY`, `AI_BAD_OUTPUT`, `SECRET_LEAK_BLOCKED`. O13: AI mặc định tắt; cấm gửi secret (service).

## Việc cần làm

1. `qualitySummaryArgs{sel, Base, ProfileName, Level, DryRun, ForceRefresh *bool, Locale}`: `level` bắt buộc metadata|diff; `locale` ≤ 16 và `^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`.
2. Kênh dùng `Timeout = codeIntelSummaryTimeout (24 s)`; hết hạn do gateway => `codeIntelChannelError` đã tạo `CODEINTEL_TIMEOUT` + hậu tố `inProgress`; **không** retry.
3. `qualityReportArgs{sel, Base, ProfileName, TurnKey, Sections[], MaxFindings, MaxReadingSteps, IncludePeople, IncludeWaiverReasons *bool}`: `sections ≤ 32` mỗi phần tử ≤ 64; hai số 1..50; không điền mặc định.
4. Dịch `AiReviewSummary` (`risks[].refs[]`, `readFirst[]` mảng; `model`, `level`, `refsDropped`), `ReviewReportModel` (enum `risk.level` HOA, `gate.verdict` chữ thường, `diagrams[].mermaid` chuỗi nguyên văn không che thêm). Văn bản AI là dữ liệu không tin cậy (U9).
5. Không log `locale`/nội dung; không đưa nội dung tóm tắt vào lỗi.

## Kiểm thử

- Validate: `level` thiếu/`full`; `locale` `xx_yy!`; `maxFindings` 51; `sections` 33.
- Fake treo (không trả tới khi ctx huỷ): kênh trả `CODEINTEL_TIMEOUT` có `inProgress:true` trong ≤ 24 s (dùng đồng hồ giả/ctx ngắn trong test), `ctx.Deadline` đúng 24 s.
- `AI_REVIEW_DISABLED`, `AI_NO_RELAY`, `SECRET_LEAK_BLOCKED` đi qua; `summary:null` khi `dryRun:true`.
- `TestCodeIntelTimeouts_ShorterThanInvokeTimeout` đã bao `quality.summary`.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityAI'`.

## Tiêu chí hoàn thành

- [x] `summary` 24 s, hậu tố `inProgress`; không retry.
- [x] `report` đúng trần; mặc định riêng tư (`includePeople`) không bị gateway thay đổi.

## Rủi ro và lưu ý

- Gateway 24 s so với `invokeTimeout` 25 s chỉ để lại 1 s; ghi phản hồi dùng `writeTimeout` 5 s độc lập nên vẫn tới; đo ở CR-071.
- Giá trị 120 s ở cột T/o là ngân sách service/infra-fleet (`ai.complete`), không phải gateway.
