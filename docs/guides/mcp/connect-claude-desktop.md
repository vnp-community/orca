# Kết nối Claude Desktop với Orca

> **Cần xác minh thủ công.** Cấu hình dưới đây là mẫu do giao diện Orca sinh ra
> (`mcp-connect-snippets.ts`) và **chưa được đối chiếu** với phiên bản Claude Desktop hiện hành.
> Đặc biệt: gói `mcp-remote` và cờ `--header` là giả định của chúng tôi. Giao diện cũng gợi ý
> rằng một số bản Claude Desktop cho thêm server từ xa qua màn hình *Connectors* thay vì sửa tệp
> cấu hình — hãy kiểm tra bản bạn đang dùng.

## Cách thực hiện (theo mẫu của tab Connect)

1. **Settings → MCP → Connect**, chọn **Claude Desktop**, sao chép khối JSON.
2. Dán vào tệp cấu hình MCP của Claude Desktop (mục `mcpServers`). Mẫu OAuth:

   ```json
   {
     "mcpServers": {
       "orca": { "command": "npx", "args": ["-y", "mcp-remote", "<URL MCP của Orca>"] }
     }
   }
   ```

   Mẫu dùng token thêm đối số `--header "Authorization: Bearer <YOUR_ACCESS_TOKEN>"` — tự thay
   bằng token của bạn (xem [PAT](./personal-access-tokens.md)); **đừng** commit tệp có token.
3. Khởi động lại Claude Desktop. Với OAuth, trình duyệt sẽ mở trang **Authorize** của Orca.
4. Kiểm tra phiên ở tab **Connect**.

## Lỗi thường gặp

Giống [Claude Code](./connect-claude-code.md#lỗi-thường-gặp). Ngoài ra: cần Node.js (`npx`) trên
máy nếu dùng mẫu `mcp-remote`.
