# Request từ agent qua MCP

**Cập nhật:** 2026-10-08 · Đối chiếu với: `backend-go/services/mcp-service/internal/usecase/tools/`,
`backend-go/services/request-service/internal/domain/approval_errors.go`, [CR-REQ-017](../../crs/v6/gateway-and-mcp/README.md).

## Tool `request_*` và hạn mức: chưa có

Trên nhánh này, `mcp-service` **chưa có** tool `request_*` (ví dụ `request_create`, `request_flowStatus`) và chưa có hạn mức tạo Request:
thư mục `internal/usecase/tools/` chỉ có `pack1_codeintel.go` và `sensitive_paths.go`, và không có tham chiếu `request_` trong mã `mcp-service`.
Các tool này thuộc CR-REQ-017; đã có mã ở nhánh khác nhưng chưa vào nhánh này, nên chưa được xem là đã có. Khi vào nhánh và có test,
cập nhật mục này (tên tool, hạn mức, `client_request_id` dedupe).

Trong lúc chờ, cổng nhận phía `request-service` đã sẵn sàng cho nguồn `mcp`: `CreateRequest` nhận `source.provider=mcp` và ghi `actor_type=agent`
trong audit `request.create`.

## Vì sao agent không duyệt được Approval

`ApprovalService` từ chối caller tự động (`actor_type=agent`) ở `Approve`, `Reject` và `Cancel` với mã
`REQUEST_APPROVAL_AGENT_FORBIDDEN` (`ErrAgentForbidden`). Agent chỉ đề xuất; quyết định luôn thuộc người trong danh sách duyệt
([Phê duyệt Request](./approving-requests.md)). `ChangeRequestType` và `ReturnToBacklog` có thể do agent gọi và được ghi `actor_type=agent` trong audit.

## Cờ luồng áp cho mọi đường vào

Cờ được thi hành ở `request-service` nên WS, HTTP và MCP đều theo. Khi tắt, lệnh đi tiếp trả `REQUEST_FLOW_DISABLED`
([Bật luồng Request](./admin-enable-request-flow.md)). Tool `request_flowStatus` để agent hỏi cờ **chưa có** (CR-REQ-017).

Kill switch riêng cho tool MCP bằng chính sách tenant của `mcp-service` đã có ở phía MCP (xem [hướng dẫn quản trị MCP](../mcp/admin-guide.md)); vì
tool `request_*` chưa có nên hiện chưa có gì để tắt.
