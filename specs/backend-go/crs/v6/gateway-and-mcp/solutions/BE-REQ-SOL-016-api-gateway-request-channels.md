# BE-REQ-SOL-016: Kênh WS và route HTTP của `api-gateway` cho Request, Solution, Approval, Backlog

> ✅ Đã triển khai (kiểm chứng 2026-10-08, 8/8 task; chưa chạy trên NATS và request-service thật). Chi tiết lệch: `../IMPLEMENTATION-NOTES.md`.

**CR:** [CR-REQ-016](../../../../../../docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md)
**Service:** `api-gateway` (`internal/adapter/wscompat`, `internal/adapter/httpgateway`, `internal/config`, `cmd/server`, `internal/adapter/mcpserver/tools/excluded_channels.yaml`)
**TDD tham chiếu:** [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (AuthN, danh tính do gateway gắn), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (gRPC, subject NATS), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (log không chứa dữ liệu nhạy cảm)
**Hợp đồng frontend:** [`CONTRACT-request-ui-api.md`](../CONTRACT-request-ui-api.md) (nguồn sự thật cho tên kênh, tham số, kết quả, mã lỗi)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `wscompat/register_production.go` (struct `ChannelDeps`, hàm `RegisterProductionChannels`), `wscompat/channels_task_source.go` (mẫu `registerTaskSourceChannels`, struct view camelCase, `decodeArg[T](args, 0)`), `wscompat/channels_task_activity.go` (mẫu stream, `taskActivitySubjects`, `ephemeralSubscriber`), `wscompat/channels_mcp.go` (`mcpChannelError`), `wscompat/tool_origin_context.go` (`ToolOrigin{ClientName, MCPSessionID, UserID}`, `toolOriginFromContext` không export), `httpgateway/router.go` (mount có điều kiện `if deps.TaskClient != nil`), `httpgateway/task_routes.go`, `config/config.go` (`OtherServiceAddrs`, 14 dịch vụ), `cmd/server/main.go` (dòng 176 dial `task-service`, dòng 371 `ChannelDeps{...}`, dòng 380 `TaskActivityEnabled: natsErr == nil`), `mcpserver/tools/parity_test.go` (`TestChannelInventory`), `excluded_channels.yaml`, `common/eventbus/eventbus.go` (`Event{ID, TenantID, OccurredAt, Version, Payload}`, `SubscribeEphemeral`).

Khác hoặc bổ sung so với CR-REQ-016:

1. **Correction relative to CR-REQ-016, tên mã lỗi.** CR mục 2.8 dùng `APPROVAL_*`, `SOLUTION_NOT_FOUND`; CR-REQ-007, 009 và README v6 mục 8 dòng 5 dùng `REQUEST_APPROVAL_*`, `REQUEST_SOLUTION_*`. Giải pháp này dùng tên có tiền tố (CONTRACT mục 8 dòng 1).
2. **Correction, timeout.** `wscompat/handler.go:247` đặt `invokeTimeout = 25 * time.Second` cho mọi dispatch (và `rpc-client.ts` 30 giây), nên không kênh nào chạy quá 25 giây dù handler đặt deadline dài hơn. `GenerateSolution` bất đồng bộ (CR-REQ-007 mục 2.5, trả `{solution_id, run_id}`): kênh 8 giây. `ClassifyRequest` gọi AI 60 giây (CR-REQ-005 mục 2.2) và `GeneratePlan` propose gọi `ai.complete` đồng bộ (CR-REQ-012): cả hai vượt trần. Gateway đặt deadline 24 giây và đòi CR-REQ-005, 012 làm RPC trả sớm (CONTRACT mục 9 Q2); đây là điểm chặn thiết kế.
3. **Backlog.** CR-REQ-015 chốt **một** RPC `ListBacklog` với `view` (CR-016 Q4 đã được trả lời), nên ba kênh `backlog.*` dùng chung một hàm.
4. **`request.subscribe` có nguy cơ phát lại lịch sử.** `SubscribeEphemeral` gọi `stream.CreateOrUpdateConsumer` với `ConsumerConfig` chỉ có `FilterSubject`, `AckPolicy`, `InactiveThreshold`: không đặt `DeliverPolicy`, mặc định JetStream là giao từ đầu stream. Chưa chạy thử; nếu đúng thì mỗi lần mở socket nhận lại sự kiện cũ. Giải pháp lọc ở gateway theo `OccurredAt` (mục 2.5).
5. **Địa chỉ rỗng.** `gatewaygrpc.Dial("")` (`internal/adapter/grpc/dial.go:31`) gọi `grpc.NewClient("")`; chưa kiểm chứng nó có lỗi khi rỗng. Giải pháp không dial khi `REQUEST_SERVICE_ADDR` rỗng và truyền client `nil`, kênh trả `REQUEST_UNAVAILABLE` (mục 2.1).
6. `request-service` chưa tồn tại (`ls backend-go/services/request-service` không có). Proto `orca.request.v1` do CR-REQ-001 tạo; mọi tên message trong tài liệu này lấy từ CR-REQ-004 đến 015, chưa biên dịch.
7. `mcpserver/tools/` đã có `parity_test.go`, `excluded_channels.yaml`, `spec_builders.go` đúng như CR mô tả. `tests/mcp/` đã tồn tại và có `check_mcp_ws_channels.py`.

## 2. Giải pháp

### 2.1 Wiring client

| File | Việc |
|---|---|
| `internal/config/config.go` | thêm `"request-service": commonconfig.StringEnv("REQUEST_SERVICE_ADDR", "")` vào `OtherServiceAddrs` |
| `cmd/server/main.go` | khối cạnh `task-service`: nếu địa chỉ khác rỗng thì `gatewaygrpc.Dial`, `requestv1.NewRequestServiceClient`, `requestv1.NewApprovalServiceClient`, `healthSrv.Register("request-service", grpcConnHealthCheck(conn))`; rỗng thì để `nil` |
| `wscompat/register_production.go` | thêm `Request requestv1.RequestServiceClient`, `Approval requestv1.ApprovalServiceClient` vào `ChannelDeps`; gọi `registerRequestChannels(r, d.Request, d.Approval, d.TaskActivityBus, d.RequestChannelConfig)` ở cuối `RegisterProductionChannels` |
| `wscompat/channels_request_unavailable.go` (mới) | `errRequestUnavailable = errors.New("REQUEST_UNAVAILABLE: request service not configured")`; mỗi handler kiểm `client == nil` đầu hàm |

Client `nil` vẫn đăng ký kênh (closure), để `productionRegistry()` của parity test dựng được inventory.

### 2.2 Cây file mới trong `wscompat` (mới)

```
channels_request.go            # registerRequestChannels + request.* vòng đời, phân loại, plan, phase
channels_request_views.go      # requestView, solutionView, analysisRunView, approvalView, typeHistoryView, backlog*View
channels_request_errors.go     # requestChannelError, errRequestUnavailable
channels_request_source.go     # luật nguồn Request (mục 2.6)
channels_solution.go           # solution.*
channels_approval.go           # approval.*
channels_request_backlog.go    # backlog.* (một hàm listBacklog(view))
channels_request_stream.go     # request.subscribe + requestEventRegistry
channels_request_flow.go       # request.flowStatus, request.flowSet
channels_request_extras.go     # request.links, request.flow, request.checks (P2, mục 2.8)
*_test.go                      # fake client theo mẫu fakeOrchestrationClient
```

Quy tắc view: không trả message proto; luôn qua struct có tag `json:"camelCase"` (BUG-023 ở `channels.go`). `solutionView.Options` là `json.RawMessage` lấy từ `options_json`, không đổi khoá (CONTRACT C13). `chosenOptionId` được suy từ `chosen_option` bằng cách đọc `options.options[i].id`; lỗi parse thì bỏ trường, không hỏng cả kênh.

### 2.3 Lỗi

```go
// requestChannelError đưa mọi lỗi kênh Request về "<CODE>: <message>" (CONTRACT C4).
func requestChannelError(err error) error // mẫu mcpChannelError: bỏ "rpc error: code = X desc = ",
                                          // giữ nếu khớp ^REQUEST_[A-Z0-9_]+: ; Unavailable -> REQUEST_UNAVAILABLE;
                                          // còn lại giữ message ngắn, cắt 300 ký tự
```

Test bắt buộc: lỗi không chứa `body` (đưa chuỗi đánh dấu `SECRET-BODY-MARKER` vào `body`, ép RPC lỗi, khẳng định chuỗi không xuất hiện trong `err.Error()`). `body` không vào `sensitiveArgChannels` (không phải secret) nhưng cũng không vào thông điệp lỗi hay span.

### 2.4 Bảng kênh và timeout

Tên, tham số, kết quả: xem CONTRACT mục 2. Mỗi handler: `decodeArg`, chỉ kiểm phía gateway những gì bảo vệ hợp đồng (enum không hợp lệ, luật nguồn); thiếu tham số bắt buộc để `request-service` trả mã của nó (Q1), `context.WithTimeout`, gọi RPC, đổi sang view. Hằng số:

```go
const (
    requestRPCTimeout        = 8 * time.Second   // tái dùng groupRPCTimeout (cùng package, channels_issuetracking_orchestration.go:27)
    requestCommitPlanTimeout = 15 * time.Second
    requestStartPhaseTimeout = 15 * time.Second
    requestAIChannelTimeout  = 24 * time.Second  // classify, generatePlan(propose): dưới invokeTimeout 25s
)
```

Ánh xạ tham số khác tên proto: `toType` thành `new_type`; `id` thành `request_id`, `approval_id`; `optionId` thành `option_id`; `linkReason` thành `link_reason`; `typeHint` thành `type_hint`; `analysisMode` ∈ `complete|agent_readonly` thành enum `COMPLETE|AGENT_READONLY` (rỗng là 0). `mode` của `generatePlan` ∈ `propose|commit` thành `PlanMode` (rỗng là `PROPOSE`). Giá trị không nằm trong enum: lỗi `INVALID_ARGUMENT` ở gateway trước khi gọi RPC.

### 2.5 `request.subscribe`

```go
type requestEventMapping struct{ Stream, Subject, EventType string }
var requestEventRegistry = []requestEventMapping{
    {"REQUEST", "orca.request.request.created", "request.created"},
    // ... một dòng cho mỗi sự kiện ở CONTRACT mục 3
}
```

Hiện thực theo `RegisterTaskActivityStreamChannel`: `RegisterStream`, mỗi dòng registry một goroutine `SubscribeEphemeral`. Các điểm khác:
- Kiểm quyền: có `id` thì gọi `GetRequest` một lần (lỗi thì trả lỗi, không mở stream). Không `id`: lọc `ev.TenantID == id.TenantID`, và cho phép nếu người dùng là admin; người dùng thường chỉ nhận sự kiện của Request mình báo cáo hoặc có quyền đọc (cách kiểm cho từng sự kiện chưa chốt, Q3; mặc định an toàn: không `id` thì chỉ nhận sự kiện có `reporter_id` hoặc `actor_id` bằng mình hoặc admin).
- Bỏ sự kiện `ev.OccurredAt` trước `subscribedAt - 2s` (nguy cơ phát lại, mục 1 điểm 4).
- Khung `RequestEventFrame` chỉ mang trường whitelist của CONTRACT mục 1; không bao giờ sao chép `payload` nguyên.
- Đăng ký kênh chỉ khi `TaskActivityEnabled` (NATS kết nối), dùng lại `TaskActivityBus`.
- Tên stream `REQUEST` lấy từ CR-REQ-024 mục 2.1 ("chưa kiểm chứng tên stream CR-REQ-001 chọn"): đặt hằng `requestStreamName` và đọc từ cấu hình nếu CR-001 đổi.

### 2.6 Nguồn Request (`channels_request_source.go`)

```go
func resolveRequestSource(ctx context.Context, in createSourceArgs) (*requestv1.RequestSource, *requestv1.RequestOrigin, error)
// ToolOrigin trong ctx  -> provider="mcp", site=ClientName, origin{kind:"mcp", client_name, mcp_session_id}
// provider in {jira,github,gitlab,linear} && ref!="" -> giữ nguyên
// provider rỗng -> "manual"
// provider in {mcp,webhook,manual} tường minh -> REQUEST_SOURCE_FORBIDDEN
```

`toolOriginFromContext` không export nhưng cùng package `wscompat`, nên dùng trực tiếp. Trường `origin` ở `CreateRequestRequest` do CR-REQ-004 thêm (CR-016 Q2); trước khi có, bỏ qua `origin` và để audit của CR-REQ-024 chịu thiếu `actor_type=agent`.

### 2.7 Route HTTP (`httpgateway/request_routes.go`, mới)

Năm route theo CONTRACT mục 4, gắn trong `router.go`: `if deps.RequestClient != nil && deps.ApprovalClient != nil { mountRequestRoutes(authed, deps.RequestClient, deps.ApprovalClient) }` cạnh `mountTaskRoutes`. Thêm hai field vào `Deps`. Dùng `writeGRPCError` (`usage_routes.go:182`). `tenant_id` không đọc từ body. `POST /v1/requests` dùng cùng `resolveRequestSource`-logic (tách hàm thuần để cả hai đường gọi, đặt trong `wscompat` hoặc gói nhỏ `requestsource` để tránh `httpgateway` import `wscompat`; xem Q4).

### 2.8 Kênh bổ sung (P2)

`request.links`, `request.flow`, `request.checks` (CONTRACT mục 2.5) chỉ làm khi mục 9 Q1 của CONTRACT được chốt; không chặn P0. File `channels_request_extras.go`.

### 2.9 Loại trừ khỏi MCP tạm

Thêm mọi kênh mới vào `excluded_channels.yaml`, mỗi dòng `pattern` đúng tên kênh (không dùng `request.*` để CR-017 gỡ từng kênh), `category: request_flow`, `reason: "Chờ BE-REQ-SOL-017 (CR-REQ-017)"`. Danh sách loại trừ vĩnh viễn nằm ở BE-REQ-SOL-017.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Plan, Phase đi qua `request.generatePlan`, `request.startPhase`; cây đọc bằng `task.getSubtree` | README v6 không có `plan.*`; Plan, Phase là Task (D2 README) |
| D2 | Hai kênh gọi AI đặt 24 giây, dưới `invokeTimeout` 25 giây, và đòi RPC trả sớm | Không thể vượt trần WS mà không sửa `handler.go` và `rpc-client.ts` |
| D3 | `approval.approve/reject` bắt buộc `expectedVersion` và `expectedDigest` | `DecideApprovalRequest` (CR-REQ-009) |
| D4 | Không kênh `approval.request` | Approval do máy trạng thái sinh |
| D5 | `request.subscribe` dùng ephemeral như `task.activity.subscribe`, lọc tenant và thời điểm ở gateway | Có mẫu chạy; không thêm cơ chế thứ ba |
| D6 | `requestEventRegistry` là bảng dữ liệu | CR 026 đến 036 thêm sự kiện bằng một dòng |
| D7 | Client `nil` vẫn đăng ký kênh | Parity test cần inventory đầy đủ không có service thật |

## 4. Phụ thuộc và thứ tự

Cần proto `orca.request.v1` đã sinh (CR-REQ-001) và message của CR-REQ-004 đến 009. Có thể bắt đầu từ TASK-REQ-016-01 khi proto `GetRequest`, `ListRequests` có (CR-001); các kênh còn lại thêm theo từng CR backend (mỗi nhóm kênh một PR). `backlog.*` cần CR-REQ-015. Mở khoá BE-REQ-SOL-017 và toàn bộ frontend v6 (CR-018 đến 023). Chi tiết ở [`../tasks/README.md`](../tasks/README.md).

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `channels_request_test.go` | từng kênh với fake client: ánh xạ tham số (bảng 2.4), camelCase, lỗi, deadline đúng 8s, 15s, 24s (đọc `ctx.Deadline()` trong fake) |
| `channels_request_source_test.go` | bảng 2.6 gồm giả mạo `mcp`, `webhook`, `manual` |
| `channels_request_errors_test.go` | `requestChannelError`, lỗi không chứa `body`, `Unavailable` thành `REQUEST_UNAVAILABLE` |
| `channels_solution_test.go`, `channels_approval_test.go` | `expectedVersion`, `expectedDigest`, `comment` bắt buộc; `listPending` dùng `Identity`; `solution.generate` trả `runId` |
| `channels_request_backlog_test.go` | `view` đúng theo kênh, phân trang, nil-slice thành `[]` |
| `channels_request_stream_test.go` | lọc tenant, lọc `id`, bỏ sự kiện cũ, không rò sự kiện tenant khác, khung không có `body` |
| `request_routes_test.go` | 5 route, 400, 401, ánh xạ lỗi (mẫu `task_routes_test.go`) |
| `registry_channels_test.go` (sửa) | inventory có đủ kênh mới |
| `go test ./internal/adapter/mcpserver/tools/...` | `TestChannelInventory` xanh nhờ mục loại trừ |
| Quét camelCase | duyệt JSON kết quả của mọi kênh, không khoá nào chứa `_` ngoài `options` (C13) |
| Không nhận `tenantId` | mỗi kênh ghi nhận gửi `tenantId` giả, khẳng định RPC nhận từ `Identity` |

Lệnh: `cd backend-go/services/api-gateway && go test ./internal/adapter/wscompat/... ./internal/adapter/httpgateway/... ./internal/adapter/mcpserver/tools/...`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Phát lại lịch sử của ephemeral consumer (mục 1 điểm 4): chưa chạy thử trên NATS thật; nếu đúng và tenant có nhiều sự kiện, kênh không `id` gây tải. Giảm nhẹ bằng lọc `OccurredAt`; sửa tận gốc cần thêm `DeliverPolicy: DeliverNew` vào `common/eventbus` (chạm mọi service, ngoài phạm vi).
- Gateway không có OPA; lỗ hổng quyền ở `request-service` lộ ra ngay qua WS, HTTP. Cần test quyền phía service (CR-REQ-009, 010).
- Quy tắc xem sự kiện khi không có `id` chưa rõ (Q3).
- `SubscribeEphemeral` có hỗ trợ subject wildcard hay không chưa kiểm chứng; giải pháp liệt kê từng subject nên không phụ thuộc.
- Mỗi socket thêm N consumer NATS (N = số dòng registry, hiện 14); chưa đo tải. Có thể giảm bằng một consumer dùng chung có fan-out trong tiến trình (việc sau).
- SSH, remote: gateway không chạm máy dev; `request.startPhase` chỉ gọi RPC, việc chạy agent đi qua `task.execute` nên giữ hành vi SSH hiện có.

## 7. Câu hỏi mở

1. Mã lỗi gateway khi thiếu tham số bắt buộc: dùng `INVALID_ARGUMENT: ...` như `task_routes.go` hay mã `REQUEST_*` riêng? Mặc định: bỏ cho `request-service` kiểm và trả mã của nó.
2. Có cần `REQUEST_ID_REQUIRED` không (CR-REQ-005 có `REQUEST_NOT_FOUND`)? Mặc định không.
3. Người dùng thường có nghe được sự kiện tenant-wide không, và kiểm quyền thế nào mà không gọi `GetRequest` cho từng sự kiện? Cần CR-REQ-010 chốt chính sách đọc theo project.
4. Hàm luật nguồn dùng chung cho WS và HTTP đặt ở đâu (gói nhỏ `internal/usecase/requestsource` hay trong `wscompat`)? Mặc định: `internal/usecase/requestsource.go` (mới).
5. Ba kênh bổ sung (mục 2.8) có làm trong cùng CR không.

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/wscompat/register_production.go`, `.../channels_task_source.go`, `.../channels_task_activity.go`, `.../channels_mcp.go`, `.../tool_origin_context.go`, `.../registry.go`, `.../registry_channels_test.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, `.../task_routes.go`, `.../task_routes_test.go`, `.../usage_routes.go`
- `backend-go/services/api-gateway/internal/config/config.go`, `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/parity_test.go`, `.../excluded_channels.yaml`
- `backend-go/common/eventbus/eventbus.go`
- CR: [CR-REQ-004](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md), 005, 006, 007, 009, 012, 013, 015, 025; [README v6](../../../../../../docs/crs/v6/README.md) mục 8
