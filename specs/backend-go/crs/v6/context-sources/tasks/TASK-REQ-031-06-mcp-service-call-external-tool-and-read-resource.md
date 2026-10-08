# TASK-REQ-031-06: `mcp-service`: RPC `CallExternalTool` và `ReadExternalResource`

**From Solution:** BE-REQ-SOL-031 (mục E)
**Priority:** P1 (đợt 2, không chặn đợt 1)
**Service:** `mcp-service`, `proto`
**File:** `backend-go/proto/orca/mcp/v1/external_server.proto` (sửa), `backend-go/services/mcp-service/internal/usecase/external_server_client.go` (mới), `.../internal/usecase/external_server_ports.go` (sửa: thêm `ToolCaller`), `.../internal/adapter/mcpprober/caller.go` (mới), `.../internal/adapter/grpc/registry_server.go` (sửa), `.../cmd/server/external_wiring.go` (sửa), `.../internal/domain/audit_event.go` (sửa: hằng audit), `.../internal/domain/external_server_errors.go` (sửa) và `_test.go` tương ứng
**Depends on:** không (độc lập với phần `request-service`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: `cd backend-go/services/mcp-service && go test ./...`; `go test -tags=integration ./internal/adapter/postgres/ -run ExternalServers`; `buf breaking --path orca/mcp` xanh)

---

## Context

- Đã đọc: `mcpprober/prober.go` (404 dòng). `Prober.ListTools` mở phiên (`initialize` → `notifications/initialized` → `tools/list` có phân trang → `close`), `TotalTimeout = 15s`, `MaxBodyBytes = 1<<20`, `session.call(ctx, id, method, params)` nhận mọi method JSON-RPC, `readRPCResult` đọc cả JSON lẫn `text/event-stream` và giới hạn `MaxBodyBytes+1`. Dialer đã resolve-then-pin và chặn SSRF (`domain.ValidateExternalURL`, `errRedirect` không theo redirect). Phiên chỉ gửi header lấy từ `Headers` đã cấu hình (không chuyển token của người gọi, `forbiddenOutbound` loại `host`, `cookie`, `content-length`, ...).
- `usecase/external_server_registry.go`: `Probe` dùng `userCaller(ctx)` (cần user), tải `GetExternalServer`, `observe` lấy header từ `SecretBroker.Get` rồi `Zero()` sau khi dùng. Đây là khuôn để theo.
- `registryInternalMethods` (`cmd/server/external_wiring.go:28`) hiện chỉ có `ResolveAgentMcpConfig`: danh sách RPC chỉ service nội bộ gọi. Hai RPC mới phải thêm vào đây (guard `internalcaller`), vì người gọi là `request-service`, không phải người dùng.
- Không có người dùng trong ngữ cảnh gọi: tenant lấy từ metadata chuẩn (`grpcmw.TenantExtractionInterceptor` đặt tenant, user từ `request-service` chuyển tiếp); `userCaller` đòi `UserID`, nên dùng biến thể `tenantCaller(ctx)` chỉ cần tenant (viết ở `caller_identity.go`, kèm test).
- `ToolInfo` không có chú thích chỉ-đọc; v1 chỉ chấp nhận tool thuộc `ApprovedTools` (được người duyệt phê chuẩn) và `request-service` đã giao với `scopes`.

## Việc cần làm

1. Proto (additive, giữ số trường tăng dần, không đổi message hiện có): thêm hai RPC vào `McpRegistryService` và bốn message như solution mục E; trường `max_bytes` 0 nghĩa là mặc định 65 536, tối đa 1 048 576; `arguments_json` ≤ 65 536 byte. Chạy `buf generate` theo quy trình repo (xem `backend-go/Makefile`), commit mã sinh nếu repo commit.
2. `domain`: hằng `AuditActionExternalCall = "mcp.external.call"`, `AuditActionExternalRead = "mcp.external.read"`; lỗi `ErrToolNotApproved` (mã `MCP_TOOL_NOT_APPROVED`, `PermissionDenied`), `ErrResultTooLarge` (`MCP_RESULT_TOO_LARGE`, `ResourceExhausted`; chỉ dùng khi **không** cho cắt), dùng lại `MCP_SERVER_NOT_USABLE` hiện có (kiểm tên hằng thật trong `external_server_errors.go` trước khi thêm, không tạo trùng).
3. `external_server_ports.go`: thêm

```go
type CallTarget struct {
    URL     string
    Headers map[string]domain.SecretValue
}
type CallResult struct {
    Text      string
    IsError   bool
    Truncated bool
    SizeBytes int
}
type ToolCaller interface {
    CallTool(ctx context.Context, t CallTarget, tool string, argsJSON []byte, maxBytes int) (CallResult, error)
    ReadResource(ctx context.Context, t CallTarget, uri string, maxBytes int) (ResourceResult, error)
}
```

