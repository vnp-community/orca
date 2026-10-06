# CR-REQ-035 — Nền tảng bảo mật và tuân thủ cho luồng Request: mô hình đe doạ, quyền mức Request, cách ly tenant, kiểm toán, lưu giữ và xoá, bí mật, giới hạn tốc độ

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-035 |
| **Tên** | Mô hình đe doạ cho luồng AI; thi hành quyền ở `request-service`; mô hình quyền mức Request (`request.rego`); RLS và `tenant_id`; kiểm toán đầy đủ (`AppendDetailed`); lưu giữ, xoá, xuất dữ liệu; `secretscan`; giới hạn tốc độ; kiểm tra bảo mật độc lập; đối chiếu `07-security-architecture.md` |
| **Loại** | Feature (nền tảng phi chức năng, chặn rollout) |
| **Priority** | 🔴 P0 (phải xong trước khi bật cờ `request_flow_enabled` cho tenant thật) |
| **Effort** | Large (10 đến 13 ngày: quyền 3, RLS và kiểm thử cách ly 2, audit 2, `secretscan` và redaction 2, giới hạn tốc độ 1, lưu giữ/xoá/xuất 3) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001, 002 (service và bảng), CR-REQ-003 (hành động theo trạng thái), CR-REQ-009, 010 (Approval và người duyệt), CR-REQ-016 (kênh gateway) |
| **Mở khoá** | CR-REQ-025 (cổng GA cần review bảo mật), CR-REQ-017 (tool MCP ghi), CR-REQ-034 (`EgressGuard`, che bí mật trong prompt) |
| **Tác động** | `request-service` (interceptor, domain, usecase, adapter, migration hai dialect), `backend-go/policy/orca-authz/request.rego` (mới), `backend-go/common/auditclient`, `common/grpcmw` (một khoá metadata), `common/secretscan` (mới), `api-gateway` (gắn `actor_type`), `auth-service` (chỉ nhận thêm hành động audit, đã có cột) |

## 1. Bối cảnh và vấn đề (đã đọc code ngày 2026-10-06)

1. **Gateway không kiểm quyền trước định tuyến.** `backend-go/services/api-gateway/README.md` ghi "No OPA authorization check ahead of routing" và "Still not production-safe"; gateway chỉ gắn `Identity` (`AttachIdentity` trong `adapter/grpc/dial.go` đặt metadata `x-orca-tenant-id`, `x-orca-user-id`, `x-orca-role`, `x-orca-client-ip`). `common/grpcmw.TenantExtractionInterceptor` **tin metadata đó** (comment của chính nó: placeholder cho tới khi có mTLS). Dial gateway tới service còn dùng thông tin xác thực không mã hoá (chỉ dev). Hệ quả: mọi RPC của `request-service` phải tự kiểm quyền, và phải chặn người gọi trong cụm giả metadata.
2. **`x-orca-role` chỉ có ở đường cookie/phiên** (`common/tenant.Role`); đường JWT bearer có thể rỗng. Rỗng phải được coi là không phải admin (CR-REQ-010 đã thống nhất).
3. **Request chưa có `Grant`.** `Grant` của `task-service` theo Task, không mô tả Request. CR-REQ-003 và CR-REQ-010 đều chưa chốt ai có quyền ghi ở mức Request (README v6 cuối mục 8, và `enterprise-readiness-checklist.md` mục 2.3).
4. **Cách ly tenant ở lớp DB không đồng đều.** `mcp-service` đặt `app.tenant_id` trong giao dịch (`internal/adapter/postgres/tenant_tx.go`, `withTenantTx`) và dùng `FORCE ROW LEVEL SECURITY` (`migrations/postgres/0002_authorization.up.sql`). Ngược lại `task-service/internal/adapter/postgres/share_link.go` ghi rõ `app.tenant_id` "never SET anywhere", tức RLS ở đó chỉ là chính sách nằm im. MySQL không có RLS. `usage-service` có RLS nhưng cũng dựa vào lọc ở ứng dụng.
5. **Audit thiếu trường.** `common/auditclient.Append` gửi 6 trường; proto `AppendAuditEntryRequest` đã có `actor_type`, `target_type`, `target_id`, `metadata_json` và bảng `auth.audit_log` đã có cột `actor_type` (`0013_audit_actor_type.up.sql`, giá trị `user|agent|system`). `outcome` chỉ `allowed|denied`. CR-REQ-024 mục 2.8 đã đề xuất `AppendDetailed` và danh sách hành động; CR này mở rộng phần dữ liệu nhạy cảm và truy vết.
6. **Bí mật.** `mcp-service/internal/domain/secret_redactor.go` có bộ mẫu (PAT GitHub, `AKIA`, Bearer, `sk-`, `xox`, JWT, khoá riêng PEM, `password=`), nhưng nằm trong `internal/` của service đó. CR-REQ-008 mục 2.4 có che bí mật ở đầu ra; CR-REQ-007 cấm `credential_ref` trong prompt (task-service `buildDecomposePrompt` hiện đưa vào).
7. **Giới hạn tốc độ.** Gateway có bộ giới hạn theo tenant trong bộ nhớ, mỗi bản sao một bộ (`internal/usecase/rate_limit.go`); không biết lớp lời gọi (đọc, ghi, AI). CR-REQ-008 đặt trần 2 run `agent_readonly` mỗi project; CR-REQ-004 giới hạn thân webhook 256 KiB, `title` 500, `body` 100 000 ký tự.
8. **Vòng đời dữ liệu.** Không thấy chính sách lưu giữ, xoá hay xuất ở bất kỳ service nào của repo (chỉ audit append-only ở `auth.audit_log`). `enterprise-readiness-checklist.md` mục 4 đánh dấu thiếu.
9. Tài liệu kiến trúc `specs/backend-go/tdd/architecture/07-security-architecture.md` tồn tại; mục 2.11 của CR này đối chiếu từng mục.

