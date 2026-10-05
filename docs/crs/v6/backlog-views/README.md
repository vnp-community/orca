# Backlog Views — Change Requests (v6)

> API đọc cho ba view backlog (D4): Request backlog, Task backlog, Execute backlog. Backlog là view tính toán, không thêm status backend. Xem hợp đồng chung ở [README v6](../README.md) mục 3.8.

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-015](./CR-REQ-015-backlog-read-views.md) | Chưa có truy vấn nào cho ba view; `task-service` không có RPC trả link thực thi và cạnh phụ thuộc theo lô | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-002 (requests, request_links)  ─┐
CR-REQ-009 (approvals)                 ├──▶ CR-REQ-015 ──▶ CR-REQ-016 (kênh backlog.*) ──▶ CR-REQ-023 (màn hình Backlog)
CR-REQ-011 (request_id, lọc ListTasks) │
CR-REQ-013 (task_run_outcomes)        ─┘
```

View REQUEST chỉ cần CR-REQ-002 nên có thể giao trước; hai view task cần CR-REQ-011, 013 và Approval.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| B1 | Một RPC `ListBacklog` với `view` ∈ {REQUEST, TASK, EXECUTE} | README 3.6; một kênh `backlog.*` |
| B2 | Truy vấn ở `request-service`; `task-service` chỉ thêm `ListExecutionStates` | Approval và loại Request ở `request-service`; không đưa chúng sang `task-service` |
| B3 | "Cổng đã duyệt" xác định theo registry (`FlowFor`) và `Approval`, hai view task loại trừ nhau | Mỗi task đúng một nơi; áp được cho loại không có Phase |
| B4 | Phân trang theo Request, tối đa 100 task mỗi Plan | Nhóm không vỡ giữa các trang |
| B5 | `task-service` không sẵn sàng thì view TASK/EXECUTE báo lỗi, không trả dữ liệu từng phần | Tránh nhầm với backlog rỗng |

## Phạm vi ngoài feature này

Màn hình Backlog (CR-REQ-023), kênh WS/HTTP (CR-REQ-016), cột Board `backlog` của frontend (CR-REQ-018). Việc mở lại và huỷ từ Request backlog là CR-REQ-006.

## Điểm lệch giữa README v6 và code, phát hiện khi viết feature này

- Execute backlog "dưới Phase đã approved" không phủ các loại không có Phase; CR-REQ-015 mở rộng bằng khái niệm cổng của task (cần xác nhận).
- README v6 3.5 không có `returned_category` và bảng `request_return_history` (CR-REQ-006 đã thêm); view Request backlog dựa vào hai thứ đó để gom theo lý do và hiện người trả, thời điểm trả.
- `execution_links` không lưu thông điệp lỗi; cột "lý do lỗi gần nhất" lấy từ `task_run_outcomes` của CR-REQ-013 nên chỉ có cho task thuộc Request.
- `ListTasks` không lọc theo grant người gọi; lọc quyền phải làm ở `request-service`.
