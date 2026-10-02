# CR-MCP-003 — Streamable HTTP transport và vòng đời `initialize`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-003 |
| **Tên** | MCP server theo chuẩn: Streamable HTTP, `initialize`/`initialized`, capabilities, `ping`, phân trang, logging |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium (5–7 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `agent/src/relay/agent-rpc-dispatch-misc.ts`, `backend-go/` |
| **Phụ thuộc** | [CR-MCP-002](../mcp-service-foundation/CR-MCP-002-gateway-mcp-endpoint-wiring.md) |

---

## Bối cảnh & Vấn đề

`agent-rpc-dispatch-misc.ts:29-68` xử lý `tools/list` và `tools/call` và format kết quả bằng `formatMcpResult`, nhưng:

- không có `initialize` / `notifications/initialized` ⇒ client chuẩn (Claude Desktop, Cursor, MCP Inspector) không hoàn tất handshake;
- không thương lượng phiên bản giao thức, không khai báo `capabilities`;
- chạy trên kênh relay riêng của agent, không phải HTTP/stdio chuẩn;
- ở `backend-go` hoàn toàn chưa có.

Hệ quả: chưa thể cấu hình `claude mcp add --transport http orca https://…/mcp` rồi dùng.

## Giải pháp đề xuất

### A. Transport — Streamable HTTP (một endpoint `/mcp`)

| Method | Hành vi |
|--------|---------|
| `POST` | Body là **một** JSON-RPC message (request/notification/response). Request ⇒ trả `application/json` **hoặc** `text/event-stream` (SSE) chứa response (có thể kèm notification/progress trước đó). Notification/response ⇒ `202 Accepted`, không body. |
| `GET` | Mở SSE để server chủ động gửi message (list_changed, resources/updated, elicitation...). Server có thể trả `405` nếu chưa hỗ trợ (CR-004 làm đầy đủ). |
| `DELETE` | Client chủ động kết thúc session (CR-004). |

Header: đọc `MCP-Protocol-Version` ở mọi request sau `initialize` (thiếu ⇒ coi là phiên bản mặc định theo spec, sai/không hỗ trợ ⇒ `400`). Kiểm `Accept` phù hợp. **Không** hỗ trợ JSON-RPC batch nếu phiên bản thương lượng đã bỏ batch.

### B. Vòng đời

1. `initialize`: thương lượng `protocolVersion` (trả phiên bản server hỗ trợ gần nhất nếu client đòi bản lạ), trả `serverInfo{name:"orca", version}` và `capabilities`:
   - `tools{listChanged:true}` (CR-007), `resources{subscribe:true,listChanged:true}` (CR-010), `prompts{listChanged:true}` (CR-011), `logging{}`, và `completions{}` nếu làm.
   - Khai báo `instructions` ngắn mô tả Orca và cách dùng an toàn (đối tượng đọc là LLM — viết ngắn, không chứa dữ liệu tenant).
2. `notifications/initialized` ⇒ session sang trạng thái `ready`; request nghiệp vụ trước đó ⇒ lỗi JSON-RPC.
3. `ping` luôn trả `{}`; `notifications/cancelled` huỷ context của request tương ứng (nối với CR-004).
4. `logging/setLevel` + `notifications/message` — mức log theo session, **không bao giờ** chứa secret/token.
5. Phân trang bằng `cursor` cho `tools/list`, `resources/list`, `prompts/list` (cursor opaque, ký HMAC để không đoán được/không bị giả mạo).

### C. Cách triển khai

Dùng SDK Go chính thức (D4) cho khung JSON-RPC + transport; viết adapter mỏng nối vào:
`tools/call` ⇒ CR-007 (`ToolExecutor`), `resources/*` ⇒ CR-010, `prompts/*` ⇒ CR-011. **Đã chốt (D4):** SDK Go chính thức là lựa chọn cố định; spike chỉ xác minh hook Streamable HTTP/session-store. Cần ADR ngắn ghi lại: phiên bản SDK, các phần SDK **chưa** hỗ trợ và cách vá.

### D. Mô hình lỗi

- Lỗi giao thức ⇒ JSON-RPC error chuẩn (`-32700/-32600/-32601/-32602/-32603`).
- Lỗi nghiệp vụ của tool ⇒ **không** phải JSON-RPC error mà là kết quả `isError:true` kèm nội dung có thể đọc được (để LLM tự sửa). Map `apperrors` → thông điệp ngắn, **không** rò stack/chi tiết nội bộ/ID service khác.

## Acceptance Criteria

- [ ] MCP Inspector (chế độ Streamable HTTP) kết nối, thấy `serverInfo` và capabilities đúng.
- [ ] `initialize` với `protocolVersion` lạ ⇒ server trả phiên bản hỗ trợ; client không tương thích ngắt kết nối (hành vi đúng spec).
- [ ] Request trước `initialized` bị từ chối; sau đó hoạt động.
- [ ] Gọi `ping`, `tools/list` có `cursor`, `logging/setLevel` đúng.
- [ ] Lỗi tool trả `isError:true`; lỗi giao thức trả JSON-RPC error; không có stack trace trong cả hai.
- [ ] Fuzz test body JSON-RPC (kích thước, lồng sâu, id trùng) không làm panic.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** spec đổi (phiên bản mới) ⇒ khoá hành vi vào test conformance (CR-015), không hard-code chuỗi phiên bản rải rác.
- **Ngoài phạm vi:** auth (CR-005/006), session bền/resume (CR-004), nội dung tool (CR-007+).