## 2. Giải pháp đề xuất

### 2.1 Mô hình đe doạ cho luồng AI

Tài sản: nội dung Request (có thể chứa mã nguồn, lỗi sản xuất, thông tin khách), mã nguồn trên dev server, bí mật (khoá nhà cung cấp, token Jira/GitHub), quyền ghi vào repo qua agent, ngân sách AI, tính toàn vẹn của cổng duyệt.

| # | Mối đe doạ | Đường vào thực tế | Kiểm soát (CR này và CR liên quan) | Còn lại |
|---|---|---|---|---|
| T1 | Chèn chỉ dẫn vào prompt | `title`/`body` từ Jira, GitHub, webhook, MCP; comment; `source_hints` | Khối dữ liệu có rào và đầu ra kiểm schema (CR-REQ-005/007); khối kết quả có `nonce` (CR-REQ-033 2.5); chỉ đọc ép bằng cờ CLI (CR-REQ-033 2.2); không cấp `trustPreset=full` ở bước phân tích; mục 2.6 che bí mật | `ai.complete` không có công cụ nên thiệt hại tối đa là nội dung xấu và người duyệt chặn; bước `execute` có shell, cần kiểm phạm vi sau chạy (CR-REQ-029 đề xuất) |
| T2 | Rò bí mật | (a) bí mật dán trong `body` rồi vào prompt gửi nhà cung cấp LLM; (b) đầu ra agent chứa bí mật đọc từ repo; (c) `env` của `execPrompt`; (d) nhật ký | `secretscan` ở cổng vào, trước prompt, và ở đầu ra (2.6); `env` chỉ cho phép `ORCA_REQUEST_ID`, `ORCA_PROJECT_ID`; log không có `title`/`body` | Mẫu regex không phủ hết; bí mật nằm trong mã nguồn đã đi qua `execPrompt` theo thiết kế |
| T3 | Vượt tenant | metadata giả từ trong cụm; thiếu `tenant_id` ở một truy vấn; consumer outbox lẫn tenant; id đoán được | `internalcaller.Guard` mọi RPC (2.2); RLS có `FORCE` và `set_config` mỗi giao dịch (2.4); test cách ly (2.4); payload sự kiện luôn có `tenant_id` và consumer kiểm khớp | MySQL không có RLS, chỉ có lọc ở ứng dụng |
| T4 | Lạm dụng tool ghi | MCP `request_*`/`approval_*` do agent gọi; agent tự duyệt kế hoạch của chính nó; gọi hàng loạt | `actor_type=agent` bị từ chối ở `approve`, `reject`, `StartPhase`, `admin` (2.3); chính sách của `mcp-service` (CR-REQ-017); audit đầy đủ | Tool ghi `request.create` do agent vẫn cho phép, có hạn mức (2.7) |
| T5 | Từ chối dịch vụ theo tenant | tạo Request hàng loạt; bấm sinh lại liên tục; webhook bão; nhiều run `agent_readonly` làm sập kết nối dev server | giới hạn tốc độ theo lớp và trần đồng thời ở `request-service` (2.7); trần body; ngân sách AI (CR-REQ-034) | Giới hạn trong bộ nhớ từng bản sao, không chặn chính xác khi nhiều bản sao |
| T6 | Giả mạo webhook, phát lại | `POST /v1/request-webhooks/*` | HMAC-SHA256 theo `(tenant, source)` (CR-REQ-004); thêm tiêu đề thời gian và cửa sổ 5 phút (2.7) | Bí mật HMAC do người cấu hình giữ |
| T7 | Tự duyệt, vượt tách nhiệm vụ | người báo cáo duyệt cổng rủi ro cao | CR-REQ-010 (`self_approval_allowed`) | Tenant một người |
| T8 | Chối bỏ hành động | không biết ai sinh, chọn, duyệt, đổi loại | Audit đầy đủ (2.5), `provenance` (CR-REQ-034) | Audit best-effort (2.5) |
| T9 | XSS qua nội dung AI hoặc Jira hiển thị trong giao diện | Solution/Plan/Request có Markdown, HTML, liên kết | Frontend không render HTML thô, làm sạch Markdown, chặn `javascript:` (CR-REQ-019/020/021 cần nêu tiêu chí); API trả `Content-Type: application/json` | Chưa đọc mã frontend của series này (chưa tồn tại) |
| T10 | Chuỗi cung ứng công cụ ngoài | OpenSpec, MCP ngoài, công cụ cài trên dev server | Ngoài phạm vi: chỉ cho chạy công cụ có trong hồ sơ năng lực; chính sách MCP ngoài thuộc `mcp-service` | Chưa có CR cho công cụ ngoài |

Tài liệu mô hình đe doạ này là nguồn cho test (mục 5): mỗi hàng T1 đến T8 có ít nhất một test tự động.

