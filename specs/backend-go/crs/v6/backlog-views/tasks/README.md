# backlog-views: tasks backend (TASK-REQ-015-01 đến 06)

> **📋 Proposed.** Mọi task `[ ] TODO`, chưa triển khai, chưa chạy test. Lệnh chạy từ `/opt/repos/orca/backend-go`; integration cần Docker (tag `integration`). Quy ước: `gitnexus_impact` trước khi sửa symbol có sẵn; không `max-lines` disable; không tên `helpers`/`utils`/`common`/`misc`.

## Bảng Solution → Task

| Solution | Task | Ghi chú |
|---|---|---|
| [BE-REQ-SOL-015](../solutions/BE-REQ-SOL-015-backlog-read-views.md) | 015-01 `ListExecutionStates` proto + usecase (task-service) · 015-02 adapter Postgres/MySQL + integration · 015-03 `backlog_gate` + `page_token` (domain thuần) · 015-04 view REQUEST (keyset, links, lịch sử trả) · 015-05 view TASK/EXECUTE lắp ráp · 015-06 RPC `ListBacklog`, lọc quyền, wiring | 01, 02 ở `task-service`; 03 đến 06 ở `request-service` |

## Sơ đồ phụ thuộc

```
015-01 ─▶ 015-02 ─────────────────────────────┐
015-03 ─▶ 015-04 ──────────────────────────────┤
          (015-03 cũng cần cho 015-05)         ▼
015-01, 015-02, 015-03, 015-04 ─────────▶ 015-05 ─▶ 015-06
   ngoài feature: SOL-011 task 04 (ListTasks lọc), SOL-013 task 03 (task_run_outcomes),
                  CR-REQ-002/006/009 (requests, return history, approvals)
```

015-01 và 015-03 làm song song được; view REQUEST (015-03 và 015-04) chỉ cần CR-REQ-002 và 006 nên giao trước.

## Quyết định chung

- Cả bốn task `request-service` dùng chung `TaskClient` với SOL-013 (`task_client.go`): thêm phương thức, không tạo kết nối thứ hai.
- Mọi truy vấn lọc `tenant_id`; hai dialect cùng kết quả (keyset khai triển, không so sánh hàng).
- Quyền xem Request lọc ở `request-service` (015-06) vì `task-service` không lọc theo grant.
- `task-service` không sẵn: TASK/EXECUTE trả `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`.

## Điểm cần người điều phối chú ý

- Chính sách "ai xem được Request" chưa chốt (CR-REQ-003/010/016): 015-06 dùng cổng `RequestVisibility`.
- Bảng cổng của task (015-03) cần chủ CR-REQ-015 xác nhận từng dòng.
- Kênh `backlog.*` và ánh xạ MCP (`parity_test.go`) thuộc CR-REQ-016, 017.
