# Backlog của Request và Task

**Cập nhật:** 2026-10-08 · Đối chiếu với: `internal/domain/request_return_category.go`, `request_return_stage.go`, `backlog_view.go`,
`internal/adapter/grpc/server.go` (ghi chú `ListBacklog`), [CR v6 mục 3.8](../../crs/v6/README.md).

Backlog là **view tính toán**, không phải trạng thái backend riêng của Task (Task không có status `backlog`). Có ba view:

| View | Định nghĩa | Tình trạng |
|---|---|---|
| Request backlog | Request ở trạng thái `request_backlog` | trạng thái và các lệnh có; **RPC `ListBacklog` chưa có handler** (trả `Unimplemented`, CR-REQ-006/015) |
| Task backlog | Task dưới Plan chưa được duyệt, hoặc Plan chưa chia Phase | **chưa có**: không tìm thấy trong mã `task-service` hay `request-service`; thuộc CR-REQ-015 |
| Execute backlog | Task dưới Phase đã duyệt ở `open`/`blocked`, hoặc `execution_link` gần nhất `failed` | **chưa có**, như trên (CR-REQ-015) |

Trong domain đã có kiểu `BacklogView` (request, task, execute) và `BacklogRequestRow`, nhưng chỉ là kiểu dữ liệu.

## Request backlog

### Vào backlog

| Cách vào | Ghi chú |
|---|---|
| `ReturnToBacklog` | người dùng trả về, kèm danh mục lý do, giai đoạn, lời giải thích |
| Bị từ chối ở Approval (`Reject`) | danh mục `rejected` |
| Approval hết hạn | do bộ quét, ghi audit `approval.expire` |

Giai đoạn trả về hợp lệ phụ thuộc trạng thái hiện tại (`StageForStatus`): `classifying`/`awaiting_type_confirmation` → `classification`;
`analyzing`/`awaiting_analysis_approval` → `analysis`; `planning`/`awaiting_plan_approval` → `plan`; `executing` → `task`, hoặc `phase` hoặc `task`
khi flow chia Phase.

Danh mục lý do (`returned_category`): `missing_info`, `infeasible`, `blocked_dependency`, `rejected`, `other`.
Giai đoạn (`returned_from_stage`): `classification`, `analysis`, `plan`, `phase`, `task`. Mỗi lần trả về ghi vào `request_return_history`.

### Ra khỏi backlog

`ReopenRequest` mở lại Request và **phân loại lại** (bộ đếm `classification_attempts` được đặt lại). `CancelRequest` huỷ hẳn.
Jira không bị đổi khi Request vào `request_backlog` hoặc `cancelled` ([Jira ↔ Orca](../jira/jira-orca-mapping.md)).

### Xem danh sách tạm thời

Cho tới khi `ListBacklog` có, dùng `ListRequests` (kênh `request.list`) rồi lọc theo trạng thái `request_backlog` ở phía gọi; `GetRequest`
trả lý do trả về. Các lệnh `ReturnToBacklog`, `ReopenRequest`, `CancelRequest` có kênh WS `request.returnToBacklog`, `request.reopen`, `request.cancel`.

Khi cờ luồng tắt: `ReturnToBacklog` và `CancelRequest` vẫn chạy (thoát an toàn); `ReopenRequest` bị chặn.
