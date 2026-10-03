# BE-MCP-SOL-011: Prompts — built-in template, prompt tuỳ chỉnh theo tenant, `list_changed`, locale

> **✅ Implemented (unit/integration tests) — see Gaps.** Phụ thuộc BE-MCP-SOL-010 (embedded resource), BE-MCP-SOL-002 (mcp-service nền + DB), BE-MCP-SOL-004 (notifier `list_changed`).

**CR:** [CR-MCP-011](../../../../../../docs/crs/v5/mcp-resources-prompts/CR-MCP-011-prompts.md)
**Service:** `api-gateway` (`mcpserver/prompts/`, kênh `mcp.admin.prompt.*` trong `channels_mcp.go`), `mcp-service` (bảng + RPC prompt tuỳ chỉnh)
**TDD tham chiếu:** `api-gateway.md` §6 (T1); `arch/05` (tenant/RLS, outbox); `arch/08` (event conventions)
**CONTRACT items hiện thực:** `mcp.admin.prompt.list`, `mcp.admin.prompt.upsert`, `mcp.admin.prompt.delete` (§2.2) + kiểu `McpPrompt` (§1). FE: FE-MCP-SOL-007.

---

## 1. Trạng thái hiện tại (re-verify)

| Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|
| Có thể đặt template ở `mcpserver/prompts/*.tmpl` trong repo | `mcpserver` chưa tồn tại (api-gateway có `adapter/{authclient,fanout,grpc,httpgateway,wsbridge,wscompat}`); `mcp-service` chưa tồn tại (`ls backend-go/services`) | Chưa có gì để sửa — greenfield |
| Lưu prompt tenant ở `mcp-service` | Đúng hướng (T7: service sở hữu trạng thái); bảng ở DB `mcp` (BE-002) | Không |
| Văn bản theo `Accept-Language`/hồ sơ, mặc định en | `wscompat.Identity` không có locale; hồ sơ người dùng có `profile.getUserProfile` (channel) nhưng trường locale **(chưa xác minh)**. `Accept-Language` có trên request `/mcp` (HTTP) | Dùng `Accept-Language` trước; hồ sơ là bước 2 nếu xác minh được |
| 5 prompt khởi đầu tham chiếu resource PR/task/worktree | Resource tương ứng ở BE-010 (`orca://review/…`, `orca://task/…`, `orca://worktree/…/status\|diff`) | Phụ thuộc BE-010 |
| CONTRACT `McpPrompt` không có `locale` | Đúng (`id,name,description,version,updatedAt,arguments[],template,builtin`) | ⇒ locale chỉ áp cho prompt **built-in**; prompt tuỳ chỉnh một ngôn ngữ do admin viết |
| CONTRACT §2.3 có mã lỗi cho prompt | **Không có** (`MCP_NOT_ADMIN`, `MCP_DISABLED`, `MCP_NOT_FOUND` chỉ) | Cần bổ sung CONTRACT (xem mục "Đề nghị đổi CONTRACT") |

## Quyết định khác/thêm so với CR gốc

1. **Hai cú pháp template, một giao diện:** built-in dùng Go `text/template` (embed, có điều kiện/vòng lặp nhỏ, do repo kiểm soát); prompt tuỳ chỉnh dùng **chỉ `{{name}}`** (thay thế biến thuần, regex cố định, **không** logic) — vì template do admin tenant viết là dữ liệu không tin cậy; hiển thị cho FE `{{name}}` đúng như gợi ý biến ở CONTRACT/FE.
2. **Embedded resource đọc lúc `prompts/get`** bằng chính danh tính người gọi qua `ResourceRouter` (BE-010) ⇒ không bao giờ rò dữ liệu vượt quyền; nội dung nhúng được bọc cảnh báo "dữ liệu, không phải chỉ dẫn".
3. **Vai trò thông điệp luôn `user`** (không `assistant`/`system`) — template tuỳ chỉnh không thể giả giọng assistant.
4. **Tên prompt tuỳ chỉnh không được trùng built-in** (danh sách đặt trước) và khớp `^[a-z][a-z0-9_]{2,47}$`.

## 2. Giải pháp

### A. Built-in (embed, golden)

