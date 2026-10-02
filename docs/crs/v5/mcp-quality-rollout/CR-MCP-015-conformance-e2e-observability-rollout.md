# CR-MCP-015 — Conformance, e2e với agent thật, observability và rollout

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-015 |
| **Tên** | Bộ kiểm chứng tuân thủ chuẩn MCP, kịch bản e2e "agent thao tác như người dùng", metrics/tracing, tài liệu, và kế hoạch rollout |
| **Loại** | Chất lượng / vận hành |
| **Priority** | 🟠 P1 — gate trước GA |
| **Effort** | Medium (6–8 ngày, rải theo các CR khác) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🟡 Triển khai một phần (2026-10-02) — metrics/tracing, conformance Go + Python, rollout tests, runbook, sửa TDD; chưa: e2e agent, parity UI↔MCP, ADR/guides. Xem `specs/backend-go/crs/v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-*.md` |
| **Tác giả** | Khảo sát CI hiện có (`backend-go/ci`, Makefile `test-integration`) |
| **Phụ thuộc** | Tăng dần theo CR-003 → CR-013 |

---

## Giải pháp đề xuất

### A. Conformance giao thức

- **Đã chốt (D4):** thêm job Python dùng **MCP Python SDK chính thức** (PyPI `mcp`, phiên bản ghim trong `backend-go/ci/mcp-conformance/requirements.txt`) làm client tham chiếu — `initialize`, `tools/list`, `tools/call`, resumability, OAuth discovery qua `streamablehttp_client`/`ClientSession` (chi tiết API chưa xác minh — kiểm khi cài đặt); harness Go giữ cho test in-process.
- Chạy bộ kiểm chứng/Inspector chính thức của MCP (CLI chế độ không giao diện; tầng 2, không chặn) trong CI chống lại server dựng bằng `make dev-up`: vòng đời, `tools/*`, `resources/*`, `prompts/*`, phân trang, lỗi, version negotiation, SSE resume, huỷ/progress.
- Test auth: luồng OAuth đầy đủ với client giả lập (discovery → DCR → code+PKCE → token → gọi `/mcp`), cùng các ca âm tính ở CR-005.
- Khoá danh sách phiên bản spec hỗ trợ trong một file duy nhất; nâng spec = đổi file + chạy lại conformance.

### B. E2E "agent như người dùng"

Kịch bản có script (agent giả lập hoặc một LLM thật chạy theo lịch, không chặn PR):
1. Liệt kê project → mở worktree → đọc diff → tạo task → commit → đọc kết quả CI (đợt 1–2, CR-008).
2. Cùng thao tác qua UI (`/ws`) và qua MCP trên dữ liệu giống hệt ⇒ **so sánh trạng thái kết quả** (parity hành vi).
3. Đợt exec: chạy test trong terminal qua approval (CR-009/013).
4. Red-team (CR-013): PR/issue/file độc hại không thể dẫn tới hành động nguy hiểm không qua người duyệt.
5. Đa replica, ngắt kết nối giữa chừng, resume (CR-004).

### C. Observability

- Metrics (Prometheus, theo chuẩn service khác): `mcp_requests_total{method,tool,decision,result}`, `mcp_tool_duration_seconds`, `mcp_sessions_active`, `mcp_sse_streams_active`, `mcp_approvals_total{outcome}`, `mcp_policy_denials_total`, `mcp_auth_failures_total{reason}`.
- Tracing: một `trace_id` xuyên `MCP request → policy → Registry.Dispatch → gRPC service`; gắn vào audit (CR-013). Nhãn không chứa dữ liệu nhạy cảm/tham số thô.
- Alert gợi ý: tỷ lệ deny bất thường, đỉnh `exec`, kill switch được kích hoạt, nhiều lần thất bại auth từ một client.

### D. Tài liệu

- Guide người dùng ở `docs/guides/` (vi): kết nối Claude Desktop / Claude Code / Cursor / agent do Orca chạy; tạo PAT; thu hồi; cách duyệt.
- Tài liệu bảo mật cho admin: mô hình quyền, policy mặc định, deny-list, kill switch, audit.
- Cập nhật `docs/hld` (C4: thêm `mcp-service` + `mcpserver` adapter), `backend-go/README.md` (hàng mới trong bảng real/stub), `api-gateway/README.md`, và sửa mô tả "MCP layer — không tài liệu hoá ở đâu khác" ở [C4-code.md:1798](../../../hld/v1/C4-code.md) cho rõ phần của `agent/` và phần của `backend-go`.
- ADR: SDK (đã chốt: Go SDK chính thức cho server, Python SDK chính thức cho client kiểm thử — D4), vị trí authorization server, lựa chọn transport.

### E. Kế hoạch rollout

| Giai đoạn | Phạm vi | Điều kiện chuyển |
|-----------|---------|------------------|
| 0. Nội bộ | `MCP_ENABLED=true` ở dev; chỉ tool đọc | CR-001..007 xong, conformance xanh |
| 1. Dogfood | Đội Orca; đợt 1–2 | CR-005/006/012/013 xong; audit hoạt động; red-team qua |
| 2. Beta theo tenant | Tenant opt-in; thêm exec có approval | Không sự cố bảo mật trong N tuần; hạn mức/alert ổn |
| 3. GA | **Đã chốt (D6):** tenant mới bật mặc định (`MCP_TENANT_DEFAULT_ENABLED=true`), admin tắt được; đợt 4 vẫn tắt mặc định | Review bảo mật độc lập |

Ghi chú D6: default-on chỉ an toàn vì phê duyệt theo rủi ro, hard-deny list, scope và kill switch là mặc định bắt buộc, không bị cờ này thay đổi (có test); `MCP_ENABLED` vẫn là công tắc tổng, mặc định `false` tới khi qua gate này.

Rollback: tắt `MCP_ENABLED` hoặc kill switch tenant (CR-013); không migration dữ liệu không thể đảo ngược.

## Acceptance Criteria

- [ ] Conformance + auth + parity UI↔MCP chạy trong CI (nhánh tích hợp) và xanh.
- [ ] Dashboard + alert được dựng, có runbook ngắn.
- [ ] Tài liệu ở mục D hoàn tất; `README` các service phản ánh đúng thực tế (không "Real" cho thứ còn stub).
- [ ] Review bảo mật hoàn tất trước giai đoạn 2; kết quả ghi lại.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** e2e với LLM thật không ổn định ⇒ tách bản xác định (agent giả lập, chạy ở CI) khỏi bản LLM thật (chạy theo lịch, báo cáo, không chặn).
- **Ngoài phạm vi:** đánh giá chất lượng quyết định của LLM (evals mô hình).
