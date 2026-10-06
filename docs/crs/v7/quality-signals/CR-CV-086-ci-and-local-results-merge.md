# CR-CV-086 — Gộp kết quả CI/PR (GitHub, GitLab) với kết quả kiểm tra cục bộ

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-086 |
| **Tên** | Nhập kết quả check/pipeline của PR/MR thành `QualityRun.source = "ci"`, ánh xạ check CI → finding/profile, so khớp theo commit SHA với kết quả `local`, xử lý lệch (cục bộ pass, CI fail), tiết kiệm rate limit, trung lập nhà cung cấp |
| **Loại** | Feature (`scm-integration-service` + `code-intel-service`) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-082 (`QualityRun`, `QualityFinding`, bảng `quality_runs/quality_findings`), CR-CV-081 (nhãn `dirty`, `headCommit` của run cục bộ), CR-CV-012 (worktree → repo binding → nhánh), CR-CV-011/013 (repository, phân quyền, audit, hạn mức). Tuỳ chọn: CR-CV-085 (cổng đọc `source`), CR-CV-084 (ánh xạ tên bước) |
| **Mở khoá** | CR-CV-085 (cổng không mâu thuẫn CI/cục bộ), CR-CV-087 (hiển thị nguồn, độ tươi), CR-CV-090 (báo cáo ghi nguồn) |
| **Tác động** | `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` (RPC mới `ListCommitChecks`), `backend-go/services/scm-integration-service/internal/{usecase,adapter/github,adapter/gitlab}` (mới), `backend-go/services/code-intel-service` (importer, bảng/cột), `backend-go/proto/orca/codeintel/v1` (`CiComparison`, RPC `RefreshCiRun`); frontend chỉ đọc qua CR-CV-087 |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-06.

### 1.1 Frontend/desktop hiện lấy checks thế nào

- **Giao diện:** `ChecksPanel.tsx` (right sidebar), `GitHubItemDialog.tsx` (tab checks, `patchCachedPRChecks`), `PullRequestPage.tsx`, `FolderWorkspacePrChecksPanel.tsx`; `ChecksPanel.tsx:307` có `isGitLabChecksPanelReview` nên **GitLab đã được xử lý** ở UI.
- **Slice:** `frontend/src/renderer/src/store/slices/github.ts` — `fetchPRChecks(repoPath, prNumber, branch, headSha, prRepo, options)` (`:3276`), `checksCache`, `CHECKS_CACHE_TTL = 60_000`, `EMPTY_CHECKS_CACHE_TTL = 10_000`, hợp nhất request đang bay (`inflightChecksRequests`), khoá cache theo `sourceScopedRepoCacheKey(... connectionId, executionHostId ...)` (tức có xét SSH). Gọi `window.api.gh.prChecks(...)` (Electron) hoặc `callRuntimeRpc('github.prChecks', ...)` (web).
- **Polling:** `components/right-sidebar/checks-panel-polling.ts` — `CHECKS_PANEL_BASE_POLL_INTERVAL_MS = 30_000`, `MAX = 120_000`, nhân đôi khi chữ ký `name:status:conclusion` không đổi.
- **Phía main (Electron/runtime):** `desktop/src/main/github/client.ts` `getPRChecks(...)`: ưu tiên GraphQL `PR_CHECKS_ROLLUP_QUERY` (`headRefOid`, `statusCheckRollup.contexts(first:100)`, `checkSuites(first:100)`), `gh api graphql --cache 60s` (bỏ cache khi người dùng bấm làm mới), trước đó `assertRateLimitBudget('graphql')`, `acquire()/release()` bể tiến trình `gh`, rồi `noteRateLimitSpend('graphql')`; thất bại → REST `commits/<sha>/check-runs?per_page=100` → `gh pr checks --json name,state,link`. **Tham số `headSha` bị bỏ qua** (`void headSha`, dòng đầu hàm): kết quả luôn là commit mới nhất của PR và `PRCheckDetail` **không mang SHA**. Rate-limit: `desktop/src/main/github/rate-limit.ts` (probe `GET /rate_limit`, cache 30 s) và `git/gh-rate-limit-breaker`.
- **Kiểu:** `desktop/src/shared/types.ts:1277` `PRCheckDetail {name, status: queued|in_progress|completed, conclusion: success|failure|cancelled|timed_out|neutral|skipped|pending|action_required|null, url, checkRunId?, workflowRunId?}`; `PRCheckRunDetails` (có `annotations[{path,startLine,endLine,annotationLevel,title,message,rawDetails}]`, `jobs[{steps[{name,status,conclusion}], logTail}]`).
- **GitLab:** `desktop/src/main/gitlab/client.ts` lấy `head_pipeline`/`pipeline` (trạng thái gộp), `gitlab/work-item-details.ts` `fetchPipelineJobs` gọi `glab api projects/<p>/pipelines/<id>/jobs?per_page=100` (chỉ 100 job đầu), chuyển bằng `shared/gitlab-pipeline-checks.ts` (`gitLabPipelineJobsToPRChecks`) sang `PRCheckDetail`.
- **Quan trọng cho SSH:** toàn bộ đường này chạy `gh`/`glab` trên host của người dùng hoặc host SSH (`connectionId`), bằng đăng nhập CLI của họ.

