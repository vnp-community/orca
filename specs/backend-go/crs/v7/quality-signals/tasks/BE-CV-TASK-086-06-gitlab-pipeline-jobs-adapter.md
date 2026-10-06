# BE-CV-TASK-086-06: Adapter GitLab `ListCommitChecks` (pipeline jobs)

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/gitlab/commit_checks.go`, test, `testdata/` (mới)
**Depends on:** BE-CV-TASK-086-02, 086-03
**Status:** [ ] TODO

## Context
Bảng ánh xạ job status ở SOL mục 2.E; `projectPath` escape sẵn; `baseURL` self-managed.

## Việc cần làm
1. PR: MR → `head_pipeline`; COMMIT: pipelines theo `sha`; jobs tối đa 2 trang (200).
2. `group=stage`; không annotation; `head_sha` = MR `sha`.
3. `RateLimit-*`, 429 ⇒ `ProviderRateLimitedError`.

## Kiểm thử
- httptest: MR có/không `head_pipeline`, 250 job → truncated, status map đủ, self-managed baseURL.

## Tiêu chí hoàn thành
- [ ] Không phụ thuộc gitlab.com.

## Rủi ro
Pipeline kết quả gộp (chưa thử instance thật).
