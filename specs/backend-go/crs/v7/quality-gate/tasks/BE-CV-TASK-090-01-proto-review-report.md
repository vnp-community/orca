# BE-CV-TASK-090-01: Proto `codeintel_review_report.proto` và `rpc ExportReviewReport`

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P1
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_review_report.proto` (mới); `codeintel_quality_gate.proto` (thêm rpc)
**Depends on:** BE-CV-TASK-085-02
**Status:** [x] DONE

## Việc cần làm
1. `ReviewReportModel` và phần con theo ui-api §4.7 (`risk.level` chuỗi chữ HOA, PQ-32); `ExportReviewReportRequest` theo §3.2 (`selector` field 1).
2. `generated_for{commit, index_commit, stale}`, `warnings[]`.
3. Số field cố định, `buf lint/breaking`.

## Kiểm thử / Tiêu chí hoàn thành
- [x] lint+breaking xanh; [ ] `maxFindings/maxReadingSteps` là `int32` (kiểm khoảng ở use case).

## Rủi ro
- Thêm trường additive (`contentWithheld`, Q6) chờ duyệt.
