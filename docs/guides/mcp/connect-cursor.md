# Kết nối Cursor với Orca

> **Cần xác minh thủ công.** Định dạng `mcpServers` / `url` / `headers` dưới đây do giao diện
> Orca sinh ra (`mcp-connect-snippets.ts`) và **chưa được đối chiếu** với tài liệu Cursor hiện
> hành (vị trí tệp cấu hình, tên trường, việc Cursor có tự chạy luồng OAuth hay không).

1. **Settings → MCP → Connect**, chọn **Cursor**, sao chép khối JSON. Mẫu OAuth:

   ```json
   { "mcpServers": { "orca": { "url": "<URL MCP của Orca>" } } }
   ```

   Mẫu dùng token thêm `"headers": { "Authorization": "Bearer <YOUR_ACCESS_TOKEN>" }`.
2. Dán vào cấu hình MCP của Cursor (theo tài liệu Cursor), tải lại cấu hình.
3. Nếu Cursor mở trình duyệt, kiểm tra tên ứng dụng/host rồi **Allow access** (xem
   [Claude Code](./connect-claude-code.md#cách-1--oauth-khuyến-nghị-cho-người-dùng-có-trình-duyệt)).
4. Xác nhận phiên ở tab **Connect**; lỗi thường gặp xem
   [Claude Code](./connect-claude-code.md#lỗi-thường-gặp).
