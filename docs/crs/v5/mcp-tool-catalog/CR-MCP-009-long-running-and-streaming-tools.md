# CR-MCP-009 — Tool dài hạn & streaming: terminal, agent run, workflow

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-009 |
| **Tên** | Ánh xạ `StreamChannelHandler`/stream/binary của wscompat sang mô hình MCP: progress, polling an toàn, và (khi chín) primitive `tasks` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large (7–9 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `wscompat` (`terminal_stream_frame.go`, `registry.go:78`) |
| **Phụ thuộc** | [CR-MCP-004](../mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md), [CR-MCP-008](./CR-MCP-008-domain-tool-packs.md) đợt 3 |

---

## Bối cảnh & Vấn đề

UI tương tác với terminal/agent bằng **luồng**: `terminal.create` vừa ack (trả `ptyId`) vừa đẩy `terminal.output/exited`; `terminal.multiplex` dùng khung nhị phân (`BinaryStreamChannelHandler`). MCP `tools/call` là *request→response*. Nếu ép tool "chạy lệnh" chờ đến khi xong, ta gặp: timeout 60s (`dispatchRPCTimeout`), nghẽn ngữ cảnh khi output khổng lồ, và agent không thể hỏi han tiến trình giữa chừng.

## Giải pháp đề xuất

### A. Mẫu "bắt đầu → quan sát → tương tác → dừng" bằng nhiều tool ngắn (hoạt động với mọi client, không phụ thuộc primitive mới)

| Tool | Hành vi |
|------|---------|
| `terminal_start` | Tạo phiên (gọi `terminal.create`), trả `terminal_id`, **không** chờ lệnh xong |
| `terminal_send` | Gửi input/lệnh; tuỳ chọn `wait_ms`/`until_idle` để lấy output ngay sau đó |
| `terminal_read` | Đọc output từ **con trỏ** (`since_seq`), giới hạn byte, trả `next_seq` + cờ `exited`/`exit_code`; có scrollback đã có (`channels_terminal_scrollback.go`) |
| `terminal_stop` | Kết thúc phiên |
| `agent_start / agent_status / agent_send / agent_stop` | Tương tự cho agent CLI do Orca chạy |
| `workflow_run / workflow_run_status` | Chạy workflow và hỏi trạng thái |

Con trỏ đọc (cursor) làm output **không mất** giữa hai lần đọc và cho phép agent tự điều tiết ngữ cảnh. Output được cắt (mặc định ví dụ 16 KB/lần) và được **gắn nhãn là dữ liệu không tin cậy** (xem CR-013 mục prompt-injection).

### B. Progress qua SSE (khi có `progressToken`)

Tool chạy lâu hơn vài giây (ví dụ `git_clone`, `worktree_create`) phát `notifications/progress` (CR-004) với mô tả ngắn. Vẫn có trần thời gian; vượt trần trả `isError` kèm hướng dẫn dùng mẫu A.

### C. Primitive `tasks` của MCP (tuỳ chọn, theo độ chín của spec)

Nếu bản spec/SDK đã ổn định (ghi ở "Điều chưa xác minh" README v5): tool dài trả **task handle**, client hỏi `tasks/get`/`tasks/result`, có thể `tasks/cancel`. Lưu trạng thái ở `mcp-service` để sống sót khi replica restart. Nếu chưa chín ⇒ **chỉ làm mẫu A**; không để tính năng này chặn GA.

### D. Giới hạn tài nguyên & dọn dẹp

- Trần số phiên terminal/agent mở đồng thời mỗi (user, session MCP) và mỗi tenant.
- Session MCP đóng/hết hạn (CR-004) ⇒ **tự động dừng** mọi terminal/agent do session đó tạo (tránh tiến trình mồ côi trên máy dev của người dùng).
- Idle quá hạn ⇒ dừng + thông báo.
- Mọi lệnh gửi vào terminal đi qua cùng cổng policy/approval (CR-012/013) — approval theo **lệnh**, không chỉ theo tool `terminal_send`.

### E. Không phơi stream nhị phân

`terminal.multiplex` nằm trong danh sách loại trừ; MCP chỉ nhận văn bản (đã lọc mã điều khiển ANSI nguy hiểm/escape; giới hạn dòng cực dài).

## Acceptance Criteria

- [ ] Agent chạy `npm test` trong worktree: start → send → đọc theo cursor tới khi `exited`, lấy exit code, không timeout 60s.
- [ ] Đọc output 50 MB lệnh `yes` không làm treo/đầy bộ nhớ: bị cắt + cảnh báo.
- [ ] Đóng session MCP ⇒ terminal liên quan bị dừng ≤ 10s (test tích hợp, không rò tiến trình).
- [ ] Vượt trần phiên đồng thời ⇒ lỗi rõ ràng.
- [ ] Hoạt động với terminal chạy qua SSH (AGENTS.md) giống cục bộ.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** agent vòng lặp gọi `terminal_read` liên tục ⇒ rate limit riêng cho nhóm tool này.
- **Rủi ro:** output chứa chỉ dẫn độc hại nhắm LLM ⇒ xử lý ở CR-013.
- **Ngoài phạm vi:** UI hiển thị phiên terminal do agent tạo (frontend).
