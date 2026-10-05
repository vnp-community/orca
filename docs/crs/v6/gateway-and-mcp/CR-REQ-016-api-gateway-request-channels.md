# CR-REQ-016 — Kênh WS/HTTP của `api-gateway` cho Request, Solution, Approval, Backlog

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-016 |
| **Tên** | Đăng ký kênh `request.*`, `solution.*`, `approval.*`, `backlog.*` trong `wscompat` và route HTTP tối thiểu |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium (4 đến 6 ngày: client wiring, 28 kênh, view camelCase, route HTTP, test) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001 (proto `orca.request.v1`), 003, 004, 005, 006, 007, 008, 009; kênh `backlog.*` cần CR-REQ-015 |
| **Mở khoá** | CR-REQ-017, CR-REQ-018 đến 023 (frontend), CR-REQ-025 |
| **Tác động** | `backend-go/services/api-gateway` (`internal/adapter/wscompat`, `internal/adapter/httpgateway`, `internal/config`, `cmd/server`, `README.md`), `internal/adapter/mcpserver/tools/excluded_channels.yaml` |

---

## 1. Bối cảnh và vấn đề

1. `request-service` (CR-REQ-001) chỉ nói gRPC. Frontend gọi qua WebSocket `wscompat`; không có kênh thì chưa UI nào dùng được (CR-REQ-018 đến 023).
2. Mẫu đăng ký đã rõ: `Registry.Register(channel, handler)`, mỗi nhóm một file `channels_*.go`, client gRPC đi vào qua `ChannelDeps` (`wscompat/register_production.go`) và `RegisterProductionChannels`. Ví dụ gần nhất: `channels_task_source.go`, `channels_orchestration.go`.
3. Gateway không có OPA trước định tuyến (README api-gateway, mục "Other known gaps"), nên mỗi RPC của `request-service` phải tự kiểm quyền. Gateway chỉ gắn `Identity` qua `gatewaygrpc.AttachIdentity` (đã làm sẵn trong `Registry.Dispatch`).
4. Kênh mới làm đỏ `TestChannelInventory` trong `mcpserver/tools/parity_test.go` nếu không có `ToolSpec` hoặc dòng loại trừ. CR này phải xử lý để không chặn CI.
5. Nguồn Request phải đáng tin: `source_provider=mcp` hay `webhook` không được do client WS tự khai.

## 2. Giải pháp đề xuất

### 2.1 Wiring client

| Việc | File |
|---|---|
| Thêm `"request-service": commonconfig.StringEnv("REQUEST_SERVICE_ADDR", "")` vào `OtherServiceAddrs` | `internal/config/config.go` |
| Dial bằng `gatewaygrpc.Dial`, tạo `requestv1.NewRequestServiceClient` và `NewApprovalServiceClient`, `healthSrv.Register("request-service", ...)` | `cmd/server/main.go` (cạnh khối `task-service`) |
| Thêm `Request requestv1.RequestServiceClient`, `Approval requestv1.ApprovalServiceClient` vào `ChannelDeps`; gọi `registerRequestChannels(r, d.Request, d.Approval, d.EventBus)` trong `RegisterProductionChannels` | `wscompat/register_production.go` |
| Mọi field nil vẫn hợp lệ (đăng ký chỉ tạo closure) để parity test dựng được inventory | như hiện có |

Địa chỉ rỗng: kênh vẫn đăng ký, gọi vào trả `REQUEST_UNAVAILABLE: request service not configured`, theo cách các client khác của gateway xử lý.

### 2.2 Tệp mới trong `wscompat` (mới)

| File | Nội dung |
|---|---|
| `channels_request.go` | `request.*` (vòng đời, phân loại, plan, phase) |
| `channels_request_views.go` | struct JSON camelCase: `requestView`, `solutionView`, `approvalView`, `typeHistoryView` |
| `channels_solution.go` | `solution.*` |
| `channels_approval.go` | `approval.*` |
| `channels_request_backlog.go` | `backlog.*` |
| `channels_request_stream.go` | `request.subscribe` (stream) |
| `channels_request_flow.go` | `request.flowStatus`, `request.flowSet` |
| `*_test.go` tương ứng | fake client theo mẫu `fakeOrchestrationClient` |

