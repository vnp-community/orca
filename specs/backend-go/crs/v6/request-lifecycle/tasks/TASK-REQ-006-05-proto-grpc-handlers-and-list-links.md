# TASK-REQ-006-05: Proto và handler gRPC: `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks`

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `proto`, `request-service`
**File:** `proto/orca/request/v1/request.proto` (sửa), `proto/gen/go/orca/request/v1/*` (sinh lại), `internal/usecase/list_request_links.go`, `internal/adapter/grpc/server.go`, `internal/adapter/grpc/request_mapper.go`, `cmd/server/main.go` (sửa)
**Depends on:** TASK-REQ-006-02, 006-03, 006-04
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run RPC (13+ kịch bản gRPC, máy trạng thái thật); go test ./internal/adapter/grpc; buf lint/breaking --path orca/request/v1/request.proto`)

---

## Context

CR-REQ-006 mục 2.2 định nghĩa message; `ListRequestLinks` do README mục 8 điểm 12 bổ sung (frontend CR-REQ-019 cần cha và con). `Request.returned_category = 25` (số dành sẵn). `ListBacklog` thuộc CR-REQ-015; để kiểm thử dùng `ListRequests(status=request_backlog)`. Danh tính người dùng từ metadata (`tenant.UserID`), `ActorKind=user` (MCP đi qua gateway nên cũng là user với chính sách riêng, CR-REQ-017).

## Việc cần làm

1. `request.proto`: thêm `ReturnToBacklogRequest{request_id, stage, category, reason, expected_version}`, `ReopenRequestRequest{request_id, note, expected_version}`, `CancelRequestRequest{request_id, reason, expected_version}`, `SpawnChildRequestRequest{parent_request_id, link_reason, title, body, type_hint, client_request_id}`, `SpawnChildRequestResponse{child, created}`, `ListRequestLinksRequest{request_id}`, `ListRequestLinksResponse{parents, children}`, `RequestLink{parent_request_id, child_request_id, reason, created_at}`; ba lệnh đầu trả `*Response{request}`; `Request` thêm `string returned_category = 25;`. Thêm năm RPC. Sinh lại stub.
2. `list_request_links.go`: `Execute(ctx, requestID)`: `tenant.RequireTenantID`; xác nhận Request tồn tại; trả `ListParents` và `ListChildren`.
3. `server.go`: năm handler; ánh xạ `stage`, `category` bằng `ParseReturnStage`, `ParseReturnCategory`; lỗi qua `apperrors.ToGRPCStatus`; `SpawnChildRequest` đặt `Provider = manual` (đường MCP do CR-REQ-017 quyết định provider `mcp` bằng cách khác: ghi chú trong proto, không có trường provider trên dây).
4. `request_mapper.go`: `returned_category` trong `toProtoRequest`.
5. `main.go`: lắp các use case.
6. README service: bảng real vs stub.

## Kiểm thử

- `TestServer_ReturnToBacklog_RoundTrip`, `_ReopenRequest_RoundTrip`, `_CancelRequest_Twice` (hai lần thành công), `_SpawnChildRequest_Idempotent`, `_ListRequestLinks_ParentsAndChildren`.
- `TestServer_ReturnToBacklog_InvalidStage` (`InvalidArgument`, `REQUEST_RETURN_STAGE_INVALID`).
- Hợp đồng: `buf lint`, `buf breaking`; test sự kiện `returned` có đủ trường `{request_id, stage, category, reason, actor_id}` (notification-service CR-REQ-010, issue-status-sync CR-REQ-024 đọc).
- Lệnh: `go test ./services/request-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [x] Năm RPC thật, `buf breaking` xanh.
- [x] `returned_category` hiện trong `Request`.
- [x] README real vs stub cập nhật.
- [x] Danh tính từ metadata, không từ body.

## Rủi ro và lưu ý

- Không có trường `provider` trong `SpawnChildRequestRequest` (theo CR); `mcp` làm nguồn con cần CR-REQ-017 thêm cách truyền (ví dụ metadata `x-orca-source: mcp`); chưa quyết, ghi vào PR.

## Ghi chú triển khai

- Năm handler ở `adapter/grpc/server_lifecycle.go` (`ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks`), `returned_category` hiện trong `Request`; actor từ metadata. Reopen đặt lại `classification_attempts` (kiểm trong `ReturnReopenCancel`).
- Kịch bản gRPC chạy trên Postgres và MySQL thật với `TransitionRequest` thật.
