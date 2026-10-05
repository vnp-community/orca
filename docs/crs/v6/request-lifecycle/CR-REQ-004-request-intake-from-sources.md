# CR-REQ-004 — Tiếp nhận Request từ Jira, GitHub, thủ công, webhook, MCP

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-004 |
| **Tên** | RPC `CreateRequest` idempotent theo nguồn; đọc Request (`GetRequest`, `ListRequests`); đường vào webhook |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-003 |
| **Mở khoá** | CR-REQ-005, 016, 017, 019, 024 |
| **Tác động** | `request-service` (use case, adapter gRPC, adapter `issuetracking` client, migration `0003`), `proto/orca/request/v1/request.proto`, `api-gateway/internal/adapter/httpgateway` (route webhook, mới). Gateway wscompat thuộc CR-REQ-016, tool MCP thuộc CR-REQ-017 |

---

## 1. Bối cảnh và vấn đề

1. Không có đường tạo Request. Từ Jira, luồng hiện tại là "Start work" tạo worktree rồi tùy chọn `task.createFromSource` (CR-TG-008); không có bước nào xử lý yêu cầu trước khi thành Task.
2. Cần một khoá duy nhất theo `(tenant, provider, site, ref)` để cùng một issue không sinh hai Request (README v6 mục 3.5). `task_sources` đã có mẫu và bài học: khoá Jira `ENG-1` chỉ duy nhất trong một site (migration `0014`).
3. Năm nguồn có dữ liệu khác nhau. `issue-tracking-service` chỉ hỗ trợ Jira và Linear (`enum IssueProvider` có `JIRA`, `LINEAR`); GitHub, GitLab không có RPC `GetIssue` nào trong repo ở thời điểm khảo sát.

CR này sở hữu tiếp nhận và idempotency. Phân loại bắt đầu sau khi Request vào `classifying` (CR-REQ-005).

## 2. Giải pháp đề xuất

### 2.1 Proto (thêm vào `request.proto`)

```
message RequestSource { string provider = 1; string ref = 2; string url = 3; string site = 4; }
message SourceHints  { string issue_type = 1; repeated string labels = 2; string priority = 3; }
message CreateRequestRequest {
  string project_id = 1; string title = 2; string body = 3;
  RequestSource source = 4; SourceHints hints = 5;
  string client_request_id = 6;   // idempotency cho manual và mcp
}
message CreateRequestResponse { Request request = 1; bool created = 2; }
```

`ListRequestsRequest` (CR-REQ-001) được thêm ba trường tùy chọn, cộng thêm không phá `buf breaking`: `source_provider = 6`, `source_site = 7`, `source_ref = 8`, để frontend hỏi "issue này đã có Request chưa".

### 2.2 Quy tắc theo nguồn

| `source.provider` | `ref` | `title`, `body` | Idempotency |
|-------------------|-------|-----------------|-------------|
| `jira`, `linear` | bắt buộc (`ENG-1`) | nếu `title` rỗng, server gọi `issue-tracking-service.GetIssue` rồi điền `title`, `body` (`description_markdown`), `url`, `hints` | khoá `(tenant, provider, site, ref)` |
| `github`, `gitlab` | bắt buộc (`owner/repo#123`) | **client gửi đủ** `title`, `body` (chưa có RPC đọc issue phía server) | như trên |
| `manual` | rỗng | client gửi | chỉ khi có `client_request_id` |
| `mcp` | rỗng | client gửi | khuyến nghị `client_request_id`; không có thì mỗi lần gọi một Request |
| `webhook` | bắt buộc (id sự kiện hoặc id đối tượng của hệ thống gửi) | từ payload đã ánh xạ | khoá `(tenant, 'webhook', site, ref)` với `site` = tên nguồn đăng ký |

Idempotency của `manual` và `mcp`: khoá `(tenant, provider, 'user:<reporter_id>', client_request_id)` ghi vào `request_idempotency`; `requests.source_ref` vẫn rỗng (khoá là khoá gọi lại, không phải nguồn).

### 2.3 Chuẩn hoá trước khi tạo khoá

| Trường | Quy tắc |
|--------|---------|
| `site` (Jira) | cùng định dạng với `task_sources.site_id` (connection workspace id, base URL): trim, scheme và host về chữ thường, bỏ `/` cuối. Rỗng được phép (không biết) nhưng khi rỗng, khoá coi là một site riêng, **không** wildcard như `task_sources` |
| `ref` Jira, Linear | trim, chữ hoa (`eng-1` thành `ENG-1`) |
| `ref` GitHub, GitLab | trim, `owner/repo` chữ thường, `#<số>` giữ nguyên |
| `title` | trim, 1 đến 500 ký tự |
| `body` | tối đa 100000 ký tự |