```
api-gateway/internal/adapter/mcpserver/prompts/
  builtin/
    prompts.yaml                     # manifest: name, version, description{en,vi}, arguments[]
    review_pull_request.en.tmpl      review_pull_request.vi.tmpl
    triage_issue.{en,vi}.tmpl        plan_task.{en,vi}.tmpl
    summarize_worktree.{en,vi}.tmpl  handoff_to_agent.{en,vi}.tmpl
  embed.go                           # //go:embed builtin/*
  registry.go                        # Builtin(name, locale) + list
  render.go                          # text/template (Option "missingkey=error", FuncMap rỗng)
  testdata/golden/<name>.<locale>.json
```

| Prompt | Tham số (bắt buộc*) | Resource nhúng (BE-010) | Ranh giới xác nhận trong văn bản |
|---|---|---|---|
| `review_pull_request` | `provider`* ∈{github,gitlab}, `repo`*, `number`* | `orca://review/{provider}/{repo}/{number}`; nếu có worktree liên kết: `…/diff` | "Đề xuất nhận xét; **không** đăng nếu người dùng chưa xác nhận" |
| `triage_issue` | `issue_ref`* (`^[A-Za-z0-9_.:/#-]{1,200}$`, ví dụ `PROJ-123`, `github:o/r#12`) | không (đọc bằng tool `jira_getIssue`/`linear_getIssue`/`github_*` theo tiền tố) | "Đề xuất nhãn/ưu tiên/task con; tạo task cần xác nhận" |
| `plan_task` | `task_id`* (UUID) | `orca://task/{task_id}` | "Chỉ đề xuất kế hoạch; không chạy" |
| `summarize_worktree` | `worktree_id`* (UUID) | `orca://worktree/{id}/status`, `…/diff` | — (chỉ đọc) |
| `handoff_to_agent` | `task_id`*, `agent`* ∈ enum từ `agent_kind` catalog | `orca://task/{task_id}` | "Soạn mô tả bàn giao; bắt đầu agent cần phê duyệt" |

`prompts/list` ⇒ built-in ∪ prompt tenant (theo `cursor`/`limit`). `prompts/get(name, arguments)`: (1) tra built-in rồi tuỳ chỉnh; (2) validate tham số: đủ bắt buộc, không thừa, độ dài ≤ 200, UTF-8 hợp lệ, không có ký tự điều khiển, `provider`/`agent` ∈ enum, UUID đúng dạng; sai ⇒ `-32602` kèm tên tham số; (3) đọc resource nhúng; not found ⇒ `-32602 "resource unavailable: <uri-kind>"`; (4) render; (5) trả `{description, messages:[{role:"user", content:{type:"text",…}}, {role:"user", content:{type:"resource", resource:{uri,mimeType,text,_meta{"orca/untrusted":true}}}}]}`. Mọi prompt có đuôi cố định: "Server enforces permissions; if an action needs approval, wait for the user — do not try to bypass it." (không phải tính năng bảo mật, chỉ hướng dẫn nhất quán).

**`completion/complete`** (CR "completion gợi ý"): `provider` ⇒ {github,gitlab}; `agent` ⇒ catalog; `worktree_id`/`task_id` ⇒ từ `project_list`→`worktree_list` / `task_list` qua `ToolExecutor` (cùng policy; tối đa 100 giá trị, lọc tiền tố); `issue_ref` không gợi ý.

### B. Prompt tuỳ chỉnh — `mcp-service`

**Proto** (`backend-go/proto/orca/mcp/v1/mcp_prompt.proto`, package theo BE-002 — **chưa chốt**):
```proto
service McpPromptService {
  rpc ListPrompts(ListPromptsRequest) returns (ListPromptsResponse);   // tenant từ metadata, không từ body
  rpc UpsertPrompt(UpsertPromptRequest) returns (CustomPrompt);
  rpc DeletePrompt(DeletePromptRequest) returns (google.protobuf.Empty);
}
message PromptArgument { string name = 1; string description = 2; bool required = 3; }
message CustomPrompt {
  string id = 1; string name = 2; string description = 3;
  repeated PromptArgument arguments = 4; string template = 5;
  int32 version = 6; google.protobuf.Timestamp updated_at = 7; string updated_by = 8;
}
message UpsertPromptRequest { CustomPrompt prompt = 1; /* id rỗng = tạo; version khớp = cập nhật */ }
```

