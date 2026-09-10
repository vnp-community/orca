# TASK-BE-NOTIF-004: Proto — `ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount`

**Solution:** BE-NOTIF-SOL-001 §2.D | **CR:** CR-NOTIF-001 (mục C)
**Service:** `notification-service` (proto dùng chung `backend-go/proto`)
**Depends on:** Không (độc lập code với TASK-BE-NOTIF-002/003 — có thể làm song song)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Thêm đúng 4 RPC + 7 message (`ListNotificationsRequest`,
> `Notification`, `ListNotificationsResponse`, `MarkAsReadRequest`,
> `MarkAllAsReadRequest`, `GetUnreadCountRequest`,
> `GetUnreadCountResponse`) vào `notification.proto` — nội dung khớp 100%
> task doc, không cần điều chỉnh gì. Chạy
> `buf generate --path orca/notification/v1/notification.proto` từ
> `backend-go/proto/` (dùng `buf.gen.yaml` local plugin
> `protoc-gen-go`/`protoc-gen-go-grpc` có sẵn trên PATH — đúng cơ chế repo
> dùng, khớp `Makefile:77`'s `cd proto && buf generate`, chỉ scope lại
> bằng `--path` để không đụng proto khác). `git diff --stat gen/go/` sau
> khi generate: CHỈ 2 file
> `orca/notification/v1/notification.pb.go`/`notification_grpc.pb.go` MỚI
> đổi so với baseline — 6 file khác (`auth.pb.go`, `auth_grpc.pb.go`,
> `automation.pb.go`, `infrafleet.pb.go`, `infrafleet_grpc.pb.go`,
> `workflow.pb.go`) đã có drift SẴN TRƯỚC KHI task này chạy bất kỳ lệnh gì
> (xác nhận bằng `git status --porcelain gen/go/` ngay sau khi sửa
> `.proto`, TRƯỚC khi gọi `buf generate` — cùng 6 file đã hiện `M` từ
> trước, không phải do task này gây ra, đúng cảnh báo môi trường ở đầu
> phiên). `go build ./...` ở `notification-service` sạch —
> `UnimplementedNotificationServiceServer` che 4 RPC mới, chưa cần
> implement handler thật (TASK-BE-NOTIF-006 sẽ làm).
>
> `buf lint --path orca/notification/v1/notification.proto` báo 3 finding
> về `google.protobuf.Empty` dùng chung cho nhiều RPC (bao gồm
> `MarkAsRead`/`MarkAllAsRead` mới) — ĐÃ XÁC NHẬN đây là convention có sẵn
> từ trước (`UnregisterPushSubscription` cũng dùng `Empty` y hệt, đã có
> lint finding này từ trước task), task doc yêu cầu dùng `Empty` cho 2 RPC
> mới đúng theo convention đó — không sửa (sửa sẽ đổi kiểu response, ngoài
> phạm vi task, cần message wrapper riêng không có trong thiết kế).

## Mục tiêu

Thêm 4 RPC + message tương ứng vào `notification.proto`, chạy `buf generate` không phá `proto/gen/go` dùng chung với service khác.

## Files cần sửa

1. `backend-go/proto/orca/notification/v1/notification.proto` (MODIFY)

## Nội dung thêm vào `service NotificationService`

```protobuf
service NotificationService {
  rpc Subscribe(SubscribeRequest) returns (SubscribeResponse);
  rpc UnregisterPushSubscription(UnregisterPushSubscriptionRequest) returns (google.protobuf.Empty);
  rpc GetVapidPublicKey(GetVapidPublicKeyRequest) returns (GetVapidPublicKeyResponse);
  rpc StreamNotifications(StreamNotificationsRequest) returns (stream NotificationServiceStreamNotificationsResponse);

  // ListNotifications/MarkAsRead/MarkAllAsRead/GetUnreadCount — CR-NOTIF-001,
  // the in-app Notification Center's list/unread-state surface. See
  // specs/backend-go/crs/v4/notification/solutions/BE-NOTIF-SOL-001.
  rpc ListNotifications(ListNotificationsRequest) returns (ListNotificationsResponse);
  rpc MarkAsRead(MarkAsReadRequest) returns (google.protobuf.Empty);
  rpc MarkAllAsRead(MarkAllAsReadRequest) returns (google.protobuf.Empty);
  rpc GetUnreadCount(GetUnreadCountRequest) returns (GetUnreadCountResponse);
}
```

## Message mới (thêm cuối file, cạnh các message hiện có)

```protobuf
message ListNotificationsRequest {
  string user_id = 1;      // resolved server-side from authenticated identity by api-gateway — never trust a client-supplied user_id for a DIFFERENT user
  string cursor = 2;       // opaque; empty = first page
  int32 limit = 3;
  bool unread_only = 4;
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

message ListNotificationsResponse {
  repeated Notification notifications = 1;
  string next_cursor = 2; // empty when there is no next page
}

message MarkAsReadRequest {
  string user_id = 1;
  string notification_id = 2;
}

message MarkAllAsReadRequest {
  string user_id = 1;
}

message GetUnreadCountRequest {
  string user_id = 1;
}

message GetUnreadCountResponse {
  int64 count = 1;
}
```

`user_id` giữ đúng convention hiện có (`SubscribeRequest.user_id`, `StreamNotificationsRequest.user_id`) — không có `tenant_id` trên bất kỳ message nào (tenant luôn từ context, xem `05-data-architecture.md`).

## Test cases cần cover

Không có Go test cho riêng file `.proto` — verify bằng generate + build (xem "Verify"). Test cho usecase/handler dùng các message này nằm ở TASK-BE-NOTIF-005/006.

## Verify

```bash
cd backend-go
buf generate --path orca/notification/v1/notification.proto   # hoặc lệnh generate proto thực tế repo dùng — xác nhận trong README/Makefile trước
git diff --stat proto/gen/go/   # PHẢI chỉ hiện notification.pb.go/notification_grpc.pb.go thay đổi
cd services/notification-service && go build ./...   # build sạch dù chưa implement handler mới (UnimplementedNotificationServiceServer che các RPC chưa override)
```

## gitnexus

Không có symbol Go business logic nào bị sửa trực tiếp ở task này (chỉ `.proto` + generated code) — không cần `impact()`. TASK-BE-NOTIF-006 sẽ chạy `impact()` cho `Server` trước khi thêm handler thật.

## Blocking

TASK-BE-NOTIF-006 (gRPC handler) phụ thuộc types generated ở đây (`ListNotificationsRequest`, v.v.) đã tồn tại trong `proto/gen/go`.