### 2.4 Use case `CreateRequest` (`internal/usecase/create_request.go`, mới)

1. `tenant.RequireTenantID` và `tenant.UserID` (`REQUEST_REPORTER_REQUIRED` nếu thiếu); `project_id` bắt buộc (`REQUEST_PROJECT_REQUIRED`); provider hợp lệ.
2. Chuẩn hoá (2.3). Tìm khoá có sẵn (`RequestIdempotencyRepository.Find`, phương thức bổ sung); có thì trả Request đó, `created=false`, kể cả khi nó đang `cancelled` hoặc `completed` (muốn làm lại phải `ReopenRequest`, CR-REQ-006; không tạo bản sao).
3. Làm giàu, **ngoài transaction**: Jira, Linear có `ref` mà `title` rỗng thì gọi `GetIssue` bằng tín nhiệm của người tạo (credential Jira lưu theo `(tenant, user)`, bài học của CR-TG-008). Không có issue: `REQUEST_SOURCE_NOT_FOUND`. Lỗi kết nối: `REQUEST_SOURCE_FETCH_FAILED`. Nếu `title` đã có thì lỗi làm giàu chỉ ghi log, vẫn tạo.
4. `TxRunner.InTx`: `Claim` khoá (thua race thì rollback và trả Request của bên thắng) → `NextNumber` → `Create` với `status=new`, `urgency=normal`, `type` rỗng → `TransitionRequest(start_classification)` trong cùng transaction → hai dòng outbox.
5. Trả `created=true`.

Dòng outbox: `orca.request.request.created` payload `{request_id, project_id, number, source_provider, source_site, source_ref, reporter_id, title}` rồi `orca.request.request.status_changed` (new sang classifying) của CR-REQ-003. Phân loại AI (CR-REQ-005) chạy khi thấy `classifying`, không phải trong transaction này.

`hints` được lưu vào cột mới `requests.source_hints` (JSONB/JSON, NULL được), thêm bằng migration `0003_request_source_hints` cho cả hai dialect, để CR-REQ-005 dùng làm gợi ý đầu vào. Thêm cột này cần cập nhật README v6 mục 3.5 (Q1).

### 2.5 Đọc Request

`GetRequest(id)`: `REQUEST_NOT_FOUND` cho id không có hoặc thuộc tenant khác (không phân biệt, tránh lộ tồn tại). `ListRequests`: `ListFilter` của CR-REQ-002 cộng lọc nguồn; `page_size` mặc định 50, tối đa 200. Quyền đọc theo project do gateway kiểm (README v6 mục 6).

### 2.6 Webhook (`api-gateway`)

Route `POST /v1/request-webhooks/{source_name}` (mới, trong `httpgateway`, cạnh `scm_webhook_routes.go`; chưa kiểm chứng cách route SCM xác thực nên CR này không tái dùng mã đó). Xác thực HMAC-SHA256 trên thân với bí mật theo `(tenant, source_name)` lấy từ Vault qua `credential-broker-service`; sai chữ ký trả 401, không phát lỗi chi tiết. Thân tối đa 256 KiB. Payload chuẩn của Orca: `{project_id, ref, title, body, url, hints}`. Gateway gọi `CreateRequest` với `provider=webhook`, `site=source_name`, và `reporter_id` là người dùng dịch vụ cấu hình cho nguồn đó (Q2). Giao lặp trả 200 với `created=false`. Không có ánh xạ trực tiếp từ webhook Jira, GitHub gốc ở v6; người dùng tạo Request từ UI (CR-REQ-019).

### 2.7 Mã lỗi

| Mã | Kind | Khi |
|----|------|-----|
| `REQUEST_PROJECT_REQUIRED`, `REQUEST_REPORTER_REQUIRED` | InvalidArgument | thiếu |
| `REQUEST_SOURCE_PROVIDER_INVALID` | InvalidArgument | provider ngoài tập |
| `REQUEST_SOURCE_REF_REQUIRED` | InvalidArgument | provider khác `manual`, `mcp` mà `ref` rỗng |
| `REQUEST_TITLE_REQUIRED`, `REQUEST_BODY_TOO_LARGE` | InvalidArgument | theo 2.3 |
| `REQUEST_SOURCE_NOT_FOUND` | NotFound | `GetIssue` không thấy |
| `REQUEST_SOURCE_FETCH_FAILED` | Internal | lỗi tới `issue-tracking-service` khi cần làm giàu |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Trả Request cũ, kể cả đã đóng | Tránh bản sao; mở lại là hành động có chủ đích |
| Làm giàu ngoài transaction | Không giữ khoá bộ đếm trong lúc gọi mạng |
| `site` rỗng là site riêng, không wildcard | Wildcard của `task_sources` cần OR trong truy vấn, MySQL và Postgres khó dùng cùng khoá duy nhất |
| `github`, `gitlab` client tự gửi nội dung | Chưa có RPC đọc issue phía server; thêm RPC là CR riêng của `issue-tracking-service` |
| Idempotency `manual`/`mcp` theo người gọi | Hai người dùng cùng `client_request_id` không đụng nhau |
| Tạo xong vào `classifying` ngay | README v6 mục 3.3; người dùng không phải bấm "bắt đầu" |

