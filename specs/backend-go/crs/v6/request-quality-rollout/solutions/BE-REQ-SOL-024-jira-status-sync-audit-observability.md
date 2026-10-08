# BE-REQ-SOL-024: Đồng bộ trạng thái Jira theo Request, audit và observability

> ✅ Đã triển khai (kiểm chứng 2026-10-08): 8/8 task xong. Chưa kiểm chứng: Jira thật, `promtool`, `solution.choose` end-to-end (RPC `ChooseSolutionOption` còn `Unimplemented`). Chi tiết: `../IMPLEMENTATION-NOTES.md`.

**CR:** [CR-REQ-024](../../../../../../docs/crs/v6/request-quality-rollout/CR-REQ-024-jira-status-sync-audit-observability.md)
**Service:** `issue-status-sync`, `request-service` (mới, audit, metric, trace, RPC tra cứu), `common/auditclient`, `backend-go/deploy/alerts`
**TDD tham chiếu:** [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (outbox, JetStream at-least-once, subject), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (metric, log, trace), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, DB-per-service), [`services/issue-tracking-service.md`](../../../../tdd/services/issue-tracking-service.md), [`services/auth-service.md`](../../../../tdd/services/auth-service.md) (audit log)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `issue-status-sync/internal/usecase/sync_issue_status.go` (201 dòng), `ports.go`, `domain/events.go`, `adapter/eventbus/subscriber.go`, `config/config.go`, `cmd/server/main.go`, `migrations/{postgres,mysql}/0001_processed_events.up.sql`, `adapter/postgres/processed_events.go`; `common/auditclient/client.go`, `common/eventbus/eventbus.go` (`consumeUntilDone`), `common/internalcaller/internalcaller.go`, `notification-service/internal/adapter/metrics/push_metrics.go` và `healthAndMetricsMux` (`cmd/server/main.go:369`), `proto/orca/auth/v1/auth.proto` (`AppendAuditEntryRequest`), `auth-service/migrations/postgres/0013_audit_actor_type.up.sql`, `deploy/alerts/mcp.rules.yaml`, `.github/workflows/backend-go-issue-status-sync.yml`.

Đúng như CR: 4 subject (`orca.project.worktree.created/deleted`, `orca.scm.pull_request.created/merged`); `syncableProviders = {"jira": true}`; `TargetState.OnlyFromCategory` là **một chuỗi**; `Client.Append` chỉ 6 trường; `main.go` chỉ phục vụ `healthSrv.Handler()` (không `/metrics`); `0001` là migration duy nhất của dịch vụ nên `0002` đúng.

Khác hoặc bổ sung so với CR-REQ-024:

1. **Sự kiện PR hiện không đồng bộ được.** `PullRequestLifecycleEvent.ActorUserID` và `LinkedIssueSite` "empty today: scm-integration-service does not publish" (`domain/events.go`); `canSync` bỏ qua sự kiện không có actor. Nên rủi ro "Done sớm ở PR đầu" của CR chỉ xảy ra khi scm-integration-service sau này phát actor. Kiểm tra sở hữu (mục 2.3) vẫn cần, nhưng cho nhánh PR hiện chưa có hành vi để hồi quy.
2. **Hợp đồng lỗi của handler khác mẫu hiện có.** Handler hiện nay nuốt lỗi và gọi `MarkSeen` bất kể kết quả (BR-PI-08, "mark seen regardless"). CR muốn lỗi tra cứu **trả lỗi** để JetStream giao lại. `consumeUntilDone` gọi `msg.Nak()` khi handler trả lỗi; không thấy cấu hình `MaxDeliver` hay backoff trong `ConsumerConfig` của `Subscribe` (chưa đọc hết). Lỗi dai dẳng có thể gây vòng giao lại liên tục. Giải pháp: lỗi tra cứu tạm thời trả lỗi tối đa 3 lần theo bộ đếm bộ nhớ theo `EventID`, rồi `MarkSeen` và đếm `result=failed` (mục 2.3).
3. **Dedup không có `tenant_id`.** `processed_events` chỉ `event_id` (Postgres schema `issuestatussync`, MySQL không schema). Bảng mới `request_sync_state` có `tenant_id` trong khoá, theo cùng quy ước hai dialect.
4. **Audit.** `AppendAuditEntryRequest` đã có `actor_type`, `target_type`, `target_id`, `metadata_json` (BE-MCP-SOL-013) và DB có `CHECK (actor_type IN ('user','agent','system'))`; chỉ client `common/auditclient` chưa dùng. `outcome` chỉ `allowed|denied`.
5. **Trace qua outbox.** `eventbus.Event{ID, TenantID, OccurredAt, Version, Payload}` không có chỗ cho `traceparent`; sửa `Event` chạm mọi service. Giải pháp tránh sửa `common`: `request-service` đặt `traceparent` (W3C) vào payload của hai sự kiện cần thiết, `issue-status-sync` đọc và tạo span link (mục 2.9).
6. `request-service` chưa tồn tại; các sự kiện `orca.request.request.status_changed`, `orca.request.request.completed` là hợp đồng từ CR-REQ-003, chưa có code. Tên stream `REQUEST` chưa kiểm chứng (CR-REQ-001).

