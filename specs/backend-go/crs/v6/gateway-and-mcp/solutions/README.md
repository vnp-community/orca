# gateway-and-mcp: solutions (backend-go, v6)

> 📋 Proposed. Chưa triển khai, chưa chạy. Viết ngày 2026-10-06 từ khảo sát code `backend-go/services/api-gateway`; `request-service` chưa tồn tại.
> CR nguồn: [`docs/crs/v6/gateway-and-mcp/`](../../../../../../docs/crs/v6/gateway-and-mcp/README.md). Hợp đồng chung: [`docs/crs/v6/README.md`](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3). **Hợp đồng backend-frontend:** [`../CONTRACT-request-ui-api.md`](../CONTRACT-request-ui-api.md).

## CR → Solution

| CR | Solution | Nội dung | Service | Priority |
|---|---|---|---|---|
| CR-REQ-016 | [BE-REQ-SOL-016](./BE-REQ-SOL-016-api-gateway-request-channels.md) | 28 kênh WS `request.*`, `solution.*`, `approval.*`, `backlog.*`, stream `request.subscribe`, 5 route HTTP, view camelCase, luật nguồn Request | `api-gateway` | P0 |
| CR-REQ-017 | [BE-REQ-SOL-017](./BE-REQ-SOL-017-mcp-request-tools-and-source.md) | 17 tool MCP (3 `Declared`), loại trừ cổng người, `ToolOrigin` trong executor, hạn mức tạo | `api-gateway/mcpserver/tools` | P1 |

## Thứ tự phụ thuộc

```
CR-REQ-001 (proto) ─▶ CR-REQ-003..009 (RPC) ─┐
                                             ▼
                              BE-REQ-SOL-016 (kênh, HTTP, stream)
                                             │
                                             ▼
                              BE-REQ-SOL-017 (tool MCP)
                                             │
                    ┌────────────────────────┴───────────┐
                    ▼                                     ▼
          CR-REQ-025 (E17 qua MCP)           frontend CR-REQ-018..023
```

SOL-016 thêm kênh theo từng nhóm khi RPC tương ứng có trong proto (không đợi hết CR-003..009). `backlog.*` đợi CR-REQ-015.

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| G1 | `tenantId`, `userId` chỉ lấy từ `Identity`; không tham số nào cho phép khai | mẫu `channels_task_source.go`; chống giả mạo tenant |
| G2 | Nguồn `mcp`, `webhook` do gateway gán; client WS không được khai | `ToolOrigin` đi theo `ctx` |
| G3 | Mỗi kênh mới phải có `ToolSpec` hoặc dòng loại trừ cùng PR | `TestChannelInventory` đỏ nếu thiếu |
| G4 | Cổng của người không mở cho MCP: `request.confirmType`, `solution.choose`, `approval.*` ghi, `request.cancel`, `request.flowSet`, `request.subscribe` | agent tự duyệt thì cổng vô nghĩa |
| G5 | Lỗi `CODE: message` ngắn, mã có tiền tố `REQUEST_`, không chứa `body` | CONTRACT C4, C9 |
| G6 | `request-service` thi hành cờ và quyền; gateway chỉ chuyển lỗi | gateway không có OPA trước định tuyến |
| G7 | `ClassifyRequest`, `GeneratePlan` (propose) vượt `invokeTimeout` 25 giây của WS: cần RPC trả sớm (CONTRACT Q2) | `wscompat/handler.go:247` |
| G8 | Tool ghi (pack 2) chỉ hiện khi `MCP_TOOL_PACKS_ENABLED=1,2` | `tools/config.go:66` mặc định chỉ pack 1 |

## Phát hiện đáng chú ý (đã đối chiếu code)

- `invokeTimeout = 25s` giới hạn mọi kênh; CR-016 ghi 25 giây cho AI nhưng AI thật cần 60 giây trở lên.
- `WithToolOrigin` chỉ được gọi cho tool terminal, agent; tool thường chưa có `ToolOrigin` (CR-017 giả định có).
- Mã lỗi CR-016 không tiền tố mâu thuẫn CR-007, 009 và README v6 mục 8 dòng 5.
- Tên kênh frontend dùng (`solution.chooseOption`, `backlog.list`, `request.listHistory`, `request.events.subscribe`, `request.getPlan`) lệch CR-016; bảng chốt ở CONTRACT mục 8.
- `SubscribeEphemeral` có thể phát lại lịch sử stream (chưa chạy thử).
