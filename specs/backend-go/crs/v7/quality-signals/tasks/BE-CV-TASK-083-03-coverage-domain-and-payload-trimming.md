# BE-CV-TASK-083-03: Domain coverage, kiểm tra nhất quán, cắt payload

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/coverage_report.go`, `coverage_validation.go`, `coverage_payload_trimming.go` + test (mới)
**Depends on:** BE-CV-SOL-010
**Status:** [ ] TODO

## Context
SOL-083 2.D bước 2-3 và C7: backend kiểm lại `diffCoverage = covered/(covered+uncovered)`, null khi mẫu số 0; `pct` khớp `covered/stmts` (1e-6); đường dẫn tương đối.

## Việc cần làm
1. Kiểu domain và `Validate()` (lỗi → `CODEINTEL_RESULT_INVALID`).
2. `Trim(report, maxFiles=2000, maxBytes=1MiB)` cắt `pct` thấp trước, đặt `truncated`, giữ `total_count`.
3. Quy tắc `estimated`: cấm `pct/stmts/covered`.

## Kiểm thử
- Bảng: số lệch, mẫu số 0, 2 500 tệp, payload 1,2 MiB, đường dẫn `..`/tuyệt đối.

## Tiêu chí hoàn thành
- [ ] Hàm thuần, chỉ stdlib.

## Rủi ro
Ngưỡng `pct<0.5` cho `uncoveredRanges` là của agent (Q6 CR-083).