## 2. Giải pháp

### 2.1 Hợp đồng sự kiện cần từ `request-service`

Payload `status_changed` (CR-REQ-003 mục 2.x): `{request_id, project_id, from, to, trigger, type, actor_id, actor_kind, stage, reason, at}`. Bổ sung bốn trường cho consumer Jira (yêu cầu với CR-REQ-003, chưa có):

| Trường | Dùng để |
|---|---|
| `source_provider`, `source_site`, `source_ref` | chọn issue mà không gọi lại `request-service` |
| `version` | chống sự kiện cũ (2.4) |
| `reporter_id` | người dự phòng cho credential Jira (2.7) |
| `number` | nội dung bình luận (2.6) |
| `traceparent` | span link (2.9), tuỳ chọn |

`completed` mang cùng tập. `returned` suy từ `status_changed` với `to=request_backlog`.

### 2.2 Ánh xạ Request sang Jira

Như CR-REQ-024 mục 2.2 (giữ nguyên bảng): `executing` (trừ `spike`, `question`) và `analyzing` của `spike`, `question` thì "In Progress" từ `todo`; `completed` thì "Done" từ `todo` hoặc `in_progress`; `request_backlog`, `cancelled`, đổi loại, mọi `awaiting_*`, `classifying`, `planning` thì không đổi. Tên trạng thái đích cấu hình được.

```go
// domain/events.go
type TargetState struct {
    TrackerState       string
    GitHubLabelPatch   string
    OnlyFromCategory   string   // giữ cho mã cũ
    OnlyFromCategories []string // mới; nếu không rỗng thì ưu tiên
}
```

`updateIssueStatus` (cùng `sync_issue_status.go:104-124`) đổi điều kiện thành "category thuộc tập cho phép". **Chạy `gitnexus_impact` trên `updateIssueStatus` trước khi sửa** (quy tắc dự án).

### 2.3 Phân xử với đồng bộ theo worktree và PR

RPC mới `LookupRequestBySource(tenant_id từ metadata, provider, site, ref) returns {found, request_id}` trên `RequestService`, chỉ cho caller nội bộ (`common/internalcaller.Guard`), chỉ trả `found=true` nếu Request chưa `completed|cancelled` **và** cờ của tenant bật (CR-REQ-025). `site` rỗng khớp mọi site (PR event không có site). Không dùng `ListRequests` vì nó kiểm quyền theo người dùng.

Trong `HandleWorktreeLifecycle` và `HandlePullRequestLifecycle`, sau `canSync` và trước `updateIssueStatus`: gọi `RequestLookupClient.Lookup`. `found` thì log, metric `skipped_request_owned`, `MarkSeen`, thoát. Lỗi tạm thời thì trả lỗi (Nak) tối đa 3 lần theo bộ đếm bộ nhớ `map[eventID]int` có giới hạn kích cỡ; lần thứ 4 `MarkSeen`, metric `failed`, không chuyển Jira (đóng an toàn: không đổi khi không chắc).

