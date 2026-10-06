# BE-CV-TASK-090-02: Domain dựng `ReviewReportModel` và `modelDigest` (thuần)

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/domain/review_report_model.go`, `review_report_digest.go` (mới)
**Depends on:** BE-CV-TASK-090-01
**Status:** [ ] TODO

## Việc cần làm
1. `ReviewReportBuilder` nhận dữ liệu nguồn (overlay, gate, findings, contract diff) → mô hình; phần thiếu **vắng** + mã cảnh báo, không giá trị giả.
2. Sắp xếp xác định: `findings.top` `(severity desc, ruleId, file, line)`; tên theo chữ cái; map → slice cặp khoá sắp.
3. `modelDigest` = sha256 JSON chuẩn hoá (không `generatedAt`, không `modelDigest`); **không** dùng `protojson`.
4. Cắt giới hạn: `max_findings` (10/≤50), `max_reading_steps` (15/≤50), `limits.truncated/totalCounts`.

## Kiểm thử
- Golden JSON từ fixture; thiếu từng nguồn; digest không đổi theo thứ tự map; đổi `runIds`/`indexCommit` ⇒ digest đổi.

## Tiêu chí hoàn thành
- [ ] cùng đầu vào ⇒ cùng digest ở hai tiến trình; [ ] cổng `unknown` giữ nguyên; [ ] `risk.level` HOA.

## Rủi ro
- Hình dạng thật overlay/finding chưa chốt (CR-036/037/038).
