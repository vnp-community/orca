# CR-REQ-024 — Đồng bộ trạng thái Jira theo Request, audit và observability

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-024 |
| **Tên** | `issue-status-sync` nghe sự kiện Request để đổi trạng thái Jira; audit mọi quyết định; metric, log, trace, cảnh báo cho luồng Request |
| **Loại** | Feature / Vận hành |
| **Priority** | 🟠 P1 |
| **Effort** | Medium (5 đến 7 ngày: consumer, bảng trạng thái hai dialect, audit, metric, alert, test) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-003 (sự kiện trạng thái), 004 (nguồn), 006 (backlog), 013 (hoàn tất); `request-service` phát outbox `orca.request.*` |
| **Mở khoá** | CR-REQ-025 (ngưỡng rollout dùng alert của CR này) |
| **Tác động** | `backend-go/services/issue-status-sync` (domain, usecase, adapter eventbus/grpcclient/postgres/mysql, migration `0002`, config, cmd), `backend-go/services/request-service` (audit, metrics, tracing), `backend-go/common/auditclient`, `backend-go/deploy/alerts/request.rules.yaml` (mới) |

---

## 1. Bối cảnh và vấn đề

1. `issue-status-sync` hiện chỉ nghe 4 subject: `orca.project.worktree.created/deleted`, `orca.scm.pull_request.created/merged` (`adapter/eventbus/subscriber.go`). Nó đổi Jira theo tên trạng thái ("In Progress", "In Review", "Done") bằng `TransitionIssue`, chỉ cho Jira (`syncableProviders`), dùng credential của `actor_user_id` trong sự kiện, và chỉ chuyển khi issue đang đúng nhóm trạng thái (`OnlyFromCategory`).
2. Với Request, mốc "xong" là `request.completed`, không phải PR merge. Một Request `change_request` có nhiều Phase và nhiều PR: nếu giữ đồng bộ theo PR, Jira sang "Done" ở PR đầu tiên.
3. Request từ nguồn Jira mang `source_provider=jira`, `source_ref`, `source_site` (README v6 3.5), đủ để chọn issue, nhưng `issue-status-sync` chưa biết gì về Request.
4. Audit: `common/auditclient.Append` chỉ gửi 6 trường; proto `AppendAuditEntryRequest` đã có `actor_type`, `target_type`, `target_id`, `metadata_json` nhưng client chưa dùng.
5. Observability: `request-service` chưa tồn tại; `issue-status-sync` chỉ có `/health` (không `/metrics`); `outbox` không thấy truyền ngữ cảnh trace; `deploy/alerts/` mới có `mcp.rules.yaml`.

## 2. Giải pháp đề xuất

### 2.1 Hợp đồng sự kiện `request-service` cần phát

CR-REQ-003 đã định nghĩa `orca.request.request.status_changed` với payload `{request_id, project_id, from, to, trigger, type, actor_id, actor_kind, stage, reason, at}` và `orca.request.request.completed`; CR-REQ-004 định nghĩa `request.created` có `source_provider`, `source_site`, `source_ref`, `reporter_id`. Consumer Jira còn thiếu ở `status_changed` và `completed`:

| Trường cần thêm | Dùng để |
|---|---|
| `source_provider`, `source_site`, `source_ref` | chọn issue Jira mà không phải gọi lại `request-service` |
| `version` (của `requests.version` sau chuyển) | chống sự kiện cũ đến muộn (2.4a) |
| `reporter_id` | người dự phòng cho credential Jira (2.7) |
| `number` | nội dung bình luận (2.6) |

Đây là yêu cầu bổ sung cho CR-REQ-003 (Q1). `actor_id` đã có; `actor_kind` ∈ `user`, `ai`, `system` quyết định có dùng làm `actor_user_id` hay không.

Subject theo mẫu `orca.<service>.<entity>.<event>`: `orca.request.request.status_changed`, `orca.request.request.completed`; `returned` được suy từ `status_changed` với `to=request_backlog`. Stream `REQUEST` (mới, do `request-service` gọi `EnsureStream`); chưa kiểm chứng tên stream CR-REQ-001 chọn.

### 2.2 Bảng ánh xạ Request sang Jira

Tên trạng thái đích cấu hình được (2.5). `OnlyFrom` là nhóm trạng thái Jira hiện tại cho phép chuyển (cơ chế `OnlyFromCategory` có sẵn).