Quy tắc: không trả message proto trực tiếp (JSON mặc định của `encoding/json` ra snake_case, xem ghi chú BUG-023 ở `channels.go`); luôn qua struct view camelCase. Slice nil được `normalizeNilSlices` chuẩn hoá thành `[]`.

### 2.3 Bảng kênh `request.*`

`Id` = `Identity`. Cột "Quyền tối thiểu" là mức `request-service` thi hành, gateway không tự kiểm.

| Kênh | Tham số (camelCase) | RPC đích | Quyền tối thiểu | Timeout | Ghi chú |
|---|---|---|---|---|---|
| `request.create` | `projectId`, `title` (≤500), `body` (≤100000), `source?` {`provider`,`ref`,`url`,`site`}, `hints?`, `clientRequestId?` | `CreateRequest` | write trên project | 8s | Xem 2.6 về nguồn; `created=false` khi gọi lặp |
| `request.get` | `id` | `GetRequest` | read | 8s | Không kèm lịch sử loại (có kênh riêng) |
| `request.typeHistory` | `id` | `ListRequestTypeHistory` (CR-REQ-005) | read | 8s | |
| `request.list` | `projectId?`, `status[]?`, `type[]?`, `sourceProvider?`, `sourceSite?`, `sourceRef?`, `pageSize`, `pageToken` | `ListRequests` | read trên project | 8s | |
| `request.classify` | `id` | `ClassifyRequest` | write | 25s | Chạy lại phân loại AI, tốn quota |
| `request.confirmType` | `id`, `type`, `size?`, `urgency?`, `reason?` | `ConfirmRequestType` | người duyệt `request_type` | 8s | Bắt buộc người (không MCP) |
| `request.changeType` | `id`, `toType`, `reason` (bắt buộc) | `ChangeRequestType` | write | 8s | Quay về `awaiting_type_confirmation` |
| `request.returnToBacklog` | `id`, `stage`, `reason` | `ReturnToBacklog` | write | 8s | `stage` ∈ một trong `classification`, `analysis`, `plan`, `phase`, `task` |
| `request.reopen` | `id` | `ReopenRequest` | write | 8s | Từ `request_backlog` |
| `request.cancel` | `id`, `reason` | `CancelRequest` | write, chủ Request hoặc admin project | 8s | |
| `request.spawnChild` | `id`, `reason`, `title`, `body`, `type?` | `SpawnChildRequest` | write | 8s | `reason` ∈ enum `request_links.reason` |
| `request.generatePlan` | `id` | `GeneratePlan` | write | 25s | Cần Solution đã duyệt (CR-REQ-012) |
| `request.startPhase` | `id`, `phaseTaskId` | `StartPhase` | exec trên project | 8s | Chạy agent qua `task.execute` |
| `request.subscribe` | `id?` | (xem 2.5) | read | stream | Push `request.event` |
| `request.flowStatus` | không | `GetRequestFlowSettings` (CR-REQ-025) | thành viên tenant | 8s | `{enabled}` |
| `request.flowSet` | `enabled` | `SetRequestFlowSettings` (CR-REQ-025) | admin toàn cục | 8s | Dùng `Identity.Role` |

### 2.4 Bảng kênh `solution.*`, `approval.*`, `backlog.*`

