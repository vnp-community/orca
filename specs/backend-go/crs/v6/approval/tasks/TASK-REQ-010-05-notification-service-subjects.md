# TASK-REQ-010-05: `notification-service` nhận hai subject `orca.request.approval.*`

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục E
**Priority:** P1
**Service/Area:** `notification-service` / eventbus, domain
**File:** `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (sửa, mảng `Subjects`), `backend-go/services/notification-service/internal/domain/notification_event.go` (sửa, `subjectRules`), `.../eventbus/consumer_test.go` (sửa), `.../domain/notification_event_test.go` (sửa)
**Depends on:** TASK-REQ-010-04 (golden payload); độc lập về mã với các task còn lại
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: `cd backend-go/services/notification-service && go test ./... ; go test -tags integration ./internal/adapter/eventbus/...`)

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

- [x] Hai subject nằm trong `Subjects` và `subjectRules` (nay ở `request_subjects.go` và `request_notification_rules.go`).
- [x] Test hợp đồng với golden payload xanh (golden đặt ở `internal/domain/testdata/request_notification/`; golden của 010-04 chưa có, xem IMPLEMENTATION-NOTES).
- [x] Không đổi hành vi các subject cũ (toàn bộ test cũ xanh, thêm test hồi quy).
- [x] Tên stream `REQUEST` được đối chiếu: `request-service/cmd/server/main.go:89` `EnsureStream(ctx, "REQUEST", {"orca.request.>"})`. Chưa chạy thử hai service cùng nhau.

## Rủi ro và lưu ý

- Danh sách subject cố định trong mã: đổi cần triển khai lại service.
- `Durable` dùng một con trỏ chung giữa các replica: chỉ một replica nhận mỗi sự kiện, nên broadcast WS chỉ tới người dùng nối với replica đó (cùng hạn chế đã ghi ở các binding Durable hiện có); chấp nhận và ghi vào PR.

## Tiến độ

Đã làm (2026-10-07): binding và rule đã có sẵn từ trước nhưng không có test và `decided` thiếu DeepLink; nay tách sang tệp riêng theo tính năng (`internal/adapter/eventbus/request_subjects.go`, `internal/domain/request_notification_rules.go`) để 028-06 và các CR sau thêm hàng không đụng `consumer.go`/`notification_event.go`. Thêm `DeepLink` cho `decided`, cờ `SameOriginDeepLink` (chặn deep_link ngoài ứng dụng). Test: `request_notification_rules_test.go`, `TestSubjects_RequestBindings*`, integration NATS `TestRequestSubjects_DurableEventsPublishedWhileConsumerDownAreDelivered`.
Việc còn lại ngoài phạm vi: triển khai lại `notification-service` sau khi `request-service` phát hành; producer (010-04) phải điền `user_ids` (hiện `publish_approval_notifications.go` của request-service còn dùng deep_link placeholder `request=req_id`).
