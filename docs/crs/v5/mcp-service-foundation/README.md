# MCP Service Foundation — Change Requests (v5)

> Dựng nền móng: service `mcp-service` và điểm nối vào `api-gateway`. Xem bối cảnh và quyết định D1–D7 ở
> [README v5](../README.md).

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-001](./CR-MCP-001-scaffold-mcp-service.md) | Chưa có service nào giữ trạng thái MCP | 🔴 P0 | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-002](./CR-MCP-002-gateway-mcp-endpoint-wiring.md) | api-gateway chưa có route/cấu hình cho `/mcp` | 🔴 P0 | Small | ✅ Implemented (unit/integration tests) — see service README for gaps |

Thứ tự: `CR-MCP-001 → CR-MCP-002`. Cả hai là điều kiện tiên quyết của mọi feature còn lại.

**Đã chốt (2026-10-01):** `mcp-service` Postgres-only tạm thời (Q-D2); allow-list Origin `WS_ALLOWED_ORIGINS`/`MCP_ALLOWED_ORIGINS` được phê duyệt (Q-D3); tenant mới bật mặc định qua `MCP_TENANT_DEFAULT_ENABLED=true`, `MCP_ENABLED` vẫn là công tắc tổng mặc định `false` (Q-D6).