### 1.2 Backend Go hiện có gì

- `scm-integration-service` có `providerregistry` và adapter `github`, `gitlab`, `gitea`, `bitbucket`, `azuredevops`; proto `scmintegration.proto` có `GetPullRequestForBranch`, `ListPullRequests`, `GetRateLimitStatus` (bảng cache `rate_limit_cache`, `get_rate_limit_status.go` TTL 60 s), `CheckHostedReviewEligibility`; chi tiết work item GitLab (`GetWorkItemDetails → WorkItemDetailsGitLab`).
- **Không có RPC check/pipeline:** `grep -i check|pipeline|ci_` ở proto chỉ ra `CheckHostedReviewEligibility`/`CheckRepositoryStarred`; `adapter/github/client.go` và `adapter/gitlab/client.go` không có `check-runs`/`statusCheckRollup`/`pipelines`. Webhook `ReceiveWebhook` chỉ xử lý sự kiện **merge** (`webhook_parse.go`: `pull_request closed+merged`, GitLab `merge_request action=merge`). Backend dùng token OAuth qua `credentialbroker`, **khác** đăng nhập `gh` của người dùng (quota khác).
- Mẫu "không hỗ trợ" có sẵn: `ListIssueCommentsBySlug` trả `capability_unsupported=true` thay vì lỗi (proto, chú thích).

### 1.3 Vấn đề

`QualityRun.source` đã có `"local"|"ci"` trong README 3.10 nhưng chưa có đường nhập `ci`. Nếu cổng chất lượng chỉ thấy kết quả cục bộ, nó có thể báo `pass` trong khi CI đỏ (hoặc ngược lại), tạo **hai nguồn sự thật mâu thuẫn** (research 11 B7). Mâu thuẫn cần được đặt cạnh nhau, ghi nguồn và commit, không gộp mờ.

## 2. Giải pháp đề xuất

### 2.1 Nguồn dữ liệu: backend đọc qua `scm-integration-service` (mới)

Phương án chọn: **RPC mới ở `scm-integration-service`** — `ListCommitChecks`, provider-neutral, dùng `providerregistry`, kết quả chuẩn hoá. Lý do: (a) lưu lịch sử/so khớp với `quality_runs` ở backend (O14); (b) hoạt động khi không có desktop online; (c) không phụ thuộc `gh` trên máy người dùng hay SSH.

