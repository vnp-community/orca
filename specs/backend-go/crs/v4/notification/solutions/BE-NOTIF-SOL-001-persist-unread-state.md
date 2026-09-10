# BE-NOTIF-SOL-001: Persist `NotificationEvent` + unread state + `ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount`

> **🔲 Designed — chưa implement.** Không phụ thuộc cứng solution/CR nào khác; khuyến nghị làm trước BE-NOTIF-SOL-002 (xem [README](./README.md)'s "Quan hệ phụ thuộc").

**CR:** [CR-NOTIF-001](../../../../../../docs/crs/v4/notification/CR-NOTIF-001-unread-state-and-persistence.md)
**Service:** `notification-service` (chính), `api-gateway` (REST mount)
**TDD tham chiếu:** [`notification-service.md`](../../../../tdd/services/notification-service.md) §2 (đảo ngược quyết định "no offline WS replay queue"), §5, §6

---

## 1. Trạng thái hiện tại (re-verify — xem [README](./README.md) bảng đối chiếu)

Xác nhận lại 100% đúng với CR gốc: `notification-service` không có bảng lưu notification, không field `is_read`, RPC surface chỉ 4 method cũ, `HandleIncomingEvent.Execute` không có bước persist. Solution này bám sát "Giải pháp đề xuất" A–F của CR, chỉ khoá cụ thể hơn 2 điểm CR cố ý để mở:

### Quyết định khác/thêm so với CR gốc

1. **CR gốc để mở** giữa "thêm field vào `NotificationEvent`" và "tạo struct `PersistedNotification` riêng" (Changes Required's dòng domain). **Quyết định ở đây: thêm trực tiếp `IsRead bool` + `ReadAt *time.Time` vào `domain.NotificationEvent`** thay vì tạo struct bọc riêng — lý do: (a) Go zero-value của `bool`/`*time.Time` là `false`/`nil`, nên `TranslateEvent` (nơi tạo notification mới) không cần đổi gì, mọi test hiện có (`TestTranslateEvent_*`) tiếp tục pass nguyên trạng; (b) 1 struct thay vì 2 giúp `NotificationRepository`, `Broadcaster`, `framePayloadJSON` dùng chung 1 type, không cần hàm convert qua lại giữa `NotificationEvent`/`PersistedNotification` ở mọi lớp gọi. Đánh đổi: `NotificationEvent` không còn "thuần" 100% theo nghĩa CR gốc dùng — chấp nhận được vì đây vẫn là 1 domain struct, không rò rỉ khái niệm hạ tầng (không có cột SQL nào lộ ra ngoài field name).
2. **CR gốc mục D** không chỉ rõ cơ chế nào tạo ra sự kiện "read receipt" phát qua WS. **Quyết định:** tái dùng nguyên `domain.NotificationEvent` (không tạo type mới) với `Type: "notification_read"`, `IsRead: true`, `RecipientUserIDs: []string{userID}` — đi qua đúng `Broadcaster.Broadcast` đã có, không mở kênh/struct mới. Xem TASK-BE-NOTIF-007.

## 2. Giải pháp

### A. Migration `0003_notification_events`

Đúng SQL đã có trong CR-NOTIF-001 mục A (bảng `notification.notification_events`, 1 row/recipient, RLS theo đúng khuôn `push_subscriptions`/`vapid_key_metadata`). Xem TASK-BE-NOTIF-001.

### B. `NotificationRepository` port + Postgres implementation

```go
// internal/usecase/ports.go — thêm cạnh SubscriptionRepository/VapidKeyRepository
type NotificationRepository interface {
    Save(ctx context.Context, event domain.NotificationEvent) error
    ListByRecipient(ctx context.Context, tenantID, userID string, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error)
    MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error
    MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error)
    CountUnread(ctx context.Context, tenantID, userID string) (int64, error)
}
```

Implement trong `internal/adapter/postgres/repository.go`, đúng khuôn tay-viết-SQL hiện có (`Repository` struct dùng chung `pool *pgxpool.Pool`, không cần struct/file Postgres mới). Xem TASK-BE-NOTIF-002.

### C. `HandleIncomingEvent` lưu trước khi broadcast

`Save` chèn vào giữa `TranslateEvent` (dòng 78) và `Broadcast` (dòng 88) hiện tại — đúng nguyên văn vị trí CR chỉ định. Lỗi `Save` phải làm `Execute` trả lỗi (JetStream NAK, redeliver) — không được nuốt lỗi rồi vẫn `Broadcast`, vì như vậy sẽ có notification hiển thị live nhưng biến mất khỏi lịch sử sau khi client reload, phá đúng acceptance criterion "Unread state persist qua restart". Xem TASK-BE-NOTIF-003.

### D. Proto + 4 usecase mới

```protobuf
rpc ListNotifications(ListNotificationsRequest) returns (ListNotificationsResponse);
rpc MarkAsRead(MarkAsReadRequest) returns (google.protobuf.Empty);
rpc MarkAllAsRead(MarkAllAsReadRequest) returns (google.protobuf.Empty);
rpc GetUnreadCount(GetUnreadCountRequest) returns (GetUnreadCountResponse);

message ListNotificationsRequest {
  string user_id = 1;
  string cursor = 2;
  int32 limit = 3;
  bool unread_only = 4;
}
message ListNotificationsResponse {
  repeated Notification notifications = 1;
  string next_cursor = 2; // rỗng nếu hết trang
}
message Notification {
  string id = 1;
  string type = 2;
  string title = 3;
  string body = 4;
  string deep_link = 5;
  string severity = 6;
  bool is_read = 7;
  string created_at = 8; // RFC3339
}
message MarkAsReadRequest { string user_id = 1; string notification_id = 2; }
message MarkAllAsReadRequest { string user_id = 1; }
message GetUnreadCountRequest { string user_id = 1; }
message GetUnreadCountResponse { int64 count = 1; }
```

`user_id` trên mỗi request theo đúng convention hiện có của `SubscribeRequest`/`StreamNotificationsRequest` (không có `tenant_id` — tenant luôn qua context). 4 usecase mới (`list_notifications.go`, `mark_as_read.go`, `mark_all_as_read.go`, `get_unread_count.go`) theo đúng khuôn 1-usecase-1-file, mỗi usecase tự gọi `tenant.RequireTenantID(ctx)` — mirror `subscribe.go`/`get_vapid_public_key.go`. Xem TASK-BE-NOTIF-004 (proto), TASK-BE-NOTIF-005 (usecase).

### E. gRPC handler + `frame.go`

`server.go` thêm 4 method mới theo đúng khuôn `Subscribe`/`GetVapidPublicKey` (translate request → usecase input → response, lỗi qua `apperrors.ToGRPCStatus`). `frame.go`'s `framePayload` thêm field `IsRead bool \`json:"is_read"\`` đọc từ `event.IsRead` — mọi notification mới phát qua `StreamNotifications` sẽ luôn có `is_read: false` (vì `TranslateEvent` không set field này), đúng kỳ vọng "mới tạo thì chưa đọc". Xem TASK-BE-NOTIF-006.

### F. WS read-receipt (real-time đa tab/đa thiết bị)

`MarkAsRead`/`MarkAllAsRead` usecase, sau khi `repo.MarkAsRead`/`MarkAllAsRead` thành công, gọi `broadcaster.Broadcast(ctx, domain.NotificationEvent{ID: notificationID, TenantID: tenantID, RecipientUserIDs: []string{userID}, Type: "notification_read", IsRead: true, CreatedAt: time.Now()})` — client phía WS nhận `type: "notification_read"` trong frame, tự trừ unread count cục bộ theo `id`, không cần round-trip `GetUnreadCount` lại. Đây là điểm bắt buộc theo AGENTS.md's "SSH Use Case" (nhiều tab/phiên trình duyệt cùng 1 user qua SSH/remote workflow). Xem TASK-BE-NOTIF-007.

### G. REST + WS ở `api-gateway`

`notification_routes.go`'s `mountNotificationRoutes` thêm 4 route theo đúng pattern `resolveSoftIdentity` + `AttachIdentity` đã có ở `handleSubscribe`/`handleGetVapidPublicKey` (không phải pattern `mountPushRoutes`'s unauthenticated — 4 route mới này PHẢI nằm trong nhóm có `authMiddleware`, vì đọc lịch sử notification cá nhân không được phép ẩn danh):

```
GET  /v1/notifications              (?cursor=&limit=&unread_only=)
POST /v1/notifications/{id}/read
POST /v1/notifications/read-all
GET  /v1/notifications/unread-count
```

`channels_push.go`'s `registerNotificationStreamChannel` không cần sửa — nó forward nguyên `item` (đã có `is_read` mới qua `payload_json`, do được generate lại từ proto `Notification`/frame's `framePayload`). Xem TASK-BE-NOTIF-008.

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Chung 1 hàm `HandleIncomingEvent.Execute` với BE-NOTIF-SOL-002 | Trung bình | Nếu 2 solution làm song song, merge conflict tại chính hàm này gần như chắc chắn — làm tuần tự (001 trước) theo README |
| `buf generate` chạm `proto/gen/go` dùng chung | Trung bình | Kiểm tra `git diff --stat proto/gen/go/` chỉ đổi file `notification.pb.go`/`notification_grpc.pb.go`, không đụng service khác — đúng bài học từ TASK-BE-STORAGE-003 |
| Bảng `notification_events` tăng vô hạn (không retention trong CR này) | Thấp (chấp nhận được, đã ghi rõ ngoài phạm vi) | Theo dõi kích thước bảng, mở CR retention riêng khi cần |
| `IsRead`/`ReadAt` thêm vào `NotificationEvent` (thay vì struct riêng) làm struct này không còn "thuần write-model" | Thấp | Đánh đổi đã cân nhắc ở mục 1; không có cột SQL nào rò rỉ ra field name, chấp nhận được |

## Không thuộc phạm vi solution này

- Retention/pruning cho `notification_events` — xem CR-NOTIF-001's "Không thuộc phạm vi".
- Per-user notification preference/mute.
- `DeliverPush` thật — xem [BE-NOTIF-SOL-002](./BE-NOTIF-SOL-002-deliver-push-usecase.md).
- Frontend Notification Center panel — cần CR frontend riêng, phụ thuộc solution này đã merge.

## Liên quan

- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:62-90`
- `backend-go/services/notification-service/internal/domain/notification_event.go:68-81`
- `backend-go/services/notification-service/internal/adapter/postgres/repository.go`
- `backend-go/services/notification-service/internal/adapter/grpc/{server.go,frame.go}`
- `backend-go/proto/orca/notification/v1/notification.proto`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go`
- [BE-NOTIF-SOL-002](./BE-NOTIF-SOL-002-deliver-push-usecase.md) (phụ thuộc mềm, xem README)
