# TASK-REQ-038-07: Kênh gateway `request.activity`, `request.executionSummary`, parity MCP và cập nhật CONTRACT

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_activity.go` (mới) và test, `internal/adapter/wscompat/channels_request_dispatch.go` (sửa, đăng ký), `internal/adapter/mcpserver/tools/pack_request_flow.go` (sửa) và `parity_test.go` (cập nhật dòng loại trừ nếu cần), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` (sửa)
**Depends on:** TASK-REQ-038-05, TASK-REQ-038-06
**Status:** [x] DONE

---

## Context

- Gateway đã có nhóm kênh `request.*` (`channels_request*.go`), mẫu gần nhất là `channels_request_flow.go` (`request.flow`) và `channels_request_engine.go` (`request.engineSettings`).
- README v6 mục 8 điểm 13: gateway không kiểm quyền OPA trước định tuyến, mọi RPC của `request-service` tự kiểm quyền; `parity_test.go` của MCP **đỏ** nếu thêm kênh mà không có `ToolSpec` hoặc dòng loại trừ.
- Hai kênh mới là chỉ đọc.

## Việc cần làm

1. `channels_request_activity.go`: hai kênh dùng helper `c.handle` như file láng giềng:
   - `request.activity` nhận `{requestId, pageSize?, pageToken?, kinds?, severity?, actorKind?, subjectId?, since?}` và trả `{entries[], nextPageToken}` (camelCase).
   - `request.executionSummary` nhận `{requestId}` và trả `ExecutionSummary` (camelCase; `needsYou[]`, `failed[]`, `running[]`, `handover`).
   Map lỗi bằng bảng lỗi hiện có (`channels_request_errors.go`) gồm `REQUEST_ACTIVITY_CURSOR_INVALID`.
2. Đăng ký ở bộ dispatch như các kênh khác; thêm vào bảng kênh được phép của runtime nếu có (đối chiếu `channels_request_dispatch.go`).
3. MCP: thêm `ToolSpec` chỉ đọc `request_activity` và `request_execution_summary` vào `pack_request_flow.go` (quy ước tên: thay `.` bằng `_`, theo CR-MCP-007), **hoặc** thêm dòng loại trừ có lý do trong danh sách của `parity_test.go`. Mặc định: thêm tool (agent đọc được tiến độ), mô tả ngắn, không trả log.
4. CONTRACT: thêm mục cho hai kênh, trường mới `engine`, `executionMode` của `request.get` và `request.list`, `openSpecProfile` của `request.flow`, `executionMode` của `request.engineSettings(Set)`; đánh dấu "(mới, CR-REQ-038)".
5. Không đổi chữ ký `common/grpcmw` hay middleware chung.

## Kiểm thử

- Test kênh với fake client (mẫu `channels_request_fake_test.go`): đủ trường, camelCase, lỗi con trỏ sai, `pageSize` quá lớn bị cắt.
- `parity_test.go` xanh với hai tool mới.
- Hồi quy các kênh `request.*` hiện có.
- `cd /opt/repos/orca/backend-go && go test ./services/api-gateway/internal/adapter/wscompat/... ./services/api-gateway/internal/adapter/mcpserver/... -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Hai kênh trả đúng dữ liệu và lỗi.
- [x] `parity_test.go` xanh.
- [x] CONTRACT cập nhật đủ kênh và trường mới.
- [x] Không đổi hành vi kênh cũ.

## Rủi ro và lưu ý

- Timeout WebSocket 25 giây (`wscompat/handler.go`): hai kênh này chỉ đọc nên nằm trong giới hạn, nhưng `request.activity` cần `page_size` hợp lý.
- Tool MCP đọc nhật ký có thể lộ `summary` của task cho agent khác; kiểm tra quyền đọc Request của người gọi MCP giống kênh WS.

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- Kênh `request.activity` và `request.executionSummary` (`channels_request_activity.go`); tham số định danh là `id`. Thêm `executionMode` vào `request.engineSettingsSet`, `engine`/`executionMode` vào view Request, `openSpecProfile` vào `request.flow`.
- MCP: hai tool `request_activity`, `request_executionSummary` trong pack `pack_request_flow.go` (19 tool pack, 13 chỉ đọc); `tools_list.golden.json`, `channel_inventory.json`, bảng RPC↔kênh và `parity_test.go` đã cập nhật và xanh. OPA `request.rego`: hai RPC đọc thêm vào `agent_rpcs`. RPC catalog + phân loại flow-gate (`classRead`) + tenant-scope đã cập nhật.
- CONTRACT: mục 2.10 mới trong `CONTRACT-request-ui-api.md`.
