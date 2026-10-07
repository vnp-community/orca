# TASK-REQ-031-07: `request-service`: client `mcp-service` và adapter nguồn `transport=mcp`

**From Solution:** BE-REQ-SOL-031 (mục E, D bước 2)
**Priority:** P1 (đợt 2)
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/mcpclient/external_source_client.go` (mới), `.../internal/adapter/sources/external_mcp.go` (mới), `.../internal/usecase/ports.go` (thêm `ExternalServerGateway`), `.../cmd/server/main.go` (sửa: dial `MCP_SERVICE_ADDR`), `.../internal/config/config.go` (sửa) và `_test.go`
**Depends on:** TASK-REQ-031-05, TASK-REQ-031-06
**Status:** `[x] DONE`

---

## Context

- `mcp-service` chỉ Postgres và giữ `ExternalServer`; `request-service` hai dialect giữ `context_sources` với `server_ref` (UUID, không FK chéo service). Hai bên nối bằng gRPC (`McpRegistryService`).
- Hai RPC của task 06 được `internalcaller.Guard` bảo vệ bằng token nội bộ `x-orca-internal-token`; `request-service` dùng `internalcaller.ClientInterceptor(token)` trên **riêng** kết nối tới `mcp-service` (chú thích của gói: chỉ gắn cho kết nối tới dịch vụ có phương thức được bảo vệ). Token cấu hình bằng `MCP_INTERNAL_TOKEN` (tên đề xuất; kiểm biến `mcp-service` dùng hiện nay, xem `cmd/server/external_wiring.go` và `internal/config`), rỗng thì không dial và nguồn `mcp` luôn `missing{not_connected}` (fail closed).
- Danh sách "đã duyệt và dùng được" của một máy chủ: `ListExternalServers` (có `status`, `tools_changed`) và `ProbeExternalServer` (có `approved_tools`) đã có; `ExternalServer` proto không có cờ `usable` sẵn, nên `Usable = status=="approved" && !tools_changed`, dựng ở client (cùng định nghĩa `domain.ExternalServer.Usable()`).
- v1 cấm tool ghi: ranh giới duy nhất là `scopes` của nguồn ∩ `approved_tools`. Nội dung nguồn **không bao giờ** được dùng để chọn tool hay tham số (CR mục 2.8).
- CR mục 2.2: nguồn `mcp` có `Trust` tối đa `medium` (task 02) và luôn bọc `<untrusted>` (Builder, task 05).

## Việc cần làm

1. Port ở `ports.go`:

```go
type ExternalServerGateway interface {
    Usable(ctx context.Context, serverID string) (bool, error)
    ApprovedToolNames(ctx context.Context, serverID string) ([]string, error)
    CallTool(ctx context.Context, serverID, tool string, argsJSON []byte, maxBytes int) (ExternalResult, error)
    ReadResource(ctx context.Context, serverID, uri string, maxBytes int) (ExternalResult, error)
}
type ExternalResult struct{ Text string; Truncated bool; SizeBytes int; Digest string; IsError bool }
```

2. `external_source_client.go`: bản cài trên `mcpv1.McpRegistryServiceClient`. `Usable` gọi `ListExternalServers{scope: ""}` rồi tìm theo `id` (có cache 5 giây theo `(tenant, serverID)` chỉ cho kết quả `Usable=true`; kết quả `false` **không** cache, để kill switch và rug-pull có hiệu lực ngay). `ApprovedToolNames` gọi `ProbeExternalServer` **không**: probe có tác dụng phụ (ghi `RecordProbe`); thay vào đó đọc `approved_tools` từ phản hồi `ListExternalServers`/`Get` nếu proto đã có; nếu proto `ExternalServer` chưa có trường danh sách tool đã duyệt, thêm `repeated ExternalToolInfo approved_tools = 21` vào message `ExternalServer` (additive) trong task 06 hoặc task này (chốt khi đọc proto, ghi vào PR). Không dùng `ProbeExternalServer` ở đường nóng.
3. Ánh xạ lỗi gRPC → miền: `PermissionDenied` kèm `MCP_TOOL_NOT_APPROVED` → `ErrScopeForbidden`; `FailedPrecondition` kèm `MCP_SERVER_NOT_USABLE` → `ErrServerNotUsable`; `ResourceExhausted` → `ErrRateLimited`; `Unavailable`, `DeadlineExceeded` → `ErrNotConnected`/`ErrSourceTimeout`.
4. `external_mcp.go`: `type ExternalMCPAdapter struct{ src domain.ContextSource; gw ExternalServerGateway }`: `Key() = src.Key`. `Search(q)`: với mỗi `tool ∈ src.Scopes` chứa `search` hoặc `list` (quy ước tên: ghi rõ trong mã là heuristic, **không** thay cho danh sách `scopes` tường minh) gọi `gw.CallTool` với `arguments_json` dựng **từ trường cấu hình của nguồn** (mẫu `{"query": <Query đã chuẩn hoá>}`; mẫu argument nằm trong `redaction`/cột mới `call_templates`? Không thêm cột ở task này: dùng `Meta` trong `scopes` dạng `"issue.search"` và hằng `queryArgName = "query"`; nếu máy chủ cần tên khác, ghi vào Rủi ro và mở CR sau). `Get(ref)`: `gw.ReadResource(uri=ref)` chỉ khi `ref` có tiền tố scheme nằm trong `scopes` (ví dụ `"resource:wiki://"`). Mỗi kết quả thành một `SourceItem` (`Trust=min(src.Trust, medium)`, `Freshness=fresh`, `Digest` do `mcp-service` trả, `Ref="<key>:<tool>"` hoặc `uri`).
5. Chống chèn: `arguments_json` chỉ chứa `Query` đã qua `NormalizeTerms` (từ khoá, không câu nguyên văn) và hằng của cấu hình; **không** nội suy bất kỳ đoạn nào của kết quả nguồn vào lời gọi kế tiếp (không có chuỗi lời gọi phụ thuộc kết quả ở v1). `max_bytes = src.MaxBytes`.
6. Hạn mức: `SourceRateLimiter` (task 05) kiểm trước khi gọi; vượt thì `ErrRateLimited` và không chạm `mcp-service`.
7. Kiểm khi `Upsert` nguồn (task 05 gọi): `gw.Usable` và `ApprovedToolNames` ⊇ `Scopes` (tool ngoài danh sách phê chuẩn: `REQUEST_SOURCE_SCOPE_FORBIDDEN`); khi chạy, kiểm lại (người duyệt có thể đã thu hồi sau đó).
8. `main.go`/`config.go`: `MCP_SERVICE_ADDR` (đã khai ở SOL-001? kiểm `config.go`, thêm nếu thiếu), `MCP_INTERNAL_TOKEN`; dial với `grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(token), metadataForwarder)`; metadata tenant (`x-orca-tenant-id`, `x-orca-user-id`) chuyển tiếp từ ctx giống mẫu `tenant_forwarding.go` của `task-service`. Địa chỉ rỗng thì dùng `NotConfiguredGateway` trả `ErrNotConnected`.
9. Sự kiện: không thêm; audit nằm ở `mcp-service` (task 06). `request-service` ghi `missing{reason}` vào pack, đủ để truy vết.

## Kiểm thử

- `external_source_client_test.go`: fake `McpRegistryServiceClient`: `Usable` true được cache 5 giây, false không cache (kiểm bằng số lần gọi); ánh xạ lỗi theo bảng.
- `external_mcp_test.go`: `TestSearch_OnlyScopedTools` (nguồn có `scopes=["issue.search"]` thì gateway chỉ nhận tool đó); `TestSearch_ArgsContainOnlyNormalizedQuery` (nội dung kết quả có chỉ dẫn gài không xuất hiện ở lời gọi nào); `TestTrustCappedAtMedium`; `TestRateLimited_NoCall`; `TestScopeRevokedAfterUpsert` (gateway báo tool không còn phê chuẩn thì `ErrScopeForbidden`).
- `build_context_pack_test.go` (mở rộng từ task 05): nguồn `mcp` với `Usable=false` cho `missing{not_connected}`; `Usable=true` cho mảnh bọc `<untrusted>`.
- Test hợp đồng `fixture` MCP giả: dựng `httptest` MCP + `mcp-service` thật bằng `testcontainers` (Postgres) ở `e2e` của CR-REQ-025 (chưa viết; ghi vào danh sách việc cho task 025-04).
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/mcpclient/... ./services/request-service/internal/adapter/sources/...`.

