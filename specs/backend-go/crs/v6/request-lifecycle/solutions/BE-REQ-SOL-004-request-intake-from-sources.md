# BE-REQ-SOL-004: Tiếp nhận Request từ Jira, GitHub, thủ công, webhook, MCP

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phụ thuộc [BE-REQ-SOL-003](./BE-REQ-SOL-003-request-state-machine-and-flow-registry.md).

**CR:** [CR-REQ-004](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md)
**Service:** `request-service` (use case, adapter gRPC, adapter `grpcclient`, migration) · `proto` · `api-gateway` (`adapter/httpgateway`, chỉ route webhook)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`api-gateway.md`](../../../../tdd/services/api-gateway.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (webhook, HMAC, secret)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `task-service/internal/usecase/create_task_from_source.go` và `domain/task_source.go` (khoá `(tenant, project, provider, site, ref)`, `Site` là connection workspace id, Jira là base URL), `git-gateway-service/internal/adapter/grpcclient/issuetracking_client.go` (gọi `GetIssue` với `Provider`, `IssueId`, chỉ chuyển tenant metadata), `proto/orca/issuetracking/v1/issuetracking.proto` (`GetIssueRequest{provider, issue_id, workspace_id}`, `Issue{title, description_markdown, url, key, labels, issue_type, priority, ...}`, `IssueProvider` chỉ `JIRA`, `LINEAR`), `proto/orca/project/v1/project.proto` (`linked_issue_provider/ref/site`), `api-gateway/internal/adapter/httpgateway/{router.go,scm_webhook_routes.go}` (route webhook SCM mount **ngoài** nhóm JWT, xác thực chữ ký nằm trong `scm-integration-service`), `proto/orca/credentialbroker/v1` (`ResolveCredentialByOwner(tenant_id, category, owner_id)`).

### Correction relative to CR-REQ-004

| # | CR nói | Mã thật | Xử lý |
|---|--------|---------|-------|
| C1 | Webhook `POST /v1/request-webhooks/{source_name}` "bí mật theo `(tenant, source_name)`" | Route không mang tenant; request ngoài nhóm JWT không có danh tính nào. Gateway không biết tra bí mật của tenant nào | Yêu cầu header `X-Orca-Tenant-Id` (xem 2.F). Bí mật sai tenant thì chữ ký không khớp, trả 401 như nhau |
| C2 | HMAC kiểm ở gateway, bí mật lấy qua `credential-broker-service` | Tiền lệ SCM kiểm chữ ký **trong service**, gateway chỉ chuyển byte thô; `credentialbroker` có `ResolveCredentialByOwner` nhưng chưa kiểm chứng có `CredentialCategory` phù hợp | Giữ vị trí kiểm HMAC ở gateway theo CR, nêu phương án tiền lệ SCM ở Q2; phạm vi task: 2.F |
| C3 | `GetIssue` "bằng tín nhiệm của người tạo" | `git-gateway-service` chỉ chuyển tenant metadata; Jira credential theo `(tenant, user)` (CR-TG-008) nên client phải chuyển cả user | `grpcclient` chuyển `x-orca-tenant-id` và `x-orca-user-id` (`grpcmw.MetadataTenantID`, `MetadataUserID`) |
| C4 | `workspace_id` suy từ `site` chưa rõ | `git-gateway` bỏ trống `workspace_id`; `task-service` coi `site` là connection workspace id | Truyền `workspace_id = site` khi `site != ""`, ngược lại để trống; chưa kiểm chứng với Jira thật |
| C5 | `source_hints` thêm bằng migration `0003` | `request-service` chưa có migration nào; `0003` đúng nếu SOL-001, 002 xong và chưa có migration khác | Task đầu tiên đọc thư mục để chốt số |

## 2. Giải pháp

### A. Migration `0003_request_source_hints` (hai dialect)

```sql
-- Postgres
ALTER TABLE request.requests ADD COLUMN source_hints JSONB;
-- MySQL
ALTER TABLE requests ADD COLUMN source_hints JSON NULL;
```
Down: `DROP COLUMN source_hints`. `domain.Request` thêm `SourceHints SourceHints` (`IssueType string; Labels []string; Priority string; TypeHint string`; `TypeHint` do CR-REQ-006 dùng). Cả hai repository đọc/ghi cột này (sửa `request_scan.go`, `Create`, `Update`).

### B. Proto (thêm vào `request.proto`)

```proto
message RequestSource { string provider = 1; string ref = 2; string url = 3; string site = 4; }
message SourceHints  { string issue_type = 1; repeated string labels = 2; string priority = 3; string type_hint = 4; }
message CreateRequestRequest {
  string project_id = 1; string title = 2; string body = 3;
  RequestSource source = 4; SourceHints hints = 5; string client_request_id = 6;
}
message CreateRequestResponse { Request request = 1; bool created = 2; }
// Request thêm: SourceHints source_hints = 24;
// ListRequestsRequest thêm: string source_provider = 6; string source_site = 7; string source_ref = 8;
```

`reporter_id` không có trên dây: lấy từ `tenant.UserID(ctx)` (metadata do gateway gắn).

### C. Chuẩn hoá (`internal/domain/request_source_normalization.go`)

| Trường | Quy tắc |
|--------|---------|
| `site` Jira | trim, scheme và host chữ thường, bỏ `/` cuối; rỗng được phép và là site riêng (không wildcard) |
| `ref` Jira, Linear | trim, chữ hoa |
| `ref` GitHub, GitLab | trim, phần `owner/repo` chữ thường, giữ `#<số>`; GitLab `group/subgroup/repo#12` giữ nhiều cấp |
| `title` | trim, 1 đến 500 ký tự (đếm rune) |
| `body` | tối đa 100000 ký tự (đếm rune) |

Hàm: `NormalizeSourceRef(SourceRef) (SourceRef, error)`, `NormalizeTitle`, `NormalizeBody`, `IdempotencyKey(provider, site, ref, reporterID, clientRequestID) (IdempotencyKey, error)`. Quy tắc khoá theo provider:

| Provider | Khoá `(source_provider, source_site, source_ref)` |
|----------|---------------------------------------------------|
| `jira`, `linear`, `github`, `gitlab` | `(provider, site, ref)` đã chuẩn hoá |
| `webhook` | `(webhook, source_name, ref)` |
| `manual`, `mcp` | `(provider, 'user:<reporter_id>', client_request_id)`; `requests.source_ref` vẫn rỗng; không có `client_request_id` thì không claim (mỗi lần một Request; `mcp` nên gửi) |

Nhánh nội dung: `github`/`gitlab` bắt buộc `title` (không có RPC đọc issue phía server).

### D. Use case `CreateRequest` (`internal/usecase/create_request.go`)

```go
type CreateRequestInput struct {
    ProjectID, Title, Body string
    Source domain.SourceRef; Hints domain.SourceHints
    ClientRequestID string
    AllowTypeHint bool   // chỉ SpawnChildRequest (SOL-006) đặt; CreateRequest công khai bỏ qua hints.type_hint
}
type CreateRequestResult struct { Request domain.Request; Created bool }
func (uc *CreateRequest) Execute(ctx context.Context, in CreateRequestInput) (CreateRequestResult, error)
func (uc *CreateRequest) CreateWithinTx(ctx context.Context, in CreateRequestInput) (CreateRequestResult, error) // lõi, SOL-006 dùng lại
```

Các bước: kiểm tenant, reporter, `project_id` (`REQUEST_PROJECT_REQUIRED`), provider; chuẩn hoá; `Idempotency.Find` (có thì trả Request đó, `Created=false`, kể cả đã `cancelled`/`completed`); **làm giàu ngoài giao dịch** qua cổng `IssueFetcher.GetIssue(ctx, provider, ref, site)` khi Jira/Linear có `ref` mà `title` rỗng (`REQUEST_SOURCE_NOT_FOUND`, `REQUEST_SOURCE_FETCH_FAILED`; nếu `title` đã có thì lỗi chỉ log); rồi `TxRunner.InTx`: `Claim` (thua race: trả Request của bên thắng, rollback giao dịch) → `NextNumber` → `Create` (`status=new`, `urgency=normal`, `type` rỗng) → `TransitionRequest.Execute` (`start_classification`, lồng trong giao dịch) → outbox `orca.request.request.created` `{request_id, project_id, number, source_provider, source_site, source_ref, reporter_id, title}` (thêm `parent_request_id`, `link_reason` ở SOL-006). Hai sự kiện cùng giao dịch giữ thứ tự nhờ cột `seq` của outbox (SOL-001, D2).

### E. Đọc Request

`GetRequest`, `ListRequests` đã thật (SOL-002). Solution này nối ba bộ lọc nguồn vào `ListFilter` ở adapter gRPC và hai repository (`WHERE source_provider = ? AND source_site = ? AND source_ref = ?`, chỉ khi đặt).

### F. Webhook (`api-gateway`)

Route `POST /v1/request-webhooks/{source_name}` mount trong `router.go` cạnh `mountSCMWebhookRoutes`, ngoài nhóm JWT. Header: `X-Orca-Tenant-Id`, `X-Orca-Signature: sha256=<hex>`. Thân tối đa 256 KiB (`http.MaxBytesReader`), vượt thì 413. Bí mật theo `(tenant, source_name)` qua `credentialbroker.ResolveCredentialByOwner(tenant_id, category, owner_id=source_name)`; so `hmac.Equal` thời gian hằng; sai hoặc thiếu hoặc không tìm thấy bí mật: 401 cùng một thông báo (không lộ lý do). Payload chuẩn `{project_id, ref, title, body, url, hints}`; gateway gọi `CreateRequest` với `provider=webhook`, `site=source_name`, kèm metadata `x-orca-tenant-id` và `x-orca-user-id` bằng `reporter_id` cấu hình theo nguồn. Giao lặp trả 200, `created=false`. Cấu hình tạm thời `REQUEST_WEBHOOK_SOURCES` (JSON `{ "<source_name>": {"reporter_id": "<uuid>"} }`) cho tới khi CR-REQ-025 có `tenant_settings`; `reporter_id` không có trong cấu hình thì 401.

### G. Mã lỗi

`REQUEST_PROJECT_REQUIRED`, `REQUEST_REPORTER_REQUIRED`, `REQUEST_SOURCE_PROVIDER_INVALID`, `REQUEST_SOURCE_REF_REQUIRED`, `REQUEST_BODY_TOO_LARGE`, `REQUEST_TITLE_REQUIRED` (đã có) là `InvalidArgument`; `REQUEST_SOURCE_NOT_FOUND` `NotFound`; `REQUEST_SOURCE_FETCH_FAILED` `Internal`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Trả Request cũ kể cả đã đóng | Tránh bản sao; mở lại là hành động có chủ đích (CR-REQ-006) |
| Làm giàu ngoài giao dịch | Không giữ khoá `request_counters` trong lúc gọi mạng |
| Tách `CreateWithinTx` | `SpawnChildRequest` (SOL-006) dùng cùng lõi, không tạo đường vào thứ hai |
| `site` rỗng là site riêng | Wildcard cần `OR` trong truy vấn, hai dialect khó dùng cùng khoá duy nhất |
| `github`/`gitlab` client tự gửi nội dung | Chưa có RPC đọc issue phía server |
| Webhook cấu hình reporter qua env tạm | Chưa có bảng cấu hình theo tenant (CR-REQ-025) |

## 4. Phụ thuộc và thứ tự

Cần SOL-003 (`TransitionRequest`, `InTransaction`) và SOL-002 (`Claim`, `Find`, `NextNumber`). Mở khoá SOL-005 (Request vào `classifying`), CR-REQ-016, 017, 019, 024. Thứ tự task: 01 migration + domain field, 02 chuẩn hoá, 03 proto, 04 use case, 05 client `issuetracking`, 06 gRPC + bộ lọc nguồn, 07 test tích hợp, 08 webhook gateway (độc lập sau 06).

## 5. Kiểm thử

- **Unit:** chuẩn hoá `site`, `ref`, `title`, `body`; khoá theo provider; use case với repo, `IssueFetcher` giả (thành công, không thấy, lỗi mạng khi có/không có `title`).
- **Integration, Postgres và MySQL:** hai `CreateRequest` cùng khoá trả cùng id (`created` đúng/sai); 12 lệnh đồng thời cùng khoá: một Request, một số, một `created` trong outbox; `eng-1` và `ENG-1` cùng khoá, hai site khác cho hai Request; lỗi giữa chừng không để `request_idempotency` mồ côi và không đốt số; Request `cancelled` nhận lại cùng nguồn trả bản cũ; migration `0003` up/down.
- **Hợp đồng:** `buf breaking` cho trường thêm; test gateway webhook với HMAC mẫu (chữ ký sai 401, đúng giao hai lần một Request, thân quá 256 KiB bị từ chối).
- **Chưa kiểm chứng:** `GetIssue` với Jira thật (README v6 mục 7: `task_sources` trên server dev có 0 dòng).

## 6. Rủi ro và điểm chưa kiểm chứng

- Thứ tự `created` rồi `status_changed` chỉ đúng nếu outbox có `seq` (SOL-001 D2); nếu `0001` đã phát hành không có cột này thì cần migration bổ sung trước khi consumer của CR-REQ-005 dựa vào thứ tự.
- `GetIssue` cần `workspace_id`: suy từ `site` chưa kiểm chứng (C4).
- Khoá không có `project_id`: một issue gắn một Request cho cả tenant (SOL-002 Q2).
- Bộ đếm theo tenant tuần tự hoá tạo Request; webhook bùng nổ có thể chậm.
- Chuẩn hoá `ref` GitLab nhiều cấp chưa kiểm với dữ liệu thật.
- Danh sách `CredentialCategory` của `credential-broker-service` chưa kiểm chứng có loại cho bí mật webhook.

## 7. Câu hỏi mở

- **Q1.** `source_hints` lưu hay lấy lại từ nguồn khi phân loại (CR giữ lưu).
- **Q2.** Kiểm HMAC ở gateway (theo CR) hay trong `request-service` theo tiền lệ SCM (gateway chỉ chuyển byte thô); phương án thứ hai tránh gateway phải biết bí mật.
- **Q3.** `reporter_id` của webhook: người dùng dịch vụ theo nguồn (đang dùng cấu hình env tạm).
- **Q4.** Có cần RPC đọc issue GitHub, GitLab phía server.
- Sẽ được CR-REQ-028 mở rộng: `CreateRequest` có thể sinh trạng thái `awaiting_information`; không phụ thuộc ở đây.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.7, 8
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/create_task_from_source.go`, `domain/task_source.go`
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/issuetracking_client.go`
- `/opt/repos/orca/backend-go/proto/orca/issuetracking/v1/issuetracking.proto`, `proto/orca/project/v1/project.proto`, `proto/orca/credentialbroker/v1/`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, `scm_webhook_routes.go`
- `/opt/repos/orca/backend-go/common/grpcmw/grpcmw.go` (`MetadataTenantID`, `MetadataUserID`)
- `/opt/repos/orca/backend-go/services/request-service/internal/usecase/create_request.go`, `adapter/grpcclient/issue_tracking_client.go` (mới)
