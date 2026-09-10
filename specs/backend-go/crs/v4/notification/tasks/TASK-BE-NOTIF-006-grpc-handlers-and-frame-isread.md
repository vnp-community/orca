# TASK-BE-NOTIF-006: gRPC handler wiring (4 RPC mới) + `frame.go`'s `is_read`

**Solution:** BE-NOTIF-SOL-001 §2.E | **CR:** CR-NOTIF-001 (mục C, D)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-004, TASK-BE-NOTIF-005
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch — `Server` struct/constructor thêm
> 4 field usecase, 4 handler mới (`ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/
> `GetUnreadCount`), `frame.go` thêm `IsRead bool \`json:"is_read"\`` vào `framePayload`.
> `cmd/server/main.go` wire 4 usecase mới, dùng lại `repo` đã có (đã implement cả
> `NotificationRepository` từ TASK-002). `impact({target: "New", direction: "upstream",
> file_path: ".../adapter/grpc/server.go"})` chạy trước khi sửa constructor: LOW risk,
> 2 impacted (`run`, `main` ở `cmd/server/main.go`) — đúng dự kiến, không có caller
> ngoài ý muốn.
>
> Package `internal/adapter/grpc` **chưa có test file nào trước task này** — viết mới
> `server_test.go` (4 test theo đúng tên yêu cầu, dùng `fakeNotificationRepository`
> cục bộ implement `usecase.NotificationRepository`) và `frame_test.go` (2 test).
> `go build ./...`, `go vet ./...`, `gofmt -l` sạch toàn service. `go test ./...`
> (toàn bộ `notification-service`) PASS 100%, gồm 6/6 test mới của task này.

## Mục tiêu

Wire 4 usecase mới (TASK-BE-NOTIF-005) thành RPC handler thật trong `server.go`; thêm `is_read` vào `StreamNotifications`'s frame payload.

## Files cần sửa

1. `backend-go/services/notification-service/internal/adapter/grpc/server.go` (MODIFY)
2. `backend-go/services/notification-service/internal/adapter/grpc/frame.go` (MODIFY)
3. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY — wire 4 usecase mới vào `grpc.New(...)`)

## `server.go` — `Server` struct + constructor + 4 handler

```go
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer

	subscribe                  *usecase.Subscribe
	unregisterPushSubscription *usecase.UnregisterPushSubscription
	getVapidPublicKey          *usecase.GetVapidPublicKey
	listNotifications          *usecase.ListNotifications
	markAsRead                 *usecase.MarkAsRead
	markAllAsRead              *usecase.MarkAllAsRead
	getUnreadCount             *usecase.GetUnreadCount
	broadcaster                usecase.NotificationBroadcaster
	signer                     usecase.VaultSigner
}

func New(
	subscribe *usecase.Subscribe,
	unregisterPushSubscription *usecase.UnregisterPushSubscription,
	getVapidPublicKey *usecase.GetVapidPublicKey,
	listNotifications *usecase.ListNotifications,
	markAsRead *usecase.MarkAsRead,
	markAllAsRead *usecase.MarkAllAsRead,
	getUnreadCount *usecase.GetUnreadCount,
	broadcaster usecase.NotificationBroadcaster,
	signer usecase.VaultSigner,
) *Server {
	return &Server{
		subscribe: subscribe, unregisterPushSubscription: unregisterPushSubscription,
		getVapidPublicKey: getVapidPublicKey, listNotifications: listNotifications,
		markAsRead: markAsRead, markAllAsRead: markAllAsRead, getUnreadCount: getUnreadCount,
		broadcaster: broadcaster, signer: signer,
	}
}

func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	events, next, err := s.listNotifications.Execute(ctx, usecase.ListNotificationsInput{
		UserID: req.GetUserId(), Cursor: req.GetCursor(), Limit: req.GetLimit(), UnreadOnly: req.GetUnreadOnly(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*notificationv1.Notification, 0, len(events))
	for _, e := range events {
		out = append(out, &notificationv1.Notification{
			Id: e.ID, Type: e.Type, Title: e.Title, Body: e.Body, DeepLink: e.DeepLink,
			Severity: string(e.Severity), IsRead: e.IsRead, CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	return &notificationv1.ListNotificationsResponse{Notifications: out, NextCursor: next}, nil
}

func (s *Server) MarkAsRead(ctx context.Context, req *notificationv1.MarkAsReadRequest) (*emptypb.Empty, error) {
	if err := s.markAsRead.Execute(ctx, req.GetUserId(), req.GetNotificationId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) MarkAllAsRead(ctx context.Context, req *notificationv1.MarkAllAsReadRequest) (*emptypb.Empty, error) {
	if _, err := s.markAllAsRead.Execute(ctx, req.GetUserId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetUnreadCount(ctx context.Context, req *notificationv1.GetUnreadCountRequest) (*notificationv1.GetUnreadCountResponse, error) {
	count, err := s.getUnreadCount.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &notificationv1.GetUnreadCountResponse{Count: count}, nil
}
```

