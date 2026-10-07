# BE-CV-SOL-086-ci-run-merge-and-comparison: Nhập run CI (`source='ci'`), `RefreshCiRun`, `CiComparison` cục bộ so với CI

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Nửa "nhập và so sánh" của CR-CV-086 ở `code-intel-service` (mới); nguồn dữ liệu là [BE-CV-SOL-086-scm-commit-checks](./BE-CV-SOL-086-scm-commit-checks.md).

**CR:** [CR-CV-086](../../../../../../docs/crs/v7/quality-signals/CR-CV-086-ci-and-local-results-merge.md)
**Service:** `code-intel-service` · `proto` (`codeintel_ci.proto`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (cô lập tenant, audit), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (deadline mọi lời gọi ra, outbox), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (retry chỉ cho gọi idempotent, bulkhead), [`services/scm-integration-service.md`](../../../../tdd/services/scm-integration-service.md) §8 (rate limit)

## 0. Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| **PQ-25** | `RefreshCiRun`, kênh `codeIntel.quality.ci`, `CiComparison`. `QualityRun.source='ci'`: `PENDING→running`, `SUCCESS\|NEUTRAL→succeeded`, `FAILURE→failed`, `UNKNOWN→failed` + `error_code='CODEINTEL_CI_RESULT_UNKNOWN'` (cổng coi `unknown`). `relation` 9 giá trị; `local_pass_ci_fail` luôn có `reasonsHint[]` và không bao giờ `pass` |
| **PQ-26** | `ruleId` CI = `CI/<group>/<name>` khớp `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$` (chuẩn hoá chữ thường, ký tự lạ → `-`, cắt 128) |
| PQ-04 | Request có `selector`; bảng khoá `repo_binding_id` |
| PQ-05, PQ-33 | `repo_id` là khoá nghiệp vụ; `unknown` là trạng thái thật (H7) |
| PQ-24 | Cờ: `quality_gate_enabled`; lỗi đọc cờ = tắt (`CODEINTEL_QUALITY_GATE_DISABLED`) |
| PQ-13 | Khi vượt 20 s: `CODEINTEL_TIMEOUT` + `{"retryAfterMs":3000,"inProgress":true}` và hoàn tất nền (singleflight) |
| C-DM §3.2 | `RefreshCiRun{force?}` → `{run, comparison[], stale, rate_limited, reset_at}`; action `quality_read`; kênh `codeIntel.quality.ci`; `GetQualityGate` kèm `comparison[]` |
| C-DM §4.2 T8, T12 | `quality_runs` đã có cột `provider, external_ref, external_url, fetched_at, stale_after, dirty` (SOL-082); UNIQUE `(tenant_id, repo_binding_id, agent_run_id)` với `agent_run_id = ci:<provider>:<sha>`; run CI **không** đặt `active_key` |
| C-DM §5 | Phát `orca.codeintel.quality.run_finished` với `source:"ci"` khi run CI sang trạng thái kết thúc |
| C-DM §3.3 | Client: `ScmIntegrationService.ListCommitChecks`, `GetPullRequestForBranch`, `GetRateLimitStatus` |
| C-UI §4.7 | Hình dạng `CiComparison`, `QualityRun.ci` |
| §8.3 | Hai dialect, `tenant_id`, cô lập tenant |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: ba file hợp đồng; `desktop/src/main/github/client.ts` (`PR_CHECKS_ROLLUP_QUERY`), `.github/workflows/pr.yml` (workflow `PR Checks`, các bước `Lint`, `Check styled scrollbars`, `Check reliability gate manifest`, `Enforce max-lines ratchet (no new bypasses)`, `Typecheck`, `Test`, `Build unpacked app`…), `.github/workflows/backend-go-scm-integration-service.yml` (workflow `backend-go / scm-integration-service`, job `test`, bước `go build`, `go vet`, `Unit tests (no Docker)`, `Integration tests (…)`), `backend-go/proto/orca/project/v1/project.proto` (`Repo{url, dev_server_id}`, `Worktree{branch, repo_id, path}`; **không có trường provider/slug**), `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`, `backend-go/services/scm-integration-service/…` (xem SOL nguồn), `common/eventbus/eventbus.go`, `common/outbox/outbox.go`. Kiểm bằng `ls`: `code-intel-service` và `proto/orca/codeintel` chưa tồn tại. `SCM_INTEGRATION_SERVICE_ADDR` (mặc định `scm-integration-service:9090`) đã được dùng ở service khác (grep).

### Correction relative to CR / hợp đồng

| # | CR-086 nói | Hợp đồng / mã thật | Xử lý |
|---|---|---|---|
| C1 | `quality_runs` cần thêm cột CI "CR-CV-011 giữ migration" | C-DM T8/§4.2: cột CI nằm sẵn trong `0003` (SOL-082) | Không migration mới ở solution này |
| C2 | `ci` là thuộc tính của run; so sánh cần ánh xạ `QualityProfile.config.ciMappings` (CR-085) | `QualityProfileDefinition` (C-UI §4.7) **không có** `ciMappings` | MVP dùng bảng mặc định tích hợp trong code (khớp `pr.yml` và `backend-go-*.yml` đã đọc); mở rộng theo tenant chờ SOL-085 thêm trường (additive) |
| C3 | Nhánh/PR "lấy từ `repo_bindings` + `GetPullRequestForBranch` provider-neutral" | GitLab adapter trả `ErrCapabilityUnsupported` cho `GetPullRequestForBranch`; `Repo` không mang provider | GitHub: `GetPullRequestForBranch`; GitLab: `ListMergeRequests(state=opened, source_branch)` lấy `iid`. Provider/slug suy từ `Repo.url` (2.D). Cần C-DM §3.3 bổ sung RPC (mục 7) |
| C4 | Finding CI mang `external_url` | `quality_findings` không có cột URL (T9) | `external_url` ở mức run; finding mức repo chỉ mang tên check và trạng thái |
| C5 | Giữ run CI 30 ngày; ≤ 20 SHA/worktree (2.6) | C-DM §4.3: `quality_runs` 180 ngày | Theo hợp đồng 180 ngày cho bản ghi; thêm trần 20 run `source='ci'` mỗi binding (CR-086 2.6, hợp đồng không cấm) |
| C6 | `fingerprint = sha256(ruleId+file+normalize(message))` | C-AG §5.5 fingerprint agent có `tool`, `anchor`, `occurrence` | Giữ công thức CR cho `tool='ci:<provider>'` (không so sánh chéo với agent; `fp_version=1`); thêm chỉ số `occurrence` để phân biệt annotation trùng |
| C7 | `scope` của run CI | T8 CHECK `worktree\|changed\|commitRange`, hợp đồng không nói với `source='ci'` | Dùng `worktree` (kết luận về HEAD); câu hỏi mở Q4 |
| C8 | Ngân sách 10% hạn mức giờ, 30 lần/phút/tenant | Con số ước lượng CR, chưa đo | Cấu hình `CODEINTEL_CI_REFRESH_PER_MINUTE`=30, `CODEINTEL_CI_RATE_BUDGET_PERCENT`=10 |
| C9 | Biến địa chỉ scm | C-DM §6.2 không liệt kê | Dùng `SCM_INTEGRATION_SERVICE_ADDR` (tên đã có ở service khác); mục 7 |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_ci.proto            # CiComparison, CiRunRef, RefreshCiRun{Request,Response}
backend-go/services/code-intel-service/internal/
  domain/ci_check_mapping.go          # bảng ánh xạ mặc định check/step -> profile cục bộ, category
  domain/ci_run_builder.go            # CommitCheckList -> QualityRun + QualityFinding (PQ-25, PQ-26)
  domain/ci_comparison.go             # CompareLocalAndCI (9 relation) + reasonsHint
  domain/hosted_review_ref.go         # parse remote URL -> provider, slug
  usecase/ci_ports.go                 # ScmCommitChecksClient, HostedReviewLocator, CiRunRepository, CiRefreshLimiter
  usecase/refresh_ci_run.go
  usecase/ci_comparison_service.go    # dùng bởi GetQualityGate (SOL-085)
  adapter/grpcclient/scmintegration/client.go
  adapter/postgres/ci_run_repository.go ; adapter/mysql/ci_run_repository.go
  config/ci_refresh.go
```

### B. Proto `codeintel_ci.proto`

`CiRunRef{provider, head_sha, url, fetched_at, stale_after}`, `CiComparison{profile, head_commit, local{run_id,status,finished_at,dirty}, ci{run_id,status,url,fetched_at,sha}, relation, reasons_hint[]}` (C-UI §4.7), `RefreshCiRunRequest{selector, force}`, `RefreshCiRunResponse{run, comparison[], stale, rate_limited, reset_at}`. `relation` là `string` (9 giá trị). Dòng `rpc RefreshCiRun` thuộc `service QualityGateService` (file của SOL-085).

### C. Bảng ánh xạ mặc định (đọc từ `pr.yml`, `backend-go-*.yml`)

| Check CI (workflow / bước) | Profile cục bộ (tên **đề xuất** C-AG §5.1) | Category |
|---|---|---|
| `PR Checks` / `Lint` | `ts-lint` | lint |
| `PR Checks` / `Check styled scrollbars` | `repo-check-styled-scrollbars` | convention |
| `PR Checks` / `Check reliability gate manifest` | `repo-check-reliability-gates` | convention |
| `PR Checks` / `Enforce max-lines ratchet…` | `repo-check-max-lines` | convention |
| `PR Checks` / `Typecheck` | `ts-typecheck-*` (tất cả) | typecheck |
| `PR Checks` / `Test` | `ts-unit-*` (tất cả) | test |
| `backend-go / <service>` / `go vet` | `go-vet` | lint |
| `backend-go / <service>` / `Unit tests (no Docker)` | `go-test` | test |
| `backend-go / <service>` / `Integration tests (*)` | (không có local) ⇒ `ci_only` | test |

Khớp glob không phân biệt hoa thường, **không** regex từ người dùng (ReDoS). Tên profile cục bộ do `AG-CV-SOL-081-quality-profile-catalog-and-preflight` chốt; bảng là dữ liệu có phiên bản, kèm cảnh báo `ci_check_unmapped` khi có check không khớp. Script `check-*` nằm ở `desktop/config/scripts/` (repo đã tách); chỉ ảnh hưởng tên profile cục bộ, không ảnh hưởng tên bước CI.

### D. `RefreshCiRun` (use case)

1. **Cổng**: `quality_gate_enabled` (lỗi = tắt), OPA `quality_read` (SOL-013), phiên thiết bị bị từ chối; phân giải `selector`→binding (SOL-012).
2. **Định vị hosted review** (`HostedReviewLocator`): `Repo.url` (ProjectService `ListRepos`) → `provider` + `slug` bằng `hosted_review_ref.go` (https/ssh/scp: `git@host:owner/repo.git`); host `github.com`→github, `gitlab.com`→gitlab, host khác tra bảng `CODEINTEL_CI_PROVIDER_HOSTS` (`host=provider`, dành cho GHE/GitLab self-managed). Không xác định được ⇒ `run=null`, `comparison=[local_only…]`, cảnh báo `provider_unknown`. Nhánh: `ProjectService.ListWorktrees` (`Worktree.branch`) — chỉ dùng cách đã có trong C-DM §3.3, không `GetWorktree` theo id.
3. **PR/MR**: GitHub `GetPullRequestForBranch`; GitLab `ListMergeRequests`; không có ⇒ `run=null`, mọi comparison `local_only`.
4. **Bộ nhớ đệm trong `quality_runs`** (không bảng mới): lấy run `source='ci'` mới nhất theo `(binding, provider, pr)`; nếu `stale_after IS NULL` (mọi check đã hoàn tất **và** `sha` = HEAD cục bộ) hoặc `stale_after > now()` (đồng hồ DB) và không `force` ⇒ trả bản lưu, **không gọi provider**. TTL: có check chạy dở 30 s nhân đôi đến 120 s (giãn khi chữ ký `name:status:conclusion` không đổi; đếm lưu trong `external_ref.unchangedCount`); `sha_mismatch` hoặc không có PR 60 s. `force` vẫn chịu ngân sách.
5. **Giới hạn gọi**: `singleflight` theo `(tenant, provider, repo, pr|sha)`; hạn mức làm mới `CODEINTEL_CI_REFRESH_PER_MINUTE` mỗi tenant (vượt ⇒ `CODEINTEL_RATE_LIMITED`); breaker trong bộ nhớ theo `(tenant, provider)` mở tới `reset_at` khi scm trả `limited` hoặc `remaining` dưới ngưỡng; breaker mở ⇒ trả bản cũ `stale:true, rate_limited:true, reset_at`. Deadline 15 s cho lời gọi scm; quá 20 s tổng ⇒ `CODEINTEL_TIMEOUT` + hậu tố, hoàn tất nền.
6. **Gọi** `ListCommitChecks{provider, repo_slug, ref_kind=PULL_REQUEST, pr_number, include_annotations=true, include_steps=true (chỉ khi có ánh xạ cần step), max_checks=200}`; `capability_unsupported` ⇒ `run=null`, cảnh báo.
7. **Dựng run** (`ci_run_builder.go`): `profile='ci'`, `scope='worktree'`, `head_commit=head_sha`, `agent_run_id='ci:<provider>:<sha>'`, `status` theo PQ-25, `summary`: `error` = check `failure|timed_out`, `warning` = `action_required|cancelled`, `info`=0 (đề xuất), `steps`/payload ≤ 256 KiB và ≤ 200 mục, `external_ref{prNumber|mrIid, headSha, checkIds[]}`, `external_url` đã bỏ query, `fetched_at`, `stale_after`, `dirty=false`. Finding: annotation → `QualityFinding` (`ruleId` PQ-26, `severity`: failure→error, warning→warning, notice→info, `category` theo bảng C hoặc `test`, `tool='ci:<provider>'`, `tool_version`=tên hệ CI, `message` ≤ 1 KiB che, tối đa 500/run); check đỏ không annotation ⇒ **một** finding mức repo (`file=''`). Không `logTail`, token, URL nhạy cảm.
8. **Ghi**: một giao dịch `UpsertCiRun` (`ON CONFLICT … DO UPDATE` / `ON DUPLICATE KEY UPDATE`, CAS theo `version`) + thay toàn bộ finding của run (`ReplaceFindings`) + outbox `run_finished` khi **lần đầu** sang trạng thái kết thúc. Run CI cũ hơn 20 mỗi binding bị tỉa.
9. **So sánh**: `CiComparisonService.ForHead(binding, head)` gom run cục bộ mới nhất theo profile tại `head` (SOL-082 `ListByRepoCommit`) và check CI đã ánh xạ ⇒ `CompareLocalAndCI`.

### E. `CompareLocalAndCI` (hàm thuần, bảng quyết định)

Thứ tự đánh giá: (1) CI chưa có (không PR / chưa fetch) ⇒ `local_only`; (2) check CI không ánh xạ profile cục bộ ⇒ `ci_only`; (3) `ci.sha ≠ local.headCommit` ⇒ `sha_mismatch`; (4) `local.dirty` ⇒ `not_comparable`; (5) CI `PENDING` ⇒ `ci_pending`; (6) cả hai pass ⇒ `agree_pass`; cả hai fail ⇒ `agree_fail`; cục bộ pass ∧ CI fail ⇒ `local_pass_ci_fail` (**kèm `reasonsHint[]`**); cục bộ fail ∧ CI pass ⇒ `local_fail_ci_pass`; còn lại (cục bộ `env_not_ready`/thiếu, CI `UNKNOWN`) ⇒ `not_comparable`. Một check CI ↔ nhiều profile cục bộ: cục bộ gộp (fail nếu có một fail; thiếu một profile ⇒ không đủ ⇒ `not_comparable`). `reasonsHint` là **mã ổn định** (H6; frontend dịch): `env_differs`, `ci_git_matrix`, `ci_integration_tests`, `local_scope_changed`, `stale_cache`, `tool_version_differs`; chọn theo dữ liệu có (ví dụ `local_scope_changed` khi `scope!='worktree'`/`scopeWidened`; `ci_integration_tests` khi bước CI tên chứa `Integration`; `tool_version_differs` khi `toolVersion` khác). Không bao giờ ra `verdict`: cổng (SOL-085) quyết, với quy tắc `unknown` khi thiếu một phía.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Cache bằng chính `quality_runs(source=ci)` | Không bảng mới (T-list chốt); bất biến theo SHA hoàn tất |
| So khớp chỉ khi cùng SHA và cục bộ sạch | Tránh so táo với cam (CR C2) |
| Hiển thị mâu thuẫn, không hoà giải | CR C3, research 11 B7 |
| Breaker trong bộ nhớ theo replica | Đơn giản; mất khi khởi động lại chấp nhận (scm vẫn trả `limited`) |
| Bảng ánh xạ mặc định trong code | Hợp đồng chưa có `ciMappings` (C2) |
| Không log CI, không lưu `logTail` | H8 |

## 4. Tiêu chí chấp nhận

- [x] Check đã hoàn tất cho SHA: lần làm mới thứ hai **không** gọi scm (đếm = 1); hai yêu cầu đồng thời cùng khoá chỉ gọi một lần.
- [x] `limited`/`remaining` dưới ngưỡng ⇒ không gọi provider, trả bản cũ `stale:true, rate_limited:true, reset_at`; breaker mở tới `reset_at`.
- [x] Run `source='ci'`: `provider`, `external_ref.headSha`, `fetched_at`, `agent_run_id='ci:<provider>:<sha>'`, không `active_key`; hai lần upsert cùng SHA không nhân đôi.
- [x] Mapping trạng thái PQ-25 đủ 5 giá trị; `UNKNOWN` ⇒ `failed` + `CODEINTEL_CI_RESULT_UNKNOWN`.
- [x] `ruleId` CI hợp regex PQ-26 với tên check Unicode/dài/ký tự lạ; fingerprint không chứa số dòng.
- [x] Check đỏ không annotation ⇒ đúng một finding `file=''`.
- [x] `CiComparison.relation` đúng bảng ca: 4 tổ hợp pass/fail cùng SHA sạch, `dirty`, `sha_mismatch`, `ci_pending`, `local_only`, `ci_only`; `local_pass_ci_fail` luôn có `reasonsHint[]`; thiếu một phía không bao giờ cho `pass`.
- [x] GitLab: MR tìm bằng `ListMergeRequests`; GHE/GitLab self-managed nhận diện bằng `CODEINTEL_CI_PROVIDER_HOSTS`.
- [x] Không `logTail`, token, URL query nhạy cảm trong bảng/log/sự kiện; `message` che.
- [x] Cô lập tenant (hai dialect); hạn mức làm mới/tenant ⇒ `CODEINTEL_RATE_LIMITED`.
- [x] `window.api.gh.prChecks`/`ChecksPanel` không đổi.

## 5. Kiểm thử (chưa chạy test nào)

Unit: `ci_comparison_test.go` (bảng đủ 9 relation), `ci_run_builder_test.go` (PQ-25, PQ-26), `hosted_review_ref_test.go` (https/ssh/scp, GHE), `refresh_ci_run_test.go` (cổng giả: cache, singleflight, breaker, hạn mức, TTL, đồng hồ giả). Adapter: `ci_run_repository` hai dialect (upsert, replace finding, tỉa 20, RLS). Tích hợp: server scm giả (gRPC) trả `limited`, `capability_unsupported`, SHA lệch, pending. Bảo mật: tập mẫu chuỗi giống token trong annotation; CR-072 cô lập tenant.

## 6. Rủi ro và chưa kiểm chứng

- Quyền token OAuth backend và quota thực chưa kiểm chứng (ngưỡng 10 %, 30 lần/phút là ước lượng CR).
- Ánh xạ phụ thuộc **tên bước** CI; đổi tên làm mất khớp (cảnh báo `ci_check_unmapped`); CI Orca chạy mọi thứ trong một job nên chỉ có ý nghĩa ở cấp bước (cần `include_steps`).
- Tên script/bước cục bộ lệch cây repo đã tách (`desktop/config/scripts/` so với `config/scripts/` ở gốc): so sánh "cục bộ đạt/CI đỏ" có thể do lệch này (CR-086 6).
- PR từ fork, check bên thứ ba, `action_required` chưa xử lý chi tiết (⇒ `UNKNOWN`/`not_comparable`).
- Provider suy từ `Repo.url`: remote lạ hoặc không phải GitHub/GitLab ⇒ `provider_unknown`.
- Breaker trong bộ nhớ không chia sẻ giữa replica.
- Go CI 1.25 vs `go.work` 1.26. SSH: không liên quan (backend gọi API). Git: không chạy lệnh git.

## 7. Điểm hợp đồng thiếu/mâu thuẫn (không tự sửa)

1. C-DM §3.3 thiếu `ScmIntegrationService.ListMergeRequests` và `ProjectService.ListWorktrees` đã có nhưng chưa nêu cho CI; §6.2 thiếu `SCM_INTEGRATION_SERVICE_ADDR`, `CODEINTEL_CI_*`.
2. `QualityProfileDefinition` thiếu `ciMappings` (C2); `ListCommitChecks` thiếu `include_steps` (SOL nguồn).
3. T8 không nêu `scope` cho `source='ci'` (C7) và C-DM §2.1 `CiComparison.reasonsHint` chưa định tập mã.

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-086-scm-commit-checks` | Nguồn dữ liệu |
| BE | `BE-CV-SOL-082-quality-run-storage-and-ingest` | Bảng, repository, `run_finished`, `agentquality` |
| BE | `BE-CV-SOL-085-quality-gate-evaluator-and-profiles` | Dùng `CiComparisonService`; tạo `codeintel_quality_gate.proto`; coi `local_pass_ci_fail` là `fail`/lý do (Q1 CR-086 chọn check bắt buộc) |
| BE | `BE-CV-SOL-012-target-resolution-and-bindings`, `013-authorization-flags-and-audit`, `040-codeintel-quality-channels` (`quality.ci`), `011-repositories-and-maintenance` (tỉa) | Theo C-DM §7.2 (`086 sau 082, 081`) |
| AG | `AG-CV-SOL-081-quality-profile-catalog-and-preflight` | Tên profile cục bộ dùng ở bảng C (không có việc agent riêng) |
| FE | `FE-CV-SOL-085-source-control-quality-notice`, `FE-CV-SOL-087-quality-scorecard-and-state` | Hiển thị nguồn, SHA, `fetchedAt`, `relation`, `reasonsHint` (dịch bằng `translate()`) |

## 9. Câu hỏi mở

- **Q1.** Check CI nào bắt buộc cho cổng (branch protection hay cấu hình Orca)?
- **Q2.** Chia sẻ cache với `ChecksPanel` hay chấp nhận hai quota (Q3 CR-086)? Mặc định: hai quota.
- **Q3.** Có `ciMappings` theo tenant (cần SOL-085 mở rộng định nghĩa)?
- **Q4.** `scope` của run CI: `worktree` hay `commitRange`?
- **Q5.** Tỉa 20 run/binding và 180 ngày có đủ?

## 10. Tham chiếu

[CR-CV-086](../../../../../../docs/crs/v7/quality-signals/CR-CV-086-ci-and-local-results-merge.md); C-DM PQ-04/05/13/24/25/26/33, §3.2–3.3, §4.2 (T8, T12), §5; C-UI §4.7; `.github/workflows/pr.yml`, `.github/workflows/backend-go-scm-integration-service.yml`; `/opt/repos/orca/AGENTS.md` (GitHub CLI usage, Git Provider Compatibility, SSH).
