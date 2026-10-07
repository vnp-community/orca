# BE-CV-TASK-093-02: Proto `codeintel_ai_review.proto` và `rpc GenerateReviewSummary`

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_ai_review.proto` (mới); `codeintel_quality_gate.proto` (thêm rpc)
**Depends on:** BE-CV-TASK-085-02
**Status:** [x] DONE

## Việc cần làm
1. `AiReviewSummary`, `AiReviewManifest` (`level`, `files[{path,bytes,hunks,withheld?}]`, `findingsCount`, `redactions`, `totalBytes`, `estimatedTokens`, `suspectedInjection`), `cache{hit, created_at, expires_at}`, `labels{ai_inferred, model, level, generated_at}`.
2. `GenerateReviewSummaryRequest` theo §3.2 (`selector` field 1). `buf lint/breaking`.

## Kiểm thử / Tiêu chí hoàn thành
- [x] lint+breaking xanh; [ ] `refs_dropped` `int32`.

## Rủi ro
- `provider` trong manifest "unknown trước khi gọi" (CR) — giữ chuỗi.