### 2.2 Thi hành quyền ở `request-service` (chuỗi interceptor)

Thứ tự cho mọi RPC `RequestService` và `ApprovalService` (file `cmd/server/main.go` và `internal/adapter/grpc/interceptors.go`, mới):

1. `internalcaller.Guard(GATEWAY_INTERNAL_TOKEN, <mọi phương thức công khai>)`: chỉ gateway (và `mcp-service` qua gateway) được gọi. Có khoá riêng `SERVICE_INTERNAL_TOKEN` cho RPC nội bộ (`ReportTaskOutcome`, callback outbox, `LookupRequestBySource`). Token rỗng thì từ chối tất cả (fail closed, đúng hành vi `Guard`). Cách này bổ sung chứ không thay mTLS (`07-security-architecture.md`, mục service-to-service).
2. `grpcmw.TenantExtractionInterceptor` (tenant, user, role, IP) và interceptor mới `ActorTypeInterceptor` đọc `x-orca-actor-type` (`user|agent|system`, thiếu thì `user`). Gateway đặt `agent` khi phiên là token MCP (mục 2.3).
3. `RequestAccessInterceptor`: dựa vào bảng hành động (2.3) tra từ tên RPC; nạp `Request` theo `tenant_id` + id (không thấy trả `NOT_FOUND`, không phân biệt "không tồn tại" và "tenant khác" để tránh dò id), nạp vai trò dự án, gọi `request.rego`.
4. Bộ giới hạn tốc độ (2.7).
5. Handler. Mọi use case vẫn gọi `tenant.RequireTenantID`.

Thất bại quyền trả `PermissionDenied` với mã `REQUEST_FORBIDDEN` (và các mã chi tiết của CR-REQ-010 cho Approval), ghi audit `denied` (không ghi nội dung). Lý do tập trung ở interceptor: CR-REQ-016 mục 6 ghi rủi ro "thiếu kiểm quyền ở một RPC lộ ngay qua WS và HTTP"; interceptor làm lỗi thiếu khai báo thành lỗi khởi động (xem tiêu chí chấp nhận thứ ba).

### 2.3 Mô hình quyền mức Request (đề xuất cụ thể, khớp CR-REQ-010)

Nguồn vai trò, không thêm hệ thống mới:
- `role=admin` toàn cục: `tenant.Role(ctx)`.
- Vai trò dự án `owner|member`: `project-service` (`ListMembers`; mẫu `policy/orca-authz/project.rego`, input `caller_project_role`, `caller_global_role`). Cache theo `(tenant, project, user)` 30 giây.
- `reporter`: `caller == requests.reporter_id`.
- Loại tác nhân: `actor_type` (`user|agent|system`).

Nhóm hành động theo RPC (tên RPC theo README v6 mục 3.6 và mục 8):

| Nhóm | RPC | Được phép |
|---|---|---|
| `read` | `GetRequest`, `ListRequests`, `ListSolutions`, `GetRequestFlow`, `ListRequestTypeHistory`, `ListRequestLinks`, `ListBacklog`, `ListApprovals` (của Request), kênh sự kiện | `admin`, `owner`, `member`, `reporter` |
| `create` | `CreateRequest`, `SpawnChildRequest` | `admin`, `owner`, `member`; `agent` được (có hạn mức) |
| `triage` | `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType` | `admin`, `owner`, `reporter`; `agent` chỉ `ClassifyRequest` |
| `analyze` | `GenerateSolution`, `ChooseSolutionOption` | `admin`, `owner`, `reporter`; `agent` chỉ `GenerateSolution` |
| `plan` | `GeneratePlan`, `CommitPlan` | `admin`, `owner`, `reporter` |
| `execute` | `StartPhase` | `admin`, `owner` (chạy agent ghi mã, cổng đắt nhất) |
| `lifecycle` | `ReturnToBacklog`, `ReopenRequest`, `CancelRequest` | `admin`, `owner`, `reporter` |
| `decide` | `Approve`, `Reject`, `Cancel` Approval | theo CR-REQ-010 (principal + tách nhiệm vụ); **`agent` luôn bị từ chối** `REQUEST_APPROVAL_AGENT_FORBIDDEN` |
| `admin` | `Get/SetRequestFlowSettings`, quản trị chính sách Approval (CR-REQ-010), ngân sách (CR-REQ-034), lưu giữ/xoá/xuất (2.8) | `admin` |

`system` (callback outbox, quét hết hạn) dùng khoá `SERVICE_INTERNAL_TOKEN`, không đi qua bảng này. Thành viên dự án thường (`member`) đọc và tạo nhưng không phân loại, không duyệt: người làm một mình vẫn là `owner` nên không bị khoá.

Hiện thực: `backend-go/policy/orca-authz/request.rego` (mới, package `orca.authz.request`, cùng khuôn `project.rego`: bảng `action_roles`, `allow` cho admin, từ chối `agent` ở nhóm `decide` và `execute` và `admin`), có `request_test.rego`. Input `{action, caller_global_role, caller_project_role, is_reporter, actor_type}`. Lý do dùng Rego ở đây dù CR-REQ-010 chọn Go cho tập người duyệt: ma trận hành động-vai trò là dữ liệu nhỏ, đúng chỗ OPA đã dùng (`project.rego`, `task_grant.rego`) và là "một nơi duyệt toàn bộ luật quyền" của `07-security-architecture.md`; còn tập người duyệt phụ thuộc dữ liệu Request, team nên giữ Go. `request-service` dùng `common/policy.Evaluator` nhúng; ảnh container phải có gói (đã có `backend-go/ci/check-opa-bundle-in-images.sh`, cần thêm `request-service`).