4. `mcpprober/caller.go`: `func (p *Prober) CallTool(...)` và `ReadResource(...)` trên cùng `Prober` (dùng lại dialer ghim). `const CallTimeout = 20 * time.Second` (không dùng `TotalTimeout`). Trình tự: `ValidateExternalURL`, mở `session`, `initialize`, `notify initialized`, `tools/call {name, arguments}` (hoặc `resources/read {uri}`), `close`. Đọc `result.content[]`: chỉ giữ phần tử `type=="text"` (và `resource.text` cho `resources/read`), nối bằng `\n`, cắt tại `maxBytes` ở biên rune, `Truncated=true`; phần tử khác (image, audio, blob) bị bỏ và đếm. `isError` của máy chủ ngoài được trả nguyên, không coi là lỗi của Orca. Tổng thân đọc ≤ `MaxBodyBytes` (đã có trong `readRPCResult`).
5. `usecase/external_server_client.go`: `type ExternalServerClient struct{repo ExternalServerRepository; broker SecretBroker; caller ToolCaller; audit ...; clock Clock}` và hai hàm `CallTool(ctx, in CallToolInput) (CallToolOutput, error)`, `ReadResource(...)`. Trình tự kiểm (fail closed, dừng ở lỗi đầu tiên): `tenantCaller`; `uuid.Parse(server_id)` (sai: `domain.ErrNotFound()`); `repo.GetExternalServer`; `Transport == http` (stdio: `MCP_SERVER_NOT_USABLE`); `s.Usable()`; `tool` ∈ `s.ApprovedTools` (so theo `Name`); `arguments_json` là JSON hợp lệ ≤ 64 KB và `domain.ValidateEgressArgs(raw, SecretRedactor{})` (không để bí mật gửi ra máy chủ ngoài); `ReadResource`: `uri` có scheme, độ dài ≤ 2048, không có ký tự điều khiển. Lấy header từ `broker.Get` (cùng cách `observe`) và `defer Zero()`.
6. Kiểm rug-pull: nếu `s.ToolsChanged()` thì `Usable()` đã false; không thăm dò lại trong lời gọi (một lời gọi không được tự làm mới digest).
7. Audit (outbox hiện có của mcp-service, mẫu `domain.NewAdminAuditEvent`): `AuditActionExternalCall` với metadata `{tool, server_id, bytes, truncated, is_error}` (**không** có `arguments` và **không** có nội dung kết quả); `outcome=denied` cho từ chối kiểm tra (metadata thêm `reason`). `ReadResource` ghi `{uri_digest, bytes}` (băm `uri`, không ghi `uri` thô vì có thể chứa token trên query).
8. `registry_server.go`: hai handler `CallExternalTool`, `ReadExternalResource`: không ghi log request (chứa `arguments_json`, giống `SetExternalServerSecret` đã tránh log); ánh xạ lỗi bằng `toStatus`.
9. `external_wiring.go`: thêm `"/orca.mcp.v1.McpRegistryService/CallExternalTool"` và `".../ReadExternalResource"` vào `registryInternalMethods`; dựng `ExternalServerClient` từ `repo`, `broker`, `prober`.
10. Cập nhật `services/mcp-service/README.md` mục "real vs stub" một dòng cho hai RPC (không viết tài liệu thêm).

## Kiểm thử

- `mcpprober/caller_test.go`: máy chủ MCP giả `httptest` (JSON-RPC thật): `TestCallTool_TextOnly` (bỏ phần tử `image`), `TestCallTool_TruncatesAtRuneBoundary` (tiếng Việt), `TestCallTool_OversizedBodyFails`, `TestCallTool_RedirectRefused`, `TestCallTool_SSRFBlocked` (URL `127.0.0.1`, `169.254.169.254`; dùng lại bộ test chặn của `prober_test.go`), `TestCallTool_TimeoutIs20s` (đồng hồ giả hoặc server ngủ vượt, dùng `CallTimeout` có thể ghi đè trong test).
- `external_server_client_test.go`: bảng từ chối: chưa duyệt (`pending_review`), `disabled`, `ToolsChanged()`, `stdio`, tool ngoài `ApprovedTools`, `arguments_json` chứa `ghp_...` (`ErrEgressSecret`), `server_id` không phải UUID, server của tenant khác (`ErrNotFound`); thành công ghi đúng một audit không có nội dung (`TestAuditHasNoArgumentsOrContent`).
- `registry_server_test.go`: hai method nằm trong `registryInternalMethods` và bị `internalcaller.Guard` từ chối khi thiếu token.
- `external_server_integration_test.go` (Postgres): server của tenant A không gọi được bằng ngữ cảnh tenant B (RLS thật).
- Lệnh: `cd backend-go/services/mcp-service && go test ./... && go test -tags=integration ./internal/adapter/postgres/ -run ExternalServers`; `cd backend-go/proto && buf breaking --against '../../.git#ref=HEAD,subdir=backend-go/proto' --path orca/mcp`. Đã chạy 2026-10-07, PASS.