### 2.4 `request_sync_state` (hai dialect)

| Cột | Postgres | MySQL |
|---|---|---|
| `tenant_id` | `TEXT NOT NULL` | `VARCHAR(255) NOT NULL` |
| `request_id` | `TEXT NOT NULL` | `VARCHAR(255) NOT NULL` |
| `last_version` | `BIGINT NOT NULL` | `BIGINT NOT NULL` |
| `last_target` | `TEXT NOT NULL` | `VARCHAR(64) NOT NULL` |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | `TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)` |
| khoá | `PRIMARY KEY (tenant_id, request_id)` | như vậy |

Postgres tạo trong schema `issuestatussync`; MySQL không schema (theo `0001`). Cập nhật có điều kiện, nguyên tử, không đọc-rồi-ghi:

```sql
-- Postgres: trả 1 dòng nếu áp dụng
INSERT INTO issuestatussync.request_sync_state (tenant_id, request_id, last_version, last_target)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id, request_id) DO UPDATE
  SET last_version = EXCLUDED.last_version, last_target = EXCLUDED.last_target, updated_at = now()
  WHERE issuestatussync.request_sync_state.last_version < EXCLUDED.last_version
RETURNING 1;
-- MySQL: INSERT ... ON DUPLICATE KEY UPDATE last_version = IF(VALUES(last_version) > last_version, VALUES(last_version), last_version), ...
--        rồi đọc ROW_COUNT() hoặc SELECT lại; xem ghi chú rủi ro.
```

Port: `RequestSyncStateStore.Advance(ctx, tenantID, requestID string, version int64, target string) (applied bool, err error)`. Lý do: sự kiện giao at-least-once và có thể đảo thứ tự giữa hai subject; `processed_events` chỉ chặn trùng `event_id`. Bảng tăng vô hạn: thêm việc dọn (retention) vào câu hỏi mở.

### 2.5 `HandleRequestStatus` (`usecase/sync_request_status.go`, mới)

Thứ tự: dedup `processed_events` theo `EventID`; `source_provider != "jira"` thì `skipped_not_jira`; cờ tenant tắt thì `skipped_flag_off` (xem Q2); ánh xạ 2.2, rỗng thì `MarkSeen`; actor 2.7 rỗng thì `skipped_no_actor`; `Advance` trả `applied=false` thì `skipped_stale`; gọi `updateIssueStatus` qua `doWithRetry`; thành công thì audit `request.jira.sync` (2.8) và metric `applied`; luôn `MarkSeen`. Thứ tự `Advance` trước hay sau khi chuyển Jira: **trước** (ưu tiên không chuyển lặp hơn chuyển sót; chuyển sót được vá bởi sự kiện `completed` sau).

### 2.6 Bình luận Jira (tuỳ chọn, tắt)

`ISSUE_SYNC_REQUEST_COMMENTS_ENABLED=false`; khi bật gọi `AddIssueComment` của `issue-tracking-service` với số Request, loại, trạng thái, liên kết Orca; không kèm `body` hay nội dung Solution; best effort.

### 2.7 Actor cho lệnh gọi Jira

`actor_user_id` = `actor_id` của sự kiện nếu `actor_kind=user`; ngược lại `reporter_id`. (CR muốn thêm "người duyệt gần nhất"; phải gọi lại `request-service`, bỏ ở v1.) Rỗng thì bỏ qua có log.

### 2.8 Audit

`common/auditclient`: thêm `AppendDetailed(ctx, e Entry)` với `Entry{TenantID, ActorID, ActorType, Action, Target, TargetType, TargetID, Outcome, IPAddress, MetadataJSON}`; `Append` giữ nguyên chữ ký (gọi `AppendDetailed`). Cùng tinh thần best-effort. Bảng action: CR-REQ-024 mục 2.8 (giữ nguyên): `request.create`, `request.type.confirm`, `request.type.change`, `request.return`, `request.reopen`, `request.cancel`, `solution.choose`, `approval.approve|reject|cancel|expire`, từ chối quyền (`outcome=denied`), `request.flow.set`, `request.jira.sync`. Metadata không chứa `title`, `body`, nội dung Solution. Điểm gọi: một port `AuditRecorder` trong `request-service/internal/usecase`, adapter bọc `auditclient`; use case gọi sau khi giao dịch thành công (không audit lỗi kỹ thuật).