`x-orca-actor-type`: thêm hằng `MetadataActorType` vào `common/grpcmw` và gateway điền ở đường MCP (`adapter/mcpserver`) là `agent`; các đường khác `user`. Điều này giải câu hỏi mở "cơ chế nhận biết nguồn máy" của CR-REQ-009 (câu 5) và CR-REQ-010 (câu 6).

### 2.4 RLS và `tenant_id` cho mọi bảng

1. Mọi bảng của `request-service` (mọi bảng ở README v6 mục 3.5 và mục 8, cộng bảng của CR-REQ-009/010/034/035) có `tenant_id NOT NULL` và chỉ mục dẫn đầu là `tenant_id`.
2. **Postgres:** `ENABLE` và `FORCE ROW LEVEL SECURITY`, chính sách `tenant_isolation USING (tenant_id = current_setting('app.tenant_id', true)::uuid)`; thêm `WITH CHECK` cùng biểu thức. Vai trò DB của ứng dụng không phải chủ bảng và không có `BYPASSRLS`. Mọi truy vấn qua `withTenantTx` (sao theo `mcp-service/internal/adapter/postgres/tenant_tx.go`, `set_config('app.tenant_id', $1, true)` cục bộ giao dịch). Bảng outbox dùng chính sách riêng cho bộ phát (`app.relay`), như `mcp-service`. Không sao theo `task-service` (RLS im lặng).
3. **MySQL:** không có RLS. Biện pháp: kiểu repository bắt buộc nhận `tenantID` ở mọi hàm; test "quét câu SQL" (`internal/adapter/mysql/tenant_scope_test.go`, mới) duyệt mọi hằng chuỗi SQL và yêu cầu có `tenant_id`; ngoại lệ phải khai báo tường minh trong danh sách cho phép (chỉ vòng phát outbox và quét hết hạn).
4. **Test cách ly (cả hai DB):** hai tenant, mỗi bảng chèn dữ liệu rồi đọc, sửa, xoá bằng ngữ cảnh tenant kia và mong 0 dòng; riêng Postgres thêm test "quên `set_config`" phải trả 0 dòng (không phải lỗi dữ liệu lẫn).
5. Id là UUID ngẫu nhiên; vẫn trả `NOT_FOUND` chung (2.2) để không lộ tồn tại.

### 2.5 Kiểm toán đầy đủ

Thêm `Client.AppendDetailed(ctx, entry)` ở `common/auditclient` (CR-REQ-024 đề xuất; CR này thống nhất tên và trường): `entry` gồm `TenantID, ActorID, ActorType, Action, TargetType, TargetID, Outcome, IP, Metadata map[string]any`; `Metadata` được mã hoá JSON vào `metadata_json`. Giữ `Append` nguyên, giữ best-effort (lỗi nuốt, đúng ghi chú của `client.go`).

Quy tắc bổ sung so với bảng hành động của CR-REQ-024 mục 2.8:
- Thêm hành động: `request.read.denied` (chỉ khi từ chối), `request.export`, `request.erase`, `request.retention.run`, `ai.budget.set`, `ai.egress.set` (CR-REQ-034), `request.access.denied`.
- `metadata_json` **không bao giờ** chứa `title`, `body`, nội dung Solution/Plan, `comment` Approval; chỉ id, loại, trạng thái, số lượng, `provenance` rút gọn (`prompt_id`, `prompt_version`, `model`).
- Kiểm soát độ mất mát: `outcome` chỉ `allowed|denied` và Append nuốt lỗi nên audit có thể mất khi `auth-service` sập. Với hành động đặc biệt (`request.export`, `request.erase`, `approval.approve` cổng `pre_deploy`, đổi `ai_egress_mode`) thêm bản ghi vào bảng `request_audit_outbox` (mới, cùng giao dịch với thay đổi, giao qua `common/outbox` rồi `AppendDetailed`) để không mất khi lỗi tạm thời. Hành động thường vẫn best-effort.
- Truy vết xuyên suốt: `metadata.request_id`, `metadata.correlation_id` (trace id nếu có; `common/outbox` không truyền trace context theo `enterprise-readiness-checklist.md` mục 4, nên dùng `request_id`).
- Không chỉnh sửa hay xoá audit: dùng chính sách của `auth.audit_log` (chỉ INSERT/SELECT ở production, `0001_init.up.sql`).

### 2.6 Bí mật, che dữ liệu và `secretscan`

