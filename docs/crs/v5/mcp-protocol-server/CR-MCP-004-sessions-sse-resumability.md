# CR-MCP-004 — Session, SSE, resumability, huỷ/tiến độ và chạy đa replica

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-004 |
| **Tên** | `Mcp-Session-Id`, kênh SSE server→client, `Last-Event-ID`, `notifications/cancelled`, `notifications/progress`, đồng bộ giữa replica |
| **Loại** | Feature |
| **Priority** | 🟠 P1 — không chặn tool đồng bộ, **chặn** tool dài (CR-009) và production nhiều replica |
| **Effort** | Large (7–10 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | ✅ Đã triển khai (unit/integration test) — xem solution và README service để biết khoảng trống |
| **Tác giả** | Khảo sát `notification-service` (SubscribeEphemeral), `wscompat` stream channels |
| **Phụ thuộc** | [CR-MCP-003](./CR-MCP-003-streamable-http-and-lifecycle.md), [CR-MCP-001](../mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md) |

---

## Bối cảnh & Vấn đề

- Orca chạy nhiều replica api-gateway. Một SSE stream gắn với **một** kết nối TCP ở **một** replica, nhưng request kế tiếp của cùng session có thể rơi vào replica khác.
- Hệ thống đã có mẫu giải bài toán này: notification-service fan-out cross-replica qua `SubscribeEphemeral` trên NATS (notification README, "Epic F"); `wscompat` có `StreamChannelHandler` cho kênh vừa ack vừa đẩy sự kiện (`registry.go:78`).
- Chưa có thứ gì tương đương cho MCP.

## Giải pháp đề xuất

### A. Session

- `initialize` thành công ⇒ cấp `Mcp-Session-Id` (UUID v4 ngẫu nhiên mật mã, không đoán được), trả trong header response.
- Lưu bền ở `mcp-service` (`mcp_sessions`: id, tenant, user, client info, protocol version, capabilities đã thương lượng, `last_seen`, trạng thái). Lưu **hash** nếu session id được coi là bí mật phiên.
- **Session bị ràng buộc với danh tính**: mọi request phải mang token cùng `sub`/`tenant` với lúc tạo session; lệch ⇒ `404`/`401` (không tin riêng session id — chống session hijacking).
- Request thiếu session id (sau initialize) ⇒ `400`; session hết hạn/không tồn tại ⇒ `404` để client tự `initialize` lại. `DELETE /mcp` ⇒ đóng session và giải phóng tài nguyên (bao gồm huỷ các tool đang chạy của session).
- TTL idle (`MCP_SESSION_IDLE_TTL`) + job dọn.

### B. SSE & resumability

- Mỗi message server gửi qua SSE có `id:` đơn điệu theo từng stream. Lưu vòng đệm gần nhất (kích thước/thời gian giới hạn) để client gửi `Last-Event-ID` nối lại **không mất và không lặp**.
- Phân phối cross-replica: replica nhận kết nối SSE đăng ký nhận sự kiện của session qua NATS ephemeral subject `mcp.session.<id>`; replica xử lý request (bất kỳ) publish kết quả/progress vào subject đó. Dùng lại cơ chế `SubscribeEphemeral` thay vì tạo bus mới.
- Heartbeat comment SSE định kỳ để qua proxy; đóng stream rác khi client đi.
- Giới hạn: số stream/session, số stream/user, kích thước event (chống bộ nhớ phình).

### C. Huỷ & tiến độ

- `notifications/cancelled{requestId}` ⇒ huỷ `context` của đúng lời gọi tool (nối với `Registry.Dispatch` vốn nhận `ctx`); handler đã có timeout 60s (`dispatchRPCTimeout`) là lưới an toàn — **không** cho tool ghi đè vô hạn.
- `notifications/progress` chỉ gửi khi client cung cấp `progressToken` trong `_meta`; throttle (ví dụ ≤ 5 event/giây/request).
- Huỷ tool có tác dụng phụ (ví dụ `git push` đang chạy) phải được ghi rõ ở mô tả tool: huỷ ≠ hoàn tác.

### D. `list_changed` / `resources/updated`

Khi catalog tool hoặc policy của tenant đổi (CR-007/012), publish sự kiện để các session đang mở nhận `notifications/tools/list_changed`. Dùng NATS, không polling.

## Acceptance Criteria

- [ ] Hai replica api-gateway: POST vào replica A, SSE mở ở replica B ⇒ client vẫn nhận progress + kết quả.
- [ ] Ngắt kết nối giữa chừng rồi nối lại với `Last-Event-ID` ⇒ nhận đúng phần còn thiếu (test với 100 event, cắt ở event 40).
- [ ] Dùng session id của user A với token của user B ⇒ bị từ chối.
- [ ] `DELETE /mcp` huỷ tool đang chạy; kiểm tra không rò goroutine (`goleak`).
- [ ] Idle quá TTL ⇒ `404`, `initialize` lại thành công.
- [ ] Test tải: 500 session đồng thời, mỗi session 1 SSE, bộ nhớ ổn định.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** bộ đệm resume tốn RAM ⇒ cấu hình giới hạn cứng, rơi về "không resume" (client `initialize` lại) thay vì OOM.
- **Rủi ro:** sticky routing không có ⇒ chấp nhận vì thiết kế cross-replica ở mục B; không dựa vào sticky session của LB.
- **Ngoài phạm vi:** primitive `tasks` bất đồng bộ lâu dài (thuộc CR-009).