### 2.9 Metric, log, trace

Bảng metric CR-REQ-024 mục 2.9 (giữ nguyên tên và nhãn; cấm nhãn `tenant_id`, `request_id`, tiêu đề). Hiện thực: gói `adapter/metrics` ở `request-service` và `issue-status-sync`, theo `push_metrics.go` (`prometheus.NewRegistry`, `Handler()`); `main.go` của `issue-status-sync` thêm mux `/metrics` như `healthAndMetricsMux`. Gauge `approvals_pending` và `stuck` lấy mẫu từ DB mỗi 30 giây bằng một goroutine; `orca_request_outbox_pending` đếm `outbox_events` chưa gửi. Log `slog` có cấu trúc, không log `title`, `body`, prompt. Trace: `tracing.Init` ở `request-service`, một span mỗi use case; `traceparent` trong payload (mục 1 điểm 5) và `issue-status-sync` tạo span link.

### 2.10 Cảnh báo

`backend-go/deploy/alerts/request.rules.yaml` (mới) cùng định dạng `mcp.rules.yaml` (6 alert của CR, ngưỡng đề xuất). Chưa có nơi nạp (như `mcp.rules.yaml`).

### 2.11 Cấu hình `issue-status-sync`

`REQUEST_SERVICE_ADDR` (mặc định `request-service:9090`), `ISSUE_SYNC_JIRA_STATUS_IN_PROGRESS` ("In Progress"), `ISSUE_SYNC_JIRA_STATUS_DONE` ("Done"), `ISSUE_SYNC_REQUEST_COMMENTS_ENABLED` (false). Thêm vào `config.Config` và `deploy/dev/docker-compose.yml`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Chuyển Jira ở `executing`, không ở `awaiting_*` | "In Progress" chỉ đúng khi việc bắt đầu |
| D2 | Request sở hữu issue thì tắt đồng bộ worktree, PR | Một nguồn ghi duy nhất |
| D3 | Backlog, huỷ không đổi Jira | Không có trạng thái tương đương, tránh mất dữ liệu |
| D4 | `Advance` có điều kiện trước khi chuyển | Chặn sự kiện cũ, ưu tiên không chuyển lặp |
| D5 | Audit chỉ `allowed|denied`; lỗi kỹ thuật vào metric | Ràng buộc enum |
| D6 | `traceparent` trong payload, không sửa `common/eventbus` | Tránh chạm mọi service |
| D7 | Lỗi tra cứu Nak tối đa 3 lần rồi bỏ | Tránh vòng giao lại vô hạn |

## 4. Phụ thuộc và thứ tự

Cần CR-REQ-003 (hai sự kiện và trường bổ sung), 004 (nguồn), 006 (backlog), 013 (hoàn tất) và `request-service` đã chạy. `LookupRequestBySource` và cờ cần BE-REQ-SOL-025 task cờ. Mở khoá ngưỡng rollout của CR-REQ-025. Task: [`../tasks/README.md`](../tasks/README.md).

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `sync_request_status_test.go` | bảng 2.2 đủ 11 loại; dedupe; phiên bản cũ; thiếu actor; provider khác Jira; cờ tắt |
| `sync_issue_status_test.go` (mở rộng) | `skipped_request_owned`; lỗi tra cứu Nak 3 lần rồi bỏ; hồi quy khi không có Request |
| `subscriber_contract_test.go`, `durable_name_test.go` (mở rộng) | 2 subject mới trên stream `REQUEST`, tên durable |
| `request_sync_state_test.go` | tích hợp testcontainers cả Postgres lẫn MySQL: `Advance` nguyên tử, hai tiến trình đồng thời |
| `common/auditclient/client_test.go` | `AppendDetailed` gửi đủ trường; `Append` không đổi |
| `request-service/.../audit_test.go` | mỗi action một bản ghi, `actor_type` đúng, từ chối ghi `denied`, không có `title`, `body` |
| `metrics_test.go` hai service | series và nhãn, không nhãn id |
| Thủ công (chưa chạy) | Jira thật: một issue qua `executing` và `completed` |