- `backend-go/common/secretscan/` (mới): `Scan(text) []Finding`, `Redact(text) (string, bool)`, phiên bản bộ mẫu `patterns_version`. Khởi tạo từ bộ mẫu của `mcp-service/internal/domain/secret_redactor.go` (không import chéo `internal/`; không sửa `mcp-service` ở CR này), bổ sung: `sk-ant-`, `AIza[0-9A-Za-z_-]{35}`, `hvs.`/`s.`-Vault token, chuỗi kết nối `scheme://user:pass@host`, khoá SSH `ssh-rsa`...(chỉ phần riêng tư), tệp `.env` dạng `KEY=value` với tên chứa `SECRET|TOKEN|KEY|PASSWORD`. Có test golden dùng chung để hai bản không lệch. Độ phủ chưa kiểm chứng; ghi trong test các ví dụ dương và âm.
- Ba điểm áp dụng: (1) **cổng vào** (`CreateRequest`, webhook, MCP): nếu `Scan` có kết quả độ tin cậy cao (khoá riêng, token có tiền tố biết trước) thì thay bằng `[REDACTED:<loại>]` trong `body` lưu trữ và đánh `requests.contains_secret_suspected=true` (cột mới, báo người dùng trong UI); chọn che ngay vì Request đã dán bí mật là sự cố, và giữ bản gốc trong DB chỉ nhân rủi ro. (2) **trước prompt**: mọi văn bản đưa vào prompt (Request, prior_artifacts, kết quả `fs.*`) qua `Redact`; không cấu hình tắt. (3) **đầu ra** AI/agent trước khi lưu (CR-REQ-008 `analysis_secret_redaction.go` đổi sang dùng gói này).
- `PII` (email, số điện thoại): mặc định không che vì làm đổi nghĩa phân tích; tenant bật `redact_pii_in_prompts` thì thay trong prompt (không trong bản lưu). Che dữ liệu trong log: không ghi `title`, `body`; `reporter_id` là UUID, không ghi email; lỗi trả client không chứa nội dung Request.
- Tiêu chuẩn cho agent: `env` của `execPrompt` chỉ gồm `ORCA_REQUEST_ID`, `ORCA_PROJECT_ID` (CR-REQ-008 2.2); không có token Orca, `credential_ref`, khoá nhà cung cấp. Khoá nhà cung cấp nằm trên dev server (biến môi trường của tiến trình agent), không đi qua `request-service`.

### 2.7 Giới hạn tốc độ và hạn mức

`RateLimitInterceptor` trong `request-service`, theo `(tenant, lớp)` với bộ xô token trong bộ nhớ (`golang.org/x/time/rate`, như gateway) cộng trần đồng thời đọc từ DB để chính xác khi nhiều bản sao:

| Lớp | RPC | Mặc định đề xuất (chưa đo) |
|---|---|---|
| `read` | nhóm `read` | 20 req/s, burst 40 |
| `write` | `create`, `triage`, `lifecycle`, `decide` | 5 req/s, burst 10 |
| `ai` | `GenerateSolution`, `GeneratePlan`, `ClassifyRequest`, `StartPhase` | 2 req/s, burst 4 |
| `webhook` | `CreateRequest` từ nguồn `webhook`/`mcp` | 10 req/s, burst 20; MCP theo token |

Trần đồng thời (đếm bằng DB, có chỉ mục): run `running` mỗi tenant 10 (`analysis_runs`); mỗi project 2 (CR-REQ-008); Request ở trạng thái chưa kết thúc mỗi tenant 2000 (đề xuất, tránh bão tạo Request). Vượt trả `ResourceExhausted` với `REQUEST_RATE_LIMITED`, kèm `retry-after` trong chi tiết. Giới hạn kích thước: `title` 500, `body` 100 000 ký tự (CR-REQ-004); `comment` Approval 4000; thân webhook 256 KiB; JSON `options` 64 KB (CR-REQ-007). Webhook thêm tiêu đề `X-Orca-Timestamp` đưa vào chuỗi ký, từ chối lệch quá 5 phút và nonce lặp (bảng `request_webhook_nonces`, hết hạn 10 phút). Giới hạn trong bộ nhớ là mỗi bản sao, giống gateway; giới hạn thật cho AI nằm ở ngân sách (CR-REQ-034).

### 2.8 Lưu giữ, xoá và xuất dữ liệu

Cài đặt theo tenant trong `tenant_settings` (CR-REQ-025): `request_retention_days` (mặc định đề xuất 730; `0` = giữ mãi), `ai_trace_retention_days` (30), `ledger_retention_days` (400).

- **Job lưu giữ** `RunRetention` (quét hằng ngày, `FOR UPDATE SKIP LOCKED` hoặc khoá tư vấn theo CR-DB-002, giới hạn lô 200): Request đã `completed|cancelled` quá hạn bị **ẩn danh hoá nội dung**: `title='[erased]'`, `body=''`, `classification_reason=''`, Solution `options` và `raw_output` về `{}`/NULL, `comment` của Approval rỗng; giữ id, trạng thái, loại, mốc thời gian, `reporter_id` băm không đảo ngược được cần cho audit (không phải xoá hàng, để thống kê và truy vết còn nguyên). Phát `request.retention.run` (số lượng).
- **Xoá theo yêu cầu** `EraseRequest(request_id, reason)` (chỉ `admin`): cùng ẩn danh hoá ngay lập tức, kể cả Request đang xử lý (không cho khi `executing`). Gọi `task-service` để xoá `Description`/`AIContext` của task có `request_id` (cột do CR-REQ-011): chưa có RPC xoá nội dung ở `task-service`, ghi ở mục 7. Audit `request.erase`. Bản sao nằm ngoài `request-service` (Jira, git, nhật ký nhà cung cấp LLM, sao lưu DB) không thuộc phạm vi và phải nêu trong chính sách của khách.
- **Xuất** `ExportRequest(request_id)` trả gói JSON (Request, lịch sử loại, Solution, Approval, liên kết, tóm tắt ledger, `provenance`), tối đa 5 MB; `ExportTenantRequests` (admin, truyền NDJSON, phân trang theo `(created_at, id)`) cho di chuyển và tuân thủ. Gói đã qua `Redact` và không chứa token. Audit `request.export` thuộc nhóm không-được-mất (2.5).
- Sao lưu và phục hồi thảm hoạ cho DB mới (RPO, RTO): ngoài phạm vi, ghi ở mục 7.

