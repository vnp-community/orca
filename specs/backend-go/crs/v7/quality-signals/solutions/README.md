# backend-go Solutions: Quality Signals (v7, "Xem code & kiểm soát chất lượng")

**CRs:** [docs/crs/v7/quality-signals](../../../../../../docs/crs/v7/quality-signals/README.md)
**Hợp đồng (nguồn sự thật):** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md) (C-DM), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md) (C-AG), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) (C-UI). README v7 mục 8 thắng mục 3; **hợp đồng thắng cả CR**.
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/scm-integration-service`](../../../../tdd/services/scm-integration-service.md), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

> 📋 Proposed. Chưa triển khai, chưa chạy test/migration nào. `code-intel-service` và `proto/orca/codeintel` chưa tồn tại (đã `ls`); mọi thứ ở đó là "(mới)".

## Bảng CR, Solution, Task

| CR | Solution (backend) | Service / Area | Priority | Task |
|----|--------------------|----------------|----------|------|
| CR-CV-080 | [BE-CV-SOL-080-auto-refresh-index](./BE-CV-SOL-080-auto-refresh-index.md) | `code-intel-service`, `infra-fleet-service` (payload), `proto` | P0 | `BE-CV-TASK-080-01` đến `-09` |
| CR-CV-081 | **Không có phần backend** (chỉ agent: `AG-CV-SOL-081-quality-runner-core`, `AG-CV-SOL-081-quality-profile-catalog-and-preflight`); điều phối `StartQualityRun` thuộc `BE-CV-SOL-085` | — | — | — |
| CR-CV-082 | [BE-CV-SOL-082-quality-run-storage-and-ingest](./BE-CV-SOL-082-quality-run-storage-and-ingest.md) | `code-intel-service`, `proto` | P0 | `BE-CV-TASK-082-01` đến `-09` |
| CR-CV-083 | [BE-CV-SOL-083-coverage-storage-and-diff](./BE-CV-SOL-083-coverage-storage-and-diff.md) | `code-intel-service`, `proto` | P1 | `BE-CV-TASK-083-01` đến `-09` |
| CR-CV-084 | **Không có phần backend** (chỉ agent: `AG-CV-SOL-084-convention-rule-pack`) | — | — | — |
| CR-CV-086 | [BE-CV-SOL-086-scm-commit-checks](./BE-CV-SOL-086-scm-commit-checks.md) | `scm-integration-service`, `proto/scmintegration` | P1 | `BE-CV-TASK-086-01` đến `-07` |
| CR-CV-086 | [BE-CV-SOL-086-ci-run-merge-and-comparison](./BE-CV-SOL-086-ci-run-merge-and-comparison.md) | `code-intel-service`, `proto` | P1 | `BE-CV-TASK-086-08` đến `-16` |
| CR-CV-091 | [BE-CV-SOL-091-security-scan-flag-and-ingest](./BE-CV-SOL-091-security-scan-flag-and-ingest.md) | `code-intel-service` | P2 | `BE-CV-TASK-091-01` đến `-06` |
| CR-CV-094 | **Không có solution** (spike tài liệu 5 ngày, kết quả vào `docs/research/view-code/spikes/094-gitnexus-unused-tools/`; nếu "đi" thì cập nhật CR-037/038/001/002/041 trước khi viết solution) | — | — | — |

Task đánh số liên tục theo CR (CR-086 có hai solution: `-01…-07` rồi `-08…-16`).

## Điều chỉnh đã biết áp dụng cho feature

| Điều chỉnh | Áp dụng |
|---|---|
| PQ-25 (run CI, `RefreshCiRun`) | SOL-086-scm + SOL-086-ci: ánh xạ `overall→status`, 9 `relation`, `local_pass_ci_fail` luôn có `reasonsHint` |
| PQ-26 (`ruleId`) | `CI/<group>/<name>`, `SEC-*`, `DEP-*` khớp `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`; backend bỏ dòng sai |
| PQ-01 | Cờ quét bảo mật tắt ⇒ `CODEINTEL_PROFILE_UNKNOWN` (không `DISABLED`) |
| PQ-16 | Bỏ `tiers`; `tools/trigger/ifStale/expectHead` |
| Repo đã tách `frontend/desktop/agent/backend` | Script `check-*`, `vitest.config.ts`, baseline nằm ở `desktop/config/`, không ở gốc; chỉ ảnh hưởng tên profile cục bộ (agent), không ảnh hưởng backend |
| CI | 18 workflow `backend-go-*` (không phải 17), `go-version: "1.25"` so với `go.work` 1.26: code mới không dùng API chỉ có ở 1.26; `make proto-lint` có `\|\| true` nên gọi `buf` trực tiếp |

## Re-verify (đối chiếu CR với mã thật, 2026-10-06; chi tiết ở từng solution)

| Khẳng định của CR | Kết quả khi đọc | Lệch? |
|---|---|---|
| `statusChanged` thiếu `worktree_id`/`dev_server_id` | Đúng; session đã có hai trường, chỉ cổng publisher thiếu tham số | Không (làm ở 080-02) |
| `GetPullRequestForBranch` provider-neutral | GitLab trả `ErrCapabilityUnsupported`; cần `ListMergeRequests` | **Lệch** ⇒ SOL-086-ci |
| Một truy vấn GraphQL GitHub | `graphQLURL` cố định `api.github.com`; GHE dùng REST | **Lệch** ⇒ SOL-086-scm |
| `rate_limit_cache` ghi từ header mọi lời gọi | Chỉ `GetRateLimitStatus` ghi, bucket `core` | **Lệch** ⇒ không ghi số GraphQL vào `core` |
| 21 module Go trong `go.work` | 19 service + 3 module chung | **Lệch** ⇒ SOL-083 |
| Cột CI thêm sau ở CR-086 | C-DM T8 đã đặt sẵn trong `0003` | **Lệch** ⇒ SOL-082 |

## Thứ tự thực thi và phụ thuộc

```
G0 (BE-CV-SOL-010, 020) ─▶ 011 ─▶ 012, 013 ─▶ 021, 023 ─▶ 024
080-02 (infra-fleet payload, độc lập) ─┐
080-01 (IndexBasis) ─▶ 082-01 ─▶ 082-02..09 ──▶ 083 (sau 082) ─┐
                                   │                          ├─▶ BE-CV-SOL-085 (cổng)
