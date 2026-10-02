# MCP Governance & Safety — Change Requests (v5)

> Cho một agent thao tác như người dùng là **mở rộng bề mặt tấn công**: agent có thể bị prompt-injection qua nội dung
> PR/issue/terminal, bị lặp vô hạn, hoặc tự gọi lại chính Orca. Feature này đặt các chốt chặn **ở server** (không dựa vào
> việc client MCP tự ngoan).
>
> Hiện trạng: README api-gateway tự ghi *"No OPA authorization check ahead of routing"*; có `policy/orca-authz` (OPA bundle)
> và `auth-service` có `opaclient` + audit log, nhưng chưa nối vào đường gọi tool.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-012](./CR-MCP-012-tool-policy-and-annotations.md) | Chưa có policy theo tool/tenant/scope; chưa có deny-list cứng | 🔴 P0 | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [CR-MCP-013](./CR-MCP-013-approvals-audit-killswitch.md) | Chưa có phê duyệt của người dùng, audit, kill switch, chống đệ quy/prompt-injection | 🔴 P0 | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |

Thứ tự: `CR-MCP-012 → CR-MCP-013`. **Cả hai là điều kiện bắt buộc** trước khi bật tool ghi/thực thi/phá huỷ cho người dùng thật.

**Đã chốt (2026-10-01):** tenant mới bật mặc định (`MCP_TENANT_DEFAULT_ENABLED=true`) mà không đổi mặc định rủi ro (Q-D6); deep link Web Push `/?section=mcp&tab=approvals&approval=<id>`, push thật phụ thuộc CR-NOTIF-002 (Q-D5).
