# BE-CV-TASK-086-02: Domain `CommitCheck`, `ComputeOverall`, `ProviderRateLimitedError`

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/domain/commit_check.go`, `provider_rate_limited.go`, test (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context
`domain` chỉ stdlib (đã đọc `domain/scm.go`). Từ vựng status/conclusion như `PRCheckDetail`.

## Việc cần làm
1. Kiểu `CommitCheck`, `CheckStep`, `CheckAnnotation`, `RateLimitInfo`.
2. `ComputeOverall` theo SOL mục 2.F.
3. `ProviderRateLimitedError{ResetAt time.Time, RetryAfter time.Duration}` với `Error()` không chứa body provider.

## Kiểm thử
- Bảng đủ nhánh `ComputeOverall`, rỗng, hỗn hợp.

## Tiêu chí hoàn thành
- [ ] Không import ngoài stdlib.

## Rủi ro
Chuẩn hoá status GitLab (task 06) phải dùng cùng từ vựng.
