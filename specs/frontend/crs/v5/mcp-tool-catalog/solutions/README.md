# frontend Solutions — MCP Tool Catalog (v5)

**CRs:** [docs/crs/v5/mcp-tool-catalog](../../../../../../docs/crs/v5/mcp-tool-catalog/README.md) (CR-MCP-007 mục E, CR-MCP-008 "UI cấu hình pack", CR-MCP-009 "UI hiển thị phiên terminal do agent tạo" đều ghi là việc frontend)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE v5:** [crs/v5/README.md](../../README.md) · **Phía backend:** [specs/backend-go/crs/v5/mcp-tool-catalog/solutions](../../../../../backend-go/crs/v5/mcp-tool-catalog/solutions/README.md)

Feature này **có UI**: 1 tab admin chỉ đọc + nhãn/điều khiển trên terminal. BE-MCP-SOL-007 (kiểm kê channel, `ToolSpec`, `ToolExecutor`) và BE-MCP-SOL-008 (nội dung pack) không có màn hình riêng — chỉ nuôi dữ liệu cho FE-005.

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-005](./FE-MCP-SOL-005-tool-catalog-admin-view.md) | CR-MCP-007/008 (UI) | Tab "Tools" trong `McpSettingsPane` (admin, chỉ đọc): bảng `McpToolView` nhóm theo namespace/pack/risk, lọc, badge rủi ro + quyết định hiệu lực + nguồn, khoá hard-deny, liên kết sang policy (FE-MCP-SOL-008) | Medium (≈2–3 ngày) | ✅ Implemented — see Gaps |
| [FE-MCP-SOL-006](./FE-MCP-SOL-006-agent-origin-badge-in-terminal-ui.md) | CR-MCP-009 (UI) | Nhãn "Tạo bởi agent <clientName>" trên tab terminal + danh sách độc lập + nút dừng; types additive (`origin`) | Medium (≈2–3 ngày) | ✅ Implemented — see Gaps |

Cả hai **không** định nghĩa lại phần nền của FE-MCP-SOL-001 (`shared/mcp-types.ts`, `window.api.mcp.*`, `store/slices/mcp-slice.ts`, section Settings `mcp`, `McpSettingsPane`); FE-005 chỉ thêm 1 tab + sub-namespace `window.api.mcp.admin.tool`, FE-006 thêm 1 slice nhỏ riêng và không đụng `McpSettingsPane` ngoài một khối danh sách.

## Tính nhất quán BE ↔ FE (kiểm chéo BE-MCP-SOL-007/008/009)

| Chủ đề | Giá trị dùng ở cả hai phía |
|---|---|
| Kênh | `mcp.admin.tool.list` (`{namespace?, risk?}` → `McpToolView[]`); dừng terminal dùng kênh **có sẵn** `terminal.stop`/`terminal.close` (`{terminal: ptyId}`) — không có kênh `mcp.*` mới |
| Trường `McpToolView` | `name, channel, title, description, namespace, risk, requiredScope, pack(1..4), hardDenied, effective, effectiveSource, annotations{readOnly,destructive,idempotent,openWorld}` — BE sinh từ `ToolSpec` + `mcp-service`; mục `listedAsHardDenied` (YAML) trả `hardDenied:true, effective:'deny', effectiveSource:'hard_deny', pack:4, risk:'admin'` |
| `origin` | `{type:'mcp'; clientName; mcpSessionId; userId}`, `omitempty` trên `terminal.list`, `terminal.create` (`terminal.…`), `agent.start/resume/switchAccount` (BE-009 §F) |
| Lỗi | `MCP_NOT_ADMIN`, `MCP_DISABLED` (tool.list); dạng `"CODE: msg"` |
| Hình dữ liệu | BE-009 ghi nhận `terminal.list` trả mảng trần; FE-006 chuẩn hoá cả `{terminals:[…]}` lẫn mảng trần |

## Re-verify: hiện trạng frontend so với giả định

| # | Giả định | Thực tế (đã đọc mã) | Drift |
|---|---|---|---|
| 1 | Có component danh sách terminal/agent để gắn nhãn | Không có danh sách terminal "toàn cục"; terminal là tab cục bộ (`TerminalTab`, `shared/types.ts:846`) + `ptyIdsByTabId` (`store/slices/terminals.ts:443`); `SortableTab.tsx` là nơi vẽ tab | Có — FE-006 gắn vào `SortableTab` + thêm danh sách riêng |
| 2 | Kết quả `terminal.*` được ánh xạ vào store ở một chỗ | Không: `terminal.create` ack được đọc trong `components/terminal-pane/remote-runtime-pty-transport.ts` (`created.terminal.handle`); `terminal.list` được đọc ở `lib/agent-hibernation-coordinator.ts`/`lib/active-agent-note-target.ts` dưới hình `RuntimeTerminalListResult` | Có — ánh xạ rải rác ⇒ FE-006 không sửa các chỗ đó mà chuẩn hoá riêng |
| 3 | `terminal.list` trả `{terminals:…}` | backend-go trả mảng trần `[]terminalSessionView` (`channels_terminal.go`) | Có — drift sẵn có giữa FE runtime contract và backend-go |
| 4 | PTY do MCP tạo hiện thành tab | **(chưa xác minh)** cơ chế mirror `session.tabs` | FE-006 làm hai lớp (tab + danh sách) |
| 5 | Tab admin nằm trong `AdminOrgConsole` | Admin MCP nằm trong `McpSettingsPane` (FE-001); `AdminOrgConsole` là Company/Departments/Users/Teams/Policies/Audit | Quyết định thiết kế (README v5 §2) |
| 6 | Primitive `switch/alert` có | Không có (`ls components/ui`); `badge` không có `warning` | Dùng icon + chữ |
| 7 | Path `web-preload-api.ts` | `renderer/src/web/web-preload-api.ts` (README v5 ghi `runtime/web/…`) | Đường dẫn README lệch (đã ghi ở FE-003) |

## Thứ tự & phụ thuộc

```
FE-MCP-SOL-001 (nền: types, window.api.mcp, slice, McpSettingsPane) ─┬─► FE-MCP-SOL-005 (tab Tools)  ◄─ BE-007/008 (mcp.admin.tool.list)
                                                                     └─► FE-MCP-SOL-006 (origin)     ◄─ BE-009 (origin, terminal.list)
FE-MCP-SOL-005 ──(liên kết "Edit policy")──► FE-MCP-SOL-008 (tab policy, tên tab khớp khi merge)
FE-MCP-SOL-006 ──(danh sách dừng)──► FE-MCP-SOL-002 (panel phiên) hoặc tab `sessions` của FE-001
```
FE-005 và FE-006 độc lập nhau, làm song song. FE-006 vô hại khi BE-009 chưa có (không có `origin`).

## Sửa TDD kèm theo (tổng hợp)
`specs/frontend/tdd/v5`: admin MCP ở `McpSettingsPane`; thư mục `components/settings/mcp/`; slice `mcp-terminal-origin`; hình `terminal.list` thực tế.

## Thay đổi CONTRACT đề nghị
Không bắt buộc. Gợi ý tương lai (không chặn): `McpToolView.inputSchema?` để tab Tools xem tham số; `origin.sessionId`/`agentSessionId?` để UI dừng agent theo `agent.stop` thay vì theo PTY.
