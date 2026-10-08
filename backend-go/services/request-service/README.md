# Request Service

Manages engineering requests and approvals.

## Chạy local

Cần Docker cho cơ sở dữ liệu và NATS. Từ gốc `backend-go`:

```bash
make dev-up
go run ./services/request-service/cmd/server
```

## Biến môi trường

Service sẽ sử dụng config mặc định nếu không khai báo biến:
- `DATABASE_CREDENTIALS_FILE`: Đường dẫn file JSON chứa thông tin nối DB.
- `NATS_URL`: Địa chỉ kết nối NATS (mặc định: `nats://localhost:4222`).
- `TASK_SERVICE_ADDR`: Địa chỉ GRPC của task-service.
- `AI_PROVIDER_SERVICE_ADDR`: Địa chỉ GRPC của ai-provider-service.
- `PROJECT_SERVICE_ADDR`: Địa chỉ GRPC của project-service.
- `REQUEST_FLOW_ENABLED`: công tắc tổng của luồng Request (mặc định `false`). Luồng chỉ chạy khi công tắc này bật VÀ tenant đã bật (`SetRequestFlowSettings`, bảng `tenant_settings`); thiếu dòng hoặc lỗi đọc là tắt. Interceptor `internal/adapter/grpc/flow_gate.go` chặn các RPC đi tiếp bằng `REQUEST_FLOW_DISABLED`, vẫn cho đọc, thoát an toàn và RPC nội bộ.
- `SERVICE_INTERNAL_TOKEN`: bí mật dùng chung mà `issue-status-sync` gửi khi gọi `LookupRequestBySource` (`issue-status-sync` đặt `REQUEST_SERVICE_INTERNAL_TOKEN` cùng giá trị); để trống thì RPC này từ chối mọi lời gọi.
- `REQUEST_STUCK_THRESHOLD_<CLASSIFYING|ANALYZING|PLANNING|EXECUTING>` (Go duration; mặc định 10m, 30m, 30m, không đặt cho executing) và `REQUEST_METRICS_SAMPLE_INTERVAL` (mặc định `30s`): ngưỡng và chu kỳ lấy mẫu cho `orca_request_stuck`, `orca_request_approvals_pending`, `orca_request_outbox_pending`.
- `REQUEST_APPROVAL_ENABLED`: mở `ApprovalService`, `ApprovalPolicyAdminService` và bộ quét hết hạn/nhắc (mặc định `true`; đặt `false` để tắt). Mỗi subject có handler thật; handler không có dịch vụ nền được gắn (solution, plan...) từ chối mở approval thay vì duyệt ngầm. `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS=true` chỉ cho dev.
- `REQUEST_APPROVAL_SWEEP_INTERVAL` (mặc định `60s`), `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS` (mặc định `50`), `TENANT_SERVICE_ADDR` (team), `AUTH_SERVICE_ADDR` (danh sách admin): thiếu địa chỉ thì tra cứu tương ứng từ chối (fail closed).
- Chính sách người duyệt: quản trị qua `ApprovalPolicyAdminService` (chỉ admin); không có chính sách khớp thì dùng mặc định trong `domain.DefaultPolicy`.
- `REQUEST_APPROVAL_SUBJECTS`: danh sách subject (phân tách dấu phẩy) cần handler; rỗng nghĩa là cả 8 loại.
- `INFRA_FLEET_SERVICE_ADDR`, `PROJECT_SERVICE_ADDR` (và tuỳ chọn `AI_PROVIDER_SERVICE_ADDR`): cần cho phân loại AI, sinh Solution và phân tích; thiếu thì `GenerateSolution` trả `REQUEST_SOLUTION_NO_CONNECTION`.
- `GIT_GATEWAY_SERVICE_ADDR`: dùng để so sánh trạng thái repo trước/sau phân tích chỉ đọc; thiếu thì run ghi `repo_check=skipped`.
- Phân tích (CR-REQ-007/008), đều là Go duration hoặc số nguyên dương, không hợp lệ thì dùng mặc định: `REQUEST_AI_COMPLETE_TIMEOUT` (120s), `REQUEST_ANALYSIS_LEASE_TTL` (90s), `REQUEST_ANALYSIS_HEARTBEAT` (30s), `REQUEST_ANALYSIS_RECOVERY_INTERVAL` (30s), `REQUEST_AGENT_READONLY_TIMEOUT_MS` (600000; hotfix 300000), `REQUEST_AGENT_READONLY_MAX_PER_PROJECT` (2).
- `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` (mặc định `true`): cho phép gửi `accessMode=readonly` tới agent có tính năng `agent.execPrompt.readonly`; `false` ép mọi run chỉ dựa prompt. `REQUEST_REQUIRE_ENFORCED_READONLY` (mặc định `false`): `true` thì từ chối chạy trên agent không ép được chế độ chỉ đọc (`REQUEST_ANALYSIS_AGENT_TOO_OLD`).
- `REQUEST_CLARIFICATION_MAX_ROUNDS`: số vòng hỏi tối đa của Definition of Ready trước khi trả về backlog `missing_info` (mặc định `3`).
- `REQUEST_CLARIFICATION_WORKERS_ENABLED`: bật vòng quét hết hạn và nhắc (60 giây) và consumer kích hoạt lại (mặc định `true`, chỉ chạy khi `REQUEST_FLOW_ENABLED=true`).
- `REQUEST_CLARIFICATION_AUTO_REGENERATE`: sau khi trả lời về `analyzing`, tự gọi sinh Solution (mặc định `true`; người bấm lại khi tắt). Chưa có tác dụng cho tới khi tính năng Solution nối `AnalysisStarter`.
- `REQUEST_DECISION_HIGH_RISK_SERVICES`: ngưỡng số service bị ảnh hưởng để một phương án tính là rủi ro cao (mặc định `3`; `RecordDecisionInput.HighRiskServices` ghi đè theo từng lần gọi).

