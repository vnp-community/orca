# BE-CV-TASK-086-13: Use case `RefreshCiRun`

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/refresh_ci_run.go`, `refresh_ci_run_test.go` (mới)
**Depends on:** BE-CV-TASK-086-09, 086-10, 086-11, 086-12, BE-CV-SOL-013-agent-call-gate-and-quotas
**Status:** [ ] TODO

## Context
SOL mục 2.D bước 1-8: cờ, quyền, locator, cache theo `quality_runs`, singleflight, hạn mức, breaker, gọi scm, dựng run, ghi, so sánh.

## Việc cần làm
1. Cài các bước; cache bất biến khi `stale_after IS NULL`; TTL 30→120 s (`external_ref.unchangedCount`).
2. `limited`/ngân sách ⇒ bản cũ `stale:true, rate_limited:true, reset_at`; breaker theo `(tenant, provider)`.
3. Quá 20 s ⇒ `CODEINTEL_TIMEOUT` + `{"retryAfterMs":3000,"inProgress":true}`, hoàn tất nền.
4. `CODEINTEL_RATE_LIMITED` khi vượt `CODEINTEL_CI_REFRESH_PER_MINUTE`.

## Kiểm thử
- Cổng giả + đồng hồ giả: đếm lời gọi scm (cache hit = 0, đồng thời = 1), breaker, TTL, force, provider unsupported, không PR.

## Tiêu chí hoàn thành
- [ ] Các tiêu chí mục 4 liên quan use case xanh.

## Rủi ro
Breaker theo replica.
