# F17 — Memory / AI Vault

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F17 |
| **Tên** | Memory / AI Vault |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | 🚧 Đang phát triển |
| **Tham chiếu PRD** | §3.10 (Memory / AI Vault) |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Lưu trữ và tái sử dụng AI session context — cho phép xem lại lịch sử sessions, filter theo tags và agent type, và resume từ session cũ.

---

## Vấn đề cần giải quyết

Context của AI sessions thường bị mất sau khi đóng terminal. Developer không thể tìm lại các quyết định thiết kế quan trọng được thảo luận trong session cũ, hoặc tiếp tục work từ đúng điểm đã dừng.

---

## Tính năng chi tiết

### Session Storage
- Lưu trữ toàn bộ session history của agent
- Metadata: agent type, worktree, timestamp, duration, token count
- Content: messages, tool calls, outcomes

### Session Viewer
- Browse lịch sử sessions
- Filter theo: agent type, worktree, date range, status
- Search trong session content

### Session Resume
- Resume session cũ từ AI Vault
- Inject session context vào agent mới

### Session Filters
- By agent (Claude, Codex, v.v.)
- By worktree / project
- By date range
- By outcome (completed, error, interrupted)

---

## Tiêu chí chấp nhận

- [ ] Sessions được lưu tự động sau khi kết thúc
- [ ] Browse và filter sessions nhanh (< 200ms)
- [ ] Resume session hoạt động với Claude Code
- [ ] Session content có thể search bằng text

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Vault types** | `src/shared/ai-vault-types.ts` |
| **Session filters** | `src/shared/ai-vault-session-filters.ts` |
| **Session display** | `src/shared/ai-vault-session-display.ts` |
| **Resume command** | `src/shared/ai-vault-resume-command.ts` |
| **Main module** | `src/main/ai-vault/` |