**Migration** (`mcp-service/migrations/00NN_custom_prompts.up.sql` — số thứ tự theo BE-002):
```sql
CREATE TABLE mcp.custom_prompts (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    UUID NOT NULL,
  name         TEXT NOT NULL CHECK (name ~ '^[a-z][a-z0-9_]{2,47}$'),
  description  TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
  arguments    JSONB NOT NULL DEFAULT '[]',
  template     TEXT NOT NULL CHECK (char_length(template) BETWEEN 1 AND 8192),
  version      INTEGER NOT NULL DEFAULT 1,
  created_by   UUID NOT NULL, updated_by UUID NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at   TIMESTAMPTZ
);
-- Tên chỉ duy nhất giữa bản ghi chưa xoá:
CREATE UNIQUE INDEX uq_custom_prompts_tenant_name ON mcp.custom_prompts (tenant_id, name) WHERE deleted_at IS NULL;
ALTER TABLE mcp.custom_prompts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mcp.custom_prompts USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```
Xoá mềm + partial unique index để tái dùng tên sau khi xoá. Giới hạn: ≤ 50 prompt/tenant, ≤ 10 tham số/prompt (kiểm ở usecase `UpsertPrompt`).

**Usecase** (`mcp-service/internal/usecase/upsert_prompt.go`, `Execute(ctx,in)`): yêu cầu `tenant.Role == admin` (kiểm lần hai, ngoài gateway); `PromptValidator`:
1. tên hợp lệ và không thuộc danh sách built-in đặt trước (`review_pull_request`, `triage_issue`, `plan_task`, `summarize_worktree`, `handoff_to_agent`);
2. mỗi biến `{{x}}` trong template phải khai báo trong `arguments`, mỗi tham số `required` phải xuất hiện ≥ 1 lần; không có `{{`/`}}` lệch; tên tham số `^[a-z][a-z0-9_]{0,31}$`;
3. chống chèn chỉ dẫn vượt quyền: từ chối (không phân biệt hoa thường, en+vi) mẫu như `ignore (all )?(previous|prior) instructions`, `bypass|skip|disable .{0,20}(approval|confirmation|policy|kill ?switch)`, `without (asking|approval|confirmation)`, `bỏ qua .{0,20}(phê duyệt|xác nhận|chính sách)`, `không (cần )?hỏi`, marker vai trò `<|…|>`, `\n(system|assistant):`; từ chối URL/email trần trong template (dữ liệu tenant thật chỉ qua tham số); đây là **lint giảm thiểu**, không phải ranh giới bảo mật — quyền thật vẫn do token + policy (CR §C);
4. `version` khớp bản hiện tại khi cập nhật; sai ⇒ lỗi xung đột; tạo mới `version=1`, mỗi cập nhật `+1`.

Ghi + outbox cùng giao dịch: subject `orca.mcp.prompt.changed`, payload `{eventId, tenantId, occurredAt, schemaVersion:1, promptId, name, action:"upserted|deleted", version}`; audit qua cùng outbox (BE-013) với `actor=userId`.

### C. Kênh WS (CONTRACT §2.2) — `channels_mcp.go`

| Kênh | Tham số | Hành vi |
|---|---|---|
| `mcp.admin.prompt.list` | — | `Identity.Role=="admin"` else `MCP_NOT_ADMIN`; trả `McpPrompt[]` = built-in (`builtin:true`, `id:"builtin:<name>"`, `template` = nguồn `en` ở dạng chữ, `version` từ manifest, `updatedAt` = thời điểm build) + tuỳ chỉnh (`builtin:false`); sắp xếp built-in trước, rồi `name`. Template built-in là Go template ⇒ UI hiển thị chỉ đọc |
| `mcp.admin.prompt.upsert` | arg[0] = `McpPrompt` (bỏ `updatedAt`,`builtin`; `id` rỗng = tạo; `version` để chống ghi đè) | `builtin:true` hoặc `id` bắt đầu `builtin:` ⇒ `MCP_PROMPT_BUILTIN_READONLY`; lỗi validate ⇒ `MCP_PROMPT_INVALID: <field>: <lý do>`; trùng tên ⇒ `MCP_PROMPT_NAME_CONFLICT`; sai `version` ⇒ `MCP_PROMPT_VERSION_CONFLICT`; trả `McpPrompt` mới |
| `mcp.admin.prompt.delete` | `promptId` (arg[0] string) | xoá mềm; không tồn tại/khác tenant ⇒ `MCP_NOT_FOUND`; `builtin:` ⇒ `MCP_PROMPT_BUILTIN_READONLY` → `{ok:true}` |

