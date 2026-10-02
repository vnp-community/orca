# Kết nối Claude Code với Orca

> **Cần xác minh thủ công.** Cú pháp lệnh/cấu hình của client bên thứ ba dưới đây do giao diện
> Orca sinh ra (`frontend/src/renderer/src/lib/mcp-connect-snippets.ts`, ghi chú "chưa đối chiếu
> với tài liệu hiện hành của client"). Trước khi dùng cho người khác, hãy đối chiếu với tài liệu
> Claude Code phiên bản bạn cài (`claude mcp add --help`).

## Cách 1 — OAuth (khuyến nghị cho người dùng có trình duyệt)

1. Mở **Settings → MCP → Connect**, chọn **Claude Code**, sao chép đoạn lệnh (chế độ OAuth):

   ```bash
   claude mcp add --transport http orca <URL MCP của Orca>
   ```

2. Chạy lệnh trong terminal. Lần đầu dùng, Claude Code mở trình duyệt tới trang
   **Authorize Claude Code** của Orca (`/oauth/consent`).
3. Kiểm tra tên ứng dụng và host chuyển hướng ("Will redirect to ..."), bỏ chọn quyền không
   cần (quyền rủi ro cao như chạy lệnh mặc định **không** được chọn sẵn), rồi **Allow access**.
4. Quay lại Claude Code, dùng `/mcp` để xem trạng thái kết nối.

## Cách 2 — Access token (CI, máy không có trình duyệt)

1. Tạo token ở tab **Access tokens** ([hướng dẫn](./personal-access-tokens.md)).
2. Đặt token vào biến môi trường rồi thêm server (giao diện hiển thị sẵn mẫu cho bash/PowerShell):

   ```bash
   export ORCA_MCP_TOKEN='<dán token>'
   claude mcp add --transport http orca <URL MCP của Orca> --header "Authorization: Bearer $ORCA_MCP_TOKEN"
   ```

Giao diện **không bao giờ** tự điền token thật vào đoạn mã sao chép (chỉ có `<YOUR_ACCESS_TOKEN>`).

## Xác nhận kết nối

Vào **Settings → MCP → Connect**: phiên của Claude Code xuất hiện trong danh sách phiên đang hoạt động
(số lần gọi công cụ, lần cuối thấy). Bạn có thể đóng phiên từ đây.

## Lỗi thường gặp

| Hiện tượng | Nguyên nhân thường gặp | Cách xử lý |
|---|---|---|
| Giao diện cảnh báo URL không an toàn | URL MCP không phải HTTPS (và không phải localhost) | Nhờ admin cấu hình HTTPS; không nối qua HTTP thường |
| 401 / token hết hạn | PAT quá hạn hoặc đã bị thu hồi | Tạo token mới |
| Mọi lời gọi bị chặn, banner "paused" | Admin bật kill switch | Chờ admin tắt; liên hệ admin |
| Trang consent báo "has expired" | Yêu cầu đăng nhập OAuth quá hạn | Chạy lại lệnh để bắt đầu lại |
| Trang consent báo "turned off" | MCP đang tắt cho tổ chức | Hỏi admin |
