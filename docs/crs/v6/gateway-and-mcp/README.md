# Feature: gateway-and-mcp — Kênh WS/HTTP và tool MCP cho Request

> **Trạng thái:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-05, chưa chạy hệ thống.
> **Hợp đồng chung:** [`../README.md`](../README.md). Tên loại, trạng thái, RPC, sự kiện lấy từ đó.

## 1. Mục tiêu

Đưa `request-service` ra ngoài qua hai đường có sẵn của `api-gateway`: kênh WS `wscompat` (frontend, nội bộ) và tool MCP (agent ngoài). Gateway không giữ DB và không tự quyết quyền nghiệp vụ: nó dịch tham số, gắn `Identity`, đặt timeout, ánh xạ lỗi. Quyền duyệt và chuyển trạng thái do `request-service` quyết (CR-REQ-009, 010).

## 2. Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc | Mở khoá |
|---|---|---|---|---|---|
| [CR-REQ-016](./CR-REQ-016-api-gateway-request-channels.md) | Kênh WS/HTTP `request.*`, `solution.*`, `approval.*`, `backlog.*` | 🔴 P0 | Medium | CR-REQ-001, 003, 004, 005, 006, 007, 008, 009 (backlog: 015) | CR-REQ-017, 018 đến 023, 025 |
| [CR-REQ-017](./CR-REQ-017-mcp-request-tools-and-source.md) | Tool MCP `request_*`, `solution_*`, `approval_*` và nguồn MCP | 🟠 P1 | Medium | CR-REQ-016, CR-MCP-007, 008, 012, 013 | CR-REQ-025 (kịch bản MCP) |

## 3. Thứ tự thực thi

```
CR-REQ-016  ──▶  CR-REQ-017
```

Hai CR phải nằm cùng nhánh tích hợp hoặc CR-016 phải thêm mục loại trừ tạm (xem quyết định G3). Lý do: `parity_test.go` của `mcpserver/tools` đỏ khi có channel đăng ký mà không có `ToolSpec` hoặc dòng trong `excluded_channels.yaml`.

## 4. Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| G1 | `tenantId` và `userId` chỉ lấy từ `Identity`, không nhận từ tham số kênh hay tool | Mẫu có sẵn ở `channels_task_source.go`; tránh giả mạo tenant |
| G2 | Nguồn `mcp` và `webhook` do gateway gán từ ngữ cảnh, client không được gửi | `ToolOrigin` đã đi theo `ctx`, client WS không giả được (`tool_origin_context.go`) |
| G3 | CR-016 thêm vào `excluded_channels.yaml` mọi channel mới (lý do "chờ CR-REQ-017"); CR-017 gỡ dòng của channel nó phủ bằng `ToolSpec` | Giữ parity test xanh ở từng PR |
| G4 | Cổng duyệt của người không mở cho MCP: `request.confirmType`, `solution.choose`, `approval.approve`, `approval.reject`, `approval.cancel`, `request.cancel`, `request.flowSet` nằm trong `excluded_channels.yaml` vĩnh viễn | Agent tự duyệt việc của chính mình thì cổng duyệt vô nghĩa |
| G4a | `request.subscribe` cũng loại trừ (kênh stream, không phải tool) | MCP có cơ chế subscription riêng ở `resources/` |
| G5 | Lỗi trả về dạng `CODE: message` ngắn, không kèm nội dung `body` của Request | `scrubSensitiveChannelError` chỉ áp cho kênh nhạy cảm; body Request có thể chứa dữ liệu nội bộ nên không đưa vào lỗi |
| G6 | Cờ `request_flow_enabled` do `request-service` thi hành (CR-REQ-025); gateway chỉ chuyển lỗi `REQUEST_FLOW_DISABLED` | Một điểm thi hành, không lệch giữa WS, HTTP, MCP |

## 5. Phát hiện chung (cần người duyệt xem)

- Gateway hiện **không có kiểm quyền OPA trước định tuyến** (README api-gateway: "No OPA authorization check"). Mọi kiểm quyền nằm ở service đích, nên `request-service` phải tự kiểm quyền từng RPC; CR-016 không thể coi gateway là lớp bảo vệ.
- README v6 mục 3.6 liệt kê `GeneratePlan` và `StartPhase` nhưng không có tiền tố kênh `plan.*` hay `phase.*`. CR-016 đặt chúng dưới `request.generatePlan` và `request.startPhase`.
- README v6 không có RPC cho: liệt kê lịch sử đổi loại, liệt kê Request con, đọc cờ `request_flow_enabled`, tra Request theo nguồn, hai view backlog Task và Execute. Chi tiết ở mục "Câu hỏi mở" của từng CR.

## 6. Tài liệu liên quan

- `backend-go/services/api-gateway/README.md`
- `docs/crs/v5/mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md`, `CR-MCP-008-domain-tool-packs.md`
- `docs/crs/v5/mcp-governance-safety/CR-MCP-012-tool-policy-and-annotations.md`, `CR-MCP-013-approvals-audit-killswitch.md`
