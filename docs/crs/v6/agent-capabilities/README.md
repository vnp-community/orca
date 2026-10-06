# Agent Capabilities, Change Requests (v6)

> Thay đổi duy nhất của series v6 ở Dev Server Agent (`agent/`) và phần backend đi kèm ở `infra-fleet-service`: chế độ chỉ đọc cho `agent.execPrompt`, vùng làm việc rõ ràng, danh sách file đổi, khối kết quả JSON, báo cáo năng lực dev server. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-033](./CR-REQ-033-agent-readonly-worktree-and-capability-report.md) | `agent.execPrompt` không ép được chỉ đọc, bắt buộc `worktreePath`, vứt khối kết quả; handshake không nói dev server có công cụ gì, `claude` đã đăng nhập chưa; hai bản agent lệch nhau | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-033 (agent trước)  ──▶  infra-fleet-service (hồ sơ năng lực)  ──▶  CR-REQ-008, 013, 029 chọn đường theo `features`
```

| Bước | Lý do thứ tự |
|------|--------------|
| 1. Agent | Chỉ thêm tham số và method mới, không đổi hành vi cũ; build `agent/out/agent.js`, tăng `AGENT_VERSION` để `relay-ssh` tự đẩy bản mới |
| 2. `infra-fleet-service` | Đọc `features` và `agent.capabilities`; gặp agent cũ thì hạ về hồ sơ `handshake_only` |
| 3. Người dùng ở `request-service` | Gọi `GetDevServerCapabilities` rồi chọn đường chạy; không gọi method mới khi `features` thiếu |

Agent trước, backend sau nên không có cửa sổ lỗi khi dev server nâng cấp lệch nhau.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| G1 | Mọi tham số và trường mới là tuỳ chọn; thiếu thì hành vi cũ | Dev server nâng cấp lệch thời điểm; backend cũ vẫn chạy |
| G2 | Nguồn thật của dev server là `agent/`; `desktop/src/relay` không sửa | Build và triển khai đều dùng `agent/out/agent.js`; bản desktop đã lệch (không có `execPrompt`) |
| G3 | Phát hiện tính năng bằng `features` trong handshake, không bằng so sánh `agentVersion` | `agentVersion` đang cố định `5.0.0` và có người dùng khác (`ResumeAgentSession`, `MinAgentVersion`) |
| G4 | Chế độ chỉ đọc từ chối chạy khi CLI không hỗ trợ cờ, không hạ cấp | Tránh lời hứa chỉ đọc không có thật |
| G5 | Không đưa bí mật qua RPC dò: biến môi trường chỉ trả có/không, công cụ dò theo danh sách cho phép cứng | Đúng nguyên tắc không ghi bí mật ở `ai-complete-handler.ts` |
| G6 | Logic mới ở file mới có tên cụ thể, không thêm `max-lines` disable | AGENTS.md |

## Điểm cần xác nhận khi duyệt feature

- **Cờ Claude CLI chưa chạy thật:** chỉ đọc `claude --help` (2.1.289), chưa thử `--print` với `--permission-mode plan` và `--tools`. Phải chạy thử nghiệm đối kháng (CR-REQ-033 mục 5) trước khi coi chỉ đọc là ép buộc.
- **Mở rộng `ai.complete`** (`usage`, `maxTokens`, lỗi có cấu trúc) nằm trong CR-REQ-033 mục 2.8 vì CR-REQ-034 cần; nếu muốn tách CR riêng cho agent thì tách mục này.
- **Mâu thuẫn với CR-REQ-008:** CR đó nói "chưa ép được chỉ đọc, chưa đọc mã agent". Đã đọc mã agent, xem mục "Tác động tới CR hiện có" của CR-REQ-033.

## Điểm lệch với README v6 và CR khác

- README v6 mục 8 dòng 2 (chỉ giảm thiểu, chưa ép được chỉ đọc) đúng với agent hiện tại; sẽ đổi khi CR này triển khai.
- Agent có ba số phiên bản (`5.0.0` handshake, `2.1.0` build, `1.4.138-rc.6` package); không thuộc series v6, chỉ ghi lại.