```
ListCommitChecks(ListCommitChecksRequest{
  provider: GITHUB|GITLAB|…, repo_slug, ref_kind: PULL_REQUEST|COMMIT,
  pr_number?, commit_sha?, include_annotations: bool (mặc định false), max_checks ≤ 200 })
→ ListCommitChecksResponse{
  head_sha, provider, items[CommitCheck], overall: PENDING|SUCCESS|FAILURE|NEUTRAL|UNKNOWN,
  fetched_at, capability_unsupported, rate_limit{remaining?, reset_at?}, truncated, total_count }
CommitCheck{ external_id, name, group?  /* workflow hoặc stage */, status, conclusion,
  url?, started_at?, completed_at?, steps[{name,status,conclusion}]?, annotations[]? }
```

- Tên **trung lập** (`CommitCheck`, không `CheckRun`/`Pipeline`). Trạng thái ánh xạ về tập của `PRCheckDetail` (`queued/in_progress/completed` + `conclusion`) để giữ nghĩa với UI hiện có.
- GitHub: một truy vấn GraphQL lấy `headRefOid` + `statusCheckRollup.contexts` (giống `PR_CHECKS_ROLLUP_QUERY`) thay vì mỗi check một lần REST. `steps[]` và `annotations[]` chỉ lấy theo yêu cầu (tab chi tiết) và chỉ cho check **thất bại**, ≤ 50 annotation/check, **không** lấy `logTail`.
- GitLab: `head_pipeline` của MR + `pipelines/:id/jobs` (phân trang, trần 200 job, nhãn `truncated`); `group` = `stage`. Không có annotation (không giả lập).
- Nhà cung cấp khác (Gitea, Bitbucket, Azure DevOps): trả `capability_unsupported=true` (theo mẫu `ListIssueCommentsBySlug`), không lỗi.
- Tách khỏi hành vi hiện có: **không đổi** đường `window.api.gh.prChecks` của frontend (CR này không động tới `ChecksPanel`).

### 2.2 Mô hình trong `code-intel-service`

Dùng lại `quality_runs` / `quality_findings` (CR-CV-082) thêm cột (CR-CV-011 giữ migration, hai dialect):

| Cột (mới) | Ý nghĩa |
|---|---|
| `source` | `local` hoặc `ci` (đã có ở README 3.10) |
| `provider` | `github`/`gitlab`/… khi `source=ci` |
| `external_ref` | `{prNumber|mrIid, headSha, checkIds[]}` (JSON) |
| `external_url` | liên kết tới trang CI, đã bỏ query nhạy cảm |
| `fetched_at`, `stale_after` | độ tươi |
| `dirty` | (cục bộ) có thay đổi chưa commit lúc chạy — lấy từ CR-CV-081/083 |

- **Một `QualityRun(source=ci)` cho mỗi `(repo_binding, provider, head_sha)`**; `profile = "ci"`, `status` suy từ `overall`, `summary` = số check `failure/…`. Check riêng lẻ giữ trong `payload` (≤ 200 mục) không thành hàng riêng.
- Annotation GitHub → `QualityFinding`: `ruleId = "CI/<group>/<name>"` chuẩn hoá (chữ thường, thay ký tự lạ bằng `-`), `severity`: `failure→error`, `warning→warning`, `notice→info`; `category`: ánh xạ theo bảng 2.3, mặc định `test` nếu không biết; `tool = "ci:<provider>"`, `toolVersion` = tên hệ CI; `file`/`line`/`endLine` từ annotation; `fingerprint = sha256(ruleId + file + normalize(message))` (không số dòng). Check thất bại **không** có annotation → một finding mức repo (`file` rỗng) mang `external_url`, để người dùng không thấy "đỏ không rõ vì sao" mà cũng không bịa vị trí.
- Không lưu `logTail`, mã nguồn, hay URL chứa token; `message` cắt ≤ 1 KiB và che chuỗi giống secret theo quy tắc chung (README mục 6).
- Tập dữ liệu cũ hết hạn sau 30 ngày (cấu hình tenant).

### 2.3 Ánh xạ check CI → profile cục bộ và `category`

