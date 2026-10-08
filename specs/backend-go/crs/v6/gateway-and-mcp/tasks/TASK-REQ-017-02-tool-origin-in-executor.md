# TASK-REQ-017-02: Gắn `ToolOrigin` trong `Executor` cho tool Request và test không có trường định danh

**From Solution:** BE-REQ-SOL-017
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/executor.go`, `.../request_flow_origin.go` (mới), `.../executor_test.go`, `.../schema_no_identity_test.go` (mới)
**Depends on:** TASK-REQ-017-01, TASK-REQ-016-02 (`resolveRequestSource`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- `wscompat.WithToolOrigin` chỉ được gọi ở `composite_env.go:45` cho tool terminal và agent (`ts.pty.clientName`, `ts.pty.id`, `ts.pty.userID`). `Executor.runGuarded` (`executor.go:277`) không gắn nó. CR-REQ-017 mục 2.3 giả định "đã có" (SOL-017 mục 1 điểm 1).
- `Guards.SessionID`, `Guards.ClientName` (`executor.go:64-69`) lấy sự kiện phiên đã xác minh từ `ctx`; `mcpserver.Principal{TenantID, UserID, ...}` (`ports.go:16`).
- Luật nguồn ở gateway: `channels_request_source.go` (TASK-REQ-016-02) đọc `toolOriginFromContext`.

## Việc cần làm

1. `request_flow_origin.go`: `func isRequestFlowNamespace(ns string) bool` cho `request`, `solution`, `approval`, `backlog`. `func (e *Executor) withRequestOrigin(ctx context.Context, p mcpserver.Principal, spec *ToolSpec) context.Context` đặt `ToolOrigin{ClientName, MCPSessionID, UserID: p.UserID}` khi namespace khớp và `Guards.SessionID`, `ClientName` khác nil; ngược lại trả `ctx` nguyên.
2. `executor.go`, `runGuarded`: trước nhánh dispatch (và sau limiter), `ctx = e.withRequestOrigin(ctx, p, spec)`. Không đổi hành vi tool khác.
3. `ClientName` rỗng: dùng `"unknown-mcp-client"` để `source_site` không rỗng; ghi chú vì sao (hiển thị, không nằm trong khoá idempotent).
4. `schema_no_identity_test.go`: duyệt `AllSpecs()` lọc namespace Request, khẳng định schema đầu vào không có thuộc tính `tenant_id`, `user_id`, `reporter_id`, `source_provider`, `source_site`.

## Kiểm thử

- `executor_test.go` (mở rộng): dùng `Dispatcher` giả ghi nhận `ctx`; gọi `request_create` thì `ctx` có `ToolOrigin` đúng `ClientName`, `MCPSessionID`, `UserID`; gọi `task_get` thì không có; input chứa `source_provider: "jira"` bị bỏ (schema từ chối trường lạ hoặc bỏ qua) và RPC nhận `mcp`.
- Kết hợp với `channels_request_source_test.go`: end-to-end mức đơn vị từ executor đến RPC giả.
- `go test ./internal/adapter/mcpserver/tools/... ./internal/adapter/wscompat/...`.

## Tiêu chí hoàn thành

- [x] `request_create` qua MCP tạo Request giả có `source_provider=mcp`, `source_site=<ClientName>`.
- [x] Tool không thuộc nhóm Request không bị đổi.
- [x] Test quét schema xanh.

## Rủi ro và lưu ý

- Nếu `Guards.SessionID` chưa được nối ở một đường khởi tạo executor (test hoặc `mcpservertest`), origin rỗng; kiểm tra các nơi gọi `WithGuards`.
- Không lấy `UserID` từ input; chỉ từ `Principal`.

## Ghi chú triển khai (2026-10-08)

`ToolOrigin` gắn trong `runGuarded` cho namespace request, solution, approval, backlog.