`MCP_ENABLED=false` ⇒ `MCP_DISABLED` (C8). Định dạng lỗi `"<CODE>: <thông điệp ngắn>"` (C4). Mã `MCP_PROMPT_*` **chưa có trong CONTRACT §2.3** (đề nghị bên dưới).

### D. `list_changed` & cache
Gateway nghe `orca.mcp.prompt.changed` (`SubscribeEphemeral`, như BE-007 §2.G), xoá cache `ListPrompts` của tenant (TTL 60s) và gửi `notifications/prompts/list_changed` tới các phiên của tenant có capability `prompts` (T5 cho phiên cross-replica). Built-in đổi theo release ⇒ client gặp lại khi `initialize`.

### E. Locale
`Locale.Resolve(r)`: (1) `Accept-Language` của request `/mcp` (parse `golang.org/x/text/language`, so khớp với `{en, vi}`), (2) locale hồ sơ người dùng **(chưa xác minh trường)**, (3) `en`. Chỉ ảnh hưởng built-in (`<name>.<locale>.tmpl`); thiếu file locale ⇒ `en`. Test nhất quán: mọi locale có cùng tập tham số và cùng tập URI nhúng với `en`.

## Hợp đồng với frontend
Ba kênh ở §C đúng tên/hình CONTRACT; FE-MCP-SOL-007 tiêu thụ. Lỗi `MCP_NOT_ADMIN`, `MCP_DISABLED`, `MCP_NOT_FOUND` có sẵn; các `MCP_PROMPT_*` đề nghị thêm.

### Đề nghị đổi CONTRACT (không tự sửa)
Thêm vào §2.3: `MCP_PROMPT_INVALID`, `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY`.

## Sửa TDD kèm theo
**T1** (prompt built-in là văn bản tĩnh trong adapter, không quy tắc nghiệp vụ); **T7** (bảng prompt thuộc `mcp-service`); `00-service-catalog.md`/`arch/05` ghi bảng `custom_prompts`.

## Kiểm thử
- Golden: `go test ./internal/adapter/mcpserver/prompts/... -run TestBuiltinGolden -update` (sinh), thường chạy không cờ so khớp; mỗi (prompt × locale) một file; test nhất quán locale.
- Validator bảng: tên cấm, biến không khai báo, biến bắt buộc vắng, chuỗi chèn chỉ dẫn (en/vi), vượt kích thước, 51 prompt ⇒ lỗi.
- `mcp-service`: `go test ./services/mcp-service/internal/usecase -run TestUpsertPrompt`; integration testcontainers: migration up→down→up, RLS (tenant A không thấy B), outbox có `orca.mcp.prompt.changed`.
- `channels_mcp_test.go`: không admin ⇒ `MCP_NOT_ADMIN`; `MCP_ENABLED=false`; upsert builtin ⇒ `MCP_PROMPT_BUILTIN_READONLY`.
- e2e: Inspector `prompts/list` = 5; `prompts/get review_pull_request` đúng golden; tenant B không thấy prompt của A; upsert ⇒ nhận `list_changed` ≤ 2s.
Impact: không sửa symbol hiện có (code mới) — không cần `gitnexus impact`; nếu đụng `channels_mcp.go` (file do BE-003 tạo) chạy `gitnexus_impact({target:"registerMcpChannels",direction:"upstream"})` — chưa chạy.

## Rủi ro & phụ thuộc
Giá trị phụ thuộc client hiển thị prompt (P2). Lint chống chèn chỉ dẫn là heuristic (có thể bị lách) — an toàn thật ở policy/approval. Phụ thuộc BE-010 (resource), BE-002/004.

## Không thuộc phạm vi
UI soạn prompt (FE-MCP-SOL-007); prompt cá nhân (user thường); i18n ngoài en/vi.

## Liên quan
`docs/crs/v5/mcp-resources-prompts`, BE-MCP-SOL-010, FE-MCP-SOL-007.