## Migration

`migrations/{postgres,mysql}` đánh số liền, đã đóng băng sau đợt R1a (bảng đánh số lại ở
`specs/backend-go/crs/v6/request-service-foundation/IMPLEMENTATION-NOTES.md`). Mọi migration mới lấy số kế tiếp (`0008`...). Dải đã cấp: `0020`..`0029` lifecycle, `0030`..`0039` solution, `0040`..`0049` approval, `0050`..`0059` plan, `0060`..`0069` artifact và clarification (`0060_request_artifact_model`, `0061_clarifications_decisions`).

## Real vs Stub

| Thành phần | Thật | Stub / chưa làm |
| --- | --- | --- |
| `internal/config` | cấu hình, cờ approval | |
| Migration `0001`..`0007`, `0040` hai dialect | outbox, request core, approvals, policies, context sources, analysis runs, solution engines (RLS thật trên Postgres) | |
| `postgres|mysql` `Repository` (`InTx`, outbox) | giao dịch lồng, bắt buộc tenant, outbox cùng giao dịch | |
| `RequestRepository`, `RequestTypeHistoryRepository`, `SolutionRecordRepository`, `RequestLinkRepository`, `RequestIdempotencyRepository` | cả hai dialect, chung bộ test `internal/adapter/contracttest` | chưa có use case gọi history/solution/link/idempotency (đợt R1b) |
| gRPC `RequestService.GetRequest`, `ListRequests` | thật, nối repository | `ListBacklog` còn `Unimplemented` (CR-REQ-006) |
| gRPC health, `/healthz`, `/readyz` | có | |
| `ApprovalService` (8 RPC), `ApprovalPolicyAdminService` (3 RPC) | thật, mở theo mặc định; test hợp đồng Postgres và MySQL thật | `SubjectArtifacts` của Solution/Plan/Phase/Findings/Answer/Task list/pre-deploy chưa gắn (mở approval các chủ thể này báo `REQUEST_APPROVAL_SUBJECT_UNAVAILABLE`); tenant-service, auth-service chưa chạy thật |
| Migration `0020` (`returned_category`, `request_return_history`), `0021` (lý do con của `request_links`, `created_by`, `created_at`) | hai dialect, backfill `other`, RLS thật cho bảng mới | |
| `domain` flow registry (11 loại), 16 trigger, `NextStatus`, `HappyPath`, `StageForStatus`, `ValidateChild` | thật, thuần | |
| Use case `TransitionRequest` (CAS, `ExpectedFrom`, outbox `status_changed`/`completed`), `GetRequestFlow` | thật, cổng `RequestTransitioner` cho các feature khác; RPC `GetRequestFlow` thật | |
| Use case `ReturnRequestToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks` | thật, có test hai dialect, dựng ở `cmd/server/wire_request_lifecycle.go`; RPC `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks` thật; con của `SpawnChildRequest` đi qua `CreateRequest` (vào `classifying`, có `type_hint`); reopen đặt lại `classification_attempts` | `ExecutionGuard` chưa chặn theo task đang chạy (CR-REQ-011/013) |
| `backlog_requests.go` | lọc `returned_category` và `LatestReturns` đọc `request_return_history` | `ListReturnedRequests`, `ParentRequestIDs` vẫn `exec(ctx)` trần: đổi sang `scoped` khi nối RPC `ListBacklog` |
| Migration `0025`..`0027` hai dialect | `source_hints`, `classification_attempts`, bảng `classification_runs` (lease; RLS thật, policy `app.relay` cho quét phục hồi và dọn `processed_events`) | |
| Use case `CreateRequest`, `LookupRequestBySource` (CR-REQ-004) + RPC `CreateRequest`, `LookupRequestBySource`, bộ lọc nguồn của `ListRequests` | thật, khoá idempotency theo tenant/provider/site/ref, làm giàu Jira/Linear, outbox cùng giao dịch | điểm móc `RequestCreationRecorder` đã nối `RequestRevisionRecorder` (xem mục Mô hình artifact) |
| Webhook `POST /v1/request-webhooks/{source_name}` | thật, HMAC-SHA256, tenant qua header `X-Orca-Tenant-Id`, thân tối đa 256 KiB; bật khi có `REQUEST_WEBHOOK_SOURCES` | bí mật lấy từ env/tệp cấu hình, chưa từ credential-broker; `api-gateway` chưa chuyển tiếp tới cổng này |
| `IssueTrackingClient` | thật (Jira, Linear), chuyển tenant và user | chưa kiểm chứng với Jira/Linear thật; thiếu `ISSUE_TRACKING_SERVICE_ADDR` thì từ chối nguồn không có title |
| Phân loại AI (CR-REQ-005): `ClassificationRunner`, `ProposeRequestClassification`, `RelayClassifier`, consumer `status_changed`; RPC `ClassifyRequest` (trả `run_id`) | thật: chạy nền theo run có lease và phục hồi, `ai.complete` qua Relay (`RelayByDevServer` khi không có connection), giới hạn 5 lần | chưa kiểm chứng với dev server agent thật |
| Use case + RPC `ConfirmRequestType`, `ChangeRequestType`, `ListRequestTypeHistory` | thật, có test DB hai dialect | `ApprovalRecorder` thật (`RequestTypeApprovalRecorder`), `ApprovalCanceller` thật (`PendingApprovalCanceller`) |
| Cờ `request_flow_enabled` (CR-REQ-025): migration `0090` (`tenant_settings`, RLS thật), RPC `GetRequestFlowSettings`/`SetRequestFlowSettings`, interceptor `FlowGate` (bảng phân loại mọi RPC + test quét proto) | thật, hai dialect, nối vào `grpc.NewServer` qua `cmd/server/wire_rollout.go`; chỉ admin được Set, có audit `request.flow.set` | chưa có cờ theo loại (cờ theo tenant); nếu RPC mới không có dòng trong `flowMethodClass` thì test đỏ |
| Audit (CR-REQ-024): `usecase.AuditTap` (suy từ sự kiện outbox đã commit) + `AuditRPC` (từ chối duyệt, `solution.choose`) + `adapter/audit` (qua `auth-service`, danh sách khoá metadata cho phép) | thật; không ghi tiêu đề hay nội dung; chỉ ghi sau khi giao dịch commit (`CommitHooks`) | `solution.choose` chỉ kiểm chứng bằng handler giả vì RPC `ChooseSolutionOption` còn `Unimplemented`; không audit khi `AUTH_SERVICE_ADDR` rỗng |
| `LookupRequestBySource` (CR-REQ-024): `ActiveSourceFinder`, migration `0091` | thật: chỉ Request chưa `completed`/`cancelled`, `site` rỗng khớp mọi site, cờ tắt thì `found=false`, chỉ id trả về, bảo vệ bằng `internalcaller.Guard` | |
| Metric `/metrics` (12 series `orca_request_*`), goroutine lấy mẫu, migration `0092` | thật: đếm từ sự kiện outbox đã commit, gauge lấy mẫu từ DB mỗi 30 giây, không nhãn nào mang id | `orca_request_task_outcome_total` và `orca_request_ai_generation_seconds{kind!=classification}` chờ `ReportTaskOutcome`/Solution/Plan gọi `RequestObserver` |
| Payload `status_changed`/`completed` | có `source_provider`, `source_site`, `source_ref`, `reporter_id`, `version`, `number`, `traceparent` (span `request.Transition`) | |
| `e2e/` (thẻ `e2e`, `E2E_DIALECT=postgres` hoặc `mysql`) và `e2e/stubs` | chạy binary thật trên DB, NATS thật; stub chỉ ở biên gRPC (relay `ai.complete`) | giai đoạn Solution, Plan, thực thi báo SKIP kèm task chặn cho tới khi RPC có thật |

