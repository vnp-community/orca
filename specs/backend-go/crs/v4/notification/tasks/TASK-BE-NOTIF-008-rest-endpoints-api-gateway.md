# TASK-BE-NOTIF-008: REST `api-gateway` — 4 route mới (authenticated)

**Solution:** BE-NOTIF-SOL-001 §2.G | **CR:** CR-NOTIF-001 (mục E)
**Service:** `api-gateway`
**Depends on:** TASK-BE-NOTIF-006
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch — 4 route mới (`GET /`,
> `POST /{id}/read`, `POST /read-all`, `GET /unread-count`) mounted trong
> `mountNotificationRoutes` (đã ở trong nhóm `authed`, xác nhận qua `router.go:154`
> so với `mountPushRoutes` — unauthenticated, mount riêng dòng 111). Thêm helper
> `requireIdentity` (fail-closed, khác `resolveSoftIdentity` — 2 route cũ
> Subscribe/GetVapidPublicKey cố ý cho phép identity rỗng). `impact({target:
> "mountNotificationRoutes", direction: "upstream"})` trước khi sửa: LOW risk,
> đúng 1 caller trực tiếp (`NewRouter`).
>
> Test file `notification_routes_test.go` **đã có sẵn stub `Unimplemented`** cho
> 4 RPC mới (từ TASK-BE-NOTIF-004's `buf generate` làm phình interface — ghi chú
> cũ trong code nhắc "TASK-BE-012", một numbering khác từ trước, không phải lỗi) —
> nâng cấp thành fake thật (request capture + response/error cấu hình được), viết
> 10 test PASS thật (nhiều hơn 5 test tối thiểu task yêu cầu — thêm
> `RequiresAuth` cho cả `MarkAllAsRead`/`GetUnreadCount`, không chỉ `List`).
>
> `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go test ./...` toàn bộ
> `api-gateway` PASS 100% (bao gồm `wsbridge`/`wscompat`/`usecase` — không có
> package nào bị ảnh hưởng ngoài dự kiến).

## Mục tiêu

Thêm 4 REST endpoint vào `mountNotificationRoutes` — khác `mountPushRoutes` (unauthenticated, dùng cho service-worker đăng ký push trước khi có session), 4 route này PHẢI nằm trong nhóm route có `authMiddleware` vì đọc lịch sử notification cá nhân không được phép ẩn danh.

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` (MODIFY)

## Nội dung sửa `mountNotificationRoutes`

```go
func mountNotificationRoutes(r chi.Router, client notificationv1.NotificationServiceClient) {
	r.Route("/v1/notifications", func(sub chi.Router) {
		sub.Post("/subscribe", handleSubscribe(client, nil))
		sub.Get("/vapid-public-key", handleGetVapidPublicKey(client, nil))
		sub.Get("/", handleListNotifications(client))
		sub.Post("/{id}/read", handleMarkAsRead(client))
		sub.Post("/read-all", handleMarkAllAsRead(client))
		sub.Get("/unread-count", handleGetUnreadCount(client))
	})
}
```

**Lưu ý thứ tự route**: `chi` khớp route cụ thể trước route có path param — xác nhận `/unread-count`, `/read-all`, `/vapid-public-key`, `/subscribe` không bị `/{id}/read` nuốt mất (path param `{id}` chỉ khớp 1 segment, các route trên đều khác segment đầu nên không xung đột — vẫn viết test xác nhận, xem "Test cases").

## Handler mới — theo đúng khuôn `handleGetVapidPublicKey`/`handleSubscribe` (dùng `resolveSoftIdentity` + `AttachIdentity`)

```go
func handleListNotifications(client notificationv1.NotificationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := resolveSoftIdentity(r, nil)
		if identity.UserID == "" {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "no authenticated session")
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListNotifications(ctx, &notificationv1.ListNotificationsRequest{
			UserId: identity.UserID, Cursor: q.Get("cursor"), Limit: int32(limit), UnreadOnly: q.Get("unread_only") == "true",
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleMarkAsRead(client notificationv1.NotificationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := resolveSoftIdentity(r, nil)
		if identity.UserID == "" {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "no authenticated session")
			return
		}
		notificationID := chi.URLParam(r, "id")
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		if _, err := client.MarkAsRead(ctx, &notificationv1.MarkAsReadRequest{UserId: identity.UserID, NotificationId: notificationID}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMarkAllAsRead/handleGetUnreadCount — cùng khuôn (identity bắt buộc,
// AttachIdentity, gọi RPC tương ứng, map lỗi qua writeGRPCError).
```

**Bảo mật — bắt buộc**: `identity.UserID` LUÔN là giá trị duy nhất gửi làm `user_id` trong request gRPC — **không đọc `user_id` từ query string/path param/body của client**. Nếu `identity.UserID == ""` (không có session hợp lệ), trả `401` NGAY, không gọi RPC với `user_id` rỗng (khác `handleSubscribe`/`handleGetVapidPublicKey` — 2 route đó cố ý cho phép identity rỗng để tương thích `mountPushRoutes`'s use case unauthenticated; 4 route MỚI này không có lý do tương tự, phải fail-closed).

## Test cases cần cover

- `TestNotificationRoutes_ListNotifications_RequiresAuth` — không có session cookie → `401`, không gọi gRPC client (dùng fake client assert `ListNotifications` không được gọi).
- `TestNotificationRoutes_ListNotifications_PassesQueryParamsThrough`.
- `TestNotificationRoutes_MarkAsRead_UsesIdentityUserIDNotClientInput` — giả lập request cố tình không có cách nào set `user_id` khác identity (route không nhận `user_id` từ client) — xác nhận request tới gRPC client luôn mang `identity.UserID`.
- `TestNotificationRoutes_RouteOrderingNoConflict` — gọi `GET /v1/notifications/unread-count` và `GET /v1/notifications/vapid-public-key` xác nhận đúng handler được gọi, không bị `/{id}/read`-style route nuốt (dù route này là POST khác method nên về lý thuyết không đụng GET, vẫn viết test cho rõ ràng vì đây là điểm dễ sai khi thêm route mới vào 1 `chi.Router` đã có nhiều route).
- `TestNotificationRoutes_MarkAsRead_ReturnsNoContentOnSuccess`.

## Verify

```bash
cd backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/httpgateway/... -run TestNotificationRoutes
gofmt -l internal/adapter/httpgateway/notification_routes.go
```

## gitnexus

`impact({target: "mountNotificationRoutes", direction: "upstream"})` trước khi sửa — xác nhận caller (`router.go`'s mounting) không giả định số lượng route cố định nào bị phá. Xác nhận lại thứ tự mounting trong `router.go` (route cụ thể đăng ký trước route có path param, theo comment hiện có ở `mountNotificationRoutes`'s doc comment về `StreamNotifications`/`/v1/notifications/stream`) không bị 4 route mới phá vỡ.

## Blocking

Không có — đây là task cuối của Track 1.
