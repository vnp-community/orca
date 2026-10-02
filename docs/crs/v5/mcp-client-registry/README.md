# MCP Client Registry — Change Requests (v5)

> Chiều ngược lại của feature server: Orca là **MCP client/điều phối** — quản lý các MCP server *bên ngoài* mà
> tenant/user muốn agent dùng, và cấp cho agent do Orca khởi chạy cả MCP của Orca lẫn các server đó.
> Hiện trạng: `mcp.servers` trong profile chỉ được **gộp cấu hình** ở `tenant-service`; không ai quản lý vòng đời,
> secret hay kiểm tra an toàn cho chúng ở backend-go.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-014](./CR-MCP-014-external-mcp-server-registry.md) | Cấu hình MCP server ngoài không có registry, secret, kiểm tra SSRF, hay cơ chế cấp cho agent | 🟡 P2 | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |

Độc lập với phần server (chỉ cần `CR-MCP-001`); nên làm sau khi server ổn định vì tái dùng governance (CR-012/013).

**Đã chốt (2026-10-01):** secret server ngoài đi plaintext một lần qua WS/TLS tới `mcp-service` rồi vào credential-broker (không dùng phong bì client), `value` bị che ở log/trace (Q-D1).