## 4. Tiêu chí chấp nhận

- [ ] Hai `CreateRequest` cùng `(provider, site, ref)` trả cùng `request.id`, lần đầu `created=true`, lần sau `false` (cả hai dialect).
- [ ] 12 lệnh đồng thời cùng khoá: đúng một Request được tạo, đúng một `number` được dùng, đúng một `request.created` trong outbox.
- [ ] `eng-1` và `ENG-1` cùng một khoá; hai site Jira khác nhau, cùng `ENG-1`, cho hai Request.
- [ ] Jira có `ref` và `title` rỗng: Request lấy `title`, `body`, `hints` từ `GetIssue` (test với client giả); `GetIssue` lỗi và `title` có sẵn thì vẫn tạo.
- [ ] `github` thiếu `title` trả `REQUEST_TITLE_REQUIRED`, không gọi service ngoài.
- [ ] Sau tạo, `status = classifying`, có đúng hai sự kiện theo thứ tự.
- [ ] Ép lỗi giữa chừng: không còn dòng `request_idempotency` mồ côi, số Request không bị đốt.
- [ ] Request `cancelled` nhận lại cùng nguồn trả Request cũ, trạng thái giữ nguyên.
- [ ] Webhook: chữ ký sai trả 401; chữ ký đúng giao hai lần trả một Request; thân vượt 256 KiB bị từ chối.
- [ ] Tenant A không thấy, không đụng khoá của tenant B.

## 5. Kiểm thử

- **Unit:** chuẩn hoá `site`, `ref`, `title`; chọn nhánh theo provider; use case với repo, `issuetracking` client giả (thành công, không thấy, lỗi mạng).
- **Integration, Postgres và MySQL:** các tiêu chí tranh chấp, rollback, khoá theo site; migration `0003` up/down.
- **Hợp đồng:** `buf breaking` cho ba trường thêm của `ListRequestsRequest`; test gateway webhook với HMAC mẫu.
- **Chưa kiểm chứng:** gọi `issue-tracking-service` thật với Jira thật (README v6 mục 7: `task_sources` trên server dev có 0 dòng, Jira chưa được chạy thật).

## 6. Rủi ro và điểm chưa kiểm chứng

- `GetIssue` cần `workspace_id`; cách suy ra từ `site` chưa kiểm chứng. Gần đây `project-service` đã lưu khoá và site Jira theo project (commit `5b3bf3ef3`), cần đọc lại để biết lấy `workspace_id` từ đâu.
- Khoá không có `project_id` (CR-REQ-002 Q2): một issue gắn một Request cho cả tenant.
- Bộ đếm theo tenant tuần tự hoá tạo Request; webhook bùng nổ có thể chậm.
- Chuẩn hoá `ref` GitLab (`group/subgroup/repo#12`) chưa kiểm với dữ liệu thật.

## 7. Câu hỏi mở

- **Q1.** README v6 mục 3.5 chưa có cột `source_hints`; thêm hay lấy lại từ nguồn khi phân loại.
- **Q2.** Webhook cần `reporter_id` NOT NULL (CR-REQ-002); dùng người dùng dịch vụ theo nguồn, hay cho phép `reporter_id` rỗng. Cần chốt.
- **Q3.** Có cần RPC đọc issue GitHub, GitLab phía server (để MCP và webhook không phải mang nội dung).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.7
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md` (khoá nguồn, site, credential theo user)
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/create_task_from_source.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0014_task_sources_site.up.sql`
- `/opt/repos/orca/backend-go/proto/orca/issuetracking/v1/` (`GetIssue`, `Issue`, `IssueProvider`)
- `/opt/repos/orca/backend-go/proto/orca/project/v1/project.proto` (`linked_issue_provider`, `linked_issue_site`)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/httpgateway/scm_webhook_routes.go`
- `/opt/repos/orca/backend-go/services/request-service/internal/usecase/create_request.go`, `adapter/grpcclient/issue_tracking_client.go` (mới)