Cần ánh xạ để so sánh "cùng một kiểm tra". Thực tế `pr.yml` gộp mọi thứ trong **một** job `verify` (các bước Lint, Check styled scrollbars, Typecheck, Test, Build…), còn Go có workflow theo service (`backend-go-<service>.yml`: bước `go build`, `go vet`, `Unit tests`, `Integration tests`). Check CI cấp job là quá thô; ánh xạ phải ở cấp **bước (`steps[]`)**.

Cấu hình (đề xuất) nằm trong `QualityProfile.config.ciMappings` của CR-CV-085 (không thêm bảng):

```yaml
ciMappings:
  - match: { group: "PR Checks", name: "verify", step: "Lint" }
    profile: lint            # profile cục bộ của CR-CV-081
    category: lint
  - match: { group: "PR Checks", name: "verify", step: "Typecheck" }
    profile: typecheck
  - match: { group: "PR Checks", name: "verify", step: "Test" }
    profile: unit
  - match: { group: "backend-go / *", name: "test*", step: "go vet" }
    profile: go-vet
```

- Khớp bằng glob không phân biệt hoa thường; không có biểu thức chính quy do người dùng cung cấp (tránh ReDoS).
- Bộ mặc định ship cùng Orca cho đúng `pr.yml` và `backend-go-*.yml` đã đọc; tenant có thể bổ sung. Bước/check không khớp → chỉ hiển thị (`relation:"ci_only"`), không tham gia so sánh.
- Lấy `steps[]` đòi hỏi gọi thêm API (GitHub: danh sách job của run); chỉ làm khi có `ciMappings` cho check đó và kèm ngân sách rate limit (2.5).

### 2.4 So khớp commit và xử lý lệch

`CiComparison` (mới, trả trong `GetQualityGate`/`ListQualityRuns` mở rộng):

```
CiComparison {
  profile, headCommit,
  local: { runId?, status?, finishedAt?, dirty? },
  ci:    { runId?, status?, url?, fetchedAt?, sha? },
  relation: "agree_pass"|"agree_fail"|"local_pass_ci_fail"|"local_fail_ci_pass"
          | "local_only"|"ci_only"|"ci_pending"|"sha_mismatch"|"not_comparable" }
```

Quy tắc:

1. **So khớp chỉ khi cùng SHA:** `local.headCommit == ci.sha` và `local.dirty == false`. Cục bộ bẩn (`dirty`) → `not_comparable` (cục bộ gồm cả thay đổi chưa push nên CI không thể phản ánh).
2. `ci.sha != HEAD cục bộ` → `sha_mismatch` (vd. CI chạy bản đã push, người dùng/agent đã sửa tiếp): hiển thị cả hai với SHA riêng, không kết luận.
3. CI `in_progress/queued` → `ci_pending`; **không** coi là pass.
4. **`local_pass_ci_fail`** là trường hợp ưu tiên cao nhất: cổng giữ `fail` nếu CI fail ở check bắt buộc (mức bắt buộc do CR-CV-085 quyết), kèm lý do "CI đỏ trong khi cục bộ đạt". Nguyên nhân dự kiến liệt kê trong `reasonsHint[]` để người dùng khỏi đoán: khác môi trường (CI cài `pnpm install --no-frozen-lockfile`, ma trận Git 2.25/2.38/2.49 trong `pr.yml`, hệ điều hành), bộ test tích hợp chỉ có ở CI (`-tags=integration`, Docker), cục bộ chạy `scope:"changed"` chứ không toàn bộ, cache cũ, phiên bản công cụ khác (`toolVersion` so sánh nếu có).
5. `local_fail_ci_pass`: thường do cục bộ chưa đồng bộ nhánh/phụ thuộc hoặc ghi nhận cũ → gợi ý chạy lại; không hạ `fail` cục bộ tự động.
6. Mỗi trường hợp **luôn kèm nguồn + SHA + `fetchedAt`**; `verdict` không bao giờ bị `pass` chỉ vì một phía thiếu (`unknown`, README 3.10).
7. PR/MR do nhánh chưa có PR: `ci` rỗng, `relation:"local_only"`.

### 2.5 Rate limit và độ tươi (AGENTS.md: không gọi thừa `gh`/API)