| Sự kiện | Điều kiện | Hành động trên Jira | OnlyFrom |
|---|---|---|---|
| `status_changed` sang `executing` | loại có thực thi (mọi loại trừ `spike`, `question`) | chuyển "In Progress" | `todo` |
| `status_changed` sang `analyzing` | loại `spike` hoặc `question` | chuyển "In Progress" | `todo` |
| `completed` | mọi loại | chuyển "Done" | `todo` hoặc `in_progress` |
| `status_changed` sang `request_backlog` | mọi loại | không đổi trạng thái; tuỳ chọn thêm bình luận (2.6) | n/a |
| `status_changed` sang `cancelled` | mọi loại | không đổi trạng thái | n/a |
| đổi loại, các trạng thái `awaiting_*`, `classifying`, `planning` | mọi loại | không làm gì | n/a |

Lý do cho "không đổi" ở backlog và huỷ: Jira không có khái niệm tương đương; kéo lùi hay đóng issue là mất dữ liệu (Q3 của README feature). Loại `hotfix` đã xong thì Request theo dõi (`followup_hotfix`) là Request khác với issue riêng nếu có, không đụng issue gốc.

### 2.3 Phân xử với đồng bộ theo worktree và PR

Trước khi áp `HandleWorktreeLifecycle` và `HandlePullRequestLifecycle`, hỏi `request-service` qua RPC mới `LookupRequestBySource(tenant, provider, site, ref)` (chỉ đọc, bảo vệ bằng `common/internalcaller.Guard` như `ReportTaskOutcome`; trả `found` nếu có Request chưa `completed` hoặc `cancelled` và tenant đang bật cờ). Không dùng `ListRequests` (đã có lọc nguồn ở CR-REQ-004) vì nó kiểm quyền đọc theo người dùng, còn consumer chỉ có tenant. Nếu `found`: bỏ qua có log và metric `skipped_request_owned`. Với sự kiện PR không có site (`LinkedIssueSite` rỗng), khớp mọi site, như `jira-orca-mapping.md` mô tả.

Lỗi tra cứu (service không trả lời): trả lỗi cho consumer để JetStream giao lại, không `MarkSeen`; không chuyển Jira. Giao lại tối đa bao nhiêu lần chưa kiểm chứng (Q4).

### 2.4 Thay đổi mã trong `issue-status-sync`

| File | Việc |
|---|---|
| `internal/domain/request_events.go` (mới) | `RequestStatusEvent` theo 2.1 |
| `internal/domain/events.go` | `TargetState` thêm `OnlyFromCategories []string` (giữ `OnlyFromCategory` cho code cũ) |
| `internal/usecase/sync_request_status.go` (mới) | `HandleRequestStatus`: dedupe `processed_events`, kiểm `source_provider=="jira"`, kiểm cờ, ánh xạ 2.2, kiểm phiên bản (2.4a), gọi `updateIssueStatus` với `actor_user_id` |
| `internal/usecase/sync_issue_status.go` | gọi `LookupRequestBySource` trong hai handler cũ; mở rộng `updateIssueStatus` cho nhiều nhóm. Trước khi sửa phải chạy `gitnexus_impact` trên `updateIssueStatus` |
| `internal/usecase/ports.go` | thêm `RequestLookupClient`, `RequestSyncStateStore`, và (tuỳ chọn) `IssueCommenter` |
| `internal/adapter/eventbus/subscriber.go` | 2 subscription mới trên stream `REQUEST` (`status_changed`, `completed`), durable theo `durableName(subject)` |
| `internal/adapter/grpcclient/request_client.go` (mới) | gọi `LookupRequestBySource`; chuyển tenant như `tenant_forwarding.go` |
| `internal/adapter/postgres`, `mysql` `request_sync_state.go` (mới) | lưu phiên bản đã áp |
| `migrations/postgres`, `migrations/mysql` `0002_request_sync_state.up/down.sql` (mới) | bảng 2.4a |
| `internal/config/config.go` | thêm 2.5 |
| `cmd/server/main.go` | dựng thêm client, metrics handler cho `/metrics` |

**2.4a Bảng `request_sync_state`** (`tenant_id`, `request_id`, `last_version`, `last_target`, `updated_at`; khoá `(tenant_id, request_id)`). Sự kiện có `version` nhỏ hơn hoặc bằng `last_version` bị bỏ (đến muộn hoặc lặp). Lý do: JetStream giao at-least-once và có thể đảo thứ tự giữa các subject; `processed_events` chỉ chặn trùng `event_id`, không chặn sự kiện cũ đến sau.

