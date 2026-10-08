# IMPLEMENTATION-NOTES: context-sources (CR-REQ-031)

Cập nhật 2026-10-07. Mới triển khai đợt M1: TASK-REQ-031-06 (`mcp-service`). Các task 01 đến 05, 07, 08 chưa làm.

## Quyết định lệch so với task

- Mã lỗi `MCP_SERVER_NOT_USABLE` mà task bảo "dùng lại" chưa tồn tại; đã thêm `CodeServerNotUsable` (FailedPrecondition) cạnh `MCP_SERVER_NOT_APPROVED` hiện có.
- Stub cũ `usecase/call_external_tool.go` (hàm `nil, nil`, không ai gọi) đã xóa.
- Audit đi qua `EnqueueOutbox` sau lời gọi, best effort: lời gọi đã xảy ra nên không làm mất kết quả khi outbox lỗi. Nếu cần "fail closed" (không audit thì không trả), phải ghi audit trước và hoàn tác sau; chưa làm.
- Kết quả bị che bí mật và cắt lại sau khi che; `digest` tính trên văn bản đã che.
- `resources/read` theo đặc tả MCP đọc `result.contents[].text` (task ghi `resource.text`, là dạng của `tools/call`).
- Test guard `internalcaller` nằm ở `cmd/server/registry_internal_guard_test.go` vì `registryInternalMethods` ở package `main`.
- Sinh lại proto làm đổi dòng phiên bản `protoc-gen-go`/`-grpc` ở `mcp.pb.go`, `mcp_grpc.pb.go` (chỉ comment, do công cụ cục bộ mới hơn).

## Chưa kiểm chứng

- Chưa gọi máy chủ MCP bên thứ ba thật (mỗi RPC mở phiên mới, tốn một vòng `initialize`; máy chủ đòi phiên dài hơi có thể lỗi).
- `gitnexus_impact` không chạy được trong phiên này; người gọi đã kiểm bằng grep (xem file task).

## Câu hỏi mở

- Có cần cờ chỉ-đọc trên `ToolInfo` để chặn tool "ghi" bị duyệt nhầm? Hiện chỉ dựa `ApprovedTools` và `scopes` ở `request-service`.
- `request-service` phải chuyển tenant (và user nếu có) qua metadata chuẩn và gửi token `internalcaller` (task 031-07).
