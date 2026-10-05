# CR-REQ-017 — Tool MCP `request_*`, `solution_*`, `approval_*` và MCP làm nguồn Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-017 |
| **Tên** | Khai báo `ToolSpec` cho agent ngoài làm việc với Request; gắn nguồn `mcp` và chống spam |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium (3 đến 5 ngày: spec, giới hạn tạo, test parity, golden, e2e MCP) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-016 (kênh), CR-MCP-007, 008 (descriptor, pack), CR-MCP-012, 013 (chính sách, approval) |
| **Mở khoá** | CR-REQ-025 (kịch bản e2e qua MCP) |
| **Tác động** | `backend-go/services/api-gateway/internal/adapter/mcpserver/tools` (`pack_request_flow.go` mới, `all_specs.go`, `config.go`, `request_create_limit.go` mới, `excluded_channels.yaml`, `testdata/tools_list.golden.json`), `docs/guides/mcp/` |

---

## 1. Bối cảnh và vấn đề

1. Mục tiêu v6 có "MCP là một nguồn Request": agent ngoài (Claude Code, Cursor) gọi `request_create` để đẩy việc vào hộp phân loại. Hiện tool pack chỉ có `task.*`, `workflow.*`, `terminal.*`, `project.*` (`tools/pack1_workspace.go`, `pack2_workspace.go`, `pack3_exec.go`).
2. Tool sinh tự `ToolSpec` gắn với channel của `wscompat` (CR-MCP-007, 008). Tên tool là tên channel thay `.` bằng `_` (`ChannelToToolName`, giữ nguyên chữ hoa, ví dụ `task_createFromSource`), nên `request.changeType` thành `request_changeType`.
3. Cổng duyệt của người là lõi của v6. Nếu agent gọi được `approval.approve`, nó tự duyệt việc của nó và cổng thành vô nghĩa. Danh sách tool phải loại các bước đó.
4. `request_create` là đường tạo dữ liệu mà người dùng không nhìn thấy lúc tạo: cần chống spam (một agent lặp vòng tạo hàng trăm Request, mỗi cái kích hoạt phân loại AI tốn quota).
5. Nội dung Request lấy từ Jira, GitHub, hoặc agent khác là văn bản không tin cậy (tiêm lệnh). Tool đọc phải đánh dấu `Untrusted` để cơ chế taint của `mcp-service` áp dụng (`UntrustedRead` đã là đầu vào của OPA, `domain/tool_policy.go`).

## 2. Giải pháp đề xuất

### 2.1 Bảng tool

Cột "Wire" là khoá trong args của channel. `Untrusted` = kết quả mang văn bản bên thứ ba. `RequiredScope` lấy từ risk (`orca:read`, `orca:write`, `orca:exec`, theo `common/mcpscope`).

| Tool | Channel | Pack, risk, scope | Tham số (snake_case → wire) | RPC đích | Ghi chú |
|---|---|---|---|---|---|
| `request_list` | `request.list` | 1, read, `orca:read` | `project_id`, `status[]`, `type[]`, `source_provider`, `page_size`, `page_token` | `ListRequests` | `asList`, `untrusted` |
| `request_get` | `request.get` | 1, read | `id` | `GetRequest` | `untrusted` (title, body từ nguồn ngoài) |
| `request_typeHistory` | `request.typeHistory` | 1, read | `id` | `ListRequestTypeHistory` | `asList`; lý do đổi loại do người viết nên `untrusted` |
| `solution_list` | `solution.list` | 1, read | `request_id`, `kind`, `status`, `page_size`, `page_token` | `ListSolutions` | `asList`, `untrusted` (AI sinh từ nội dung ngoài) |
| `approval_get` | `approval.get` | 1, read | `id` | `GetApproval` | |
| `approval_list` | `approval.list` | 1, read | `request_id`, `subject_type`, `status`, `page_size`, `page_token` | `ListApprovals` | `asList` |
| `approval_listPending` | `approval.listPending` | 1, read | `subject_type`, `page_size`, `page_token` | `ListPendingForUser` | cho agent biết còn gì chờ người |
| `backlog_requests` | `backlog.requests` | 1, read | `project_id`, `page_size`, `page_token` | `ListBacklog` | `asList`, `untrusted` |
| `backlog_tasks` | `backlog.tasks` | 1, read | `project_id`, `plan_task_id`, phân trang | RPC CR-REQ-015 | `asList` |
| `backlog_execute` | `backlog.execute` | 1, read | `project_id`, `phase_task_id`, phân trang | RPC CR-REQ-015 | `asList` |
| `request_flowStatus` | `request.flowStatus` | 1, read | không | `GetRequestFlowSettings` | agent biết cờ có bật không |
| `request_create` | `request.create` | 2, write_reversible, `orca:write` | `project_id` (bắt buộc), `title` (≤500, bắt buộc), `body` (≤100000), `client_request_id` (≤128) | `CreateRequest` | Xem 2.3; luôn nên gửi `client_request_id` |
| `request_classify` | `request.classify` | 2, write_reversible, `openWorld` | `id` | `ClassifyRequest` | tốn quota AI |
| `request_changeType` | `request.changeType` | 2, write_reversible | `id`, `to_type` (enum 11 loại), `reason` (bắt buộc) | `ChangeRequestType` | quay về chờ người xác nhận |
| `request_returnToBacklog` | `request.returnToBacklog` | 2, write_reversible | `id`, `stage` (enum), `reason` (bắt buộc) | `ReturnToBacklog` | agent báo "không thực thi được"; hoàn tác bằng `request_reopen` |
| `request_reopen` | `request.reopen` | 2, write_reversible | `id` | `ReopenRequest` | |
| `request_spawnChild` | `request.spawnChild` | 2, write_reversible | `id`, `reason` (enum), `title`, `body` | `SpawnChildRequest` | tính vào hạn mức tạo (2.3) |
| `solution_generate` | `solution.generate` | 2, write_reversible, `openWorld` | `request_id`, `idempotency_key`, `feedback` (≤2000) | `GenerateSolution` | tốn quota AI |
| `request_generatePlan` | `request.generatePlan` | 2, write_reversible, `openWorld` | `id` | `GeneratePlan` | chỉ khi Solution đã duyệt |
| `request_startPhase` | `request.startPhase` | 3, exec, `orca:exec` | `id`, `phase_task_id` | `StartPhase` | `Declared` ở v1 (xem D4) |

