# frontend Solutions — MCP Protocol Server (v5)

**CRs:** [docs/crs/v5/mcp-protocol-server](../../../../../../docs/crs/v5/mcp-protocol-server/README.md) (CR-MCP-003/004)
**Hợp đồng bắt buộc:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE:** [crs/v5/README.md](../../README.md) · **Nền:** [FE-MCP-SOL-001](../../mcp-service-foundation/solutions/FE-MCP-SOL-001-mcp-frontend-foundation.md)
**Phía backend:** [BE-MCP-SOL-003](../../../../../backend-go/crs/v5/mcp-protocol-server/solutions/BE-MCP-SOL-003-streamable-http-and-lifecycle.md), [BE-MCP-SOL-004](../../../../../backend-go/crs/v5/mcp-protocol-server/solutions/BE-MCP-SOL-004-sessions-sse-resumability.md)

## Feature này có UI không?

**Có, nhỏ:** một tab "Connect" trong Settings > MCP (địa chỉ server, snippet kết nối, bảng phiên đang hoạt động). Giao thức Streamable HTTP/SSE/resume (CR-003/004) bản thân **không có UI** — client MCP của bên thứ ba nói chuyện trực tiếp với `/mcp`; trình duyệt không bao giờ gọi `/mcp` (cookie bị từ chối, CONTRACT §3).

## Re-verify (khảo sát mã frontend, 2026-10-01)

| Khẳng định | Kết quả | Lệch? |
|---|---|---|
| FE có chỗ gắn tab trong Settings | Chưa có gì cho MCP; FE-001 tạo `McpPane` + `MCP_TABS`. Mẫu tab: `AdminOrgConsole.tsx` dùng `components/ui/tabs` | Không |
| Bảng/xác nhận/copy có sẵn | `ui/table`, `ui/tabs`, `ui/checkbox`, `ui/tooltip`, `ui/skeleton`, `useConfirmationDialog` (`confirmation-dialog.tsx:120`), `navigator.clipboard.writeText` (đã dùng ở `workflow/ExecutionMonitor.tsx`), `toast` của `sonner`. **Không có** `switch`, `alert`, `alert-dialog` | Xác nhận README FE |
| Helper thời gian tương đối dùng chung | Không có (mỗi file tự viết, vd `LinearItemDrawer.tsx:104`) | Thêm `lib/mcp-relative-time.ts` |
| Làm tươi định kỳ | `installWindowVisibilityInterval` (`lib/window-visibility-interval.ts`) đúng nhu cầu | Không |
| `Mcp-Session-Id` có thể hiển thị | Không: là bí mật, DB BE chỉ lưu hash; UI nhận **row id** (BE-004 quyết định 2) | Ghi nhận ràng buộc |
| Đóng session của người khác (admin) | `mcp.session.close` chỉ cho chủ session (BE-004); admin dùng kill switch scope `session` (FE-008) | Ràng buộc UI: nút Close vô hiệu hoá trên hàng của người khác |
| Snippet theo client | Định dạng lệnh/JSON của Claude Code, Claude Desktop (qua `mcp-remote`), Cursor viết theo hiểu biết hiện tại, **chưa đối chiếu tài liệu hiện hành** | Chưa xác minh ⇒ bước kiểm thủ công bắt buộc trong FE-002 |

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-002](./FE-MCP-SOL-002-connect-panel-and-active-sessions.md) | CR-MCP-003/004 (phần FE) | `components/settings/mcp/McpConnectTab.tsx`, `hooks/useMcpSessions.ts`, `lib/mcp-connect-snippets.ts` | Small–Medium | 🔲 Designed — chưa implement |

## Thứ tự thực thi & phụ thuộc

```
FE-MCP-SOL-001 ─▶ FE-MCP-SOL-002
```

- Phần snippet chỉ cần `mcp.server.info` (BE-003) ⇒ phát hành được trước BE-004; bảng phiên cần BE-004 (khi chưa có: trạng thái "unavailable", không lỗi).
- `session.closed` real-time cần BE-013 (`mcp.events.subscribe`); không có thì dựa vào làm tươi 15 giây.
- Liên kết "Create an access token" chỉ hiện khi FE-004 đã thêm tab `tokens`.