| Kênh | Tham số | RPC đích | Quyền tối thiểu | Timeout |
|---|---|---|---|---|
| `solution.list` | `requestId`, `kind?`, `status?`, `pageSize`, `pageToken` | `ListSolutions` | read | 8s |
| `solution.generate` | `requestId`, `idempotencyKey?`, `feedback?` (≤2000) | `GenerateSolution` | write | 25s |
| `solution.choose` | `requestId`, `solutionId`, `optionId`, `comment?` | `ChooseSolutionOption` | người duyệt `solution` | 8s |
| `approval.get` | `id` | `GetApproval` | read Request chứa nó | 8s |
| `approval.list` | `requestId`, `subjectType?`, `status?`, `pageSize`, `pageToken` | `ListApprovals` | read | 8s |
| `approval.listPending` | `subjectType?`, `pageSize`, `pageToken` | `ListPendingForUser` | người dùng hiện tại | 8s |
| `approval.approve` | `id`, `expectedVersion`, `expectedDigest`, `comment?` | `Approve` | người duyệt theo CR-REQ-009, 010 | 8s |
| `approval.reject` | `id`, `expectedVersion`, `expectedDigest`, `comment` (bắt buộc) | `Reject` | như trên | 8s |
| `approval.cancel` | `id`, `reason` | `Cancel` | người yêu cầu hoặc admin | 8s |
| `backlog.requests` | `projectId?`, `groupBy?` (`reason`), `pageSize`, `pageToken` | `ListBacklog` | read | 8s |
| `backlog.tasks` | `projectId?`, `planTaskId?`, phân trang | RPC của CR-REQ-015 (tên chốt ở đó) | read | 8s |
| `backlog.execute` | `projectId?`, `phaseTaskId?`, phân trang | RPC của CR-REQ-015 | read | 8s |

`RequestApproval` không có kênh: Approval do `request-service` tự tạo theo registry luồng, người dùng không được tạo tuỳ ý. `ReportTaskOutcome` là RPC nội bộ, không có kênh.

### 2.5 Sự kiện đẩy `request.subscribe`

Theo mẫu `channels_task_activity.go`: `RegisterStream`, mỗi kết nối nghe NATS bằng `commoneventbus.Consumer.SubscribeEphemeral` các subject `orca.request.*` của tenant, lọc theo `id` nếu có. Trước khi nhận, gọi `GetRequest` một lần để kiểm quyền read. Khung `request.event`: `{requestId, eventType, status, type, occurredAt}`, không kèm `body`. NATS không kết nối thì kênh không đăng ký (cùng cách `TaskActivityEnabled`).

### 2.6 Nguồn Request

| Tình huống | `source_provider` gửi xuống |
|---|---|
| Ngữ cảnh có `ToolOrigin` (gọi từ MCP) | `mcp`; `source_site` = `ClientName`; client không được ghi đè |
| Client WS gửi `source.provider` ∈ `jira`, `github`, `gitlab` hoặc `linear` kèm `ref` | giữ nguyên (từ nút "Tạo Request" ở trang Tasks, CR-REQ-019) |
| Client WS không gửi `source` | `manual` |
| Client WS gửi `mcp`, `webhook` hoặc `manual` tường minh | từ chối `REQUEST_SOURCE_FORBIDDEN` |

`webhook` chỉ do route của CR-REQ-004 gán. Gateway cũng chuyển `origin` (loại, tên client, id phiên MCP) vào `CreateRequestRequest` để audit ghi `actor_type=agent` (CR-REQ-024); trường proto này cần CR-REQ-004 thêm (câu hỏi mở Q2).

### 2.7 Route HTTP (`httpgateway/request_routes.go`, mới)

Chỉ những thứ script, CLI hay thông báo cần; phần còn lại đi WS.

| Method và đường dẫn | RPC | Ghi chú |
|---|---|---|
| `POST /v1/requests` | `CreateRequest` | cùng luật nguồn như 2.6 |
| `GET /v1/requests`, `GET /v1/requests/{id}` | `ListRequests`, `GetRequest` | |
| `GET /v1/approvals/pending` | `ListPendingForUser` | deep link từ thông báo |
| `POST /v1/approvals/{id}/approve`, `.../reject` | `Approve`, `Reject` | body `{version, comment}` |

Gắn trong `router.go` cạnh `mountTaskRoutes(authed, deps.TaskClient)`: `mountRequestRoutes(authed, deps.RequestClient, deps.ApprovalClient)`. Body decode lỗi trả `INVALID_ARGUMENT` như `task_routes.go`; `tenant_id` không có trong body.

### 2.8 Mã lỗi

Gateway chuyển lỗi `request-service` dạng `CODE: message`. Tên lấy từ CR-REQ-003, 004, 005, 007, 009; dòng "mới" chưa có CR nào sở hữu:

