# backlog-views: solutions backend (BE-REQ-SOL-015)

> **✅ Đã triển khai (SOL-015 6/6 task, kiểm chứng 2026-10-08).**

Nguồn: [docs/crs/v6/backlog-views](../../../../../../docs/crs/v6/backlog-views/README.md). README v6 [mục 8](../../../../../../docs/crs/v6/README.md) thắng mục 3 khi mâu thuẫn (điều 7: Execute backlog mở rộng theo cổng của task).

## Bảng CR → Solution → Task

| CR | Solution | Service | Task |
|---|---|---|---|
| CR-REQ-015 API đọc ba view backlog | [BE-REQ-SOL-015](./BE-REQ-SOL-015-backlog-read-views.md) | `request-service` (`ListBacklog`), `task-service` (`ListExecutionStates`) | TASK-REQ-015-01 đến 06 ([tasks/README](../tasks/README.md)) |

## Thứ tự phụ thuộc

```
CR-REQ-002 (requests, request_links) ─┐
CR-REQ-006 (returned_category,        ├─▶ SOL-015 task 03, 04 (view REQUEST, giao trước)
            request_return_history)  ─┘
BE-REQ-SOL-011 (ListTasks lọc) ───────┐
BE-REQ-SOL-013 (task_run_outcomes) ───┼─▶ SOL-015 task 05 (view TASK, EXECUTE) ─▶ task 06 (RPC)
CR-REQ-009 (approvals) ───────────────┘
SOL-015 task 01, 02 (ListExecutionStates ở task-service) độc lập, làm trước
                                   SOL-015 ─▶ CR-REQ-016 (kênh backlog.*) ─▶ CR-REQ-023 (màn hình)
```

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| B1 | Một RPC `ListBacklog` với `view` ∈ {REQUEST, TASK, EXECUTE} | README 3.6; một kênh `backlog.*` |
| B2 | Truy vấn ở `request-service`; `task-service` chỉ thêm `ListExecutionStates` | Approval và loại Request ở `request-service`; không đưa sang `task-service` |
| B3 | "Cổng đã duyệt" theo `FlowFor` và `Approval`; hai view task loại trừ nhau | Mỗi task đúng một nơi; áp được cho loại không có Phase |
| B4 | Phân trang theo Request, tối đa 100 task mỗi Plan | Nhóm không vỡ giữa các trang |
| B5 | `task-service` không sẵn thì TASK/EXECUTE lỗi, không trả từng phần | Tránh nhầm với backlog rỗng |
| B6 | Lọc quyền xem ở `request-service` trước khi gọi `task-service` | `ListTasks`, `ListExecutionStates` chỉ kiểm tenant |

## Đã kiểm chứng và điểm khác CR gốc

- `execution_links` có `idx_execution_links_task (task_id, started_at DESC)` ở cả hai dialect; tên bảng Postgres có tiền tố `task.`, MySQL không.
- Không RPC nào hiện trả `execution_links`; `GetDependencies` chỉ cho một task: đúng như CR.
- README v6 mục 3.5 vẫn thiếu `returned_category`, `request_return_history` mà CR-REQ-006 đã thêm; SOL-015 giả định chúng tồn tại.
- Bảng cổng 2.4 của solution là mở rộng của CR, README mục 8 điều 7 chấp nhận hướng nhưng chưa xác nhận từng dòng.
- Chưa chạy test nào.

## Tham chiếu tiến (không phụ thuộc)

Các CR bổ sung 026 đến 036 (ReadinessGate ở CR-REQ-029, Clarification ở CR-REQ-028...) có thể muốn hiển thị trạng thái "chưa sẵn sàng" hay câu hỏi chờ trong backlog; SOL-015 chỉ thêm trường mới (additive) khi cần, không phụ thuộc.
