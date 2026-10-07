# TASK-REQ-010-05: `notification-service` nhận hai subject `orca.request.approval.*`

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục E
**Priority:** P1
**Service/Area:** `notification-service` / eventbus, domain
**File:** `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (sửa, mảng `Subjects`), `backend-go/services/notification-service/internal/domain/notification_event.go` (sửa, `subjectRules`), `.../eventbus/consumer_test.go` (sửa), `.../domain/notification_event_test.go` (sửa)
**Depends on:** TASK-REQ-010-04 (golden payload); độc lập về mã với các task còn lại
**Status:** `[x] DONE`

## Context

- `Subjects` hiện có `{StreamName, Subject, Durable}`; `Durable` dùng cho sự kiện mất là hại (ví dụ `INFRAFLEET terminal.closed`). Mẫu thêm MCP: dòng `{StreamName: "MCP", Subject: "orca.mcp.approval.requested"}`.
- `subjectRule` có `Locked`: Locked bỏ qua title/body/deep_link của payload. Hai rule mới không Locked vì `request-service` đặt `title` cụ thể; đổi lại payload phải sạch (task 04).
- Tên stream `REQUEST` phải khớp `EnsureStream` của `request-service` (CR-REQ-001); chưa kiểm chứng vì service chưa tồn tại. Xác nhận trước khi merge.
- Không có migration ở service này.

## Việc cần làm

1. `consumer.go`: thêm `{StreamName: "REQUEST", Subject: "orca.request.approval.requested", Durable: "notification-service-request-approval-requested"}` và `{StreamName: "REQUEST", Subject: "orca.request.approval.decided"}`, kèm một dòng chú thích lý do `Durable`.
2. `notification_event.go`: thêm rule `Type: "request.approval_requested"`, `Title: "Approval needed"`, `Severity: SeverityWarning`, `Channels: ws + push`, `DeepLink: "/?section=requests"`; và `Type: "request.approval_decided"`, `Title: "Approval decided"`, `Severity: SeverityInfo`, `Channels: ws + push`.
3. Cập nhật mọi test đang đếm số binding hoặc liệt kê subject (`consumer_test.go`).
4. Thêm test dịch sự kiện dùng golden payload của task 04 (sao chép tệp vào `testdata` của service này, không import chéo).
5. Báo chủ `notification-service` về việc phải triển khai lại service sau khi `request-service` phát hành.

## Kiểm thử

- `notification_event_test.go`: `TranslateEvent` với payload mẫu cho từng subject: đúng `Type`, người nhận, kênh, dùng `title` của payload; payload không có `user_ids` thì `ErrNoRecipients`; `deep_link` của payload thắng `DeepLink` mặc định.
- `consumer_test.go`: hai binding có mặt; `Durable` đúng tên cho `requested`.
- Lệnh: `cd backend-go/services/notification-service && go test ./internal/domain/... ./internal/adapter/eventbus/...`.

## Tiêu chí hoàn thành

- [x] Hai subject nằm trong `Subjects` và `subjectRules`.
- [x] Test hợp đồng với golden payload xanh.
- [x] Không đổi hành vi các subject cũ (toàn bộ test cũ xanh).
- [x] Tên stream `REQUEST` được đối chiếu với `request-service` (ghi vào PR).

## Rủi ro và lưu ý

- Danh sách subject cố định trong mã: đổi cần triển khai lại service.
- `Durable` dùng một con trỏ chung giữa các replica: chỉ một replica nhận mỗi sự kiện, nên broadcast WS chỉ tới người dùng nối với replica đó (cùng hạn chế đã ghi ở các binding Durable hiện có); chấp nhận và ghi vào PR.
