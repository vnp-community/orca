# MCP server bên ngoài và secret

Tab **External servers** cho phép đăng ký một MCP server khác (HTTP hoặc stdio) để Orca dùng làm
nguồn công cụ. Phạm vi: `tenant` (admin), `team`, hoặc `user` (người dùng thường chỉ quản lý server
phạm vi của mình).

## Quy trình

1. **Add server**: tên, loại (http/stdio), URL hoặc lệnh+đối số, tên biến môi trường/header cần secret.
2. **Probe**: server Orca kết nối thử (có chặn SSRF — `MCP_SERVER_SSRF_BLOCKED`), liệt kê công cụ và
   tính **digest** danh sách công cụ.
3. **Review** (admin): duyệt hoặc từ chối dựa trên danh sách công cụ. Server `stdio` **luôn** cần
   admin duyệt. Nếu công cụ thay đổi sau này, trạng thái chuyển lại cần duyệt (diff được hiển thị;
   `MCP_SERVER_DIGEST_MISMATCH` nếu duyệt bản đã cũ).

## Secret: điều cần biết rõ (đừng hiểu nhầm)

- Secret (API key, header Authorization, biến môi trường) được nhập ở ô riêng và gửi **một lần**
  tới server Orca qua **WebSocket/TLS đã xác thực** (`mcp.externalServer.setSecret`).
- Server **mã hoá khi lưu** (credential-broker, Vault Transit). Giá trị bị che trong log/trace và
  **không bao giờ được trả lại** cho giao diện; giao diện chỉ biết `hasSecret: true/false`.
- **Đây KHÔNG phải mã hoá đầu-cuối (end-to-end).** Server Orca nhìn thấy plaintext khi nhận và khi dùng
  để gọi server ngoài. Nếu bạn không tin quản trị hạ tầng Orca với secret đó, đừng nhập vào đây.
- Hệ quả vận hành: chỉ chạy Orca sau HTTPS/WSS; xoay (rotate) secret bằng cách nhập lại; xoá server
  sẽ xoá secret liên quan. Giao diện không lưu secret vào store hay storage của trình duyệt.

## Lỗi hay gặp

| Mã | Nghĩa |
|---|---|
| `MCP_SERVER_INVALID` | cấu hình sai (URL, tên, ...) |
| `MCP_SERVER_NAME_CONFLICT` | trùng tên trong cùng phạm vi |
| `MCP_SERVER_STDIO_NOT_ALLOWED` | chính sách không cho stdio ở phạm vi này |
| `MCP_SERVER_NOT_APPROVED` | chưa được admin duyệt |