### 2.2 Kênh không thành tool (loại trừ vĩnh viễn)

Ghi vào `excluded_channels.yaml` với `reason` rõ, `listedAsHardDenied: false` (khác `credentials.*`):

| Channel | Lý do |
|---|---|
| `request.confirmType` | xác nhận loại là cổng của người (bắt buộc người với `hotfix`, `security`) |
| `solution.choose` | chọn phương án là quyết định của người duyệt |
| `approval.approve`, `approval.reject`, `approval.cancel` | agent không được tự duyệt, từ chối hay huỷ cổng của chính nó |
| `request.cancel` | huỷ Request không hoàn tác được trong máy trạng thái (v6 README 3.3); để người làm |
| `request.flowSet` | quản trị tenant, người dùng thao tác trong UI |
| `request.subscribe` | kênh stream; MCP có `resources/` subscription riêng |

### 2.3 MCP làm nguồn Request

Theo CR-REQ-004: nguồn `mcp` không có `ref`; khoá idempotent là `(tenant, 'mcp', 'user:<reporter_id>', client_request_id)` ghi ở `request_idempotency`, và thiếu `client_request_id` thì mỗi lần gọi tạo một Request.

| Thuộc tính | Giá trị | Ai đặt |
|---|---|---|
| `source_provider` | `mcp` | gateway, từ `ToolOrigin` (`wscompat/tool_origin_context.go`); input tool không có field này |
| `source_site` | `ClientName` của phiên MCP (ví dụ `claude-code`), chỉ để hiển thị, không nằm trong khoá | gateway (cần CR-REQ-004 xác nhận, Q2) |
| `source_ref`, `source_url` | rỗng | |
| `client_request_id` | giá trị agent gửi; mô tả tool nhắc agent đặt một giá trị ổn định cho mỗi việc | agent |
| `reporter_id` | `Identity.UserID` của người sở hữu token MCP | gateway |
| Audit | `actor_type=agent`, `metadata` có `client_name`, `mcp_session_id` | CR-REQ-024 |

Gọi lại với cùng `client_request_id` trả Request cũ với `created=false`.

### 2.4 Chống spam (nhiều lớp)

| Lớp | Cơ chế | Mặc định | Nơi thi hành |
|---|---|---|---|
| 1 | Dedupe theo `client_request_id` (CR-REQ-004) | luôn bật khi agent có gửi | `request-service` |
| 2 | Hạn mức tạo theo `(tenant, user, ClientName)`, token bucket theo giờ, kiểu `scmLimiter` | `MCP_REQUEST_CREATE_PER_HOUR=20`, `0` = tắt, thêm vào `tools.Config.ApplyEnv` | gateway (`request_create_limit.go`, mới); áp cho `request_create` và `request_spawnChild` |
| 3 | Trần Request chưa qua bước xác nhận loại (`new`, `classifying`, `awaiting_type_confirmation`) cho nguồn `mcp` theo user | 10 | `request-service`, lỗi `REQUEST_PENDING_LIMIT`; CR-REQ-004 chưa có quy tắc này, cần thêm (Q2) |
| 4 | Hạn mức write chung của `mcp-service` và phát hiện lặp (`LoopSlowDown` 5, `LoopBlock` 20, write 120/phút) | có sẵn (`DefaultGovernanceConfig`) | `mcp-service` |
| 5 | Giới hạn độ dài tham số (`Len(...)`) | theo 2.1 | schema tool |
| 6 | Chính sách tenant có thể hạ `request_create` về `require_approval` | admin chọn | `mcp-service` (CR-MCP-012) |

