# BE-CV-TASK-086-11: Client gRPC `scm-integration-service` và định vị hosted review

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpcclient/scmintegration/client.go`, `backend-go/services/code-intel-service/internal/domain/hosted_review_ref.go`, `backend-go/services/code-intel-service/internal/usecase/ci_ports.go`, config `SCM_INTEGRATION_SERVICE_ADDR`, `CODEINTEL_CI_PROVIDER_HOSTS` (mới)
**Depends on:** BE-CV-TASK-086-01, BE-CV-SOL-012-target-resolution-and-bindings
**Status:** [ ] TODO

## Context
Cổng `ScmCommitChecksClient` (`ListCommitChecks`, `GetPullRequestForBranch`, `ListMergeRequests`, `GetRateLimitStatus`), `HostedReviewLocator`. `Repo.url` không có provider (đã đọc `project.proto`). Mọi lời gọi có deadline (arch/08: mặc định 5 s, ngoại lệ 15 s ghi lý do).

## Việc cần làm
1. Client: truyền `tenant_id` từ ctx, deadline 15 s, retry chỉ cho gọi đọc idempotent, không retry khi `limited`.
2. `ParseRemote(url)`: https, `ssh://`, scp-like → host, slug (không `..`); host→provider theo bảng + env.
3. Locator: `ListRepos`/`ListWorktrees` (không `GetWorktree` theo id) → branch.

## Kiểm thử
- Parse: github/gitlab.com/GHE/self-managed/URL lạ; client với server gRPC giả (deadline, lỗi).

## Tiêu chí hoàn thành
- [ ] URL lạ ⇒ `provider_unknown`, không panic.

## Rủi ro
Self-managed không khai báo host ⇒ không nhận diện.
