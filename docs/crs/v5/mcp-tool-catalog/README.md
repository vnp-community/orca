# MCP Tool Catalog — Change Requests (v5)

> Biến bề mặt chức năng mà UI đang dùng (`wscompat.Registry`, ~417 lần `Register*("ns.method")` theo grep
> literal, 58 namespace) thành **tool MCP có schema**, để agent làm được những gì người dùng làm qua giao diện.
> Nguyên tắc: **một đường thực thi duy nhất** (D3) — tool gọi lại đúng handler của UI.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-007](./CR-MCP-007-registry-introspection-and-descriptors.md) | Registry không liệt kê được; handler không có schema ⇒ không sinh được `tools/list` | 🔴 P0 | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-008](./CR-MCP-008-domain-tool-packs.md) | Chưa có tool nào cho các domain UI (project/worktree/git/task/…) | 🔴 P0 | XL (chia đợt) | ✅ Packs 1–4 đã triển khai cho các channel hiện có (unit test); không có channel force-push/`git.reset`/GitLab merge nên không tạo tool — xem solution |
| [CR-MCP-009](./CR-MCP-009-long-running-and-streaming-tools.md) | Terminal/agent/workflow là stream hoặc chạy lâu; tool đồng bộ không biểu diễn được | 🟠 P1 | Large | ✅ Đã triển khai (unit/integration test) — xem solution BE-009 để biết khoảng trống |

Thứ tự: `CR-007 → CR-008 → CR-009`. CR-008 triển khai theo từng "pack" (đọc trước, ghi sau) và **không** bật pack ghi ở production trước khi xong governance ([`mcp-governance-safety`](../mcp-governance-safety/README.md)).