- **Không polling nền mặc định.** Làm mới chỉ khi: (a) người dùng mở Review/Gate của worktree có PR; (b) một run cục bộ vừa xong cùng SHA đã push; (c) người dùng bấm "làm mới CI" (`RefreshCiRun{force}`); (d) `stale_after` đã qua **và** có người đang xem (RPC có cờ `viewer_active`).
- **Bộ nhớ đệm:** theo `(provider, repo, head_sha)`. Check **đã hoàn tất toàn bộ** là bất biến theo SHA → không gọi lại (TTL dài, tới khi SHA đổi). Có check đang chạy → TTL 30 s tăng gấp đôi đến 120 s (cùng hằng số với `checks-panel-polling.ts`) và chữ ký không đổi thì giãn.
- **Hợp nhất gọi:** `singleflight` theo khoá trên; mọi người xem cùng worktree dùng chung một lần gọi.
- **Ngân sách:** trước mỗi lần gọi đọc `rate_limit_cache`/`GetRateLimitStatus` (đã có, TTL 60 s); `remaining` dưới ngưỡng (đề xuất 10 % hạn mức giờ) → trả bản cũ kèm `stale:true, rateLimited:true, resetAt`, không gọi. Gặp 403/429 có `Retry-After`/reset → đặt "cầu dao" theo `(tenant, provider)` tới `resetAt` (mẫu `gh-rate-limit-breaker` phía desktop; phía Go tái dùng `adapter/backoff`).
- **Một truy vấn cho cả danh sách check** (GraphQL GitHub); annotation/steps chỉ gọi theo nhu cầu và chỉ cho check thất bại/được ánh xạ.
- **Hạn mức theo tenant** (CR-CV-013): ≤ N lần làm mới CI/phút/tenant (đề xuất 30); vượt → `CODEINTEL_RATE_LIMITED`.
- Token backend khác quota `gh` của người dùng; ghi rõ để không kỳ vọng chia sẻ cache với `ChecksPanel`. Làm cầu nối cache chéo là ngoài phạm vi (Q3).
- **Webhook (tuỳ chọn, P2):** mở rộng `ReceiveWebhook` (hiện chỉ merge) cho `check_run`/`workflow_run`/`pipeline` sẽ bỏ polling; cần hạ tầng webhook công khai và xác thực chữ ký (`webhookverify` có sẵn). Chưa đề xuất làm trong CR này.

### 2.6 Giới hạn dữ liệu

| Hạng mục | Trần mặc định |
|---|---|
| check/job mỗi SHA | 200 (`truncated`, `total_count`) |
| annotation mỗi check thất bại | 50 |
| tổng finding từ CI mỗi run | 500 |
| `payload` run CI | ≤ 256 KiB |
| `message` finding | 1 KiB |
| số SHA giữ mỗi worktree | 20 hoặc 30 ngày |

### 2.7 RPC `RefreshCiRun` và kênh

`RefreshCiRun{worktree_id, force?: bool}` → `{run: QualityRun, comparison: CiComparison[], stale, rateLimited, resetAt?}`. Bổ sung vào `QualityGateService` và kênh `codeIntel.quality.ci` (**bổ sung ngoài README 3.10**, ghi ở mục "Điều chỉnh hợp đồng" của README folder). Phát `orca.codeintel.quality.run_finished` với `source:"ci"` khi nhập xong.

### 2.8 SSH, GitLab, ngoài GitHub

