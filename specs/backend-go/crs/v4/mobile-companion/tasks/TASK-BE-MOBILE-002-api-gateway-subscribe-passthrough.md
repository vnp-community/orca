# TASK-BE-MOBILE-002: `api-gateway`'s `handleSubscribe` — passthrough `channel`/`device_label`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `api-gateway`
**Depends on:** TASK-BE-MOBILE-001
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch. `impact({target: "handleSubscribe"})`
> trả về ambiguous (5 symbol trùng tên toàn repo, gồm cả TS) — disambiguate đúng
> theo `file_path` ra symbol `api-gateway`'s `notification_routes.go:handleSubscribe`:
> LOW risk, 4 impacted (2 direct). Cả 2 test case yêu cầu PASS, cộng `gofmt -w`
> (task doc's code block lệch format 1 dòng comment, tự sửa).
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./...` PASS 100% toàn `api-gateway`.

---

## Mục tiêu

2 route REST hiện có (`POST /v1/notifications/subscribe`,
`POST /api/push-subscribe`) đều gọi `handleSubscribe`, hiện chỉ forward
`endpoint`/`p256dh_key`/`auth_key`. Thêm khả năng forward `channel`/
`device_label` cho bất kỳ caller đã xác thực nào — **không tạo route mới**
(route mới riêng cho mobile không-cookie là việc của BE-MOBILE-SOL-002,
task riêng, `🔲 BLOCKED`).

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` (MODIFY)
2. `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes_test.go` (MODIFY — thêm test case)

## Thay đổi cụ thể

```go
// subscribeRequestBody — thêm 2 field, giữ nguyên comment "user_id deliberately absent"
type subscribeRequestBody struct {
	Endpoint    string `json:"endpoint"`
	P256dhKey   string `json:"p256dh_key"`
	AuthKey     string `json:"auth_key"`
	Channel     string `json:"channel"`      // mới — rỗng == "web", KHÔNG đổi hành vi hiện tại
	DeviceLabel string `json:"device_label"` // mới
}

func handleSubscribe(client notificationv1.NotificationServiceClient, cookieValidator CookieSessionValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := resolveSoftIdentity(r, cookieValidator)

		var body subscribeRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Subscribe(ctx, &notificationv1.SubscribeRequest{
			UserId:      identity.UserID,
			Endpoint:    body.Endpoint,
			P256DhKey:   body.P256dhKey,
			AuthKey:     body.AuthKey,
			Channel:     body.Channel,
			DeviceLabel: body.DeviceLabel,
		})
		// ... không đổi phần còn lại
	}
}
```

Không sửa `resolveSoftIdentity`/`mountPushRoutes`/`mountNotificationRoutes`
— identity vẫn luôn từ `CookieSessionValidator`/context đã xác thực, không
từ body (giữ đúng comment tại chỗ "`user_id` deliberately absent").

## Test cases cần cover

- `TestHandleSubscribe_ChannelPassthrough` — body có `channel: "ios"` →
  `client.Subscribe` (fake/mock gRPC client) nhận đúng
  `req.Channel == "ios"`
- `TestHandleSubscribe_EmptyChannelStillWorks` (regression guard — body cũ
  không có field `channel` vẫn phải gọi `Subscribe` thành công, giống hệt
  hành vi trước task này)

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test ./internal/adapter/httpgateway/... -run TestHandleSubscribe -v
gofmt -l internal/adapter/httpgateway/notification_routes.go
```

## gitnexus

`impact({target: "handleSubscribe", direction: "upstream"})` trước khi sửa
— xác nhận cả 2 mount (`mountNotificationRoutes`, `mountPushRoutes`) vẫn
build sạch sau khi đổi chữ ký `subscribeRequestBody` (không đổi chữ ký hàm
`handleSubscribe` nên rủi ro thấp, nhưng vẫn phải chạy theo quy tắc bắt
buộc chung).

## Blocking

Không task nào trong nhóm BE-MOBILE-SOL-001 phụ thuộc ngược task này.
BE-MOBILE-SOL-002's route mobile mới tham chiếu `subscribeRequestBody`'s
convention (field `channel`/`device_label`) làm mẫu, nhưng không import
trực tiếp — không blocking cứng.