### 2.9 Kiểm tra bảo mật độc lập và quy trình

Điều kiện GA (nhắc ở CR-REQ-025 giai đoạn 4): (a) review thiết kế bảo mật bởi người không viết CR; (b) kiểm thử thâm nhập hoặc review chuyên sâu các điểm T1, T3, T4, T5, đặc biệt đường webhook và MCP; (c) `govulncheck` và quét ảnh (Trivy) theo `07-security-architecture.md` mục Input validation & supply chain; (d) kết quả và việc còn mở được ghi vào `docs/guides/request/` (CR-REQ-025). Không có số liệu về tải hay khai thác thật; đây là yêu cầu quy trình, không phải kết quả.

### 2.10 Tệp sẽ tạo

`backend-go/policy/orca-authz/request.rego`, `request_test.rego`; `backend-go/common/secretscan/` (`scan.go`, `patterns.go`, `*_test.go`); sửa `backend-go/common/auditclient/client.go` (thêm `AppendDetailed`), `common/grpcmw/grpcmw.go` (`MetadataActorType`), gateway `adapter/grpc/dial.go` và `adapter/mcpserver` (điền); `request-service`: `internal/adapter/grpc/interceptors.go`, `internal/usecase/authorize_request_action.go`, `rate_limit_policy.go`, `run_request_retention.go`, `erase_request.go`, `export_request.go`, `internal/domain/request_access.go`, `internal/adapter/postgres/tenant_tx.go`, `internal/adapter/mysql/tenant_scope_test.go`, migration `NNNN_security_compliance.{up,down}.sql` hai dialect (`contains_secret_suspected`, `request_audit_outbox`, `request_webhook_nonces`, cột `tenant_settings`).

### 2.11 Đối chiếu `specs/backend-go/tdd/architecture/07-security-architecture.md`

| Mục của tài liệu | Yêu cầu | Trạng thái với `request-service` | Ở đâu |
|---|---|---|---|
| AuthN, Browser/CLI | cookie HttpOnly, JWT RS256, xác thực ở gateway | Dùng nguyên (gateway) | không đổi |
| AuthN, MCP | token có `aud`, không có role claim; PAT audit | Tool MCP đi qua gateway; `actor_type=agent` | 2.3 |
| Origin check (T11) | `WS_ALLOWED_ORIGINS` | Kênh WS Request đi qua `wscompat` | CR-REQ-016; cần đặt biến ở production |
| Service-to-service | mTLS giữa mọi service | Chưa có trong repo (dial không mã hoá ở gateway); thêm `internalcaller` | 2.2, rủi ro mục 6 |
| AuthZ | một gói OPA `orca-authz`, mỗi service nhúng | `request.rego` mới | 2.3 |
| Multi-tenancy, 4 lớp | DB riêng, lọc ứng dụng, RLS, OPA input tenant từ JWT | Đủ bốn lớp; RLS bắt buộc đặt biến | 2.4 |
| Audit logging | append-only, mọi service phát sự kiện bảo mật, lưu lâu | `AppendDetailed`, outbox cho hành động quan trọng | 2.5 |
| Secrets | không giữ bí mật trong DB service | Khoá nhà cung cấp ở dev server; `secretscan` | 2.6 |
| Input validation | `protovalidate` trước `usecase/` | Thêm ràng buộc proto cho độ dài và enum | CR-REQ-001 (cần nêu) |
| Supply chain | `govulncheck`, Trivy | Thêm vào CI của `request-service` | 2.9 |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Thi hành quyền bằng interceptor ở `request-service`, không ở gateway | Gateway không có OPA (README); gateway chỉ gắn danh tính |
| 2 | `internalcaller.Guard` cho mọi RPC | Metadata do `TenantExtractionInterceptor` được tin vô điều kiện |
| 3 | Ma trận quyền dùng Rego, tập người duyệt dùng Go | Tách "ai làm hành động gì" (dữ liệu nhỏ, đúng chỗ OPA) khỏi "ai duyệt Approval này" (phụ thuộc dữ liệu) |
| 4 | Không phân biệt "không có" và "tenant khác" | Không lộ tồn tại |
| 5 | `agent` không bao giờ duyệt, không `StartPhase` | Cổng tồn tại để người kiểm soát agent |
| 6 | `FORCE RLS` + `set_config` mỗi giao dịch (theo `mcp-service`) | RLS của `task-service` đang im lặng; chỉ chính sách mà không đặt biến thì vô nghĩa |
| 7 | MySQL dùng test quét SQL thay RLS | Không có RLS ở MySQL |
| 8 | Che bí mật ngay tại cổng vào khi tin cậy cao | Giữ bản gốc chỉ nhân rủi ro |
| 9 | Audit không chứa nội dung; hành động đặc biệt đi qua outbox | Audit là dữ liệu dài hạn; không được mất hành động quan trọng |
| 10 | Ẩn danh hoá thay vì xoá hàng | Giữ thống kê và khả năng truy vết |

