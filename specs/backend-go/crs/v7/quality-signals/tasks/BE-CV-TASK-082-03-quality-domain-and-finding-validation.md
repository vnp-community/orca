# BE-CV-TASK-082-03: Domain `QualityRun`/`QualityFinding`, kiểm tra, ánh xạ trạng thái

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/quality_run.go`, `quality_run_transitions.go`, `quality_finding.go`, `quality_finding_limits.go`, `quality_errors.go` + test (mới)
**Depends on:** BE-CV-SOL-010
**Status:** [x] DONE

## Context
SOL-082 mục 2.D. PQ-26 regex `ruleId`; `fingerprint ^[0-9a-f]{32}$`; 10 `category`.

## Việc cần làm
1. `QualityFinding.Validate()`: fingerprint, severity, category, ruleId (PQ-26), `file` tương đối an toàn, số ≥ 0, `message` ≤ 2048 byte (cắt ở ranh giới ký tự).
2. `CanTransition`; ánh xạ agent→DB (`cancelling→running`, `interrupted→failed + CODEINTEL_QUALITY_RUN_INTERRUPTED`).
3. Hằng giới hạn 5000/20000/2048.
4. Lỗi: `ErrRunActive` (`CODEINTEL_RUN_IN_PROGRESS`), `ErrRunNotFound`.

## Kiểm thử
- Bảng đối kháng: Unicode, CRLF, đường dẫn Windows/tuyệt đối/`..`, ruleId sai, message 5 KiB.

## Tiêu chí hoàn thành
- [x] Chỉ import stdlib; không `max-lines` disable.

## Rủi ro
Mã `..._INTERRUPTED` chưa có trong hợp đồng (SOL-082 mục 7).