### 2.5 Cấu hình (mới)

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `REQUEST_SERVICE_ADDR` | `request-service:9090` | cho `LookupRequestBySource` |
| `ISSUE_SYNC_JIRA_STATUS_IN_PROGRESS` | `In Progress` | tên trạng thái đích |
| `ISSUE_SYNC_JIRA_STATUS_DONE` | `Done` | tên trạng thái đích |
| `ISSUE_SYNC_REQUEST_COMMENTS_ENABLED` | `false` | bình luận khi `returned` và `completed` |

Workflow Jira không có đúng tên: `ErrTransitionUnavailable` (adapter `jira/client.go`), ghi log và bỏ, như hiện nay.

### 2.6 Bình luận Jira (tuỳ chọn, mặc định tắt)

Dùng RPC `AddIssueComment` của `issue-tracking-service`. Nội dung ngắn: số Request, loại, trạng thái, liên kết Orca; không kèm `body` hay Solution. Best effort, thất bại chỉ ghi metric.

### 2.7 Actor cho lệnh gọi Jira

Credential Jira theo `(tenant, user)`. `request-service` điền `actor_user_id` bằng người dùng cuối cùng thực hiện hành động; nếu chuyển do hệ thống (callback task xong) thì dùng `decided_by` của Approval gần nhất, rồi `reporter_id`. Rỗng thì bỏ qua có log, như `canSync` hiện tại.

### 2.8 Audit

Thêm `Client.AppendDetailed(ctx, entry)` vào `common/auditclient` (mới; giữ `Append` nguyên) gửi đủ `actor_type`, `target_type`, `target_id`, `metadata_json`. `outcome` chỉ nhận `allowed|denied` (`domain/audit.go` của auth-service), nên thất bại kỹ thuật không audit, chỉ metric và log.

| Action | Khi nào | actor_type | target | metadata_json (không chứa title, body) |
|---|---|---|---|---|
| `request.create` | tạo Request | `user` hoặc `agent` nếu nguồn `mcp` | `request:<id>` | `source_provider`, `client_name` |
| `request.type.confirm` | `ConfirmRequestType` | `user` | `request:<id>` | `type`, `type_source` |
| `request.type.change` | `ChangeRequestType` | `user` hoặc `agent` | `request:<id>` | `from`, `to` |
| `request.return` | vào backlog | `user`, `agent`, `system` | `request:<id>` | `returned_from_stage` |
| `request.reopen`, `request.cancel` | tương ứng | `user` | `request:<id>` | |
| `solution.choose` | chọn phương án | `user` | `solution:<id>` | `option` |
| `approval.approve`, `approval.reject`, `approval.cancel` | quyết định | `user` | `approval:<id>` | `subject_type`, `stage` |
| `approval.expire` | quá hạn | `system` | `approval:<id>` | `subject_type` |
| `approval.approve` bị từ chối quyền | không đủ quyền | `user` | `approval:<id>` | outcome `denied` |
| `request.flow.set` | đổi cờ (CR-REQ-025) | `user` | `tenant:<id>` | `enabled` |
| `request.jira.sync` | `issue-status-sync` chuyển Jira thành công | `system` | `request:<id>` | `issue_ref`, `to_status` |

### 2.9 Metric (Prometheus, cổng health `/metrics`, mẫu `notification-service/internal/adapter/metrics/push_metrics.go`)

Không đặt `tenant_id`, `request_id`, tiêu đề vào nhãn.

| Metric | Loại, nhãn | Service |
|---|---|---|
| `orca_request_created_total` | counter `source_provider` | request-service |
| `orca_request_transitions_total` | counter `type`, `from`, `to` | request-service |
| `orca_request_classification_total` | counter `outcome` (`confirmed_as_proposed`, `changed`, `failed`) | request-service |
| `orca_request_classification_confidence` | histogram | request-service |
| `orca_request_approvals_total` | counter `subject_type`, `outcome` | request-service |
| `orca_request_approval_wait_seconds` | histogram `subject_type` | request-service |
| `orca_request_approvals_pending` | gauge `subject_type` (lấy mẫu từ DB mỗi 30s) | request-service |
| `orca_request_stuck` | gauge `status` (số Request ở trạng thái đó quá ngưỡng, ngưỡng cấu hình) | request-service |
| `orca_request_returned_total` | counter `returned_from_stage` | request-service |
| `orca_request_ai_generation_seconds` | histogram `kind`, `outcome` | request-service |
| `orca_request_task_outcome_total` | counter `outcome` | request-service |
| `orca_request_outbox_pending` | gauge | request-service |
| `orca_issuesync_request_events_total` | counter `event`, `result` (`applied`, `skipped_not_jira`, `skipped_flag_off`, `skipped_stale`, `skipped_no_actor`, `skipped_category`, `skipped_transition_unavailable`, `failed`) | issue-status-sync |
| `orca_issuesync_skipped_request_owned_total` | counter `source` (`worktree`, `pr`) | issue-status-sync |
| `orca_issuesync_jira_transition_seconds` | histogram | issue-status-sync |