## 4. Tiêu chí chấp nhận

- [ ] Gọi RPC công khai không có `x-orca-internal-token` đúng bị `Unauthenticated`/`PermissionDenied`; token rỗng cấu hình thì từ chối tất cả.
- [ ] Mỗi RPC trong bảng 2.3 có test phân quyền đủ `admin`, `owner`, `member`, `reporter`, người lạ, `agent`; metadata `x-orca-role` rỗng bị coi không phải admin.
- [ ] Khởi động `request-service` thất bại nếu có RPC mới chưa khai báo nhóm hành động.
- [ ] `agent` gọi `Approve`, `StartPhase`, `SetRequestFlowSettings` nhận `PermissionDenied`.
- [ ] Test cách ly hai tenant đạt trên Postgres (có và không có `set_config`) và MySQL cho mọi bảng; test quét SQL của MySQL đỏ khi thêm câu SQL không có `tenant_id`.
- [ ] Mọi hành động ở CR-REQ-024 2.8 và 2.5 tạo đúng một bản ghi `AppendDetailed` với `actor_type` đúng; không có `title`/`body` trong `metadata_json` (test thuộc tính duyệt mọi bản ghi).
- [ ] `request.export` và `request.erase` không mất khi `auth-service` tạm sập (bản ghi nằm trong `request_audit_outbox` và được giao sau).
- [ ] `secretscan`: bộ test dương và âm (gồm tiếng Việt có dấu); `body` chứa khoá riêng PEM được lưu với `[REDACTED:private_key]` và `contains_secret_suspected=true`; prompt dựng ra không chứa chuỗi đó.
- [ ] `env` của `execPrompt` không chứa khoá nào ngoài danh sách cho phép (test trên bộ dựng tham số).
- [ ] Vượt giới hạn lớp `ai` trả `REQUEST_RATE_LIMITED` kèm `retry-after`; thân webhook lệch thời gian 6 phút hoặc nonce lặp bị từ chối.
- [ ] `RunRetention` ẩn danh hoá đúng Request quá hạn và không chạm Request chưa kết thúc; chạy hai bản sao không xử lý trùng.
- [ ] `ExportRequest` không chứa chuỗi bí mật mẫu; vượt 5 MB trả lỗi rõ.
- [ ] Bảng đối chiếu 2.11 được điền kết quả (đạt, chưa, ngoài phạm vi) trong PR đầu tiên của CR.

## 5. Kiểm thử

- **Unit:** `request_test.rego` bằng `opa test`; `authorize_request_action` bảng; `secretscan` golden; bộ giới hạn tốc độ với đồng hồ giả; `RunRetention` và `EraseRequest` với repo giả.
- **Integration hai DB:** cách ly tenant, `FORCE RLS`, `SKIP LOCKED` của job lưu giữ, giao dịch "đổi dữ liệu + `request_audit_outbox`".
- **Hợp đồng:** `buf breaking` cho RPC mới; test `parity_test.go` của MCP (CR-REQ-017) khi thêm kênh; golden của `AppendDetailed` so với `AppendAuditEntryRequest`.
- **An ninh (adversarial):** bộ Request độc (chuỗi "bỏ qua mọi chỉ dẫn", khối `ORCA_RESULT_BEGIN` giả, Markdown với `javascript:`, id tenant khác trong thân) chạy trong e2e (CR-REQ-025). Chưa chạy.
- **Chưa kiểm chứng:** hành vi thật của `FORCE RLS` với pool và PgBouncer nếu có; mTLS giữa gateway và service ở môi trường thật; độ phủ `secretscan` với dữ liệu khách.

## 6. Rủi ro và điểm chưa kiểm chứng

- Nếu mTLS và NetworkPolicy chưa bật, `internalcaller` là lớp bảo vệ duy nhất; token chung có thể lộ. Cần xoay vòng (chưa thiết kế).
- Cache vai trò dự án 30 giây có thể cho phép thành viên vừa bị gỡ làm thêm vài thao tác; chấp nhận được với v1, cần xác nhận.
- Che bí mật ngay cổng vào có thể làm mất thông tin hữu ích khi dương tính giả (đề xuất: người dùng thấy cờ và sửa lại).
- Ẩn danh hoá không xoá bản sao ở Jira, git, nhà cung cấp LLM, sao lưu; không nên hứa với khách "xoá hoàn toàn".
- `x-orca-actor-type` do gateway đặt; nếu có đường vào khác ngoài gateway, nó có thể giả `user`. `internalcaller` giảm rủi ro nhưng không loại bỏ.
- Số liệu giới hạn tốc độ, trần đồng thời, thời hạn lưu giữ là đề xuất, chưa đo và chưa có yêu cầu pháp lý của khách.
- Không thấy chính sách lưu giữ nào ở `auth.audit_log` ngoài "dài hơn nhật ký vận hành" (`07-security-architecture.md`); thời hạn cụ thể chưa có.

## 7. Câu hỏi mở