080 (sau 012, 024) ───────────────┘                 086-scm ─▶ 086-ci (sau 082) ─┘
                                                                  091 (sau 085)
```

Song song được: 080-02 với mọi thứ; SOL-086-scm (không DB, không phụ thuộc `code-intel-service`) với SOL-082; 083 với 086-ci sau 082. Dòng `rpc` của `QualityGateService` (`GetCoverage`, `RefreshCiRun`) cần `codeintel_quality_gate.proto` do SOL-085 tạo.

## Quyết định chung

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Hai dialect, `tenant_id` mọi truy vấn, test cô lập tenant | C-DM §4.1, §8.3 |
| F2 | Không bảng mới ngoài danh sách C-DM §4.2 (debounce dùng `reindex_jobs`, cache CI dùng `quality_runs`) | Hợp đồng chốt 14 bảng |
| F3 | `unknown`/`null` là trạng thái thật; `estimated` tách khỏi `measured`; không `pass` từ thiếu dữ liệu | H7 |
| F4 | Backend trả khoá/mã, frontend dịch (`reasonsHint`, `estimatedNote`) | H6 |
| F5 | Agent chỉ nhận tên profile/tham số hẹp | H3, C-AG §2.1 |
| F6 | Không `max-lines` disable; tên file theo khái niệm | AGENTS.md |

## Điểm hợp đồng thiếu/mâu thuẫn tổng hợp (không tự sửa hợp đồng)

1. `dirty_fingerprint=15` và `tree_fingerprint=20` trùng nghĩa; `CODEINTEL_QUALITY_RUN_INTERRUPTED` chưa có (SOL-082).
2. `scope_key` T13 không có nguồn từ agent theo module; `GetCoverageResponse.reason`; `language=mixed` (SOL-083).
3. `ListCommitChecks` thiếu `tenant_id`, `include_steps`, `RateLimitInfo.limited`; §3.3 thiếu `ListMergeRequests`; §6.2 thiếu `SCM_INTEGRATION_SERVICE_ADDR`, `CODEINTEL_CI_*`, `CODEINTEL_AUTOREFRESH_*` (SOL-086).
4. `QualityProfileDefinition` thiếu `ciMappings`; `scope` của run CI; tập mã `reasonsHint` (SOL-086-ci).
5. Thiếu chỉ mục `(tenant_id, worktree_id)` trên `repo_bindings`, `(tenant_id, created_at)` trên `reindex_jobs`, và retention `reindex_jobs` (SOL-080).
6. Suite mặc định của agent có thể chứa profile bảo mật (SOL-091).