Tool MCP đã có `orca_mcp_tool_calls_total{tool}`; không thêm metric riêng cho `request_*`.

### 2.10 Log và trace

- Log `slog` có cấu trúc: `request_id`, `tenant_id`, `type`, `from`, `to`, `actor_kind`; không log `title`, `body`, nội dung Solution. Lỗi AI log mã lỗi, không log prompt.
- Trace: `request-service` gọi `tracing.Init` như các service khác; một span cho mỗi use case (`request.ClassifyRequest`, `approval.Approve`...) với thuộc tính `request.id`, `request.type`, `request.status_from`, `request.status_to`. `common/outbox` chưa truyền ngữ cảnh trace sang consumer; CR này đưa `traceparent` vào header sự kiện và đọc lại ở `issue-status-sync` (xem rủi ro).

### 2.11 Cảnh báo (`backend-go/deploy/alerts/request.rules.yaml`, mới, cùng định dạng `mcp.rules.yaml`, chưa có nơi nạp)

| Alert | Điều kiện đề xuất | Mức |
|---|---|---|
| `RequestApprovalBacklog` | `orca_request_approvals_pending` > 50 trong 1 giờ | warning |
| `RequestStuck` | `orca_request_stuck{status="classifying"}` hoặc `status="analyzing"` hoặc `status="planning"` > 0 trong 30 phút | warning |
| `RequestAIGenerationFailures` | tỷ lệ `outcome="error"` > 20% trong 15 phút | warning |
| `RequestOutboxLag` | `orca_request_outbox_pending` > 100 trong 10 phút | page |
| `RequestReturnedSpike` | `rate(orca_request_returned_total[1h])` gấp 3 lần trung bình 1 ngày | warning |
| `IssueSyncRequestFailures` | `result="failed"` > 10% trong 15 phút | warning |

Ngưỡng là đề xuất ban đầu, chưa có số đo thực để hiệu chỉnh.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Chuyển Jira ở `executing`, không ở `awaiting_*` hay `planning` | "In Progress" chỉ nên đúng khi việc thật sự bắt đầu |
| D2 | Request sở hữu issue, tắt đồng bộ worktree và PR cho issue đó | Tránh Done sớm; một nguồn ghi duy nhất |
| D3 | Backlog và huỷ không đổi Jira | Không có trạng thái Jira tương đương, tránh mất dữ liệu |
| D4 | `request_sync_state` theo phiên bản | Chặn sự kiện cũ đến muộn, việc mà `processed_events` không làm được |
| D5 | Audit chỉ ghi `allowed` và `denied`; lỗi kỹ thuật vào metric | Ràng buộc enum của `auth.audit_log` |
| D6 | Bình luận Jira tắt mặc định | Ghi vào hệ thống ngoài của người dùng cần họ chủ động bật |

## 4. Tiêu chí chấp nhận

- [ ] Request nguồn Jira vào `executing` thì issue đang `To Do` chuyển "In Progress"; issue đã `In Progress` hoặc `Done` không bị đổi.
- [ ] Request `completed` chuyển issue "Done" từ `todo` hoặc `in_progress`; Request `spike` hoàn tất mà chưa qua `executing` vẫn sang "Done".
- [ ] Request `request_backlog` hoặc `cancelled` không đổi trạng thái Jira.
- [ ] Issue có Request chưa kết thúc: PR merge và worktree created không đổi Jira (`skipped_request_owned` tăng); không có Request: hành vi cũ giữ nguyên (test hồi quy).
- [ ] Sự kiện trùng `event_id` và sự kiện `version` cũ hơn không đổi Jira lần nữa.
- [ ] Nguồn `github`, `gitlab`, `linear`, `mcp`, `manual` bị bỏ qua có log.
- [ ] Thiếu `actor_user_id` bị bỏ qua có log, không thử lại.
- [ ] Mỗi action ở 2.8 tạo đúng một bản ghi audit với `actor_type` đúng; không bản ghi nào chứa `title` hay `body`.
- [ ] `/metrics` của `request-service` và `issue-status-sync` trả các series ở 2.9; không nhãn nào có id.
- [ ] `request.rules.yaml` qua `promtool check rules` (chưa kiểm chứng: cần công cụ).
- [ ] Migration `0002` chạy lên và xuống trên Postgres và MySQL.

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `sync_request_status_test.go` (mới) | bảng 2.2 đủ 11 loại; dedupe; phiên bản cũ; thiếu actor; provider khác Jira; cờ tắt |
| `sync_issue_status_test.go` (mở rộng) | `skipped_request_owned`; lỗi tra cứu không `MarkSeen` |
| `subscriber_contract_test.go` (mở rộng) | 2 subject mới, tên durable |
| `request_sync_state_test.go` hai dialect | tích hợp (testcontainers), workflow CI sẵn có chạy ma trận `postgres`, `mysql` |
| `common/auditclient/client_test.go` (mở rộng) | `AppendDetailed` gửi đủ trường |
| `request-service/.../audit_test.go` | từng action tạo đúng bản ghi; lệnh bị từ chối ghi `denied` |
| `metrics_test.go` ở hai service | series và nhãn |
| Thủ công (chưa chạy) | Jira thật: một issue qua `executing` và `completed` |

