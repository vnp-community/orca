# Tasks backend: approval (v6)

> 📋 Proposed. Mỗi task 0,5 đến 2 ngày. Đường dẫn tương đối tới `backend-go/services/request-service/` (mới) trừ khi ghi khác.

## Bảng Solution, Task

| Solution | Task | Nội dung | Ưu tiên |
|----------|------|----------|---------|
| [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) | [TASK-REQ-009-01](./TASK-REQ-009-01-approvals-migration.md) | Migration `approvals` hai dialect | P0 |
| | [TASK-REQ-009-02](./TASK-REQ-009-02-approval-domain-state-machine.md) | Domain `Approval`, máy trạng thái | P0 |
| | [TASK-REQ-009-03](./TASK-REQ-009-03-approval-repository-postgres-mysql.md) | Repository hai dialect | P0 |
| | [TASK-REQ-009-04](./TASK-REQ-009-04-approval-usecases-and-subject-handler.md) | Use case, `SubjectHandler`, đăng ký | P0 |
| | [TASK-REQ-009-05](./TASK-REQ-009-05-approval-proto-and-grpc-server.md) | `approval.proto`, gRPC server | P0 |
| | [TASK-REQ-009-06](./TASK-REQ-009-06-approval-contract-tests-and-wiring.md) | Test hợp đồng handler, wiring `main.go` | P0 |
| [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) | [TASK-REQ-010-01](./TASK-REQ-010-01-approval-policy-migration.md) | Migration chính sách và người duyệt | P1 |
| | [TASK-REQ-010-02](./TASK-REQ-010-02-approval-policy-and-authorization-domain.md) | Domain chính sách, `Decide` | P1 |
| | [TASK-REQ-010-03](./TASK-REQ-010-03-policy-repository-and-resolver.md) | Repository, resolver, thay cài tạm | P1 |
| | [TASK-REQ-010-04](./TASK-REQ-010-04-recipient-expansion-and-notification-payload.md) | Mở rộng người nhận, payload thông báo | P1 |
| | [TASK-REQ-010-05](./TASK-REQ-010-05-notification-service-subjects.md) | `notification-service`: hai subject mới | P1 |
| | [TASK-REQ-010-06](./TASK-REQ-010-06-expire-and-remind-workers.md) | Worker hết hạn và nhắc | P1 |
| | [TASK-REQ-010-07](./TASK-REQ-010-07-policy-admin-api-and-integration-tests.md) | API quản trị (có điều kiện), test toàn luồng | P2 |

## Thứ tự phụ thuộc

```
009-01 ─┐
        ├─▶ 009-03 ─▶ 009-04 ─▶ 009-05 ─▶ 009-06
009-02 ─┘                │
                         ▼
010-01 ─▶ 010-03 ◀─ 010-02 (cần 009-02)
             │
             ├─▶ 010-04 ─▶ 010-05 (độc lập về mã, cần golden payload của 010-04)
             └─▶ 010-06 (cần thêm 010-04)
                    └─▶ 010-07
```
Làm song song được: 009-01 với 009-02; 010-02 với 010-01; 010-05 với 010-06.

## Ghi chú

- Task migration (009-01, 010-01) bắt buộc đọc số thật trong `request-service/migrations/*`.
- Mọi CR sở hữu một `SubjectHandler` (005, 007, 008, 012, 013, 014) phải chạy `RunSubjectHandlerContract` của TASK-REQ-009-06.
- TASK-REQ-010-05 sửa service khác (`notification-service`): cần phối hợp phát hành.
- TASK-REQ-010-07 có phần phụ thuộc câu hỏi mở (RPC quản trị chính sách); không làm phần đó nếu chưa được chấp nhận.
- Tham chiếu tiến (không phụ thuộc): CR bổ sung có thể thêm `subject_type` mới (thêm vào CHECK và handler).