Vượt lớp 2 hoặc 3: tool trả lỗi `REQUEST_RATE_LIMITED` hoặc `REQUEST_PENDING_LIMIT` kèm thời gian chờ (cùng cách `scm_rate_limit.go` báo `retry after`).

### 2.5 Tệp thay đổi

| File | Việc |
|---|---|
| `tools/pack_request_flow.go` (mới) | `packRequestFlow()` trả các `ToolSpec` bằng builder `read`, `write`, `execTool` trong `spec_builders.go` |
| `tools/all_specs.go` | thêm `packRequestFlow()` vào danh sách nhóm |
| `tools/config.go` | đọc `MCP_REQUEST_CREATE_PER_HOUR` trong `ApplyEnv` |
| `tools/request_create_limit.go` (mới) | bộ giới hạn theo giờ, bộ nhớ bị chặn như `maxRateBuckets` |
| `tools/excluded_channels.yaml` | gỡ dòng tạm của CR-016 cho kênh có tool; thêm dòng vĩnh viễn mục 2.2 |
| `tools/testdata/tools_list.golden.json` | cập nhật golden |
| `docs/guides/mcp/task-worktree-tools.md` | thêm đoạn Request; `docs/guides/mcp/README.md` liên kết |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `request_create` mặc định `allow` (write_reversible), không `require_approval` | Request mới chỉ vào hàng chờ xác nhận loại, đã có cổng người; bắt approval mỗi lần làm agent vô dụng. Tenant hạ được về approval |
| D2 | Không tool nào duyệt, chọn phương án hay xác nhận loại | Giữ nguyên ý nghĩa cổng của người |
| D3 | `source_provider=mcp` do gateway gán từ ngữ cảnh | Client không giả được; dùng cơ chế `ToolOrigin` đã có |
| D4 | `request_startPhase` là `exec`, ở v1 đánh `Declared` (có trong catalog, chưa chạy) | Chạy agent trên máy dev là rủi ro cao nhất; CR-MCP-008 đã làm exec ở đợt sau. Bật khi CR-REQ-013 ổn định và red-team qua |
| D5 | `request_cancel` không có; `request_returnToBacklog` có | Trả về backlog hoàn tác được, huỷ thì không; agent cần cách báo "không làm được" |
| D6 | Tool đọc đánh `Untrusted` | Body Request đến từ Jira, GitHub, agent khác; để taint và che theo `redaction_rules.go` |
| D7 | Tên tool giữ chữ hoa của channel (`request_changeType`) | Quy ước `ChannelToToolName`; đổi tên cần `NameReason` và không có lý do chính đáng |

## 4. Tiêu chí chấp nhận