## Mô hình artifact (CR-REQ-027)

Mô hình chuẩn là JSON trong DB; Markdown/YAML chỉ là bản chiếu. Migration `0060_request_artifact_model` thêm 5 cột nội dung vào `requests`, 5 cột vào `solutions` và 4 bảng chỉ-thêm (`request_revisions`, `artifact_index`, `artifact_relations`, `request_coverage`, RLS thật trên Postgres).

| RPC | Ghi chú |
| --- | --- |
| `EditRequestContent` | cần `expected_revision`; chỉ khi Request ở `new`, `classifying`, `awaiting_type_confirmation` hoặc `request_backlog`; quyền tạm: reporter hoặc admin; `title`/`body` rỗng nghĩa là giữ nguyên |
| `ListRequestRevisions`, `GetRequestRevision` | danh sách không trả snapshot; snapshot đọc lại dạng JSON chuẩn tắc, khớp `digest` |
| `GetRequestCoverage`, `GetArtifactGraph` | bảng phủ AC → task → check; đồ thị gộp `artifact_relations`, `request_links` (`spawned_by`) và cây task (`contains`, `depends_on`); task-service lỗi thì `partial=true` |
| `ResolveArtifactRef` | `REQ-n`, `REQ-n#AC-k`, `SOL-n.s`, `SOL-n.s/opt-k`, `PLN-`, `PH-`, `TSK-`; theo tenant |
| `ExportArtifactProjection` | `markdown`; Request và Solution; Plan chưa (cần đọc cây từ task-service, task 027-07) |

