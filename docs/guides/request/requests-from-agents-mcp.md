# Request từ agent qua MCP

**Cập nhật:** 2026-10-09 · Đối chiếu với: `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/` (`pack_request_flow.go`,
`request_create_limit.go`, `config.go`), `backend-go/services/request-service/internal/domain/approval_errors.go`,
[CR-REQ-017](../../crs/v6/gateway-and-mcp/README.md).

## Tool `request_*` và hạn mức

Các tool request-flow nằm ở **api-gateway**, không phải `mcp-service`: `pack_request_flow.go` khai báo đọc (`request_list`, `request_get`,
`request_typeHistory`, `solution_list`, `approval_get`, `approval_list`, `approval_listPending`, `backlog_requests`, `backlog_tasks`,
`backlog_execute`, `request_flowStatus`) và ghi (`request_create`, `request_changeType`, `request_returnToBacklog`, `request_reopen`,
`request_spawnChild`, `solution_generate`, `request_generatePlan`, và `request_startPhase` ở nhóm exec). Tool không nhận tenant, user hay nguồn:
danh tính lấy từ principal đã xác thực, nguồn Request được đặt từ phiên MCP.

Hạn mức tạo Request: `request_create` và `request_spawnChild` dùng chung một ngân sách theo (tenant, user, MCP client), mặc định 20 mỗi giờ
(`MCP_REQUEST_CREATE_PER_HOUR`, số nguyên không âm; 0 giữ mặc định). Bộ đếm nằm trong tiến trình api-gateway, nên N bản sao cho N lần hạn mức; giới hạn cứng
là số Request đang chờ của `request-service`.

Tình trạng kiểm chứng: có test đơn vị của api-gateway cho danh sách tool (tệp vàng `testdata/tools_list.golden.json`). Bộ e2e `orca-flows` chỉ
kiểm `MCP-01/02` (metadata OAuth và từ chối ẩn danh) và hai test này `BLOCKED` khi `MCP_ENABLED` tắt (mặc định); **chưa có** e2e gọi tool `request_*`
qua MCP.

Phía `request-service`, `CreateRequest` nhận `source.provider=mcp` và ghi `actor_type=agent` trong audit `request.create`.

## Vì sao agent không duyệt được Approval

`ApprovalService` từ chối caller tự động (`actor_type=agent`) ở `Approve`, `Reject` và `Cancel` với mã
`REQUEST_APPROVAL_AGENT_FORBIDDEN` (`ErrAgentForbidden`). Agent chỉ đề xuất; quyết định luôn thuộc người trong danh sách duyệt
([Phê duyệt Request](./approving-requests.md)). `ChangeRequestType` và `ReturnToBacklog` có thể do agent gọi và được ghi `actor_type=agent` trong audit.

## Cờ luồng áp cho mọi đường vào

Cờ được thi hành ở `request-service` nên WS, HTTP và MCP đều theo. Khi tắt, lệnh đi tiếp trả `REQUEST_FLOW_DISABLED`
([Bật luồng Request](./admin-enable-request-flow.md)). Tool `request_flowStatus` cho agent hỏi cờ hiệu lực của tenant.

Kill switch riêng cho tool MCP bằng chính sách tenant của MCP (xem [hướng dẫn quản trị MCP](../mcp/admin-guide.md)) và công tắc `MCP_ENABLED`
tắt cả bề mặt MCP.
