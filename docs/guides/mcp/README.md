# MCP trong Orca — hướng dẫn người dùng và quản trị

**Cập nhật:** 2026-10-05 · Đối chiếu với: `frontend/src/renderer/src/components/settings/mcp/*`,
`frontend/src/renderer/src/components/mcp/*`, `backend-go/services/mcp-service/README.md`,
[CONTRACT-mcp-ui-api.md](../../../specs/backend-go/crs/v5/CONTRACT-mcp-ui-api.md).

MCP (Model Context Protocol) cho phép một **agent AI bên ngoài** (Claude Code, Claude Desktop,
Cursor, ...) gọi các công cụ của Orca (đọc dữ liệu, thao tác terminal, ...) với **danh nghĩa của
bạn** và **trong phạm vi quyền bạn cho phép**. Toàn bộ phần quản lý nằm ở **Settings → MCP**.

## Ai cần đọc gì

| Bạn là | Đọc |
|---|---|
| Người dùng muốn nối một agent vào Orca | [Claude Code](./connect-claude-code.md), [Claude Desktop](./connect-claude-desktop.md), [Cursor](./connect-cursor.md) |
| Người dùng chạy script/CI không có trình duyệt | [Personal Access Token](./personal-access-tokens.md) |
| Người dùng được hỏi "agent xin quyền chạy lệnh" | [Phê duyệt hành động của agent](./approving-agent-actions.md) |
| Quản trị viên (admin) | [Hướng dẫn quản trị](./admin-guide.md) |
| Quản trị viên muốn nối MCP server bên ngoài | [MCP server bên ngoài và secret](./external-servers-and-secrets.md) |
| Người dùng/agent muốn giao việc cho task và worktree qua MCP | [Task và worktree qua MCP](./task-worktree-tools.md) |

## Luồng kết nối (tóm tắt)

```
Agent (Claude Code/Desktop/Cursor)
   │  1. gọi <URL MCP của Orca> (tab Connect hiển thị URL này)
   ▼
Đăng nhập Orca trên trình duyệt  ──►  trang "Authorize <ứng dụng>" (/oauth/consent)
   │  2. bạn chọn quyền (scope) rồi bấm "Allow access"
   ▼
Agent nhận token  ──►  gọi công cụ  ──►  việc nguy hiểm (exec/destructive) hỏi bạn lại
```

Cách khác không cần trình duyệt: tạo **Access token** (PAT) ở tab *Access tokens* và đưa token
vào cấu hình agent.

## Điều kiện để thấy mục MCP

- Mục **MCP** chỉ xuất hiện trong Settings khi máy chủ Orca đã bật MCP (`server.info.enabled`)
  **và** tổ chức của bạn chưa tắt MCP. Nếu không thấy, hãy hỏi quản trị viên.
- Quản trị viên vẫn thấy mục này khi MCP đang tắt để có chỗ bật lại.
- Khi quản trị viên bật **kill switch**, đầu mọi tab có banner "MCP access is paused by an
  administrator." — các agent bị chặn tạm thời, bạn vẫn thu hồi được token/ứng dụng.

## Các tab trong Settings → MCP

| Tab | Ai thấy | Dùng để |
|---|---|---|
| Connect | mọi người | lấy URL và đoạn cấu hình cho từng client; xem phiên agent đang hoạt động |
| Connected apps | mọi người | xem/thu hồi ứng dụng đã được cấp quyền (OAuth) |
| Access tokens | mọi người | tạo/thu hồi PAT |
| Approvals | mọi người | hộp thư các yêu cầu phê duyệt |
| External servers | mọi người (phạm vi riêng) / admin (toàn tổ chức) | nối MCP server bên ngoài |
| OAuth clients, All grants, Tools, Prompts, Policies, Audit log | chỉ admin | quản trị, xem [admin-guide](./admin-guide.md) |

> Lưu ý: tên tab trong bảng là nhãn tiếng Anh hiện tại của giao diện; nếu giao diện đổi nhãn, tài
> liệu cần cập nhật theo (chưa có kiểm tra tự động).
