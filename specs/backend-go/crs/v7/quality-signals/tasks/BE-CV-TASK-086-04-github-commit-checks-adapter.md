# BE-CV-TASK-086-04: Adapter GitHub `ListCommitChecks` (GraphQL + REST)

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/github/commit_checks.go`, test, `testdata/commit_checks_*.json` (mới)
**Depends on:** BE-CV-TASK-086-02, 086-03
**Status:** [x] DONE

## Context
`graphQLRequest` có sẵn (`client.go:609`); `graphQLURL` cố định nên GHE dùng REST. Truy vấn theo SOL mục 2.D (dựng từ `PR_CHECKS_ROLLUP_QUERY`, chưa chạy).

## Việc cần làm
1. PR/COMMIT qua GraphQL (`baseURL == DefaultBaseURL`), REST khi khác; trần 200, `truncated`, `total_count`.
2. Điền `rate_limit` từ `rateLimit{}`/header; 403/429 ⇒ `ProviderRateLimitedError`.
3. Không ghi `RateLimitCache`.
4. Chuẩn hoá status/conclusion, `group` = workflow.

## Kiểm thử
- httptest: PR 3 check, StatusContext, > 200, GHE REST, 403 secondary, JSON lỗi GraphQL.

## Tiêu chí hoàn thành
- [x] Một lời gọi HTTP cho danh sách (đếm).

## Rủi ro
Fixture dựng tay (chưa có mẫu thật).
