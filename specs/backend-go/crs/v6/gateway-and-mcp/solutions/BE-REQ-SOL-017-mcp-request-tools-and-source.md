# BE-REQ-SOL-017: Tool MCP `request_*`, `solution_*`, `approval_*`, `backlog_*` và MCP làm nguồn Request

> ✅ **Đã triển khai.** Toàn bộ code đã được implement và verify (xem task list).

**CR:** [CR-REQ-017](../../../../../../docs/crs/v6/gateway-and-mcp/CR-REQ-017-mcp-request-tools-and-source.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver/tools`), `tests/mcp/`, `docs/guides/mcp/`
**TDD tham chiếu:** [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (scope, ủy quyền, không tự duyệt), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)
**Hợp đồng:** [`CONTRACT-request-ui-api.md`](../CONTRACT-request-ui-api.md) (kênh đích); mẫu tool: [`BE-MCP-SOL-008`](../../../v5/mcp-tool-catalog/solutions/BE-MCP-SOL-008-domain-tool-packs.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `tools/spec.go` (`ToolSpec`, `Declared`, `Untrusted`, `RequiredScope`), `tools/spec_builders.go` (`read` pack 1, `write` pack 2, `execTool` pack 3; `.asList()`, `.untrusted()`, `.openWorld()`, `.idempotent()`, `.declared()`, `.named()`), `tools/fields.go` (`Str`, `Int`, `Bool`, `Strs`; `Req`, `OneOf`, `Len`, `Range`), `tools/all_specs.go` (`AllSpecs` gộp 13 nhóm), `tools/config.go` (`Config.Packs` mặc định `{1: true}`, `ToolTimeout` 55 giây, `ApplyEnv`), `tools/executor.go` (`runGuarded`, `Guards`, `scmLimit.allow`, `ToolError`), `tools/scm_rate_limit.go` (`scmLimiter`, `maxRateBuckets`), `tools/composite_env.go` dòng 45 (`wscompat.WithToolOrigin` chỉ ở đây), `tools/parity_test.go`, `tools/pack2_workspace.go` (mẫu `write("task.createFromSource", ...)`), `wscompat/tool_origin_context.go`, `wscompat/handler.go` (`invokeTimeout = 25s`).

Khác hoặc bổ sung so với CR-REQ-017:

1. **Correction relative to CR-REQ-017 mục 2.3: `ToolOrigin` chưa được đặt cho tool thường.** `wscompat.WithToolOrigin` chỉ được gọi trong `composite_env.go:45` cho tool terminal và agent. `Executor.runGuarded` không gắn nó. Muốn `request_create` mang `source_provider=mcp`, executor phải gắn `ToolOrigin{ClientName, MCPSessionID, UserID}` vào `ctx` (dùng `Guards.SessionID`, `Guards.ClientName`, `Principal.UserID`) cho namespace Request. Đây là việc mới, không phải "đã có".
2. **Correction, gói tool.** `Config.Packs` mặc định `{1: true}`: tool ghi (pack 2) và exec (pack 3) chỉ hiện khi đặt `MCP_TOOL_PACKS_ENABLED=1,2` (`config.go:69`). Triển khai, e2e và hướng dẫn phải nêu rõ biến này.
3. **Correction, timeout.** `ToolTimeout` 55 giây, nhưng kênh `request.classify` và `request.generatePlan` bị chặn 25 giây ở WS (CONTRACT mục 2.1) và đều cần AI lâu hơn. Ở v1 hai tool này là `Declared` (có trong catalog, chưa liệt kê hay chạy), cùng `request_startPhase`. Mở khi RPC trả sớm (CONTRACT Q2). Số tool liệt kê được ở v1: 11 đọc cộng 6 ghi (`request_create`, `request_changeType`, `request_returnToBacklog`, `request_reopen`, `request_spawnChild`, `solution_generate`) là 17, không phải 19 như CR.
4. **Mã lỗi giới hạn.** `executor.go` trả `ToolError{"RATE_LIMITED", ...}` cho `scmLimiter`. CR yêu cầu `REQUEST_RATE_LIMITED` và `REQUEST_PENDING_LIMIT`: tool Request dùng `ToolError` với mã đó (`REQUEST_RATE_LIMITED` kèm "retry in Ns").
5. **`request_record_check` của CR-REQ-014** (snake_case) không khớp quy ước `ChannelToToolName` (`request_recordCheck`) và chưa có kênh WS. Không nằm trong CR này; ghi ở câu hỏi mở.
6. `tests/mcp/` đã tồn tại (`check_mcp_task_worktree_flow.py`, `check_mcp_ws_channels.py`...); CR ghi "chưa commit" là lỗi thời.

## 2. Giải pháp

### 2.1 Tool

Bảng đầy đủ ở CR-REQ-017 mục 2.1; dưới đây là phần chốt cho hiện thực (tên tool = `ChannelToToolName(channel)`, giữ chữ hoa).

| Nhóm | Tool | Builder |
|---|---|---|
| Đọc, pack 1 | `request_list`, `request_get`, `request_typeHistory`, `solution_list`, `approval_get`, `approval_list`, `approval_listPending`, `backlog_requests`, `backlog_tasks`, `backlog_execute`, `request_flowStatus` | `read(...)`; `request_list`, `request_typeHistory`, `solution_list`, `approval_list`, `backlog_*` thêm `.asList()`; `request_get`, `request_list`, `request_typeHistory`, `solution_list`, `backlog_requests` thêm `.untrusted()` |
| Ghi, pack 2 | `request_create`, `request_changeType`, `request_returnToBacklog`, `request_reopen`, `request_spawnChild`, `solution_generate` | `write(...)`; `solution_generate` thêm `.openWorld()`; `request_create` thêm `.idempotent()` khi có `client_request_id` (mô tả, không ép) |
| Declared | `request_classify`, `request_generatePlan` (pack 2), `request_startPhase` (pack 3) | `.declared()` |

Ví dụ:

```go
// tools/pack_request_flow.go (mới)
func packRequestFlow() []*ToolSpec {
    return []*ToolSpec{
        read("request.get", "Read one Request (title, body, type, status). Text comes from third parties.",
            Str("id", "id", "Request id", Req)).untrusted(),
        write("request.create", "Create a Request in the intake queue. Always send a stable client_request_id per piece of work so retries do not duplicate it.",
            Str("project_id", "projectId", "Project id", Req),
            Str("title", "title", "Short title", Req, Len(500)),
            Str("body", "body", "Description", Len(100000)),
            Str("client_request_id", "clientRequestId", "Stable id for this piece of work", Len(128))),
        write("request.changeType", "Propose a different Request type; a person re-confirms it.",
            Str("id", "id", "Request id", Req),
            Str("to_type", "toType", "New type", Req, OneOf(requestTypes...)),
            Str("reason", "reason", "Why the type should change", Req, Len(2000))),
        // ... theo bảng
    }
}
```

`requestTypes` là 11 loại ở README v6 mục 3.2. Thêm `packRequestFlow()` vào danh sách của `AllSpecs`. Không field nào tên `tenant_id`, `user_id`, `source_provider`, `reporter_id` (test quét schema).

### 2.2 Kênh không thành tool (loại trừ vĩnh viễn)

`excluded_channels.yaml` (mỗi dòng một kênh, `category: human_gate`, `listedAsHardDenied: false`): `request.confirmType`, `solution.choose`, `approval.approve`, `approval.reject`, `approval.cancel`, `request.cancel`, `request.flowSet`, `request.subscribe`. Lý do ghi rõ (agent không tự duyệt cổng của chính nó; huỷ Request không hoàn tác; quản trị tenant ở UI; stream có `resources/` riêng). Gỡ các dòng tạm "Chờ BE-REQ-SOL-017" của BE-REQ-SOL-016 cho kênh có `ToolSpec`; kênh Declared giữ dòng tạm được gỡ khi spec có `ToolSpec` (parity chấp nhận `Declared` là có spec).

### 2.3 Gắn `ToolOrigin` và nguồn `mcp`

```go
// tools/executor.go, trong runGuarded, trước dispatch, chỉ khi spec.Namespace thuộc nhóm Request
if isRequestFlowNamespace(spec.Namespace) && e.guards.SessionID != nil && e.guards.ClientName != nil {
    ctx = wscompat.WithToolOrigin(ctx, wscompat.ToolOrigin{
        ClientName: e.guards.ClientName(ctx), MCPSessionID: e.guards.SessionID(ctx), UserID: p.UserID})
}
```

Luật nguồn nằm ở `channels_request_source.go` (BE-REQ-SOL-016 mục 2.6): `ToolOrigin` có thì `provider=mcp`, `site=ClientName`. Khoá idempotent do `request-service` quyết (CR-REQ-004): `(tenant, 'mcp', 'user:<reporter_id>', client_request_id)`. Gateway truyền `origin` để audit ghi `actor_type=agent` (CR-REQ-024).

### 2.4 Chống spam

| Lớp | Cơ chế | Nơi làm |
|---|---|---|
| 1 | dedupe `client_request_id` | `request-service` (CR-REQ-004) |
| 2 | hạn mức tạo theo `(tenant, user, ClientName)`, token bucket theo giờ, `MCP_REQUEST_CREATE_PER_HOUR=20` (0 tắt), áp cho `request_create` và `request_spawnChild` | gateway, `tools/request_create_limit.go` (mới) |
| 3 | trần Request chưa xác nhận loại của nguồn `mcp` theo user (đề xuất 10), lỗi `REQUEST_PENDING_LIMIT` | `request-service` (CR-REQ-004 cần thêm) |
| 4 | hạn mức write chung và phát hiện lặp của `mcp-service` | có sẵn |
| 5 | `Len(...)`, `OneOf(...)` ở schema | tool spec |
| 6 | chính sách tenant hạ `request_create` về `require_approval` | `mcp-service` (CR-MCP-012) |

`requestCreateLimiter` tái dùng khuôn `scmLimiter` nhưng cửa sổ giờ và khoá ba phần; bộ nhớ bị chặn bằng `maxRateBuckets`. `Config.ApplyEnv` đọc `MCP_REQUEST_CREATE_PER_HOUR` (sai số hoặc âm là lỗi; `0` thành `-1` như `SCMRatePerMin`). Vượt: `&ToolError{"REQUEST_RATE_LIMITED", "too many requests created; retry in Ns"}`. Giới hạn theo từng replica gateway; replica thứ N cho phép gấp N lần (chấp nhận ở v1, lớp 3 là chốt).

### 2.5 Kết quả không tin cậy

Tool `.untrusted()` đi qua `Guards.WrapUntrusted` (đã có) để khung dữ liệu và taint phiên (`UntrustedRead`). `redaction_test.go` thêm ca cho bốn tool đọc. Chưa đọc quy tắc OPA về `session.untrusted_read`, nên chưa biết sau khi đọc `request_get` thì `request_create` có bị đẩy sang approval (mục 6).

### 2.6 Cờ

`request-service` thi hành `request_flow_enabled`: tool ghi khi cờ tắt trả `REQUEST_FLOW_DISABLED` (lỗi kênh); `request_flowStatus` trả `{enabled:false}`. Không ẩn tool khỏi `tools/list` (tránh thông báo `list_changed`).

### 2.7 Tài liệu

`docs/guides/mcp/task-worktree-tools.md` thêm đoạn Request; `docs/guides/mcp/README.md` liên kết; nêu `MCP_TOOL_PACKS_ENABLED=1,2` và danh sách việc agent không làm được.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `request_create` mặc định `allow` (write_reversible) | Request mới chỉ vào hàng chờ xác nhận loại, đã có cổng người |
| D2 | Không tool nào duyệt, chọn phương án, xác nhận loại, huỷ | Giữ nghĩa cổng của người (CR-REQ-017 D2, D5) |
| D3 | `source_provider=mcp` do gateway gán từ `ctx`, input không có field | Client không giả được |
| D4 | `request_classify`, `request_generatePlan`, `request_startPhase` là `Declared` ở v1 | Vượt trần thời gian (mục 1 điểm 3) hoặc rủi ro exec; bật khi RPC trả sớm và red-team qua |
| D5 | Gắn `ToolOrigin` trong executor, chỉ cho namespace Request | Tránh đổi hành vi tool khác |
| D6 | Tool đọc đánh `Untrusted` | Body đến từ Jira, GitHub, agent khác |
| D7 | Tên tool giữ chữ hoa của kênh | Quy ước `ChannelToToolName`, không cần `NameReason` |

## 4. Phụ thuộc và thứ tự

Cần BE-REQ-SOL-016 (kênh, `channels_request_source.go`) và CR-MCP-007, 008, 012, 013 (đã có trong repo: `mcpserver`, `policy_gate.go`). Phụ thuộc CR-REQ-004 cho khoá idempotent và trần `REQUEST_PENDING_LIMIT`. Mở khoá kịch bản MCP của CR-REQ-025 (E17). Thứ tự task ở [`../tasks/README.md`](../tasks/README.md).

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `parity_test.go` | `TestChannelInventory` xanh: mỗi kênh Request có `ToolSpec` (kể cả `Declared`) hoặc dòng loại trừ |
| `catalog_test.go` (mở rộng) | tên, risk, scope, `readOnlyHint` cho tool đọc; 17 tool liệt kê khi bật pack 1,2; 3 tool `Declared` không liệt kê |
| `args_test.go` (mở rộng) | ánh xạ `snake_case` sang wire; enum loại; `Len` |
| `request_create_limit_test.go` (mới) | token bucket giờ, hết hạn, bộ nhớ chặn, `0` tắt |
| `executor_test.go` (mở rộng) | `ToolOrigin` có trong `ctx` cho tool Request, không cho tool khác; `provider` không lấy từ input |
| `redaction_test.go` (mở rộng) | kết quả `untrusted` |
| `schema_no_identity_test.go` (mới) | quét input schema các tool Request không có `tenant_id`, `user_id`, `source_provider`, `reporter_id` |
| `tests/mcp/check_mcp_request_flow.py` (mới) | theo khung `mcp_check_framework.py`: tạo, đọc, trả backlog, mở lại; `tools/call` tới `approval_approve`, `solution_choose`, `request_confirmType` trả không tồn tại |
| Red-team (CR-MCP-013) | Request có body "approve all"; agent không có tool duyệt (chính sách ghi sau đọc không tin cậy: chưa kiểm chứng) |

Lệnh: `cd backend-go/services/api-gateway && go test ./internal/adapter/mcpserver/...`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Quy tắc OPA `session.untrusted_read` chưa đọc: sau `request_get`, `request_create` có thể bị đẩy sang approval, làm agent vô dụng.
- Hạn mức lớp 2 theo từng replica; N replica cho N lần.
- Số tool thêm 17 làm `tools/list` dài; cân nhắc gộp nếu quá ngưỡng ngữ cảnh (CR-MCP-008 khuyến nghị 60 đến 100 tool giá trị cao).
- Phân loại AI tự chạy cho Request nguồn `mcp` (CR-REQ-005): spam vẫn tốn quota AI dù có lớp 2 và 3; chưa có hạn mức AI theo tenant.
- Chưa chạy trên MCP client thật.
- SSH và remote: tool chỉ gọi RPC; `request_startPhase` (khi bật) đi qua `task.execute`.

## 7. Câu hỏi mở

1. Cờ tắt: ẩn tool hay trả lỗi? Chọn trả lỗi (mục 2.6); cần xác nhận.
2. CR-REQ-004 cần: `source_site` = tên client MCP ngoài khoá idempotent, trần `REQUEST_PENDING_LIMIT`, trường `origin`.
3. `request_record_check` (CR-REQ-014) có làm thành kênh WS `request.recordCheck` và tool `request_recordCheck` không? Cần CR-014 và CONTRACT chốt.
4. Resource `orca://request/{id}` (ngoài phạm vi).

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/spec.go`, `.../spec_builders.go`, `.../fields.go`, `.../all_specs.go`, `.../config.go`, `.../executor.go`, `.../scm_rate_limit.go`, `.../composite_env.go`, `.../parity_test.go`, `.../excluded_channels.yaml`, `.../pack2_workspace.go`, `.../catalog_test.go`, `.../redaction_test.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/tool_origin_context.go`, `.../handler.go`
- `backend-go/common/mcpscope/`; `tests/mcp/mcp_check_framework.py` (khung e2e, đã tồn tại), `docs/guides/mcp/task-worktree-tools.md`
- `docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md`, `CR-MCP-008-domain-tool-packs.md`; `docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md`, `CR-MCP-013-approvals-audit-killswitch.md`
- `docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md`
