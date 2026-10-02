# Access token (PAT) cho script và CI

PAT cho phép script/CI dùng MCP của Orca **không cần trình duyệt**. Quản lý ở
**Settings → MCP → Access tokens**.

## Tạo token

1. **Create token** → nhập **Name** (tối đa 80 ký tự), chọn **quyền (scope)** và **thời hạn**.
2. Quyền: `orca:read` (đọc), `orca:write` (thay đổi có thể hoàn tác), `orca:exec` (chạy lệnh),
   `orca:admin` (chỉ admin mới chọn được). Chỉ cấp quyền tối thiểu cần dùng.
3. Thời hạn tối đa do admin đặt (`maxTokenDays`, mặc định tạo theo cấu hình tổ chức); chọn quá mức
   sẽ báo `MCP_TOKEN_TOO_LONG` ("Maximum lifetime is N days."). Có giới hạn số token đang hoạt động
   (`MCP_TOKEN_LIMIT`).
4. Sau khi tạo, hộp thoại **"Copy your token now"** hiện **một lần duy nhất**:
   - Giá trị bị ẩn mặc định; bấm **Show** để xem hoặc **Copy token** để chép.
   - Phải tick **I have saved this token** thì mới bấm **Done** được; Esc không đóng hộp thoại
     khi chưa xác nhận.
   - Sau khi đóng, **không thể xem lại**: danh sách chỉ giữ tên, quyền, ngày tạo/hết hạn, lần dùng cuối.
     Mất token thì tạo token mới.

## Vì sao secret chỉ hiện một lần

Server chỉ trả secret đúng một lần ở lời gọi tạo. Giao diện không lưu secret vào store, localStorage,
sessionStorage, URL hay log (có test tự động kiểm tra điều này). Hãy lưu vào trình quản lý secret
hoặc biến môi trường CI; không dán vào mã nguồn hay chat.

## Dùng token

Xem mẫu lệnh ở cuối tab (bash/PowerShell), hoặc dùng REST tương đương cho script:
`POST/GET/DELETE /v1/auth/mcp-tokens` (cùng kiểu dữ liệu với giao diện — CONTRACT §3).

## Thu hồi

Bấm **Revoke** ở dòng token, xác nhận. Mọi thứ đang dùng token sẽ ngừng hoạt động trong khoảng một
phút. Thu hồi **không hoàn tác**. Thu hồi vẫn dùng được khi kill switch đang bật (chỉ *tạo mới* bị chặn).

## Token sắp hết hạn

Dòng có nhãn **Expires soon** khi còn dưới 7 ngày — tạo token thay thế trước hạn.