Lệnh: `cd backend-go/services/issue-status-sync && go test ./... && go test -tags=integration ./internal/adapter/postgres/...` (và `mysql`); `cd backend-go/common && go test ./auditclient/...`; `promtool check rules deploy/alerts/request.rules.yaml` (cần công cụ, chưa kiểm chứng).

## 6. Rủi ro và điểm chưa kiểm chứng

- Luồng đổi Jira chưa từng chạy trên Jira thật (`task_sources` trên server dev có 0 dòng, README v6 mục 7).
- MySQL `ON DUPLICATE KEY UPDATE` với điều kiện: `ROW_COUNT()` trả 0 khi giá trị không đổi, 1 chèn, 2 cập nhật; cần test trên MySQL thật để phân biệt "chưa áp dụng" (chưa kiểm chứng).
- `MaxDeliver` và backoff của consumer chưa đọc; kiểm `common/eventbus.Consumer.Subscribe`.
- Workflow Jira tuỳ biến thiếu "In Progress" hoặc "Done" thì `ErrTransitionUnavailable`, log và bỏ.
- Người duyệt cuối có thể chưa kết nối Jira, issue không đổi.
- `request_sync_state` không có dọn dẹp.
- SSH và remote: dịch vụ chỉ gọi tracker, không chạm máy dev.

## 7. Câu hỏi mở

1. Bốn trường bổ sung và `traceparent` ở payload cần CR-REQ-003 nhận.
2. Cờ tắt: `issue-status-sync` bỏ qua theo `LookupRequestBySource.found` (chỉ cho Request ownership) hay cần RPC đọc cờ trực tiếp? Mặc định: `HandleRequestStatus` không kiểm cờ (Request đã không phát sự kiện khi cờ tắt ở lệnh đi tiếp; sự kiện từ callback vẫn tới).
3. `hotfix`, `security` có chuyển Jira ngay khi xác nhận loại vì tính khẩn? Mặc định: không.
4. Bình luận Jira khi `returned` bật mặc định cho tenant beta? Mặc định: tắt.
5. Dọn `request_sync_state` (ví dụ giữ 90 ngày) và `processed_events` (CR đã nhắc, chưa làm).

## 8. Tham chiếu

- `backend-go/services/issue-status-sync/internal/usecase/sync_issue_status.go`, `.../ports.go`, `.../sync_issue_status_test.go`, `.../domain/events.go`, `.../adapter/eventbus/subscriber.go`, `.../adapter/eventbus/subscriber_contract_test.go`, `.../adapter/grpcclient/tenant_forwarding.go`, `.../adapter/postgres/processed_events.go`, `.../adapter/mysql/processed_events.go`, `.../config/config.go`, `.../cmd/server/main.go`, `.../migrations/postgres/0001_processed_events.up.sql`, `.../migrations/mysql/0001_processed_events.up.sql`
- `backend-go/common/auditclient/client.go`, `backend-go/common/eventbus/eventbus.go`, `backend-go/common/internalcaller/internalcaller.go`, `backend-go/common/tracing/tracing.go`, `backend-go/common/outbox/outbox.go`
- `backend-go/proto/orca/auth/v1/auth.proto` (`AppendAuditEntryRequest`), `backend-go/services/auth-service/internal/domain/audit.go`, `.../migrations/postgres/0013_audit_actor_type.up.sql`
- `backend-go/services/notification-service/internal/adapter/metrics/push_metrics.go`, `backend-go/services/notification-service/cmd/server/main.go`, `backend-go/deploy/alerts/mcp.rules.yaml`
- `.github/workflows/backend-go-issue-status-sync.yml`, `docs/guides/jira/jira-orca-mapping.md`, `docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `CR-REQ-004-request-intake-from-sources.md`