## 6. Rủi ro và điểm chưa kiểm chứng

- Luồng đổi Jira chưa từng chạy trên Jira thật (`task_sources` trên server dev có 0 dòng); toàn bộ bảng 2.2 chưa kiểm chứng ngoài unit test.
- `common/outbox` chưa truyền trace; sửa nó chạm mọi service dùng outbox. Nếu quá lớn, tách khỏi CR này và để trace chỉ liền trong từng service.
- Số lần giao lại tối đa của consumer (`common/eventbus`) chưa đọc; nếu có giới hạn thấp, lỗi tra cứu kéo dài vẫn làm mất chuyển trạng thái.
- Workflow Jira tuỳ biến có thể không có tên "In Progress" hay "Done" (đã ghi ở `jira-orca-mapping.md` mục 5).
- Credential Jira theo người: người duyệt cuối có thể chưa kết nối Jira, khi đó issue không đổi.
- SSH và remote: dịch vụ này chỉ gọi tracker, không chạm máy dev.

## 7. Câu hỏi mở

1. Bốn trường bổ sung ở 2.1 cần CR-REQ-003 nhận vào payload `status_changed` và `completed`.
2. `LookupRequestBySource`, `GetRequestFlowSettings` chưa có trong README v6 mục 3.6.
3. Với `hotfix` và `security`, có cần chuyển Jira ngay khi `awaiting_type_confirmation` được xác nhận vì tính khẩn không? Mặc định: không.
4. Giới hạn giao lại tối đa của consumer và hành vi khi lỗi tra cứu kéo dài.
5. Bình luận Jira khi `returned` có nên bật mặc định cho tenant beta? Mặc định đề xuất: tắt.

## 8. Tham chiếu

- `backend-go/services/issue-status-sync/internal/usecase/sync_issue_status.go`, `backend-go/services/issue-status-sync/internal/usecase/ports.go`, `backend-go/services/issue-status-sync/internal/usecase/sync_issue_status_test.go`, `backend-go/services/issue-status-sync/internal/domain/events.go`
- `backend-go/services/issue-status-sync/internal/adapter/eventbus/subscriber.go`, `backend-go/services/issue-status-sync/internal/adapter/eventbus/durable_name_test.go`, `backend-go/services/issue-status-sync/internal/adapter/eventbus/subscriber_contract_test.go`, `backend-go/services/issue-status-sync/internal/adapter/grpcclient/tenant_forwarding.go`
- `backend-go/services/issue-status-sync/internal/config/config.go`, `backend-go/services/issue-status-sync/cmd/server/main.go`, `backend-go/services/issue-status-sync/migrations/postgres/0001_processed_events.up.sql`
- `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go` (`UpdateIssue`, `ErrTransitionUnavailable`)
- `backend-go/proto/orca/issuetracking/v1/` (`UpdateIssue`, `AddIssueComment`, `ListTransitions`)
- `backend-go/common/auditclient/client.go`, `backend-go/common/auditclient/client_test.go`; `backend-go/proto/orca/auth/v1/` (`AppendAuditEntryRequest`); `backend-go/services/auth-service/internal/domain/audit.go`
- `backend-go/common/tracing/tracing.go`, `backend-go/common/outbox/outbox.go`, `backend-go/common/internalcaller/internalcaller.go`
- `backend-go/services/notification-service/internal/adapter/metrics/push_metrics.go`, `backend-go/services/mcp-service/cmd/server/main.go`
- `backend-go/deploy/alerts/mcp.rules.yaml`
- `docs/guides/jira/jira-orca-mapping.md`, `docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md`
- `.github/workflows/backend-go-issue-status-sync.yml`
