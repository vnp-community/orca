# Request Lifecycle — Change Requests (v6)

> Vòng đời của Request: máy trạng thái và registry luồng, tiếp nhận từ nguồn, phân loại và đổi loại, trả backlog, mở lại, hủy, Request con. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-003](./CR-REQ-003-request-state-machine-and-flow-registry.md) | Chưa có nơi duy nhất quyết định Request đi qua trạng thái nào theo loại | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-004](./CR-REQ-004-request-intake-from-sources.md) | Không có đường tạo Request từ Jira, GitHub, thủ công, webhook, MCP, và không có idempotency theo nguồn | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-005](./CR-REQ-005-request-classification-and-type-change.md) | Chưa có phân loại AI đề xuất, người xác nhận, đổi loại có lịch sử | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-006](./CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md) | Chưa có đường ra khỏi luồng: trả backlog, mở lại, hủy, sinh Request con | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-001, 002 (foundation)
      └──▶ CR-REQ-003 ──▶ CR-REQ-004 ──▶ CR-REQ-005 ──▶ CR-REQ-006
```

| Bước | Lý do thứ tự |
|------|--------------|
| 003 trước | Mọi thay đổi `requests.status` đi qua use case `TransitionRequest` của CR-REQ-003; CR còn lại gọi nó, không tự ghi `status` |
| 004 sau 003 | Tạo Request xong phải chuyển `new` sang `classifying` bằng `TransitionRequest` |
| 005 sau 004 | Phân loại chạy trên Request đã tạo; dùng lại khoá nguồn để biết gợi ý từ Jira/GitHub |
| 006 cuối | Trả backlog và Request con cần đủ trạng thái và loại; mở lại quay về `classifying` (CR-REQ-005) |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| L1 | Chỉ `TransitionRequest` (CR-REQ-003) ghi cột `status`, `returned_from_stage`, `return_reason`; các CR khác truyền `Trigger` | Một nơi giữ bất biến; dễ kiểm thử bảng chuyển |
| L2 | Registry luồng là mã Go tĩnh trong `internal/domain`, không phải bảng DB | Đổi luồng đi cùng release và test; tránh cấu hình lệch giữa môi trường |
| L3 | Mọi lệnh ghi một Request là một transaction: kiểm tra, ghi, `version+1`, ghi `outbox_events` | At-least-once an toàn; không có sự kiện ma |
| L4 | Giao lặp là bình thường: lệnh đã áp dụng trả thành công, không lỗi | Callback và consumer outbox có thể lặp (README v6 mục 6) |
| L5 | Phân loại bắt buộc người xác nhận, không bỏ bước ở phiên bản đầu, kể cả `question` và `task` size S | Quyết định D5 và nghiên cứu mục 7 điểm 3 |
| L6 | Loại luôn đi qua `new → classifying → awaiting_type_confirmation` | README v6 mục 3.3 |
| L7 | Mọi lệnh ghi qua gateway kiểm quyền theo project; `request-service` tự kiểm tenant | README v6 mục 6 |

## Điểm lệch giữa README v6 và nghiên cứu, đã phát hiện khi viết feature này

- README v6 mục 3.3 ghi "đổi loại từ mọi trạng thái đang xử lý" nhưng không nói rõ tập trạng thái; CR-REQ-003 định nghĩa tập này (bảng ở mục 2.3 của CR đó).
- README v6 mục 3.4 đặt cổng `phase` (mỗi phase) và `pre_deploy` (ops_request, trước bước không đảo ngược) nhưng không nói chúng xảy ra ở trạng thái Request nào. CR-REQ-003 chọn: `phase` và `pre_deploy` của `ops_request` diễn ra khi `status = executing` (không đổi trạng thái Request); `pre_deploy` của `hotfix` và `security` chiếm `awaiting_plan_approval`. Cần xác nhận.
- README v6 mục 3.6 không có RPC nào để frontend đọc registry luồng (CR-REQ-019 cần biết các bước của loại đã chọn). CR-REQ-003 đề nghị thêm `GetRequestFlow`; xem Câu hỏi mở Q2 của CR đó.
- Nghiên cứu mục 5 ghi `returned_from_stage` gồm solution, plan, phase, task; README v6 dùng `classification`, `analysis`, `plan`, `phase`, `task`. Series theo README.
- Mở lại từ `request_backlog`: nghiên cứu ghi "quay về phân loại"; README v6 không có trạng thái đích cho `ReopenRequest`. CR-REQ-006 chọn `classifying`.