Quy tắc bất biến: cột nội dung của Request chỉ đổi qua `AppendRequestRevision` (test kiến trúc `request_content_write_guard_test.go`); mỗi lần đổi là một revision mới cùng transaction và một sự kiện `orca.request.request.revised` không chứa nội dung. AC không bao giờ bị xoá (chỉ `retired`) và số AC không tái dùng, nên `GetRequestRevision(1)` luôn đọc được AC cũ. Solution `approved` bất biến và khoá spec Plan khi duyệt thuộc task 027-07 (chưa làm).

Schema JSON nằm ở `schemas/v1/*.schema.json` (8 `kind`, nhúng bằng `go:embed`, draft 2020-12); thư viện `santhosh-tekuri/jsonschema/v6` (lý do chọn ở `IMPLEMENTATION-NOTES.md`). Bộ vector `testdata/artifacts/task/canonical_cases.json` phải giống hệt bản của task-service: chạy `scripts/check-artifact-samples.sh`.

**Chưa kiểm chứng:** chi phí `Validate` ở kích thước Plan 256 KB (benchmark mẫu nhỏ 16 µs/lần); hiệu năng `GetArtifactGraph` với cây lớn; bản chiếu với đầu ra agent thật.

## Clarification và Decision (CR-REQ-028)

Trạng thái thứ 12 `awaiting_information` và hai trigger `information_required` / `information_provided` (đích là `resume_status` của Clarification, qua `NextStatusWithResume`; `NextStatus` giữ chữ ký cũ). Migration `0061_clarifications_decisions`: đổi CHECK `requests.status`, 5 bảng (`clarifications`, `clarification_questions`, `clarification_assignees`, `decisions`, `decision_history`), chỉ mục duy nhất một phần (Postgres) hoặc cột sinh `open_key`/`live_key` (MySQL): một Clarification `open` mỗi Request, một Decision sống mỗi chủ thể.