1. Quyền theo bảng 2.3 (đặc biệt `member` không phân loại, `StartPhase` chỉ `owner|admin`) cần chủ sản phẩm xác nhận.
2. Có thêm vai trò dự án `viewer` (chưa có trong `project-service`, chỉ `owner|member`) cho người chỉ đọc không?
3. Thời hạn lưu giữ mặc định (730, 400, 30 ngày) và liệu khách có yêu cầu luật định cụ thể.
4. Xoá nội dung ở `task-service` và các bản sao ngoài hệ thống: ai sở hữu và có CR riêng không.
5. Trích `common/secretscan` rồi để `mcp-service` chuyển sang dùng sau, hay giữ hai bản mãi với test golden chung?
6. Bật `internalcaller` cho RPC gateway có làm phức tạp triển khai không khi nhiều replica đổi khoá (cần quy trình xoay vòng)?
7. Sao lưu và phục hồi thảm hoạ của DB `request` (RPO, RTO) thuộc CR vận hành nào?

## 8. Tác động tới CR hiện có (không sửa trong lần này)

| CR | Cần sửa gì |
|---|---|
| CR-REQ-003 | Chốt nhóm hành động và RPC nào thuộc nhóm nào (bảng 2.3); câu hỏi mở "quyền ghi ở mức Request" đóng lại bằng mô hình này |
| CR-REQ-010 | Mục 2.2 và 2.4 thêm điều kiện `actor_type=agent` đã có cơ chế (metadata `x-orca-actor-type`); câu hỏi 6 đóng; nhắc `request.rego` cho nhóm `decide` chỉ là cửa trước, tập người duyệt vẫn Go |
| CR-REQ-009 | Ghi `actor_type` vào `decided_by_kind` (cùng CR-REQ-034); `Approve` đi qua `RequestAccessInterceptor` |
| CR-REQ-016 | Mục 2 "Quyền tối thiểu": thay cột bằng nhóm hành động 2.3; gateway điền `x-orca-actor-type`; thêm `GATEWAY_INTERNAL_TOKEN` vào cấu hình dial của gateway |
| CR-REQ-017 | Tool ghi MCP gắn `actor_type=agent`; không đăng ký `approval_approve`/`approval_reject`; kiểm hạn mức `create` của agent |
| CR-REQ-004 | Thêm tiêu đề thời gian và nonce cho webhook (2.7); `CreateRequest` qua `secretscan` ở cổng vào |
| CR-REQ-005, 007, 008, 012 | Mọi văn bản vào prompt qua `secretscan.Redact`; CR-008 mục 2.4 dùng `common/secretscan` thay bộ mẫu riêng |
| CR-REQ-002 | Thêm vào README mục 8 dòng 3 các bảng mới; `requests.contains_secret_suspected`; FORCE RLS và `WITH CHECK` cho mọi bảng; test quét SQL cho MySQL |
| CR-REQ-024 | Hợp nhất `AppendDetailed` với mục 2.5 (một nơi định nghĩa); thêm hành động `request.export`, `request.erase`, `request.retention.run`, `request.access.denied`; `request_audit_outbox` |
| CR-REQ-025 | Thêm `request_retention_days`, `ai_trace_retention_days`, `ledger_retention_days` vào `tenant_settings`; cổng GA bao gồm 2.9; e2e bộ Request độc |
| CR-REQ-011 | Cần RPC xoá nội dung task theo `request_id` (hiện không có) phục vụ `EraseRequest`; hoặc ghi giới hạn |
| CR-REQ-001 | Thêm `GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN`, gói OPA vào ảnh, ràng buộc `protovalidate` |
| README v6 mục 8 | Dòng 13 thêm: mọi RPC dùng `internalcaller.Guard`; dòng 14 thống nhất `AppendDetailed`; "Điểm chưa ai chốt" về quyền ghi mức Request được chốt ở CR này (cần xác nhận) |

## 9. Tham chiếu

- `/opt/repos/orca/backend-go/services/api-gateway/README.md` ("Still not production-safe", "Other known gaps"); `internal/adapter/grpc/dial.go` (`AttachIdentity`)
- `/opt/repos/orca/backend-go/common/grpcmw/grpcmw.go`; `common/internalcaller/internalcaller.go`; `common/tenant/tenant.go`; `common/auditclient/client.go`; `common/policy/evaluator.go`
- `/opt/repos/orca/backend-go/policy/orca-authz/project.rego`, `task_grant.rego`, `admin.rego`; `backend-go/ci/check-opa-bundle-in-images.sh`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go`; `migrations/postgres/0002_authorization.up.sql`; `internal/domain/secret_redactor.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/share_link.go` (RLS không đặt biến)
- `/opt/repos/orca/backend-go/services/auth-service/migrations/postgres/0001_init.up.sql`, `0013_audit_actor_type.up.sql`; `backend-go/proto/orca/auth/v1/auth.proto` (`AppendAuditEntryRequest`)
- `/opt/repos/orca/backend-go/services/usage-service/migrations/postgres/0001_init.up.sql` (mẫu RLS)
- `/opt/repos/orca/specs/backend-go/tdd/architecture/05-data-architecture.md`, `07-security-architecture.md`
- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-010-*.md`, `gateway-and-mcp/CR-REQ-016-*.md`, `request-quality-rollout/CR-REQ-024-*.md`, `CR-REQ-025-*.md`, `request-lifecycle/CR-REQ-004-*.md`
- `/opt/repos/orca/docs/research/receive-request/enterprise-readiness-checklist.md` (mục 4, 7)
