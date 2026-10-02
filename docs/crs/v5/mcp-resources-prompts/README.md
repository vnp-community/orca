# MCP Resources & Prompts — Change Requests (v5)

> Ngoài *tools* (hành động), MCP có *resources* (ngữ cảnh chỉ-đọc mà client/LLM đính kèm) và *prompts* (mẫu quy trình
> do người dùng chọn). Hai primitive này giúp agent nắm bối cảnh Orca mà không phải gọi tool tốn kém.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-010](./CR-MCP-010-resources-templates-subscriptions.md) | Chưa có cách đính kèm ngữ cảnh (task, diff, PR, scrollback) và nhận cập nhật | 🟠 P1 | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-011](./CR-MCP-011-prompts.md) | Chưa có mẫu quy trình dùng lại (review PR, triage, bàn giao task) | 🟡 P2 | Small | ✅ Implemented (unit/integration tests) — see service README for gaps |

Thứ tự: `CR-MCP-010 → CR-MCP-011` (prompt tham chiếu resource).
