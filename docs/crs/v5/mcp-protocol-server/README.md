# MCP Protocol Server — Change Requests (v5)

> Hiện thực **giao thức MCP chuẩn** phía server: Streamable HTTP, vòng đời `initialize`, session, SSE, huỷ/tiến độ.
> Hiện trạng: `agent/` chỉ có `tools/list`/`tools/call` mô phỏng, không có handshake hay transport chuẩn;
> `backend-go` chưa có gì.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-003](./CR-MCP-003-streamable-http-and-lifecycle.md) | Chưa có endpoint MCP, chưa có `initialize`/capabilities/version negotiation | 🔴 P0 | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-004](./CR-MCP-004-sessions-sse-resumability.md) | Chưa có session, SSE, resume, cancel, progress; chưa chạy được nhiều replica | 🟠 P1 | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |

Thứ tự: `CR-MCP-003 → CR-MCP-004`. CR-003 đủ để một client gọi tool đồng bộ; CR-004 cần cho tool dài và production đa replica.

**Đã chốt (2026-10-01):** SDK Go chính thức cho server; Python SDK chính thức làm client tham chiếu trong CI (Q-D4). Spike chỉ xác minh hook và ghi khoảng trống.
