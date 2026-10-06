# BE-REQ-SOL-035: Nền tảng bảo mật và tuân thủ: quyền mức Request, RLS thật, `secretscan`, audit chi tiết, giới hạn tốc độ, lưu giữ, xoá, xuất

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. P0: phải xong trước khi bật cờ `request_flow_enabled` cho tenant thật (README v6, CR-REQ-025 cổng GA).

**CR:** [CR-REQ-035](../../../../../../docs/crs/v6/security-compliance/CR-REQ-035-security-and-compliance-baseline.md)
**Service:** `request-service` (interceptor, domain, usecase, adapter, migration hai dialect) · `backend-go/common` (`secretscan` mới, `grpcmw`, `tenant`, `auditclient` dùng) · `backend-go/policy/orca-authz/request.rego` (mới) · `api-gateway` (gắn `actor_type`, token nội bộ, webhook) · `backend-go/ci`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (4 lớp tenant, RLS), [`arch/06`](../../../../tdd/architecture/06-secrets-vault-architecture.md) (bí mật), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (AuthZ, audit, input validation, supply chain), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), service: [`api-gateway`](../../../../tdd/services/api-gateway.md), [`auth-service`](../../../../tdd/services/auth-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-REQ-035; `common/grpcmw/grpcmw.go` (`TenantExtractionInterceptor`, `ChainUnary`, `StatsHandler`, hằng `MetadataTenantID|UserID|Role|ClientIP`), `common/internalcaller/internalcaller.go` (`Guard`, `StreamGuard`, `ClientInterceptor`, khoá `x-orca-internal-token`, token rỗng ⇒ từ chối hết), `common/tenant/tenant.go` (`WithRole`, `Role`, `ClientIP`, `RequireTenantID`), `common/auditclient/client.go` (`Append`, 6 trường, nuốt lỗi), `common/policy/evaluator.go` (`Decision`, `Value`), `common/outbox/outbox.go` (relay đẩy lên NATS), `policy/orca-authz/project.rego`, `services/project-service/internal/adapter/opaclient/client.go` (mẫu gọi Evaluator), `services/project-service/internal/usecase/authorization.go:109` (tra vai trò thành viên), `proto/orca/project/v1/project.proto` (`ListMembers`), `api-gateway/internal/adapter/grpc/dial.go` (`AttachIdentity`, `Dial` không mã hoá), `api-gateway/cmd/server/mcp_governance_wiring.go` (đọc `MCP_INTERNAL_CALLER_TOKEN`, gắn `internalcaller.ClientInterceptor`), `api-gateway/internal/adapter/mcpserver/tools/executor.go:126,138` (`CallTool` dựng `wscompat.Identity` từ `Principal`), `mcp-service/internal/domain/secret_redactor.go` (9 mẫu), `mcp-service/internal/adapter/postgres/tenant_tx.go`, `auth-service/migrations/postgres/0013_audit_actor_type.up.sql` (`actor_type ∈ user|agent|system`), `proto/orca/auth/v1/auth.proto:371-382`, `ci/check-opa-bundle-in-images.sh`. Spec v6 đã có: `request-quality-rollout/tasks/TASK-REQ-024-01` (`auditclient.AppendDetailed`), `TASK-REQ-024-02` (`AuditRecorder`), `TASK-REQ-025-01, 02` (`tenant_settings`, `flow_gate`), `request-lifecycle/tasks/TASK-REQ-004-08` (webhook), `approval/solutions/BE-REQ-SOL-010`. `request-service` chưa có trên đĩa.

### Correction relative to CR-REQ-035

| # | CR nói | Mã thật | Xử lý |
|---|--------|---------|-------|
| C1 | Thêm `x-orca-actor-type` vào `common/grpcmw`, "gateway điền" | `AttachIdentity` được gọi ở ~20 file `wscompat/channels_*.go` với `usecase.Identity` (không có trường actor); `grpcmw.ChainUnary` và `TenantExtractionInterceptor` dùng chung 16 service (blast radius CRITICAL theo comment trong `grpcmw.go`) | Chỉ **thêm hằng** `MetadataActorType` ở `grpcmw` và cặp `tenant.WithActorType/ActorType` ở `common/tenant` (additive); `AttachIdentity` đọc actor từ ctx như đã làm với `tenant.ClientIP` (không đổi chữ ký); `Executor.CallTool` đặt `agent` vào ctx. Phía `request-service` có `ActorTypeInterceptor` riêng, **không** sửa `TenantExtractionInterceptor` |
| C2 | `internalcaller.Guard(GATEWAY_INTERNAL_TOKEN, ...)` và khoá riêng `SERVICE_INTERNAL_TOKEN` | `Guard` nhận **một** token cho một danh sách phương thức | Hai thể hiện `Guard`: phương thức công khai với token gateway, phương thức nội bộ (`ReportTaskOutcome`, `LookupRequestBySource`, callback) với token service. Danh sách lấy từ `RequestService_ServiceDesc`/`ApprovalService_ServiceDesc`, không gõ tay |
| C3 | `ExportTenantRequests` truyền NDJSON | Chuỗi chuẩn `grpcmw.ChainUnary` chỉ unary; `Guard` không phủ stream | Task 04 thêm `grpc.ChainStreamInterceptor` (`StreamGuard`, `streamIdentity`, `streamAccess`, `streamRateLimit`) |
| C4 | `AppendDetailed` nhận `Metadata map[string]any` | TASK-REQ-024-01 (đã có spec) định nghĩa `Entry{... MetadataJSON string}` | Một nơi định nghĩa: TASK-REQ-024-01; request-service tự marshal bằng `audit_metadata.go` và **không** chứa `title`/`body` |
| C5 | `request_audit_outbox` "giao qua `common/outbox`" | `common/outbox` đẩy lên NATS, không gọi RPC | Bảng riêng + bộ giao `AuditOutboxDeliverer` (`FOR UPDATE SKIP LOCKED`, gọi `AppendDetailed`, đánh `delivered_at`); at-least-once ⇒ `metadata.audit_id` để người đọc khử trùng (`AppendAuditEntry` không có khoá idempotency) |
| C6 | `reporter_id` băm không đảo ngược khi ẩn danh | `reporter_id UUID NOT NULL` dùng để cấp quyền reporter | Thay bằng UUID dẫn xuất `HMAC-SHA256(REQUEST_ERASE_HMAC_KEY, tenant|reporter_id)` cắt 128 bit; khoá rỗng ⇒ từ chối xoá (fail closed) |
| C7 | Xoá nội dung Request | Nhiều bảng của feature khác cũng chứa văn bản tự do: `context_packs.body`, `evidence.excerpt` (CR-031), `ai_trace_blobs` (CR-034), `analysis_runs.raw_output` (CR-007) | `ErasableColumns` là registry trong domain; test tích hợp quét `information_schema` đòi mọi cột `TEXT`/`JSON` hoặc nằm trong registry hoặc trong danh sách miễn trừ |
| C8 | `secretscan` khớp mẫu `mcp-service` | `mcp-service` giữ bản riêng trong `internal/`; không sửa ở CR này | Vector kiểm thử chung `testdata/vectors.json` để hai bản không lệch (không import chéo) |

## 2. Giải pháp

### A. Cây thư mục

```
backend-go/common/
  secretscan/{scan.go,patterns.go,kinds.go,scan_test.go,testdata/vectors.json}     (mới)
  grpcmw/grpcmw.go                                  (sửa nhỏ: hằng MetadataActorType)
  tenant/tenant.go                                  (sửa nhỏ: WithActorType, ActorType)
backend-go/policy/orca-authz/{request.rego,request_test.rego}                       (mới)
backend-go/services/request-service/
  internal/domain/{request_access.go,rpc_catalog.go,erasable_columns.go,retention_policy.go}   (mới)
  internal/usecase/{authorize_request_action.go,rate_limit_policy.go,run_request_retention.go,erase_request.go,export_request.go,audit_outbox.go}
  internal/adapter/grpc/{interceptors.go,actor_type.go,request_access.go,rate_limit.go,stream_interceptors.go,server_compliance.go}
  internal/adapter/opaclient/request_policy.go      (mới; mẫu project-service/opaclient)
  internal/adapter/grpcclient/project_role_resolver.go
  internal/adapter/{postgres,mysql}/{tenant_tx.go,audit_outbox.go,webhook_nonces.go,retention.go}
  internal/adapter/mysql/tenant_scope_test.go
  migrations/{postgres,mysql}/NNNN_security_compliance.{up,down}.sql
backend-go/services/api-gateway/                    (sửa nhỏ: actor type, token nội bộ, webhook thời gian)
backend-go/ci/check-opa-bundle-in-images.sh         (sửa: thêm request-service)
```

`NNNN`: `ls backend-go/services/request-service/migrations/{postgres,mysql}` lúc làm, hai dialect cùng số; phụ thuộc TASK-REQ-025-01 (`tenant_settings`). Cấm tên `helpers`, `utils`, `common`, `misc` cho file của `request-service` (thư mục `backend-go/common` là của repo, không đổi); không `max-lines` disable.

### B. Chuỗi interceptor và danh mục RPC

```go
// cmd/server/main.go (thứ tự thực thi)
grpc.NewServer(
    grpcmw.ChainUnary(logger), grpcmw.StatsHandler(),                // recovery, tenant/user/role/IP, logging
    grpc.ChainUnaryInterceptor(
        internalcaller.Guard(cfg.GatewayInternalToken, publicMethods...),
        internalcaller.Guard(cfg.ServiceInternalToken, internalMethods...),
        ActorTypeInterceptor(),                                       // x-orca-actor-type -> ctx, thiếu = user
        flowGate(...),                                                // TASK-REQ-025-02
        RequestAccessInterceptor(catalog, authz),                     // mục C
        RateLimitInterceptor(limiter, catalog),                       // mục F
    ),
    grpc.ChainStreamInterceptor(internalcaller.StreamGuard(...), streamIdentity(), streamAccess(...), streamRateLimit(...)),
)
```

`rpc_catalog.go` (domain, hàm thuần): `type Group string` (`read`, `create`, `triage`, `analyze`, `plan`, `execute`, `lifecycle`, `decide`, `admin`, `authenticated`, `internal`) và `type Locator int` (`LocNone`, `LocRequestID`, `LocApprovalID`, `LocProjectID`); `var Catalog = map[string]Entry{fullMethod: {Group, Locator, Class(rate), AgentAllowed bool}}`. Khởi động gọi `ValidateCatalog(serviceDescs ...grpc.ServiceDesc)`: mọi phương thức của `RequestService`, `ApprovalService`, `AiBudgetAdminService` phải có dòng, nếu thiếu thì `log.Fatal` (tiêu chí chấp nhận 3). Danh sách nhóm theo RPC (README v6 mục 3.6 và 8, cộng RPC của CR 031, 034, 035):

| Nhóm | RPC |
|---|---|
| `read` | `GetRequest`, `ListRequests`, `ListSolutions`, `GetRequestFlow`, `ListRequestTypeHistory`, `ListRequestLinks`, `ListBacklog`, `ListApprovals`, `GetApproval`, `ListRequestChecks`, `GetEvidence`, `GetContextPack`, `SearchContextSources` |
| `create` | `CreateRequest`, `SpawnChildRequest` |
| `triage` | `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType` (agent: chỉ `ClassifyRequest`) |
| `analyze` | `GenerateSolution`, `ChooseSolutionOption` (agent: chỉ `GenerateSolution`) |
| `plan` | `GeneratePlan`, `CommitPlan` |
| `execute` | `StartPhase`, `RecordRequestCheck` (chốt khi đọc proto của CR-014) |
| `lifecycle` | `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `RequestApproval` |
| `decide` | `Approve`, `Reject`, `Cancel` (Approval): cửa trước, tập người duyệt do Go (CR-010) |
| `authenticated` | `ListPendingForUser`, `GetRequestFlowSettings` (đọc trạng thái cờ cho UI) |
| `admin` | `SetRequestFlowSettings`, `ListContextSources`, `UpsertContextSource`, `SetContextSourceStatus`, `PreviewContextPack`, mọi RPC của `AiBudgetAdminService`, `EraseRequest`, `ExportRequest`, `ExportTenantRequests`, quản trị chính sách Approval |
| `internal` | `ReportTaskOutcome`, `LookupRequestBySource` (và callback nội bộ khác) |

### C. `request.rego` và `RequestAccessInterceptor`

```rego
package orca.authz.request
import rego.v1

# nhóm -> vai trò được phép; "reporter" là cờ riêng, "owner|member" là vai trò dự án
group_roles := {
	"read":      {"owner", "member", "reporter"},
	"create":    {"owner", "member"},
	"triage":    {"owner", "reporter"},
	"analyze":   {"owner", "reporter"},
	"plan":      {"owner", "reporter"},
	"execute":   {"owner"},
	"lifecycle": {"owner", "reporter"},
}
agent_rpcs := {"GetRequest", "ListRequests", "ListSolutions", "GetRequestFlow", "ListRequestTypeHistory", "ListRequestLinks",
	"ListBacklog", "CreateRequest", "SpawnChildRequest", "ClassifyRequest", "GenerateSolution"}

caller_roles contains r if { r := input.caller_project_role; r != "" }
caller_roles contains "reporter" if input.is_reporter

default allow := false
allow if { input.actor_type != "agent"; input.caller_global_role == "admin" }
allow if { input.actor_type != "agent"; some r in caller_roles; r in group_roles[input.action] }
allow if { input.actor_type == "agent"; input.rpc in agent_rpcs; input.caller_global_role == "admin" }
allow if { input.actor_type == "agent"; input.rpc in agent_rpcs; some r in caller_roles; r in group_roles[input.action] }
allow if { input.action == "decide"; input.actor_type != "agent" }          # cửa trước; tập người duyệt do Go
allow if { input.action == "authenticated"; input.actor_type != "agent" }
```

Input `{action, rpc, caller_global_role, caller_project_role, is_reporter, actor_type}`. `agent` bị từ chối ở `decide`, `execute`, `admin`, `lifecycle`, `plan` dù người dùng phía sau là admin (không có luật `allow` nào cho agent ngoài `agent_rpcs`). `RequestAccessInterceptor` (`interceptors.go`): tra `Catalog[info.FullMethod]`; `internal` do `Guard` đã chặn; `authenticated` cần tenant và user; còn lại: định vị Request theo `Locator` (nạp theo `tenant_id` + id, **không thấy trả `NOT_FOUND` chung**, không phân biệt "không tồn tại" và "tenant khác"), lấy `project_id`, `reporter_id`; vai trò dự án qua `ProjectRoleResolver` (gọi `project-service` `ListMembers`, cache 30 giây theo `(tenant, project, user)`); gọi `opaclient.RequestPolicy.Decision(ctx, input)` (`data.orca.authz.request.allow`); `false` ⇒ `PermissionDenied` `REQUEST_FORBIDDEN` và audit `outcome=denied` (`request.access.denied`, không nội dung). `x-orca-role` rỗng ⇒ không admin. RPC không có `Request` cụ thể (`ListRequests` không lọc, `ListBacklog`): interceptor chỉ cho qua với `read`, **usecase** lọc theo dự án mà người gọi là thành viên (admin xem hết); danh sách dự án lấy từ `ProjectRoleResolver.ProjectsOf(user)`.

### D. RLS, `tenant_id`, tách vai trò DB

Mọi bảng của `request-service` (mục 3.5, 8 của README v6, và bảng của CR 009, 010, 031, 034, 035): `tenant_id NOT NULL`, chỉ mục dẫn đầu `tenant_id`, Postgres `ENABLE` + `FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` `USING` và `WITH CHECK` với `NULLIF(current_setting('app.tenant_id', true), '')::uuid`, mọi truy vấn qua `InTx` đặt `set_config('app.tenant_id', $1, true)` (mẫu `mcp-service/tenant_tx.go`; không theo `task-service`). Bảng `outbox_events` thêm policy riêng bộ phát `app.relay`. Vai trò DB của ứng dụng **không phải chủ bảng, không `BYPASSRLS`, không superuser**: migration chạy bằng vai trò chủ, dịch vụ kết nối bằng `request_app` (test `TestAppRoleCannotBypassRLS` đọc `pg_roles`). MySQL không có RLS: kiểu repository nhận `tenantID` ở mọi hàm và `tenant_scope_test.go` duyệt AST (`go/parser`) mọi hằng chuỗi SQL trong `adapter/mysql/*.go`, đòi `tenant_id` trừ danh sách cho phép tường minh (vòng phát outbox, quét hết hạn, `AuditOutboxDeliverer`, `RunRetention`). Meta-test Postgres: mọi bảng schema `request` phải có `relrowsecurity` và `relforcerowsecurity` (trừ danh sách miễn trừ rỗng).

### E. Audit chi tiết, `secretscan`, che dữ liệu

- **Audit:** `AuditRecorder` (TASK-REQ-024-02) gọi `AppendDetailed` (TASK-REQ-024-01); bổ sung hành động `request.read.denied`, `request.access.denied`, `request.export`, `request.erase`, `request.retention.run`, `ai.budget.set`, `ai.egress.set`. `metadata_json` chỉ có id, loại, trạng thái, số lượng, `provenance` rút gọn, `request_id`, `audit_id`; **không bao giờ** `title`, `body`, nội dung Solution/Plan, `comment` Approval (test thuộc tính duyệt mọi bản ghi). Hành động không-được-mất (`request.export`, `request.erase`, `approval.approve` cổng `pre_deploy`, đổi `ai_egress_mode`) ghi `request_audit_outbox` cùng giao dịch; `AuditOutboxDeliverer` giao bằng `AppendDetailed`.
- **`secretscan`:** `Scan(text) []Finding{Kind, Confidence, Start, End}`, `Redact(text) (string, bool)`, `RedactKinds(text) (string, []string)`, hằng `PatternsVersion = "ss/1"`. Mẫu: 9 mẫu của `mcp-service` + `sk-ant-`, `AIza[0-9A-Za-z_-]{35}`, `hvs.<24+>`, `scheme://user:pass@host`, khối khoá riêng OpenSSH, dòng `.env` có tên chứa `SECRET|TOKEN|KEY|PASSWORD`. Không có mẫu `s.<24>` của Vault cũ (quá nhiều dương tính giả). Ba điểm áp dụng: cổng vào (chỉ độ tin cậy cao ⇒ `[REDACTED:<kind>]` và `requests.contains_secret_suspected=true`), trước prompt (mọi độ tin cậy), đầu ra AI/agent trước khi lưu.
- **PII:** tenant bật `redact_pii_in_prompts` thì che trong prompt (không trong bản lưu); log không có `title`/`body`/email.

### F. Giới hạn tốc độ, webhook, lưu giữ, xoá, xuất

- **Rate limit:** bốn lớp `read|write|ai|webhook` (bảng CR 2.7) bằng `golang.org/x/time/rate` theo `(tenant, lớp)` + trần đồng thời đếm bằng DB (`analysis_runs running` 10 mỗi tenant, 2 mỗi project; Request chưa kết thúc 2000 mỗi tenant); vượt: `ResourceExhausted` `REQUEST_RATE_LIMITED` kèm `retry-after` (chi tiết gRPC `RetryInfo`). Bộ trong bộ nhớ mỗi bản sao một bộ (giống gateway).
- **Webhook:** `X-Orca-Timestamp` đưa vào chuỗi ký; lệch quá 5 phút bị từ chối; nonce lặp (bảng `request_webhook_nonces`, hết hạn 10 phút) bị từ chối. Sửa route của TASK-REQ-004-08; gateway ghi nonce qua RPC nội bộ hoặc bảng nằm ở `request-service` (chọn: RPC nội bộ `RecordWebhookNonce`, nhóm `internal`).
- **Lưu giữ:** `RunRetention` hằng ngày, `FOR UPDATE SKIP LOCKED` (hoặc khoá tư vấn theo CR-DB-002), lô 200, ẩn danh hoá nội dung Request `completed|cancelled` quá `request_retention_days` (730); `ai_trace_blobs` quá `ai_trace_retention_days` (30) bị xoá; `ai_usage_ledger` quá `ledger_retention_days` (400) bị xoá. **Xoá theo yêu cầu** `EraseRequest` (admin, không khi `executing`), cùng ẩn danh hoá ngay, gọi cổng `TaskContentEraser` (RPC xoá nội dung ở `task-service` chưa có: `ErrNotSupported`, trả trong `external_erasure[]`). **Xuất** `ExportRequest` (≤ 5 MB, qua `secretscan.Redact`) và `ExportTenantRequests` (stream NDJSON, phân trang `(created_at, id)`). Mọi thao tác ghi audit không-được-mất.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Thi hành quyền bằng interceptor ở `request-service` | Gateway không kiểm OPA trước định tuyến |
| 2 | Hai `Guard`, hai token | `Guard` nhận một token; tách cổng công khai và nội bộ |
| 3 | Chỉ thêm hằng, không sửa interceptor chung | Blast radius CRITICAL của `grpcmw` |
| 4 | Ma trận quyền Rego, tập người duyệt Go | Dữ liệu nhỏ đúng chỗ OPA; tập người duyệt phụ thuộc dữ liệu |
| 5 | `agent` không có luật `allow` ngoài `agent_rpcs` | Cổng tồn tại để người kiểm soát agent |
| 6 | `NOT_FOUND` chung cho "không có" và "tenant khác" | Không lộ tồn tại |
| 7 | MySQL dùng test quét AST thay RLS | Không có RLS ở MySQL |
| 8 | Ẩn danh hoá, không xoá hàng; `reporter_id` HMAC | Giữ thống kê và truy vết |
| 9 | `ErasableColumns` + test quét `information_schema` | Cột mới không lọt khỏi việc xoá |

## 4. Phụ thuộc và thứ tự

```
SOL-001, 002 ─▶ 035-01 (secretscan, độc lập) ─▶ 031-02, 034-04, 008 dùng
035-02 (actor type, gateway) ─▶ 035-04 ◀─ 035-03 (rego)
035-05 (RLS) ─▶ mọi repository; 035-06 (migration + audit outbox) ─▶ 035-08
TASK-REQ-024-01, 02 (AppendDetailed, AuditRecorder) ─▶ 035-06
035-04 ─▶ 035-07 (rate limit) ─▶ 035-08 (retention, erase, export) ─▶ 035-09 (áp dụng, CI)
```

## 5. Kiểm thử

- **Unit:** `opa test` cho `request_test.rego` (bảng đủ `admin|owner|member|reporter|stranger|agent` × nhóm); `authorize_request_action` bảng; `secretscan` vector dương, âm, tiếng Việt có dấu; bộ giới hạn tốc độ với đồng hồ giả; `RunRetention`, `EraseRequest`, `ExportRequest` với repo giả; `ValidateCatalog`.
- **Integration hai DB:** cách ly hai tenant mọi bảng; "quên `set_config`" trả 0 dòng; `SKIP LOCKED` của job; giao dịch "đổi dữ liệu + `request_audit_outbox`"; `TestEveryTextColumnDeclaredErasableOrExempt`; meta-test RLS.
- **Hợp đồng:** `buf breaking` cho RPC mới; golden `AppendDetailed` so với `AppendAuditEntryRequest`; `parity_test.go` khi thêm kênh.
- **An ninh (adversarial):** bộ Request độc (chỉ dẫn gài, khối `ORCA_RESULT_BEGIN` giả, Markdown `javascript:`, id tenant khác trong thân) chạy ở e2e (CR-REQ-025), chưa chạy.
- Lệnh: `cd backend-go && go test ./common/secretscan/... ./common/tenant/... ./services/request-service/... && opa test policy/orca-authz/ -v && go test -tags=integration ./services/request-service/internal/adapter/...`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Nếu mTLS và NetworkPolicy chưa bật, `internalcaller` là lớp bảo vệ duy nhất, token chung có thể lộ; xoay vòng chưa thiết kế.
- Cache vai trò dự án 30 giây: thành viên vừa bị gỡ làm được thêm vài thao tác.
- Che bí mật ngay cổng vào có thể làm mất thông tin khi dương tính giả (người dùng thấy cờ và sửa).
- Ẩn danh hoá không xoá bản sao ở Jira, git, nhà cung cấp LLM, sao lưu; không hứa "xoá hoàn toàn".
- `x-orca-actor-type` do gateway đặt; đường vào khác gateway có thể giả `user`; `Guard` giảm rủi ro, không loại bỏ.
- Hành vi `FORCE RLS` với pool/PgBouncer chưa kiểm chứng.
- Số liệu giới hạn tốc độ, trần đồng thời, thời hạn lưu giữ là đề xuất, chưa đo, chưa có yêu cầu pháp lý.

## 7. Câu hỏi mở

1. Bảng nhóm hành động cần chủ sản phẩm xác nhận (đặc biệt `member` không phân loại, `StartPhase` chỉ `owner|admin`).
2. Webhook chạy với tư cách `reporter_id` đã cấu hình (một `user`, CR-004): có nên đánh `actor_type=system` và cho nhóm `create` hay không.
3. Vai trò dự án `viewer` (chưa có ở `project-service`, chỉ `owner|member`).
4. RPC xoá nội dung ở `task-service` (CR-011) ai sở hữu; sao lưu và phục hồi thảm hoạ DB `request` (RPO, RTO) thuộc CR vận hành nào.
5. Trích `common/secretscan` rồi để `mcp-service` chuyển sang dùng, hay giữ hai bản mãi với vector chung.
6. Quy trình xoay vòng `GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN` khi nhiều bản sao.

## 8. Tham chiếu

- `backend-go/common/{grpcmw/grpcmw.go,internalcaller/internalcaller.go,tenant/tenant.go,auditclient/client.go,policy/evaluator.go,outbox/outbox.go}`
- `backend-go/policy/orca-authz/{project.rego,task_grant.rego,admin.rego}`; `backend-go/ci/check-opa-bundle-in-images.sh`
- `backend-go/services/mcp-service/internal/{adapter/postgres/tenant_tx.go,domain/secret_redactor.go}`; `migrations/postgres/0002_authorization.up.sql`
- `backend-go/services/api-gateway/{internal/adapter/grpc/dial.go,cmd/server/mcp_governance_wiring.go,internal/adapter/mcpserver/tools/executor.go}`
- `backend-go/services/project-service/internal/{adapter/opaclient/client.go,usecase/authorization.go}`; `proto/orca/project/v1/project.proto`
- `backend-go/services/auth-service/migrations/postgres/0013_audit_actor_type.up.sql`; `backend-go/proto/orca/auth/v1/auth.proto`
- `specs/backend-go/crs/v6/request-quality-rollout/tasks/TASK-REQ-024-01-*.md`, `TASK-REQ-024-02-*.md`, `TASK-REQ-025-01-*.md`, `TASK-REQ-025-02-*.md`; `request-lifecycle/tasks/TASK-REQ-004-08-*.md`
- `specs/backend-go/tdd/architecture/07-security-architecture.md`; `docs/research/receive-request/enterprise-readiness-checklist.md`
