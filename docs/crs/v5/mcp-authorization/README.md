# MCP Authorization — Change Requests (v5)

> Cho agent/client MCP đăng nhập **thay mặt người dùng** theo chuẩn OAuth 2.1 của MCP, đồng thời có đường token
> dài hạn cho headless/CI. Mọi hành động của agent phải quy về một `Identity{TenantID, UserID, Role}` thật để tái dùng
> kiểm soát quyền của UI.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-005](./CR-MCP-005-oauth21-resource-server.md) | Chưa có OAuth cho MCP: không metadata, không đăng ký client, không consent | 🔴 P0 | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-006](./CR-MCP-006-mcp-tokens-scopes-and-role.md) | Token hiện có sai audience cho MCP; nhánh Bearer không có `Role`; chưa có scope | 🔴 P0 | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |

Thứ tự: `CR-MCP-005` và `CR-MCP-006` làm gần nhau; CR-006 cung cấp phần "token + role" mà CR-005 phát hành.