- **Sẵn sàng:** `ConfirmRequestType` chạy `ReadinessPolicy` (dùng `ValidateRequestContent(ready)` và `RequiredFields` của CR-027). Thiếu khoá bắt buộc → `awaiting_information` + Clarification `readiness` (một câu hỏi mỗi khoá thiếu); vẫn ghi `type_confirmed` và Approval `request_type`. Quá `REQUEST_CLARIFICATION_MAX_ROUNDS` → `request_backlog` (`missing_info`). `WaiveReadiness` (admin, cấm `hotfix|security|ops_request`) ghi revision có `meta.waiver`.
- **Trả lời:** `AnswerClarification` (nháp hoặc hoàn tất) tạo revision `clarification_answered`, kiểm sẵn sàng lại, huỷ thứ sinh từ dữ kiện cũ (Decision; Solution qua cổng `SolutionSuperseder`), rồi `information_provided`. Hết hạn: vòng quét `ClarificationSweeper` đặt `expired` và trả về backlog `missing_info`; nhắc một lần ở nửa thời hạn. Consumer `request-service-clarification-resume` đọc `status_changed` (`trigger=information_provided`), khử trùng bằng `processed_events`.
- **Thông báo:** payload `clarification.requested|expired` khớp golden của notification-service (`testdata/notification/*.json`, sao chép); không chứa nội dung câu hỏi hay trả lời. Người nhận mở rộng được `user`, `reporter`; `team` và `role:admin` cần resolver (chưa nối, ghi log).
- **Decision:** `RecordDecision` (rủi ro cao → `chosen`, chờ `ConfirmDecision` gõ lại tên phương án), `ApprovalGates` (Decision phải `effective` đúng digest; câu hỏi `blocking`, giả định `needs_confirmation` phải có Clarification đã trả lời). `RecordDecision` và `ApprovalGates` chưa có nơi gọi trong service này: `ChooseSolutionOption` và các `SubjectHandler` thuộc tính năng Solution/Plan.