- Backend không phụ thuộc host người dùng nên chạy như nhau với dev server SSH; nhánh/PR lấy từ `repo_bindings` + `GetPullRequestForBranch` (đã có, provider-neutral).
- Tên trong proto và code: `CommitCheck`, `provider` — không `Github*`.
- GitLab self-hosted: URL cơ sở từ cấu hình tích hợp; không giả định `gitlab.com`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| C1 | Nhập CI ở backend qua `scm-integration-service` | Lịch sử, hoạt động khi desktop tắt, SSH-độc lập; không đổi `ChecksPanel` |
| C2 | So khớp chỉ khi cùng SHA và cục bộ sạch | Tránh so sánh táo với cam |
| C3 | Hiển thị mâu thuẫn, không "hoà giải" tự động | Hai nguồn sự thật phải thấy được (research 11 B7) |
| C4 | Ánh xạ ở cấp bước (step), cấu hình được | `pr.yml` gộp mọi thứ trong một job |
| C5 | Không polling nền; cache bất biến theo SHA hoàn tất | Rate limit `gh`/API (AGENTS.md) |
| C6 | Không lưu log CI | Dễ chứa secret; ngoài nhu cầu |
| C7 | `capability_unsupported` cho nhà cung cấp chưa làm | Mẫu hiện có; không lỗi giả |

## 4. Tiêu chí chấp nhận

- [ ] `ListCommitChecks` GitHub: một truy vấn GraphQL trả đủ check (≤ 200) và `head_sha`; kiểm thử có fake `gh`/HTTP.
- [ ] `ListCommitChecks` GitLab: đọc `head_pipeline` + job, nhãn `truncated` khi > 200; không annotation.
- [ ] Nhà cung cấp chưa hỗ trợ → `capability_unsupported=true`, không lỗi.
- [ ] Check hoàn tất toàn bộ cho một SHA không bị gọi lại khi làm mới lần hai (đếm số cuộc gọi provider = 1).
- [ ] Hai yêu cầu đồng thời cùng `(provider, repo, sha)` chỉ gọi provider một lần.
- [ ] Hết quota (hoặc `remaining` dưới ngưỡng) → không gọi provider, trả bản cũ `stale:true, rateLimited:true, resetAt`.
- [ ] 403/429 mở "cầu dao" theo `(tenant, provider)` đến `resetAt`.
- [ ] `QualityRun(source=ci)` có `provider`, `external_ref.headSha`, `fetched_at`; annotation → `QualityFinding` có `file/line`, `fingerprint` không chứa số dòng.
- [ ] Check đỏ không annotation → đúng một finding mức repo có `external_url`.
- [ ] `CiComparison.relation` đúng cho bảng ca: cùng SHA sạch (4 tổ hợp pass/fail), `dirty`, `sha_mismatch`, `ci_pending`, `local_only`, `ci_only`.
- [ ] `local_pass_ci_fail` luôn kèm `reasonsHint[]` và không làm `verdict=pass`.
- [ ] Khi thiếu một phía, `verdict` là `unknown` không phải `pass`.
- [ ] Không có `logTail`, token, hay query nhạy cảm trong dữ liệu lưu; `message` được che.
- [ ] Cô lập tenant: tenant B không đọc/ghi được run CI của tenant A.
- [ ] Hạn mức làm mới/tenant được áp (`CODEINTEL_RATE_LIMITED`).
- [ ] Không đổi hành vi `window.api.gh.prChecks`/`ChecksPanel` (test hiện có giữ nguyên).

## 5. Kiểm thử