| Mã | Khi nào |
|---|---|
| `REQUEST_NOT_FOUND`, `APPROVAL_NOT_FOUND`, `SOLUTION_NOT_FOUND` | id sai hoặc khác tenant |
| `REQUEST_TRANSITION_NOT_ALLOWED`, `REQUEST_STATE_STALE` | thao tác không hợp lệ ở trạng thái hiện tại (CR-REQ-003) |
| `REQUEST_TYPE_REQUIRED`, `REQUEST_TYPE_CHANGE_NOT_ALLOWED`, `REQUEST_TYPE_CHANGE_USE_CHILD` | sai loại hoặc đường đổi loại (CR-REQ-005) |
| `REQUEST_CLASSIFICATION_LIMIT` | quá 5 lần phân loại AI (CR-REQ-005) |
| `REQUEST_VERSION_CONFLICT`, `APPROVAL_VERSION_CONFLICT`, `SOLUTION_VERSION_CONFLICT` | `version` cũ |
| `APPROVAL_ALREADY_DECIDED`, `APPROVAL_EXPIRED` | duyệt lặp, quá hạn |
| `APPROVAL_NOT_APPROVER` | không đủ quyền duyệt |
| `APPROVAL_COMMENT_REQUIRED`, `REQUEST_REASON_REQUIRED` | thiếu `comment` hoặc `reason` |
| `REQUEST_SOURCE_PROVIDER_INVALID` | nguồn ngoài tập hợp lệ |
| `REQUEST_SOURCE_FORBIDDEN` (mới, của gateway) | client WS tự khai nguồn `mcp`, `webhook`, `manual` |
| `REQUEST_FLOW_DISABLED` (mới, CR-REQ-025) | cờ tắt |
| `REQUEST_RATE_LIMITED`, `REQUEST_PENDING_LIMIT` (mới, CR-REQ-017 và 004) | chống spam |
| `REQUEST_UNAVAILABLE` (mới, của gateway) | chưa cấu hình địa chỉ service |

### 2.9 Redaction và ghi log

Gateway không log args (xem `channel_args_redaction.go`). `body` của Request có thể chứa dữ liệu nội bộ nên: không thêm vào `sensitiveArgChannels` (không phải secret), nhưng không đưa `body` vào thông điệp lỗi hay span; có test khẳng định lỗi không chứa `body`.

### 2.10 Loại trừ khỏi MCP (tạm)

Thêm vào `mcpserver/tools/excluded_channels.yaml` mọi kênh mục 2.3 và 2.4 với `reason` "chờ CR-REQ-017". CR-017 gỡ dòng của kênh nó phủ; các kênh nhóm G4 của README feature giữ lại với lý do vĩnh viễn.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Plan và Phase đi qua `request.generatePlan`, `request.startPhase`, đọc cây bằng `task.getSubtree` với `plan_task_id` | README v6 không có tiền tố `plan.*`; Plan, Phase là Task nên tái dùng kênh task |
| D2 | Timeout 8s cho thao tác thường, 25s cho kênh gọi AI | `groupRPCTimeout` là 8s; `INVOKE_TIMEOUT_MS` của handler là 30s; 25s để lỗi là của ta |
| D3 | `approval.approve/reject` bắt buộc `expectedVersion` và `expectedDigest` | Theo `DecideApprovalRequest` của CR-REQ-009; chặn duyệt trên bản đã đổi |
| D4 | Không có kênh `approval.request` | Approval sinh từ máy trạng thái, không do người dùng tạo |
| D5 | `request.subscribe` dùng NATS ephemeral như `task.activity` | Đã có mẫu chạy; không thêm cơ chế thứ ba |
| D6 | HTTP chỉ 5 route | Giữ bề mặt nhỏ; mọi thứ khác qua WS |

## 4. Tiêu chí chấp nhận