## Tiêu chí hoàn thành

- [x] Không có đường nào để `request-service` gọi tool ngoài `scopes` ∩ `approved_tools`.
- [x] Kết quả `Usable=false` không bao giờ được cache.
- [x] Nội dung nguồn không xuất hiện trong tham số của bất kỳ lời gọi tool nào (test).
- [x] Thiếu `MCP_SERVICE_ADDR`, `MCP_INTERNAL_TOKEN`: nguồn `mcp` thành `missing`, service vẫn khởi động.
- [x] Tool ghi bị chặn: nguồn chỉ khai tool chỉ-đọc theo tên `scopes` (người duyệt chịu trách nhiệm, ghi rõ ở README feature).

## Ví dụ tham khảo

Một dòng `context_sources` cho nguồn MCP (JSON hiển thị ở RPC quản trị):

```json
{"key": "wiki_search", "kind": "external_knowledge", "transport": "mcp", "server_ref": "<uuid ExternalServer>",
 "scopes": ["wiki.search"], "trust": "medium", "ttl_seconds": 900, "max_bytes": 32768,
 "enabled_for": {"projects": ["*"], "request_types": ["change_request"], "stages": ["solution", "plan"]}, "status": "active"}
```

Bảng ánh xạ lỗi gRPC của `mcp-service` sang lỗi miền: `PermissionDenied` + `MCP_TOOL_NOT_APPROVED` thành `ErrScopeForbidden`; `FailedPrecondition` + `MCP_SERVER_NOT_USABLE` thành `ErrServerNotUsable`; `ResourceExhausted` thành `ErrRateLimited`; `Unavailable` thành `ErrNotConnected`; `DeadlineExceeded` thành `ErrSourceTimeout`.

## Rủi ro và lưu ý

- Heuristic "tên tool chứa `search` hoặc `list`" chỉ để đặt mặc định đối số; ranh giới an toàn là `scopes` tường minh, không phải heuristic. Nếu máy chủ ngoài có tool tên lạ, cần cột mẫu đối số (CR sau).
- Cache 5 giây của `Usable=true` có thể trễ 5 giây sau khi người duyệt thu hồi (kill switch của nguồn ở `request-service` vẫn tức thì).
- Mỗi lời gọi mở phiên MCP mới ở `mcp-service` (task 06): độ trễ chưa đo.
- Chưa thử với máy chủ MCP thật nào (Jira, GitHub, CI): chỉ có fixture.
- `ListExternalServers` theo `scope` của người gọi: dịch vụ gọi không có người dùng thật; cần xác nhận quyền (`tenantCaller`) cho phép liệt kê mọi scope của tenant (kiểm khi làm task 06).
