# BE-CV-TASK-086-09: Ánh xạ check CI, dựng run/finding CI (PQ-25, PQ-26)

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/ci_check_mapping.go`, `ci_run_builder.go` + test (mới)
**Depends on:** BE-CV-TASK-082-03, BE-CV-TASK-086-01 (stub scm)
**Status:** [x] DONE

## Context
Bảng SOL mục 2.C (từ `pr.yml`, `backend-go-*.yml`); PQ-25 ánh xạ trạng thái; PQ-26 `ruleId`; C6 fingerprint.

## Việc cần làm
1. Bảng ánh xạ mặc định (glob không phân biệt hoa thường, không regex người dùng).
2. `BuildCiRun(list)`: status, `CODEINTEL_CI_RESULT_UNKNOWN`, summary, `agent_run_id='ci:<provider>:<sha>'`, `scope='worktree'`, `external_ref`/`external_url` (bỏ query), TTL `stale_after`.
3. Finding: `ruleId` chuẩn hoá hợp PQ-26, severity/category, `fingerprint=sha256(ruleId+file+norm(message))[0:32]` + occurrence; check đỏ không annotation ⇒ một finding `file=''`; trần 500.

## Kiểm thử
- Bảng: tên check Unicode/dài/ký tự lạ, 5 giá trị `overall`, annotation trùng, URL có token trong query.

## Tiêu chí hoàn thành
- [x] Regex PQ-26 luôn thoả; không chuỗi nhạy cảm.

## Rủi ro
Tên profile cục bộ là đề xuất (C-AG §5.1).
