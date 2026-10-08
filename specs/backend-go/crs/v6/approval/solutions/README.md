# Solutions backend: approval (v6)

> ✅ Đã triển khai trong `request-service` (kiểm chứng 2026-10-08, 13/13 task; 010-05 ở `notification-service` 2026-10-07). Phạm vi backend của CR-REQ-009 và CR-REQ-010; một phần nhỏ ở `notification-service`. Hợp đồng chung: [README v6](../../../../../../docs/crs/v6/README.md), mục 8 thắng mục 3.

## Bảng CR, Solution, Task

| CR | Solution | Task | Ghi chú |
|----|----------|------|---------|
| [CR-REQ-009](../../../../../../docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md) | [BE-REQ-SOL-009](./BE-REQ-SOL-009-generic-approval-domain-and-api.md) | TASK-REQ-009-01 đến 06 | Bảng `approvals`, `ApprovalService`, `SubjectHandler` · ✅ 6/6 (2026-10-08) |
| [CR-REQ-010](../../../../../../docs/crs/v6/approval/CR-REQ-010-approval-authorization-notification-expiry.md) | [BE-REQ-SOL-010](./BE-REQ-SOL-010-approval-authorization-notification-expiry.md) | TASK-REQ-010-01 đến 07 | Chính sách, thông báo, hết hạn; sửa `notification-service` · ✅ 7/7 (2026-10-08) |

## Thứ tự phụ thuộc

```
CR-REQ-002, 003 ─▶ SOL-009 ─▶ SOL-010
                      │
                      └─▶ SOL-007, SOL-008 (handler solution, findings, answer), CR-REQ-005, 012, 013, 014
```
SOL-009 đi trước SOL-007 vì mọi handler cần `OpenApproval` và `SubjectHandler`.

## Quyết định chung

| # | Quyết định | Lý do |
|---|-----------|-------|
| P1 | `Approve`, cập nhật chủ thể, chuyển trạng thái Request cùng một transaction; hiệu ứng sang `task-service` qua outbox | Không trạng thái nửa vời |
| P2 | Thứ tự khoá cố định: Request trước, Approval sau (bổ sung so với CR) | Tránh khoá chết giữa `Approve` và đổi loại, huỷ |
| P3 | Một `pending` mỗi chủ thể bằng chỉ mục (Postgres partial, MySQL cột sinh) | Hai DB cùng ngữ nghĩa |
| P4 | `NoopSubjectHandler` chỉ chạy khi bật cờ `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` | Làm tuần tự các CR mà không bỏ lọt cổng ở production |
| P5 | Chính sách trong DB, Go thuần; chụp người duyệt lúc mở (`approval_approvers`) | Join portable hai DB |
| P6 | Làm giàu payload (`user_ids`, `title`) trước khi publish; `approval.requested` dùng `Durable` | `notification-service` chỉ biết `user_ids`; thông báo duyệt mất là cổng kẹt |
| P7 | Giờ DB cho `due_at`, hết hạn, nhắc | Nhiều replica |
| P8 | Agent/MCP không duyệt; không đăng ký tool duyệt cho tới khi có nhận biết nguồn gọi | Cổng tồn tại để người kiểm soát agent |

## Số migration

`request-service/migrations` chưa tồn tại lúc soạn. `NNNN_approvals` (TASK-REQ-009-01) và `NNNN_approval_policies` (TASK-REQ-010-01) lấy số kế tiếp theo thư mục thật. `notification-service` không có migration mới.

## Mâu thuẫn và điểm chưa kiểm chứng

- README 3.5 thiếu cột của `approvals` và định nghĩa `stage`; README 3.6 thiếu RPC quản trị chính sách và gia hạn.
- `Durable` và rule không `Locked` ở `notification-service` cần chủ service xác nhận; tên stream `REQUEST` chưa đối chiếu được (chưa có `request-service`).
- `ListUsers` có nhận `tenant_id` từ request: chưa kiểm chứng cách server dùng.
- Nhận biết nguồn gọi MCP chưa có.