- **Nối với Solution:** `ChooseSolutionOption` nhận `rationale` và ghi Decision cùng giao dịch; handler `solution` chặn `Approve` khi Decision chưa `effective` đúng digest, khi câu hỏi `blocking` chưa trả lời, hoặc khi phương án chọn chưa trả lời mọi AC. Trả lời Clarification đưa Request về `analyzing` thì consumer tự sinh lại Solution (khoá `clr-<id>`, tắt bằng `REQUEST_CLARIFICATION_AUTO_REGENERATE=false`). Mỗi Solution có `seq` (`SOL-<n>.<seq>`), `provenance` (không chứa khoá, env hay `credential_ref`) và `content_digest`.

**Chưa kiểm chứng:** hạn mặc định (7 ngày, 72 giờ, 24 giờ), ngưỡng 3 vòng và ngưỡng rủi ro cao chỉ là đề xuất; thời gian trả lời thực của người báo cáo; chất lượng gợi ý mặc định; kích hoạt lại thật (`GenerateSolution`, `AdvanceExecution` chưa tồn tại, mặc định ghi log và bỏ qua).

## Chế độ chỉ đọc và giới hạn đã biết

`GenerateSolution` chạy nền (trả `{solution_id, run_id}` ngay) rồi kết quả đọc qua `ListSolutions`. `solution` dùng `ai.complete`; `diagnosis`, `findings`, `answer` dùng `agent.execPrompt` trên `repo_path` của dự án, không tạo worktree, không tạo Task.

Ba lớp bảo vệ chỉ đọc, không lớp nào tuyệt đối; mỗi run ghi lớp nào đã chạy (`analysis_runs.enforcement`, `repo_check`):

1. **Agent ép chế độ chỉ đọc** (`enforcement=agent_enforced`): chỉ khi hồ sơ năng lực của dev server có tính năng `agent.execPrompt.readonly`. Service gửi `accessMode=readonly`, `workspaceKind=repo_root`, `reportChanges=true`, kiểm `applied.accessMode`, và loại kết quả có `READONLY_VIOLATION`, `changes` khác rỗng hoặc `headMoved`.
2. **Prompt** cấm sửa/xoá/tạo file, `git commit/checkout/reset/clean/push` và gọi mạng ngoài. `trustPreset` luôn `default`, không bao giờ `full`; `env` chỉ có `ORCA_REQUEST_ID`, `ORCA_PROJECT_ID`.
3. **So sánh repo trước và sau** qua git-gateway `GetStatus(worktree_id)`: khác nhau thì run `failed` `REQUEST_ANALYSIS_REPO_MODIFIED` và kết quả bị bỏ.

Giới hạn đã biết:

- Agent cũ (không có `features`) hoặc không đọc được hồ sơ năng lực chỉ còn lớp 2 và 3 (`enforcement=prompt_only`). Đặt `REQUEST_REQUIRE_ENFORCED_READONLY=true` để từ chối thay vì chạy như vậy. Lớp 2 không ép được gì: agent vẫn có shell trên repo gốc.
- `HEAD` không được so sánh (git-gateway không trả); chỉ `(branch, danh sách file + trạng thái)`. Với agent mới, `changes.headMoved` bù phần này. File bị `.gitignore` và thay đổi ngoài repo không thấy.
- Thiếu `worktree_id` (kết nối không có, hoặc dự án đi qua dev server mặc định) thì lớp 3 không chạy và run ghi `repo_check=skipped`; thiếu `repo_path` thì `REQUEST_ANALYSIS_NO_REPO_PATH`.
- Tối đa `REQUEST_AGENT_READONLY_MAX_PER_PROJECT` run chỉ đọc đồng thời mỗi `(tenant, project)`; vượt thì `REQUEST_ANALYSIS_BUSY` và không để lại dòng nào.
- Không có đường tự hạ từ agent xuống `ai.complete` khi agent lỗi; người dùng gọi lại. Phục hồi sau khi tiến trình chết đánh run `failed` (`REQUEST_SOLUTION_RUN_INTERRUPTED`), không chạy tiếp.
- Chưa kiểm chứng với dev server thật: hành vi `accessMode=readonly` và `trustPreset=default`, thời gian và giới hạn của `ai.complete`.