## Tiêu chí hoàn thành

- [x] `CallExternalTool` từ chối khi server không `Usable()`, tool ngoài `ApprovedTools`, `transport=stdio`, vượt `max_bytes` (cắt, `truncated=true`) hoặc `arguments_json` chứa bí mật.
- [x] Mỗi lời gọi có bản ghi audit không chứa `arguments` hay nội dung.
- [x] Hai RPC chỉ gọi được bằng token nội bộ.
- [x] SSRF: IP nội bộ, redirect đều bị chặn (dùng lại test hiện có).
- [x] `buf breaking` xanh.

## Rủi ro và lưu ý

- Đường `tools/call` chưa có tiền lệ trong repo và chưa thử với máy chủ MCP thật; một số máy chủ yêu cầu phiên giữ nhiều lời gọi (`Mcp-Session-Id`): mỗi RPC mở phiên mới, tốn thêm một vòng `initialize`. Chấp nhận ở v1, đo sau.
- `ApprovedTools` là ảnh chụp lúc duyệt; không có nhãn chỉ-đọc, nên một tool "ghi" bị người duyệt phê chuẩn nhầm vẫn gọi được. Giảm thiểu: `request-service` chỉ gọi tool trong `scopes` của nguồn đã khai (task 07).
- `MaxBodyBytes` 1 MiB giới hạn cả đường đọc tài nguyên lớn; chủ ý.
- Thay đổi `registry_server.go` chạm cùng file với handler hiện có: chạy `gitnexus_impact` cho `RegistryServer` trước khi sửa (quy ước repo).

## Tiến độ (2026-10-07)

Đã làm đủ mọi việc 1 đến 10. Điều đã kiểm chứng và lệch so với mô tả:

- Có sẵn `usecase/call_external_tool.go` là stub (`nil, nil`, không ai gọi: đã kiểm bằng grep); đã xóa và thay bằng `external_server_client.go`.
- `MCP_SERVER_NOT_USABLE` chưa tồn tại trong mã (chỉ có `MCP_SERVER_NOT_APPROVED`): thêm `CodeServerNotUsable` (FailedPrecondition), cùng `CodeToolNotApproved`, `CodeResultTooLarge` (hàm `ErrResultTooLarge` có nhưng v1 luôn cắt nên chưa dùng).
- `CallTimeout` là hằng 20s; ghi đè trong test qua `Config.CallTimeout` (trường mới, mặc định 0 = dùng hằng).
- Kết quả trả về được che bí mật (`domain.SecretRedactor`) rồi cắt lại theo `max_bytes`; `digest` là sha256 của văn bản đã che.
- `resources/read` đọc `result.contents[].text` theo đặc tả MCP (không phải `content[]`); blob bị bỏ. `tools/call` nhận `text` và `resource.text`.
- Audit qua `OutboxWriter.EnqueueOutbox` (best effort, sau lời gọi), `actor_type` lấy từ `tenant.ActorType(ctx)`; hàm mới `domain.NewExternalCallAuditEvent` vì `NewAdminAuditEvent` cố định `actor_type=user`. Từ chối ghi `outcome=denied` + `reason`; lỗi upstream ghi `outcome=error`.
- Đã thêm `tenantCaller` (chỉ cần tenant) ở `caller_identity.go`; `uri` chứa userinfo hoặc bí mật bị từ chối.
- Test: `mcpprober/caller_test.go` (HTTP/TLS giả chạy trong test), `usecase/external_server_client_test.go`, `cmd/server/registry_internal_guard_test.go` (thay cho `registry_server_test.go`, vì guard nằm ở `cmd/server`), integration Postgres `TestExternalServers_CallExternalToolRespectsTenantRLS`. `memRepo` (fake test) được sửa để chụp `ApprovedTools` khi duyệt như SQL.
- Người gọi đã kiểm: `NewRegistryServer` (một nơi gọi, `external_wiring.go`; tham số `client` mới, nil an toàn), `McpRegistryService*` ở api-gateway (`wscompat/channels_mcp*`, `mcp_external_server_wiring.go`) vẫn build và test xanh.

Chưa kiểm chứng: máy chủ MCP bên thứ ba thật (phiên giữ `Mcp-Session-Id`, định dạng SSE khác), audit của MySQL (mcp-service chỉ Postgres), `gitnexus_impact` (không chạy được MCP trong phiên này; thay bằng grep tên đầy đủ).
