# BE-CV-TASK-086-10: Hàm `CompareLocalAndCI` và `reasonsHint`

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/ci_comparison.go`, `ci_comparison_test.go` (mới)
**Depends on:** BE-CV-TASK-086-09
**Status:** [x] DONE

## Context
SOL mục 2.E; PQ-25: 9 relation; `local_pass_ci_fail` không bao giờ `pass`, luôn có hint.

## Việc cần làm
1. Cài thứ tự đánh giá (1)-(6); gộp nhiều profile cục bộ cho một check CI.
2. `reasonsHint` là mã ổn định (`env_differs`, `ci_git_matrix`, `ci_integration_tests`, `local_scope_changed`, `stale_cache`, `tool_version_differs`).
3. Hàm thuần; không trả verdict.

## Kiểm thử
- Bảng đủ 9 relation + ca thiếu một phía, `dirty`, `sha_mismatch`, cục bộ `env_not_ready`.

## Tiêu chí hoàn thành
- [x] Mỗi relation có ≥ 1 ca; `local_pass_ci_fail` luôn hint không rỗng.

## Rủi ro
Tập mã hint chưa có trong hợp đồng (SOL mục 7).
