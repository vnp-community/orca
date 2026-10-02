# MCP Quality & Rollout — Change Requests (v5)

> "Đầy đủ theo chuẩn" phải **chứng minh được** bằng test, không chỉ bằng tuyên bố. Feature này gom conformance, e2e với
> agent thật, observability, tài liệu và kế hoạch bật dần.

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MCP-015](./CR-MCP-015-conformance-e2e-observability-rollout.md) | Chưa có bộ kiểm chứng chuẩn, e2e agent, metrics/tracing, tài liệu, kế hoạch rollout | 🟠 P1 | Medium (xuyên suốt) | 🟡 Partially implemented — see specs/backend-go/crs/v5/mcp-quality-rollout/solutions |

Chạy song song với các feature khác; là **gate** trước khi bật `MCP_ENABLED` ở production.

**Đã chốt (2026-10-01):** job Python dùng SDK `mcp` chính thức (`backend-go/ci/mcp-conformance/`) làm client tham chiếu (Q-D4); GA: tenant mới bật mặc định, an toàn nhờ mặc định rủi ro/hard-deny/scope/kill switch không đổi và có test (Q-D6).
