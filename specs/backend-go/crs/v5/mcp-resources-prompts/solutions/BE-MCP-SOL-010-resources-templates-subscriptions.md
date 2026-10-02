# BE-MCP-SOL-010: Resources, resource templates, subscriptions

> **🔲 Designed — chưa implement.** Phụ thuộc BE-MCP-SOL-007 (ToolExecutor/Catalog/redaction), BE-MCP-SOL-004 (phiên, notifier, resume), BE-MCP-SOL-012 (policy), BE-MCP-SOL-009 (ring terminal).

**CR:** [CR-MCP-010](../../../../../../docs/crs/v5/mcp-resources-prompts/CR-MCP-010-resources-templates-subscriptions.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver/resources/`), đề xuất sửa nhỏ `git-gateway-service` (symlink)
**TDD tham chiếu:** `api-gateway.md` §6 (T1, T2), §5 (fan-out); `arch/08` (T5 ephemeral); `arch/07` (authz)
**CONTRACT items hiện thực:** không có kênh UI nào cho resources (CONTRACT không có `mcp.resource.*`). Phần giao thức `resources/list`, `resources/templates/list`, `resources/read`, `resources/subscribe|unsubscribe`, `notifications/resources/updated|list_changed` thuộc `/mcp` (BE-003) — solution này cấp handler. UI không cần thay đổi (xem FE README).

---

## 1. Trạng thái hiện tại (re-verify)

| Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|
| Có nguồn dữ liệu task/annotation/worktree/diff/PR/file | Có, qua channel: `project.list/get`, `worktree.list{projectId}`, `task.get/getDependencies/listComments`, `git.status/diff/branchDiff`, `files.readPreview/readChunk/readDir/stat`, `github.project.workItemDetailsBySlug{itemSlug}`, `gitlab.workItemDetails{repo,iid,itemType}`, `hostedReview.forBranch` | Không |
| Có "task activity bus" `channels_task_activity.go` | Đúng: `task.activity.subscribe` (StreamHandler) gom 7 subject JetStream bằng `Consumer.SubscribeEphemeral` (`orca.orchestration.task.dispatched/statuschanged`, `…message.posted`, `…decision_gate.opened`, `orca.workflow.step.completed/failed`, `orca.task.agent_output_partial`), lọc theo `origin_task_id`. **Không có API lịch sử** hoạt động: `orca://task/{id}` "activity" phải là `task.listComments` + trạng thái, không phải log sự kiện | Lệch nhẹ: "task + hoạt động" ⇒ chỉ comment + trạng thái |
| Đường dẫn file khoá trong gốc worktree, chặn symlink | **Chỉ chặn lexical:** `git-gateway-service/internal/adapter/localfs/executor.go` `resolve()` = `filepath.Clean(Join)` + kiểm tiền tố; **không `EvalSymlinks`**, `StatFileResponse` không có `is_symlink`. Đường SSH/relay chưa xác minh | **Lệch quan trọng** ⇒ cần sửa git-gateway (§3) và cờ tắt mặc định cho resource `file` |
| `files.read` có giới hạn | `files.read` trả cả file (base64 do `encoding/json`); `readPreview{maxBytes}` trả `truncated` | Dùng `readPreview` |
| Có bộ danh sách che dữ liệu nhạy cảm dùng chung với `files_read` | Chưa có ở đâu. Tạo mới một nguồn duy nhất `sensitive_path_rules.go` dùng cho cả `files_read` (BE-008) và resource file | **Lệch** |
| Cơ chế ephemeral (T5) | `Consumer.SubscribeEphemeral(ctx, stream, subject, fn)` có thật (`common/eventbus`) nhưng là **JetStream ephemeral consumer** (`AckExplicit`, `InactiveThreshold`), mỗi tiến trình nhận bản đầy đủ — không phải core-NATS. Hợp với "mỗi replica phải thấy mọi sự kiện" | Làm rõ T5 |
| `worktree`/`diff`/`status` có sự kiện đẩy | Không có sự kiện NATS cho thay đổi git. Có `files.watch` (streamChannel → `git-gateway` `WatchWorktree`, thông báo `fs.changed`); `workspace.subscribe` chỉ bắc cầu `orca.task.task.statuschanged` và `orca.workflow.execution.completed/failed` | Subscribe worktree dựa `WatchWorktree` (chưa xác minh chi phí) |
| PR/review có sự kiện | Không có; chỉ gọi `gh`/GitLab (rate limit, AGENTS.md) | `resources/subscribe` cho `review` **không hỗ trợ v1** |

## Quyết định khác/thêm so với CR gốc

1. **Một đường thực thi:** `resources/read` = `ResourceRouter` ánh xạ URI → (tool read, args) rồi gọi **cùng `ToolExecutor`** (BE-007) với cờ `origin=resource`. Authorize/policy/audit/redaction/cắt kích thước là một mã; không có đường đọc thứ hai (khớp D3).
2. **Resource `file` tắt mặc định** (`MCP_RESOURCE_FILE_ENABLED=false`) cho tới khi git-gateway chặn symlink thoát gốc ở cả localfs và đường relay.
3. **`review` khác biệt hoá URI:** `repo` có thể chứa `/` (GitLab subgroup) ⇒ phải percent-encode (`org%2Fsub%2Frepo`); template RFC 6570 `{repo}` mở rộng đơn giản tự mã hoá `/`.
4. **Subscribe v1 chỉ cho:** `task`, `worktree/{id}/status|diff`, `terminal/{id}/scrollback`. Các URI khác trả lỗi `-32602` "subscription not supported for this resource".
5. **Lỗi không tồn tại:** JSON-RPC `-32002` (Resource not found, theo spec — **(chưa xác minh)** mã ở bản 2025-06-18) cho cả "không có" và "không quyền" (khớp `MCP_NOT_FOUND`).

## 2. Giải pháp

### A. Không gian URI & template

`resources/templates/list` (cố định, không phân trang) — mỗi mục có `mimeType`, `annotations{audience,priority}`:

| Template | Tool read ánh xạ (policy/audit dùng tên này) | Channel thật | mimeType |
|---|---|---|---|
| `orca://projects` (resource gốc) | `project_list` | `project.list` | `application/json` |
| `orca://project/{projectId}` | `project_get` + `worktree_list` | `project.get`, `worktree.list{projectId}` | `application/json` |
| `orca://task/{taskId}` | `task_get`,`task_getDependencies`,`task_listComments` | `task.get`, `task.getDependencies`, `task.listComments` | `application/json` |
| `orca://worktree/{worktreeId}/status` | `git_status` | `git.status{worktree:"id:…"}` | `application/json` |
| `orca://worktree/{worktreeId}/diff{?base,staged,path}` | `git_diff` / `git_branchDiff` (khi có `base`) | `git.diff{worktree,filePath,staged}` / `git.branchDiff{worktree,baseRef,filePath}` | `text/x-diff` |
| `orca://worktree/{worktreeId}/file/{+path}{?offset,length}` | `files_read` | `files.readPreview{worktreeId,path,maxBytes}` / `files.readChunk` khi có `offset` | theo đuôi file (`mime.TypeByExtension`), mặc định `text/plain` |
| `orca://review/{provider}/{repo}/{number}` | `hostedReview_get` (Composite mới, pack 1) | `provider=github` ⇒ `github.project.workItemDetailsBySlug{itemSlug}` (định dạng `itemSlug` **chưa xác minh**, giả định `owner/repo#number`); `gitlab` ⇒ `gitlab.workItemDetails{repo,iid,itemType:"merge_request"}` | `application/json` |
| `orca://terminal/{terminalId}/scrollback` | `terminal_read` | ring của `ToolSession` (BE-009); terminal không do phiên này tạo ⇒ not found | `text/plain` |

`resources/list` (phân trang C7, `cursor`/`limit` mặc định 50): `orca://projects` + một mục `orca://project/{id}` cho mỗi project user thấy (từ `project.list`); không liệt kê task/worktree/file (dùng template). Danh sách lọc theo policy: nếu tenant `deny` tool ánh xạ thì template **ẩn**.

Giải URI: `ResourceRouter.Parse(uri) (kind, params, error)` — parser nghiêm: scheme `orca` chữ thường, host rỗng, từ chối userinfo/fragment, mỗi tham số khớp regex (`projectId|taskId|worktreeId` UUID; `number` `^[0-9]{1,9}$`; `provider` ∈ {github,gitlab}), độ dài URI ≤ 2048. Mọi id sai định dạng ⇒ not found (không lộ lý do).

### B. Truy cập, dữ liệu không tin cậy, kích thước

- Scope tối thiểu `orca:read` trên token; `ToolExecutor` gọi `PolicyGate.Authorize` với tên tool ánh xạ + `resource:true` để audit phân biệt. Cross-tenant/không quyền ⇒ cùng lỗi not found (gateway nhận `NotFound/PermissionDenied` từ gRPC hạ nguồn → gộp). Đề xuất Rego (tên package theo BE-012, **chưa chốt**):

```rego
package orca.authz.mcp.resource
import rego.v1
default allow := false
# Đọc resource = quyết định của tool read ánh xạ; không có quy tắc riêng để tránh lệch quyền.
allow if { data.orca.authz.mcp.tool.allow with input.tool as input.mapped_tool }
```
- Giới hạn: `MCP_RESOURCE_MAX_BYTES` (256 KiB, văn bản). Vượt ⇒ cắt tại ranh giới dòng/rune, thêm cuối `\n[truncated — read more with ?offset=<n>&length=<m>]` (file) hoặc `?path=` (diff) và `_meta.truncated=true`. Nhị phân ⇒ `blob` base64 chỉ khi ≤ 256 KiB và mime ∈ allowlist ảnh; ngoài ra lỗi `-32602` "binary resource too large".
- `annotations`: `audience:["user","assistant"]`, `priority` 0.3–0.8, `lastModified` nếu có. `_meta{"orca/untrusted":true}` cho `review`, `terminal`, `file`, và mọi `task` có nội dung từ ticket ngoài — **không** trộn vào `instructions`/mô tả tool (BE-003).
- Redaction: dùng bộ che của BE-007 §2.D cho **mọi** nội dung text (URL có token, token dạng `ghp_…`, PEM…), kể cả nội dung file.

### C. Quy tắc đường dẫn file (`file` resource và `files_read`)

**File:** `mcpserver/tools/sensitive_path_rules.go` (NEW, dùng chung), `mcpserver/resources/worktree_file_path.go` (NEW)

1. Giải mã phần trăm **đúng một lần**; sau đó từ chối: byte NUL, ký tự điều khiển, `\` (mọi OS — chuẩn hoá bằng `/`), đường tuyệt đối (bắt đầu `/` hoặc `X:`), `//`, mọi đoạn `..` hoặc `.` sau `path.Clean`, tên có dấu `.`/khoảng trắng ở cuối (Windows), dấu hiệu mã hoá kép (`%25`, `%2e` còn sót).
2. Chuẩn hoá Unicode NFC và so khớp **không phân biệt hoa thường** với deny-list (macOS/Windows).
3. Deny-list (chỉ chặn đọc nội dung; có thể thấy tên trong `readDir`): `.env`, `.env.*` (trừ `.example|.sample|.template`), `*.pem|*.key|*.p12|*.pfx|*.keystore`, `id_rsa*|id_ed25519*|id_ecdsa*`, `.npmrc|.pypirc|.netrc|.git-credentials`, mọi thứ dưới `.git/` (gồm `.git/config`), `.aws/|.ssh/|.gnupg/|.kube/config|.docker/config.json`, `*.tfstate*`, `secrets.*|*.secret`. Cấu hình thêm theo tenant qua `MCP_SENSITIVE_PATH_EXTRA` (glob, chỉ mở rộng, không thu hẹp).
4. **Symlink:** adapter không thể `lstat` ⇒ yêu cầu hạ nguồn. **Sửa kèm theo `git-gateway-service`:** `localfs.resolve()` gọi `filepath.EvalSymlinks` trên phần đã tồn tại rồi kiểm lại tiền tố gốc (cải thiện cho mọi caller; lỗi `ErrPathEscapesWorktree` đã có và đã được ánh xạ qua `toFileGRPCStatus`); đường relay SSH: agent phải áp quy tắc tương đương (**chưa xác minh** mã agent; việc cần xác nhận trước khi bật cờ). Trước khi hai điều kiện đạt, `MCP_RESOURCE_FILE_ENABLED=false` ⇒ template `file` không có trong `templates/list` và tool `files_read` trả `FILE_ACCESS_DISABLED`.
5. Thêm kiểm sau đọc: nếu `readPreview` trả nội dung khớp mẫu khoá riêng PEM ⇒ che toàn bộ.

### D. Subscriptions & `notifications/resources/updated`

Khai báo capabilities `resources{subscribe:true,listChanged:true}` (BE-003).

**Hợp đồng nội bộ (BE-004):** notifier `SessionNotifier.Notify(sessionID, method, params)`; sự kiện đóng phiên. Phiên SSE nằm ở một replica ("owner"); POST `resources/subscribe` có thể rơi replica khác (T5):

```
POST resources/subscribe(uri) @replica B
  → ResourceSubscriptions.Add(session, uri)  (validate uri, quota ≤ MCP_RESOURCE_MAX_SUBS=50/phiên, đúng loại subscribe được)
  → publish core-NATS  orca.ephemeral.mcp.session.<sid>  {op:"resource.subscribe", uri}
  → owner replica A nhận, arm nguồn sự kiện (idempotent theo (sid,uri))
  → lưu tập URI cùng bản ghi phiên (mcp-service RPC do BE-004 chốt — yêu cầu: UpdateSessionSubscriptions) để resume sau restart
```

Nguồn sự kiện (đều chạy ở owner replica, dùng `SubscribeEphemeral` nên mọi replica thấy mọi sự kiện, không mất khi scale):

| URI | Nguồn | Lọc | Debounce |
|---|---|---|---|
| `orca://task/{id}` | 7 subject trong `taskActivitySubjects` + `orca.task.task.statuschanged` (stream `TASK`) — tái dùng `translateToTaskActivity(subject, ev, taskID)` | `origin_task_id == taskID` **và** `ev.TenantID == tenant của phiên` | 250 ms |
| `orca://worktree/{id}/status\|diff` | `files.watch` (git-gateway `WatchWorktree`) mở bằng `DispatchStreamChannel` trên `ToolSession`, dùng chung 1 watcher/worktree/phiên | mọi `fs.changed` | 1 s |
| `orca://terminal/{id}/scrollback` | `ptyOutputRing.notify` (BE-009) | — | 500 ms |

Phát: `notifications/resources/updated {uri}` (chỉ URI; client tự `resources/read`). Trước khi phát, kiểm lại quyền cơ bản (token còn hiệu lực, policy không `deny` tool ánh xạ) ⇒ nếu mất quyền: huỷ subscription và không phát. `resources/unsubscribe`, phiên đóng, kill switch ⇒ huỷ watcher và goroutine (test rò goroutine bằng `goleak`).

`notifications/resources/list_changed`: phát khi policy đổi làm đổi tập template hiển thị (nghe `orca.mcp.policy.changed`, BE-007 §2.G) và khi cờ `MCP_RESOURCE_FILE_ENABLED` đổi. Tạo/xoá project ⇒ **(chưa xác minh)** có sự kiện `orca.project.*`; nếu không thì v1 không phát.

### E. Mã & cấu trúc

```
mcpserver/resources/
  router.go            // Parse(uri) + kind→ToolCall mapping (bảng ở A)
  templates.go         // Template list tĩnh + lọc theo policy
  reader.go            // Read(ctx, session, uri) → ToolExecutor
  subscriptions.go     // ResourceSubscriptions (per-session set, quota, owner arm/disarm)
  event_sources.go     // task bus, worktree watch, terminal ring → debounce → Notify
  worktree_file_path.go
```
Config (T8): `MCP_RESOURCE_MAX_BYTES`, `MCP_RESOURCE_MAX_SUBS`, `MCP_RESOURCE_FILE_ENABLED`, `MCP_SENSITIVE_PATH_EXTRA`.

## Hợp đồng với frontend
Không có kênh UI cho resources. FE chỉ thấy gián tiếp: audit (`McpAuditEntry.tool` = tên tool ánh xạ, `argsSummary` = URI bỏ query) và `McpSessionView.activeStreams`.

## Sửa TDD kèm theo
**T1/T2** (như BE-007); **T5**: ghi rõ `SubscribeEphemeral` là JetStream ephemeral consumer và `orca.ephemeral.mcp.session.<id>` là core-NATS (không bền) — dùng cho điều phối `resource.subscribe`/`notifications/*` cross-replica. `git-gateway-service.md` §3 (symlink containment).

## Kiểm thử
`go test ./internal/adapter/mcpserver/resources/...`: bảng `Parse` (hợp lệ/độc: `%2e%2e`, `..%2f`, `\`, NUL, unicode NFC, URI quá dài); golden đọc 8 URI với client gRPC giả; test bảo mật (`TestFileResource_*`): `..`, symlink thoát (khi có git-gateway mới), `.env`, `.git/config`, `Id_Rsa` (hoa/thường), cross-tenant ⇒ cùng lỗi not found; truncation (`>256KiB` kèm hint); đổi trạng thái task ⇒ `resources/updated` ≤ 2s (fake `ephemeralSubscriber`, mẫu `channels_task_activity_test.go`); đóng phiên ⇒ không rò goroutine. git-gateway: `go test ./services/git-gateway-service/internal/adapter/localfs -run TestResolveSymlinkEscape`.
Impact (chưa chạy): `gitnexus_impact({target:"translateToTaskActivity",direction:"upstream"})` (tái dùng, không sửa; rủi ro LOW), `gitnexus_impact({target:"resolve",direction:"upstream"})` trong `localfs` (sửa; rủi ro dự kiến MEDIUM — mọi RPC file đi qua).

## Rủi ro & phụ thuộc
Id nội bộ lộ trong URI ⇒ UUID + luôn kiểm quyền. `WatchWorktree` tốn tài nguyên ⇒ trần subs + 1 watcher/worktree. Phụ thuộc cơ chế resume của BE-004 để giữ subscription qua restart.

## Không thuộc phạm vi
Prompts (BE-011), resources của server bên thứ ba (CR-014), UI.

## Liên quan
`wscompat/channels_task_activity.go`, `channels_git.go` (`registerFilesChannels`), `git-gateway-service/internal/adapter/localfs/executor.go`, `common/eventbus`.
