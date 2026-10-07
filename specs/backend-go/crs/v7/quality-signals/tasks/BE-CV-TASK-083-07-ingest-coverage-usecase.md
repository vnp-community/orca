# BE-CV-TASK-083-07: Use case `IngestCoverage`

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/ingest_coverage.go`, `ingest_coverage_test.go`, `internal/adapter/agentquality/coverage_decode.go` (mới)
**Depends on:** BE-CV-TASK-083-03, 083-04, 083-05, BE-CV-TASK-082-08
**Status:** [x] DONE

## Context
C-AG §5.6; SOL-083 2.D. Chạy sau `run_finished` có bước `kind=coverage`.

## Việc cần làm
1. Gọi `quality.coverage{workspaceRoot, runId}`; `available:false` → bỏ; `ENV_NOT_READY(coverage_provider_missing)` → ghi `envReason`, không tạo báo cáo.
2. Kiểm head/dirty khớp run, `Validate`, `Trim`, `tree_hash` khi bẩn, `scope_key='all'`, upsert.
3. `RUN_IN_PROGRESS` → thử lại sau (bội lùi, tối đa 3 lần).

## Kiểm thử
- Cổng giả; golden JSON `quality.coverage`; không số 0 khi thiếu provider.

## Tiêu chí hoàn thành
- [x] Báo cáo không nhất quán bị từ chối, run giữ nguyên.

## Rủi ro
`scope_key='all'` (SOL-083 mục 7).
