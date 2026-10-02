# Phê duyệt hành động của agent

Một số công cụ MCP (chạy lệnh, thao tác phá hủy, ...) theo chính sách mặc định **phải được bạn
phê duyệt** trước khi chạy. Khi agent gọi chúng, Orca hiện hộp thoại **"An AI agent is asking
permission"**. **Không có gì chạy cho tới khi bạn bấm Approve.**

## Đọc hộp thoại

- **Tool / client**: công cụ nào và agent nào (tên client, mã phiên rút gọn).
- **Mức rủi ro**: `exec`, `destructive`, ... Mức càng cao, nút **Approve** bị khóa càng lâu
  (đếm ngược "Approve (2s)" — chỉ đếm khi cửa sổ đang hiển thị và có focus; quay lại cửa sổ
  sẽ đếm lại) để tránh bấm nhầm.
- **Exact tool arguments**: tham số **nguyên văn** sẽ được chạy, hiển thị dạng văn bản thuần
  (không phải HTML). Ký tự ẩn/điều khiển được hiện dạng `\u{...}` để bạn thấy mánh đánh lừa.
  Nhãn **Secrets hidden** nghĩa là server đã che giá trị bí mật khỏi bản xem trước.
- **Expires in**: thời gian còn lại (mặc định phía server là 10 phút, admin chỉnh được). Hết hạn
  thì yêu cầu bị từ chối tự động và nút bị vô hiệu.

## Bạn có thể

| Hành động | Kết quả |
|---|---|
| **Approve** | Chỉ tính khi bạn *tự bấm chuột* (click do script không có tác dụng). Orca gửi kèm mã băm tham số (`paramsHash`) của đúng yêu cầu bạn đã xem. |
| **Deny** | Lời gọi của agent kết thúc với lỗi (`isError`). Focus mặc định đặt ở Deny; Enter không phê duyệt. |
| **Decide later** | Đóng hộp thoại, yêu cầu vẫn chờ trong tab **Approvals**. |

## Trường hợp đặc biệt

- **"This request changed. Review it again."** — nội dung yêu cầu đổi sau khi bạn mở (mã băm không
  khớp, `MCP_APPROVAL_HASH_MISMATCH`). Orca tải lại phiên bản mới nhất; bạn phải đọc lại và bấm lại.
- **"This request expired" / "Already handled (another device)"** — hết hạn hoặc đã xử lý ở thiết bị khác.
- **"MCP access is suspended."** — kill switch đang bật; không thể quyết định cho tới khi admin tắt.
- Nhiều yêu cầu cùng lúc: hộp thoại hiển thị "Request i of n"; xử lý lần lượt.

## Thông báo đẩy và deep link

- Khi Orca đang mở, hộp thoại là đường chính (nhận qua luồng sự kiện, không cần tải lại trang).
- Khi cửa sổ không focus, thông báo đẩy (Web Push) có thể dẫn tới
  `/?section=mcp&tab=approvals&approval=<id>`; bấm vào thông báo sẽ mở Settings → MCP → Approvals
  và đưa yêu cầu lên đầu.
- **Hạn chế đã biết:** phía backend chưa có usecase gửi push thật (CR-NOTIF-002), nên hiện chỉ đường
  "Orca đang mở" hoạt động ổn định.
