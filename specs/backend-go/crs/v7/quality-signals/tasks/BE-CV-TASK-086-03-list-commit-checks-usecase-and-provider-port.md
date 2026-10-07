# BE-CV-TASK-086-03: Port `CommitCheckProvider` và use case `ListCommitChecks`

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/commit_check_provider.go`, `list_commit_checks.go`, `list_commit_checks_test.go` (mới)
**Depends on:** BE-CV-TASK-086-02
**Status:** [x] DONE

## Context
Khuôn `WorkItemProvider` (type-assert) và `GetPullRequestForBranch` (credential + registry). Không sửa interface `ScmProvider`.

## Việc cần làm
1. Port, `ListCommitChecksQuery/Result`.
2. Validate đầu vào (tenant khớp ctx, provider, slug an toàn, ref_kind, `pr_number`, `commit_sha` regex, `max` 1..200 mặc định 100).
3. Không hỗ trợ ⇒ `CapabilityUnsupported=true`; `ProviderRateLimitedError` ⇒ kết quả `Limited`.
4. Lỗi khác ⇒ `apperrors.New(KindInternal, "SCM_LIST_COMMIT_CHECKS_FAILED", ...)` không lộ body.

## Kiểm thử
- Cổng giả: ma trận 5 provider; tenant lệch; max sai; rate limited.

## Tiêu chí hoàn thành
- [x] Không import adapter; không `max-lines` disable.

## Rủi ro
Credential dùng chung theo tenant (không theo user): ghi nhận như mọi RPC scm.