- [ ] `REQUEST_SERVICE_ADDR` được đọc; gateway khởi động được khi để trống (kênh trả `REQUEST_UNAVAILABLE`).
- [ ] 28 kênh ở 2.3 và 2.4 có trong `Registry.Channels()`; test inventory liệt kê đúng tên.
- [ ] Không kênh nào nhận `tenantId` hay `userId` trong tham số; test gửi giá trị giả và khẳng định RPC nhận giá trị từ `Identity`.
- [ ] `request.create` từ WS với `source.provider=mcp` trả `REQUEST_SOURCE_FORBIDDEN`; với ngữ cảnh `ToolOrigin` thì RPC nhận `source_provider=mcp` và `source_site=ClientName`.
- [ ] Kết quả danh sách rỗng là `[]`, không `null`.
- [ ] Mọi view JSON là camelCase; không có khoá snake_case (test quét).
- [ ] Lỗi không chứa `body` (test với `body` chứa chuỗi đánh dấu).
- [ ] Route HTTP trả 400 `INVALID_ARGUMENT` cho JSON hỏng, 401 khi chưa xác thực, và ánh xạ lỗi gRPC qua `writeGRPCError`.
- [ ] `go test ./internal/adapter/mcpserver/tools/...` xanh sau khi thêm dòng loại trừ.
- [ ] `README.md` của api-gateway ghi thêm `request-service` và các kênh mới.

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `channels_request_test.go` | từng kênh với fake client: ánh xạ tham số, camelCase, lỗi, timeout (context deadline đúng 8s hoặc 25s) |
| `channels_request_source_test.go` | bảng 2.6, gồm giả mạo nguồn |
| `channels_approval_test.go` | `version` và `comment` bắt buộc, `listPending` dùng `Identity` |
| `channels_request_stream_test.go` | lọc theo tenant và `id`, không rò sự kiện tenant khác |
| `channels_request_backlog_test.go` | phân trang, nil-slice |
| `request_routes_test.go` | 5 route, mẫu `task_routes_test.go` |
| `registry_channels_test.go` (sửa) | inventory có đủ kênh mới |
| `mcpserver/tools/parity_test.go` (chạy lại) | xanh nhờ mục loại trừ |

Chưa chạy: toàn bộ danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa xác nhận AI trong `ClassifyRequest`, `GenerateSolution`, `GeneratePlan` chạy đồng bộ hay bất đồng bộ. Nếu đồng bộ và vượt 25s thì cần RPC trả ngay và hoàn tất qua `request.event`.
- Chưa xác nhận NATS subject đầy đủ của sự kiện (`orca.request.request.status_changed` theo mẫu `orca.project.worktree.created`) vì README v6 chỉ ghi `orca.request.*`.
- Gateway không có OPA; nếu `request-service` thiếu kiểm quyền ở một RPC thì lỗ hổng lộ ngay qua WS và HTTP. Cần test quyền ở phía service (CR-REQ-009, 010).
- `request.subscribe` thêm một kết nối NATS ephemeral cho mỗi socket; chưa đo tải.
- Đường SSH và remote: gateway không chạm máy dev; `request.startPhase` chỉ gọi RPC, việc chạy agent đi qua `task.execute` nên giữ nguyên hành vi SSH hiện có.

## 7. Câu hỏi mở

1. Không có RPC liệt kê Request con (`request_links`) trong README v6 và CR-REQ-001 đến 009 mà tôi đọc; cần CR-REQ-006 xác nhận `GetRequest` có trả `children` hay thêm RPC.
2. `CreateRequestRequest` của CR-REQ-004 chưa có trường `origin` (loại, tên client, id phiên MCP) cho audit `actor_type=agent`. Cần CR-REQ-004 thêm.
3. Ba mã lỗi "mới" ở 2.8 cần CR-REQ-025, 017 và 004 nhận sở hữu.
4. Hai RPC view backlog Task và Execute chưa có tên trong README v6 mục 3.6 (chỉ `ListBacklog`).
5. Route webhook do CR-REQ-004 sở hữu (`httpgateway`, mới); CR này không mount.

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/register_production.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_task_source.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_orchestration.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_task_activity.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channel_args_redaction.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/tool_origin_context.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/registry_channels_test.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/orchestration_routes.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes_test.go`
- `backend-go/services/api-gateway/internal/config/config.go`, `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/README.md`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/parity_test.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`
- `docs/crs/v6/README.md` mục 3.5 đến 3.7; CR-REQ-003, 004, 005, 007, 009 cùng thư mục cha
- `docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md`
