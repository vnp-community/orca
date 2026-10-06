# BE-CV-SOL-086-scm-commit-checks: RPC `ListCommitChecks` ở `scm-integration-service` (GitHub, GitLab; provider khác `capability_unsupported`)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Nửa "nguồn dữ liệu" của CR-CV-086; nửa nhập và so sánh ở [BE-CV-SOL-086-ci-run-merge-and-comparison](./BE-CV-SOL-086-ci-run-merge-and-comparison.md).

**CR:** [CR-CV-086](../../../../../../docs/crs/v7/quality-signals/CR-CV-086-ci-and-local-results-merge.md) (2.1, 2.5, 2.6, 2.8)
**Service:** `scm-integration-service` · `proto/orca/scmintegration/v1`
**TDD tham chiếu:** [`services/scm-integration-service.md`](../../../../tdd/services/scm-integration-service.md) §3 (API), §8 (rate limit là ràng buộc hạng nhất, breaker theo provider), §9 (token không log, scope tối thiểu); [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md); [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (OAuth scope tối thiểu); [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (deadline, buf breaking)

## 0. Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-25 | Chấp nhận `scmintegration.ListCommitChecks`; `overall` ∈ `PENDING\|SUCCESS\|FAILURE\|NEUTRAL\|UNKNOWN` (bảng ánh xạ sang `QualityRun.status` thuộc SOL-086-ci) |
| C-DM §2.4 | Message `CommitCheck, CheckStep, CheckAnnotation, RateLimitInfo`; request `provider, repo_slug, ref_kind (PULL_REQUEST\|COMMIT), pr_number, commit_sha, include_annotations, max_checks ≤ 200`; response `head_sha, provider, items[], overall, fetched_at, capability_unsupported, rate_limit, truncated, total_count`; provider chưa hỗ trợ ⇒ `capability_unsupported=true`, không lỗi; **không đổi** `window.api.gh.prChecks` |
| C-DM §3.3 | Client là code-intel-service (`ScmIntegrationService`) |
| PQ-03 | `common/apperrors` có `KindResourceExhausted`/`KindUnavailable` sau SOL-010 (dùng cho lỗi hạ tầng) |
| H8 | Không `logTail`, không token, không URL chứa token; `message` che |
| H9, AGENTS.md | Không phụ thuộc `gh`/`glab` của người dùng hay SSH; GitLab (kể cả self-managed) ngang hàng GitHub; tên trung lập (`CommitCheck`) |
| §8.3 | Test hai dialect **không áp dụng** (không DB mới); workflow `backend-go-scm-integration-service.yml` giữ nguyên ma trận |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` (service :12, `ListIssueCommentsBySlug`, `GetPullRequestForBranch*` :460, `ListMergeRequests*` :695, `GetRateLimitStatus*`, `ScmProvider` enum, 847 dòng), `services/scm-integration-service/internal/usecase/{ports.go (524 dòng), get_rate_limit_status.go, get_pull_request_for_branch.go, list_issue_comments_by_slug.go}`, `internal/domain/scm.go`, `internal/adapter/github/client.go` (`DefaultGraphQLURL` cố định, `graphQLRequest`, `GetRateLimitStatus`), `internal/adapter/gitlab/client.go` (`ListMergeRequests`, `GetPullRequestForBranch` trả `ErrCapabilityUnsupported`), `internal/adapter/backoff/backoff.go` (`isNonTransient` dò chuỗi `status 4xx`), `internal/adapter/grpc/server.go` (`New` ~47 tham số vị trí), `cmd/server/main.go`, `internal/adapter/postgres/rate_limit_cache.go` (bucket luôn `core`), `.github/workflows/backend-go-scm-integration-service.yml`, `desktop/src/main/github/client.ts` (`PR_CHECKS_ROLLUP_QUERY`), `.github/workflows/pr.yml`. Không có RPC/adapter check nào (đã grep trong CR; đối chiếu `ports.go`: không có).

### Correction relative to CR / hợp đồng

| # | CR/hợp đồng nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | `GetPullRequestForBranch` "provider-neutral" (CR-086 2.8) | GitLab adapter trả `ErrCapabilityUnsupported` (`gitlab/client.go`); GitLab dùng port riêng `GitLabMergeRequestProvider.ListMergeRequests` | SOL-086-ci dùng `ListMergeRequests(source_branch)` cho GitLab; C-DM §3.3 **thiếu** RPC này trong danh sách client (mục 7) |
| C2 | Request không có `tenant_id` (CR/C-DM §2.4) | Mọi RPC scm hiện có mang `tenant_id` trong message (ví dụ `GetRateLimitStatusRequest`); credential resolve cần `tenant_id` | Thêm `tenant_id = 1`; use case đối chiếu với `tenant.TenantID(ctx)` nếu ctx có (từ chối khi khác). Lệch arch/08 ("tenant qua metadata") nhưng đúng quy ước scm hiện có |
| C3 | "Một truy vấn GraphQL" cho GitHub | `graphQLURL` cố định `https://api.github.com/graphql` (không suy từ `baseURL`); GHE không hỗ trợ GraphQL ở adapter | GraphQL khi `baseURL == DefaultBaseURL`; ngược lại REST (`/commits/{sha}/check-runs` + `/commits/{sha}/status`) |
| C4 | `steps[]` lấy theo yêu cầu | Request không có cờ steps; chỉ `include_annotations` | Thêm `include_steps` (additive); lệch hợp đồng (mục 7) |
| C5 | `rate_limit{remaining?, reset_at?}` | Không có cách báo "đã bị giới hạn" | `RateLimitInfo` thêm `bool limited`; khi 403/429, **không lỗi**: trả `items=[]`, `overall=UNKNOWN`, `limited=true`, `reset_at` (mục 7) |
| C6 | Ghi `rate_limit_cache` từ header mỗi lời gọi (TDD §8) | Cache chỉ có bucket `core`; số GraphQL thuộc bucket khác | **Không** ghi số GraphQL vào cache `core`; chỉ trả trong response. Breaker/ngân sách do caller giữ |
| C7 | Thêm phương thức vào `ScmProvider` | Có 5 adapter; thêm vào interface buộc thêm stub ở 3 adapter không hỗ trợ | Port riêng tuỳ chọn `CommitCheckProvider` (khuôn `WorkItemProvider`); adapter không cài ⇒ `capability_unsupported=true` |
| C8 | Go CI 1.25 | `go.work` 1.26 | Không dùng API chỉ có ở 1.26 |

## 2. Giải pháp

### A. Cây file

```
backend-go/proto/orca/scmintegration/v1/scmintegration.proto           # SỬA: rpc + message cuối file
backend-go/services/scm-integration-service/internal/
  domain/commit_check.go                 # CommitCheck, CheckStep, CheckAnnotation, ComputeOverall, chuẩn hoá status/conclusion
  domain/provider_rate_limited.go        # ProviderRateLimitedError{ResetAt, RetryAfter}
  usecase/commit_check_provider.go       # port CommitCheckProvider, ListCommitChecksQuery/Result
  usecase/list_commit_checks.go          # use case
  adapter/github/commit_checks.go        # GraphQL + REST fallback
  adapter/github/commit_check_details.go # annotations, steps (theo yêu cầu)
  adapter/gitlab/commit_checks.go        # pipeline jobs
  adapter/grpc/server_commit_checks.go   # handler (+ WithCommitChecks)
```

### B. Proto

Thêm `rpc ListCommitChecks(ListCommitChecksRequest) returns (ListCommitChecksResponse);` vào `ScmIntegrationService`, message ở cuối file. Số field do chủ CR gán theo thứ tự khai báo (C-DM §2). Enum `CommitCheckRefKind {…_UNSPECIFIED=0, PULL_REQUEST=1, COMMIT=2}`, `CommitCheckOverall {…_UNSPECIFIED=0, PENDING, SUCCESS, FAILURE, NEUTRAL, UNKNOWN}`; `status`/`conclusion` là `string` theo từ vựng `PRCheckDetail` (`queued|in_progress|completed`; `success|failure|cancelled|timed_out|neutral|skipped|pending|action_required|""`). `ScmProvider provider` dùng enum hiện có. File proto hiện chưa sạch `buf lint` STANDARD (RPC trả `PullRequest` dùng chung); `make proto-lint` có `|| true`: CI phải gọi `buf breaking` trực tiếp.

### C. Port và use case

`CommitCheckProvider.ListCommitChecks(ctx, cred, q) (CommitCheckList, error)`; `q{Repo, RefKind, PRNumber, CommitSHA, IncludeAnnotations, IncludeSteps, Max}`. Use case: kiểm `tenant_id` (+ctx), `provider.Valid()`, `repo_slug` an toàn (không `..`, không NUL; GitHub `owner/name`, GitLab đường dẫn nhiều cấp), `ref_kind` đúng, `pr_number > 0` khi PR, `commit_sha ^[0-9a-f]{7,64}$` khi COMMIT, `max` mặc định 100, 1..200 (ngoài khoảng: `INVALID_ARGUMENT`, không kẹp ngầm); resolve credential; resolve provider; type-assert `CommitCheckProvider` ⇒ không có thì trả `capability_unsupported=true`. `ProviderRateLimitedError` ⇒ response `limited` (C5). Lỗi khác bọc `apperrors` (`SCM_LIST_COMMIT_CHECKS_FAILED`), **không** đưa body provider hay token vào thông điệp.

### D. Adapter GitHub

- PR: GraphQL `repository(owner,name){ pullRequest(number){ headRefOid commits(last:1){nodes{commit{statusCheckRollup{state contexts(first:100){totalCount nodes{__typename ... on CheckRun{databaseId name status conclusion detailsUrl startedAt completedAt checkSuite{workflowRun{workflow{name}}}} ... on StatusContext{context state targetUrl createdAt}}}}}}} } }`, bắt nguồn từ `PR_CHECKS_ROLLUP_QUERY` của desktop (đã đọc) — **hình dạng chưa chạy**; thêm `rateLimit{remaining resetAt}` để điền `rate_limit`.
- COMMIT: `object(oid:){... on Commit{statusCheckRollup…}}`.
- `group` = tên workflow (`workflowRun.workflow.name`); `StatusContext` có `group=""`.
- `head_sha` = `headRefOid` (PR) hoặc `commit_sha`.
- REST (GHE): `GET /repos/{r}/commits/{sha}/check-runs?per_page=100` và `/status`; với PR cần `GET /repos/{r}/pulls/{n}` lấy `head.sha`.
- Chi tiết theo yêu cầu, **chỉ check thất bại**: annotations `GET /repos/{r}/check-runs/{id}/annotations?per_page=50` (mức `failure|warning|notice`); steps `GET /repos/{r}/actions/jobs/{id}` (id check run = id job Actions; chỉ khi `workflowRun` có). Không lấy log.
- Header: `X-RateLimit-Remaining/Reset`, `Retry-After`; 403 kèm `retry-after` hoặc `remaining=0` ⇒ `ProviderRateLimitedError`; 429 tương tự.

### E. Adapter GitLab

PR: `GET /projects/:id/merge_requests/:iid` → `sha`, `head_pipeline{id,sha,status,web_url}` (nếu thiếu: `/merge_requests/:iid/pipelines` lấy đầu); COMMIT: `GET /projects/:id/pipelines?sha=<sha>&per_page=1&order_by=id&sort=desc`. Job: `GET /projects/:id/pipelines/:pid/jobs?per_page=100&page=1..2` (trần 200, `truncated`+`total_count`). `group` = `stage`. Trạng thái job → `status/conclusion`: `created|pending|waiting_for_resource|preparing|scheduled`→`queued`; `running`→`in_progress`; `success`→`completed/success`; `failed`→`completed/failure`; `canceled`→`completed/cancelled`; `skipped`→`completed/skipped`; `manual`→`completed/action_required` (chờ thao tác). Không annotation (không giả lập). `head_sha` = `sha` của MR (nguồn), không phải sha pipeline kết quả gộp. `RateLimit-*`, `Retry-After`; `baseURL` từ `cfg.GitLabBaseURL` (không giả định gitlab.com). Chưa thử trên instance thật.

### F. `ComputeOverall`

Thứ tự: có check chưa `completed` ⇒ `PENDING`; có `failure|timed_out` ⇒ `FAILURE`; có `action_required|cancelled` ⇒ `UNKNOWN` (cần người/không kết luận); có ít nhất một `success` ⇒ `SUCCESS`; chỉ `neutral|skipped` ⇒ `NEUTRAL`; danh sách rỗng ⇒ `UNKNOWN`. Bảng quyết định trong `domain`, có test.

### G. Wiring

`New(...)` của server đã ~47 tham số vị trí: thêm `(*Server).WithCommitChecks(uc)` thay vì tham số thứ 48. `main.go` tạo use case với `credentials`, `registry`. `registry` trả cùng `*github.Client`/`*gitlab.Client` nên type-assert đạt.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Port tuỳ chọn thay vì mở rộng `ScmProvider` | Không stub thừa ở 3 adapter; mẫu `WorkItemProvider` |
| Rate-limited không là lỗi | Caller cần `reset_at` để mở breaker; tránh lẫn với lỗi mạng |
| Không ghi cache `core` từ GraphQL | Tránh làm sai `GetRateLimitStatus` |
| Annotations/steps chỉ cho check thất bại | Tiết kiệm quota (AGENTS.md "GitHub CLI Usage": hạn chế gọi) |
| Không log, không lưu log CI | H8, C6 của CR |

## 4. Tiêu chí chấp nhận

- [ ] GitHub PR: một lời gọi GraphQL trả ≤ 200 check và `head_sha`; GHE đi REST (httptest).
- [ ] GitLab: `head_pipeline` + job, `truncated` khi > 200, không annotation.
- [ ] Bitbucket/Azure DevOps/Gitea ⇒ `capability_unsupported=true`, không lỗi.
- [ ] 403/429 ⇒ response `limited=true` + `reset_at`, không retry, không gọi thêm.
- [ ] Check đã `completed` toàn bộ: `overall` đúng bảng F; rỗng ⇒ `UNKNOWN`.
- [ ] `max_checks` ngoài 1..200, `commit_sha` sai, `repo_slug` có `..` ⇒ `INVALID_ARGUMENT`.
- [ ] Không chuỗi token, `logTail`, URL có query nhạy cảm trong response/log (test canary).
- [ ] `tenant_id` khác ctx ⇒ từ chối.
- [ ] `window.api.gh.prChecks`/`ChecksPanel` không đổi (không file frontend nào bị sửa).
- [ ] Không `max-lines` disable; tên file không `helpers/utils/common/misc`.

## 5. Kiểm thử (chưa chạy test nào)

Unit: `commit_check_test.go` (bảng `ComputeOverall`), `list_commit_checks_test.go` (cổng giả, ma trận provider). Adapter: httptest với fixture JSON rút gọn dựng từ tài liệu API và `PR_CHECKS_ROLLUP_QUERY` (**chưa có mẫu thật**; ghi rõ trong tệp fixture). Rate limit: header giả `Retry-After`, `X-RateLimit-Reset`, 403 secondary. gRPC: handler + `buf lint/breaking`.

## 6. Rủi ro và chưa kiểm chứng

- Quyền token OAuth của backend (`checks:read`/`read_api`, scope `repo`) chưa kiểm chứng; thiếu quyền ⇒ cần ánh xạ lỗi quyền rõ (CR-086 6).
- Hình dạng GraphQL/REST chưa chạy; GitLab `head_pipeline` có thể là pipeline kết quả gộp (sha khác).
- `ListCommitChecks` không bị `internalcaller` bảo vệ (giống mọi RPC scm hiện có); cân nhắc `Guard` nếu mở rộng bề mặt.
- Quota GraphQL khi nhiều worktree xem cùng lúc: ngân sách ở caller, chưa đo.
- SSH: không liên quan (backend gọi API trực tiếp). Git: không chạy lệnh git.

## 7. Điểm hợp đồng thiếu/mâu thuẫn (không tự sửa)

1. Request thiếu `tenant_id`, `include_steps`; `RateLimitInfo` thiếu `limited` (C2, C4, C5).
2. C-DM §3.3 thiếu `ScmIntegrationService.ListMergeRequests` (cần cho GitLab).
3. C-DM §6.2 thiếu biến `SCM_INTEGRATION_SERVICE_ADDR` của code-intel-service (đã có tên này ở service khác).

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-010` (`KindResourceExhausted`), `BE-CV-SOL-086-ci-run-merge-and-comparison` (client duy nhất) | G0 cho `apperrors` |
| AG | không có | CR-086 không có việc agent |
| FE | `FE-CV-SOL-087-quality-scorecard-and-state` (gián tiếp qua `comparison`) | `ChecksPanel` không đổi |

## 9. Câu hỏi mở

- **Q1.** Nhà cung cấp ưu tiên sau GitHub/GitLab (Q4 CR-086)? **Q2.** Guard `internalcaller` cho RPC này? **Q3.** Webhook `check_run`/pipeline (P2) có làm trong series? Mặc định: không.

## 10. Tham chiếu

[CR-CV-086](../../../../../../docs/crs/v7/quality-signals/CR-CV-086-ci-and-local-results-merge.md); C-DM PQ-03, PQ-25, §2.4, §3.3; `/opt/repos/orca/AGENTS.md` (GitHub CLI usage, Git Provider Compatibility, SSH).
