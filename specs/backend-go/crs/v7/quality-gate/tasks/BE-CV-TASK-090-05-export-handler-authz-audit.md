# BE-CV-TASK-090-05: Handler `ExportReviewReport`: tham số, quyền `quality_read ∧ read`, cờ, audit

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/adapter/grpc/review_report_server.go` (mới)
**Depends on:** BE-CV-TASK-090-04, BE-CV-TASK-085-07
**Status:** [ ] TODO

## Việc cần làm
1. Kiểm khoảng `max_findings ≤ 50`, `max_reading_steps ≤ 50` (ngoài khoảng ⇒ `CODEINTEL_INVALID_PARAMS`, không kẹp), `sections` ⊂ tập hợp lệ, `turn_key` ≤ 128.
2. OPA `quality_read` **và** `read`; cờ chất lượng tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED` (L1).
3. Audit `codeintel.report.export` (`target: review:<binding>`, `allowed`); `slog audit=true` kèm `modelDigest`.

## Kiểm thử
- Thiếu một trong hai quyền ⇒ `NOT_AUTHORIZED`; cờ tắt; tham số biên; audit gọi đúng chữ ký `Append`; tenant khác.

## Tiêu chí hoàn thành
- [ ] khớp ui-api §3.2; [ ] không log nội dung mô hình.

## Rủi ro
- `auditclient.Append` nuốt lỗi; audit có thể mất âm thầm.
