# IMPLEMENTATION-NOTES: gateway-and-mcp (CR-REQ-016, CR-REQ-017)

Ngày 2026-10-08, nhánh `rf/gw`. Phạm vi: `backend-go/services/api-gateway` (và xoá 3 vỏ rỗng ở `mcp-service/internal/usecase/tools`).

## Đã làm
- Gateway dial `request-service` khi có `REQUEST_SERVICE_ADDR` (token `REQUEST_INTERNAL_CALLER_TOKEN` cho unary và stream, actor-type qua `AttachIdentity`); rỗng thì không dial, kênh trả `REQUEST_UNAVAILABLE`, route HTTP không mount. Health `request-service` đăng ký khi có kết nối.
- 28 kênh WS đăng ký trong `RegisterProductionChannels` (`channels_request*.go`, `channels_solution.go`, `channels_approval.go`), thêm 4 kênh đọc: `request.links`, `request.flow`, `request.checks`, `request.planProposal`. `request.subscribe` (14 sự kiện, bảng `requestEventRegistry`) chỉ đăng ký khi NATS kết nối.
- 5 route HTTP gọi lại đúng handler kênh (registry riêng cho kênh unary) nên luật nguồn, view và mã lỗi dùng chung.
- 17 tool MCP + 3 `Declared`, loại trừ vĩnh viễn cổng của người trong `excluded_channels.yaml`, golden thêm 20 mục (tổng 260), `ToolOrigin` gắn trong `runGuarded`, hạn mức `MCP_REQUEST_CREATE_PER_HOUR` thật.
- Xoá vỏ rỗng: `adapter/http/request_{routes,webhook}.go`, `adapter/websocket/{request_client,lifecycle_channels,solution_channels}.go`, `mcp-service/.../tools/{request_flow_tools,origin_executor,create_rate_limiter}.go` (không ai tham chiếu).

## Quyết định lệch so với task
1. `request.planProposal` là kênh thêm: proto D3 làm `GeneratePlan` (propose) bất đồng bộ trả `run_id`, nên UI cần `GetPlanProposal` để đọc kết quả. Đã loại trừ khỏi MCP (chờ `request_generatePlan` ra khỏi `Declared`).
2. Không có trường `origin` trong `CreateRequestRequest` (proto không sửa): nguồn MCP đi qua `source.provider="mcp"`, `source.site=ClientName`; `SpawnChildRequest` không có trường nguồn nên chỉ dựa vào actor-type `agent` trong metadata.
3. Lỗi không mã: PermissionDenied thành `REQUEST_FORBIDDEN` (kênh approval: `REQUEST_APPROVAL_FORBIDDEN`), Unimplemented thành `REQUEST_NOT_IMPLEMENTED`, Internal/lạ thành `REQUEST_INTERNAL` với thông điệp cố định (không rò dữ liệu). Kiểm tra đầu vào của gateway dùng `INVALID_ARGUMENT:` như `task_routes.go`.
4. Tool danh sách (`request_list`, ...) đưa mảng vào `items` bằng `Post` cho khớp envelope list; `solution_list` dùng `KeepKeys` để giữ khoá snake_case trong `options` (C13).
5. `request.classify`/`request.generatePlan` giữ deadline 24 giây dù RPC đã trả sớm (D3); `request.classify` trả thêm `runId`.
6. Sự kiện không có `id`: người thường chỉ thấy sự kiện có `reporter_id` hoặc `actor_id` là mình (admin thấy hết). Hệ quả: sự kiện `approval.*` không mang hai khoá này sẽ không tới người duyệt không phải admin; UI dùng polling `approval.listPending` (CONTRACT mục 3). Cần CR-REQ-010 chốt (Q3).

## Phát hiện đáng chú ý cho agent khác
- `request-service/internal/domain/rpc_catalog.go` đặt `AgentAllowed:false` cho `GenerateSolution`, `GeneratePlan`, `StartPhase`, `CancelRequest`, `ConfirmRequestType`, `Approve`, `Reject`. Tool MCP `solution_generate` (agent) sẽ bị từ chối cho tới khi danh mục (CR-035) cho phép; cần thống nhất với SOL-017.
- request-service chưa dùng `internalcaller.Guard`; gateway đã gửi token sẵn.
- Một số RPC ở request-service còn `Unimplemented`; gateway trả `REQUEST_NOT_IMPLEMENTED` (có test bufconn).

## Chưa kiểm chứng
- Chưa chạy với request-service và NATS thật: hành vi phát lại lịch sử của `SubscribeEphemeral` (đã có lọc `OccurredAt`), tên stream `REQUEST`, wildcard.
- Hành vi OPA `session.untrusted_read` sau `request_get`; red-team CR-MCP-013.
- Hạn mức tạo theo từng replica.
- TASK-REQ-017-05 (script `tests/mcp/check_mcp_request_flow.py`, `docs/guides/mcp`, `docs/guides/mcp/admin-guide.md` cho `MCP_REQUEST_CREATE_PER_HOUR`) chưa làm: ngoài phạm vi và cần stack dev.
- `deploy/dev/docker-compose.yml` chưa thêm `REQUEST_SERVICE_ADDR` cho api-gateway.
- GitNexus `impact`/`detect_changes` không chạy (chỉ mục của cây chính, không phản ánh worktree); mọi thay đổi ở symbol có sẵn (`RegisterProductionChannels`, `ChannelDeps`, `Executor.runGuarded`, `NewExecutor`, `Config`, `dialRequestService`, `httpgateway.Deps`) đều cộng thêm, người gọi còn nguyên chữ ký (`dialRequestService` thêm tham số variadic).
- `go test -race` toàn `wscompat` đỏ ở `TestBrowserScreencastChannel_ErrorFirstFrame_FailsSynchronously` (có sẵn, không thuộc phạm vi); bộ test Request chạy `-race` xanh.
- `gofmt -l` còn liệt kê vài file có sẵn không do thay đổi này (codeintel_*, channels_git_test.go, sensitive_path_rules_test.go...).

## Câu hỏi mở
- Q3 CONTRACT (quyền xem sự kiện theo project), Q5 (`request.returned` đã ánh xạ subject riêng `orca.request.request.returned`, khớp `ReturnedPayload`).
- Có tạo tool cho `request.checks`/`request.links`/`request.flow` không (hiện loại trừ có lý do).