- Hợp đồng adapter: fixture JSON thật rút gọn (GraphQL rollup, REST check-runs, GitLab pipelines/jobs) cho từng nhà cung cấp; golden theo phiên bản API.
- Bảng quyết định `CiComparison` (đơn vị thuần, bảng ca ở mục 4).
- Rate limit: giả lập `X-RateLimit-Remaining`, `Retry-After`, đồng thời hàng loạt; kiểm đếm số lần gọi.
- Tích hợp hai dialect cho cột mới (testcontainers theo mẫu `backend-go-task-service.yml`).
- Bảo mật: finding/payload không chứa chuỗi giống token (kiểm bằng tập mẫu); cô lập tenant (CR-CV-072).
- Không chạy ở thời điểm soạn; chỉ mô tả.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa kiểm chứng** quyền token OAuth của backend đủ đọc check-runs/annotations/pipelines (scope `checks:read`/`read_api`); nếu thiếu → `capability_unsupported` hoặc lỗi quyền cần ánh xạ rõ.
- **Chưa kiểm chứng** quota GraphQL/REST thực tế khi nhiều worktree xem cùng lúc; ngưỡng 10 % và hạn mức 30 lần/phút là ước lượng.
- `getPRChecks` hiện bỏ qua `headSha` và `PRCheckDetail` không mang SHA: ở phía frontend không thể so khớp SHA nếu dùng dữ liệu này; vì vậy CR chọn nguồn backend. Nếu cần so khớp tạm ở UI, phải mở rộng kiểu này (ngoài phạm vi).
- Ánh xạ bước phụ thuộc **tên bước trong workflow** (`pr.yml`: "Lint", "Typecheck", "Test"): đổi tên làm mất khớp → cần `ciMappings` có phiên bản và cảnh báo "check không khớp ánh xạ nào".
- CI của Orca dùng bước/đường dẫn script đã lệch cây (xem CR-CV-084 1.2); so sánh "cục bộ đạt/CI đỏ" có thể do lệch này chứ không do mã.
- PR từ fork, check của bên thứ ba, và check `action_required` (cần phê duyệt chạy) chưa được xử lý chi tiết; ánh xạ về `pending`/`not_comparable`.
- GitLab: pipeline MR (`merge_request_event`) và nhánh khác nhau; `head_pipeline` có thể là pipeline hợp nhất kết quả; chưa thử với instance thật.
- Dữ liệu CI có thể thay đổi sau khi lưu (chạy lại job) → run `ci` là ảnh chụp tại `fetched_at`.
- Webhook merge-only hiện tại nghĩa là "đẩy" chưa có; độ tươi dựa vào kéo theo yêu cầu.

## 7. Câu hỏi mở

- **Q1.** Check CI nào là **bắt buộc** cho cổng (theo branch protection của nhà cung cấp hay cấu hình của Orca)?
- **Q2.** Có chấp nhận thêm RPC/kênh `RefreshCiRun` ngoài danh sách README 3.10 không?
- **Q3.** Có muốn chia sẻ cache với `ChecksPanel` (cùng người dùng) để tránh hai nguồn gọi, hay chấp nhận hai quota?
- **Q4.** Nhà cung cấp ưu tiên sau GitHub/GitLab: Azure DevOps, Gitea, Bitbucket?
- **Q5.** Có mở rộng webhook (check_run/pipeline) trong series này không?
- **Q6.** Ngưỡng thời gian giữ run CI và số lần làm mới/tenant.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.10, O9, O14)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (B7)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/github.ts` (`fetchPRChecks`, `checksCache`)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/ChecksPanel.tsx`, `.../checks-panel-polling.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/GitHubItemDialog.tsx`
- `/opt/repos/orca/desktop/src/main/github/client.ts` (`getPRChecks`, `PR_CHECKS_ROLLUP_QUERY`), `/opt/repos/orca/desktop/src/main/github/rate-limit.ts`
- `/opt/repos/orca/desktop/src/main/gitlab/client.ts`, `/opt/repos/orca/desktop/src/main/gitlab/work-item-details.ts`, `/opt/repos/orca/desktop/src/shared/gitlab-pipeline-checks.ts`, `/opt/repos/orca/desktop/src/shared/types.ts` (`PRCheckDetail`)
- `/opt/repos/orca/backend-go/proto/orca/scmintegration/v1/scmintegration.proto`, `/opt/repos/orca/backend-go/services/scm-integration-service/internal/usecase/get_rate_limit_status.go`, `.../usecase/webhook_parse.go`, `.../adapter/{github,gitlab,backoff,providerregistry}`
- `/opt/repos/orca/.github/workflows/pr.yml`, `/opt/repos/orca/.github/workflows/backend-go-task-service.yml`
- `/opt/repos/orca/AGENTS.md` (GitHub CLI Usage, Git Provider Compatibility, SSH)
- Cùng folder: `./README.md`; CR-CV-081, CR-CV-082 (tham chiếu theo ID)