## `frame.go` — thêm `is_read`

```go
type framePayload struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link,omitempty"`
	Severity string `json:"severity"`
	IsRead   bool   `json:"is_read"`
}

func framePayloadJSON(e domain.NotificationEvent) string {
	b, err := json.Marshal(framePayload{
		Title: e.Title, Body: e.Body, DeepLink: e.DeepLink, Severity: string(e.Severity), IsRead: e.IsRead,
	})
	...
}
```

Mọi notification MỚI phát qua `StreamNotifications` (từ `HandleIncomingEvent`) sẽ luôn có `is_read: false` (vì `TranslateEvent` không set field này — xem TASK-BE-NOTIF-002) — đúng kỳ vọng. Frame cho read-receipt (TASK-BE-NOTIF-007) sẽ có `is_read: true`.

## `cmd/server/main.go` — wiring

```go
listNotificationsUC := usecase.NewListNotifications(repo)
markAsReadUC := usecase.NewMarkAsRead(repo) // TASK-BE-NOTIF-007 đổi chữ ký nếu cần thêm broadcaster
markAllAsReadUC := usecase.NewMarkAllAsRead(repo)
getUnreadCountUC := usecase.NewGetUnreadCount(repo)

notificationv1.RegisterNotificationServiceServer(grpcServer, notificationgrpc.New(
	subscribeUC, unregisterPushSubscriptionUC, getVapidPublicKeyUC,
	listNotificationsUC, markAsReadUC, markAllAsReadUC, getUnreadCountUC,
	broadcast, signer,
))
```

## Test cases cần cover

- `TestServer_ListNotifications_MapsUsecaseOutputToProto`
- `TestServer_ListNotifications_UsecaseErrorMapsToGRPCStatus`
- `TestServer_MarkAsRead_ReturnsEmptyOnSuccess`
- `TestServer_GetUnreadCount_ReturnsCountFromUsecase`
- `TestFramePayloadJSON_IncludesIsRead` — `e.IsRead = true` → JSON output chứa `"is_read":true` (regression guard cho field mới)
- `TestFramePayloadJSON_DefaultsIsReadFalse` — event mới từ `TranslateEvent` (chưa set `IsRead`) → JSON `"is_read":false`

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/adapter/grpc/... ./cmd/...
gofmt -l internal/adapter/grpc/server.go internal/adapter/grpc/frame.go cmd/server/main.go
```

## gitnexus

`impact({target: "Server", direction: "upstream"})` (package `adapter/grpc`) trước khi sửa constructor `New(...)` — theo `docs/crs/v4/notification/README.md`'s khảo sát trước, LOW risk, 3 impacted. Xác nhận không có caller nào khác ngoài `cmd/server/main.go` và test file trước khi thêm 4 tham số usecase mới vào `New(...)`.

## Blocking

TASK-BE-NOTIF-008 (REST `api-gateway`) phụ thuộc `notificationv1.NotificationServiceClient` đã có 4 method mới (regenerate qua `buf generate`, TASK-BE-NOTIF-004, là điều kiện tiên quyết cho cả 2 phía client/server).