- [ ] `tools_list` có đủ 19 tool đọc và ghi của bảng 2.1 khi pack tương ứng bật; `request_startPhase` có trong catalog nhưng không được liệt kê hay thực thi (`Declared`).
- [ ] `parity_test.go` xanh: mọi channel của CR-016 hoặc có `ToolSpec` hoặc có dòng trong `excluded_channels.yaml` kèm lý do.
- [ ] Không tool nào có field `tenant_id`, `user_id`, `source_provider`, `reporter_id` trong schema đầu vào (test quét schema).
- [ ] Gọi `request_create` qua MCP tạo Request có `source_provider=mcp` và `source_site=<ClientName>`; gọi lại cùng `client_request_id` trả cùng id, `created=false`.
- [ ] Gọi `request_create` lần thứ 21 trong một giờ (mặc định) bị từ chối `REQUEST_RATE_LIMITED`; `MCP_REQUEST_CREATE_PER_HOUR=0` tắt giới hạn.
- [ ] Có 10 Request `mcp` chưa xác nhận loại thì lần tạo tiếp theo trả `REQUEST_PENDING_LIMIT`.
- [ ] `tools/call` tới `approval_approve`, `solution_choose`, `request_confirmType` trả "tool không tồn tại" (không có trong catalog).
- [ ] Kết quả `request_get`, `request_list`, `solution_list`, `backlog_requests` mang cờ `untrusted` và qua bộ che (`redaction_test.go` mở rộng).
- [ ] Cờ `request_flow_enabled` tắt: mọi tool ghi trả `REQUEST_FLOW_DISABLED`, tool đọc `request_flowStatus` trả `enabled=false`.
- [ ] Golden `tools_list.golden.json` được cập nhật có chủ đích trong cùng PR.

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `parity_test.go` (chạy lại) | inventory + loại trừ |
| `catalog_test.go` (mở rộng) | tên, risk, scope, annotation đúng bảng 2.1; read-only có `readOnlyHint` |
| `args_test.go` (mở rộng) | ánh xạ snake_case sang wire; enum loại và `stage` |
| `request_create_limit_test.go` (mới) | token bucket theo giờ, hết hạn, bộ nhớ bị chặn |
| `executor_test.go` (mở rộng) | `ToolOrigin` được truyền; `source_provider` không lấy từ input |
| `redaction_test.go` (mở rộng) | kết quả `untrusted` |
| e2e MCP (`tests/mcp/check_mcp_request_flow.py`, mới) | theo khung `mcp_check_framework.py`: tạo, đọc, trả backlog, mở lại; xác nhận không có tool duyệt |
| Red-team (CR-MCP-013) | Request có body chứa lệnh "approve all"; agent không có tool để duyệt và policy chặn ghi sau đọc không tin cậy theo quy tắc OPA hiện hành (chưa kiểm chứng quy tắc cụ thể) |

Chưa chạy: toàn bộ.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa đọc quy tắc OPA về `session.untrusted_read`; chưa biết sau khi đọc `request_get` thì `request_create` có bị đẩy sang approval hay không.
- Hạn mức lớp 2 ở bộ nhớ từng replica gateway (như `scmLimiter`), nên N replica cho phép gấp N lần. Chấp nhận ở v1, `request-service` lớp 3 là chốt cuối.
- Số tool tăng 20, ảnh hưởng ngữ cảnh LLM (CR-MCP-008 khuyến nghị 60 đến 100 tool giá trị cao). Cân nhắc gộp nếu `tools/list` quá dài.
- Chưa chạy trên MCP client thật; `tests/mcp/` hiện là thư mục chưa commit.
- SSH và remote: tool chỉ gọi RPC; `request_startPhase` (khi bật) đi qua `task.execute` nên giữ cùng đường SSH hiện có.

## 7. Câu hỏi mở

1. Auto-phân loại chạy ngay cho Request nguồn `mcp` (CR-REQ-004 vào `classifying` ngay khi tạo, CR-REQ-005 giới hạn 5 lần AI mỗi Request). Chưa có hạn mức AI theo tenant; spam nhiều Request vẫn tốn quota. Cần quyết định.
2. Cần CR-REQ-004: (a) cho phép `source_site` = tên client MCP, ngoài khoá idempotent; (b) thêm trần `REQUEST_PENDING_LIMIT` cho nguồn `mcp`; (c) thêm trường `origin` ở `CreateRequestRequest`. Nếu CR-004 giữ "mỗi lần gọi một Request" khi thiếu `client_request_id`, lớp 2 và 3 là phòng tuyến chính.
3. Khi cờ `request_flow_enabled` tắt, ẩn hẳn tool khỏi `tools/list` hay trả lỗi? CR này chọn trả lỗi (đơn giản, không phải thông báo `list_changed`); cần xác nhận.
4. Có thêm resource `orca://request/{id}` cho `resources/plans.go` không? Ngoài phạm vi CR này.

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/spec.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/spec_builders.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/all_specs.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/config.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/scm_rate_limit.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/parity_test.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/pack2_workspace.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/pack3_exec.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/catalog_test.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/redaction_test.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/testdata/tools_list.golden.json`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/policy_gate.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tool_policy_view.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/tool_origin_context.go`
- `backend-go/common/mcpscope/` (scope `orca:read`, `orca:write`, `orca:exec`, `orca:admin`)
- `backend-go/services/mcp-service/internal/usecase/governance_core.go`, `backend-go/services/mcp-service/internal/usecase/governance_ports.go`, `backend-go/services/mcp-service/internal/domain/approval.go`, `backend-go/services/mcp-service/internal/domain/tool_policy.go`
- `docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md`, `docs/crs/v5/mcp-tool-catalog/CR-MCP-008-domain-tool-packs.md`
- `docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md`, `docs/crs/v5/mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md`
- `docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md`
- `tests/mcp/mcp_check_framework.py`, `docs/guides/mcp/task-worktree-tools.md`
