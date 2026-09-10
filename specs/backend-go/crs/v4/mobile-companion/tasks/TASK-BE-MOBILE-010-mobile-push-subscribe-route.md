# TASK-BE-MOBILE-010: Route `POST /api/mobile/push-subscribe` — auth không qua cookie

**Solution:** BE-MOBILE-SOL-002 | **CR:** [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md)
**Service:** `api-gateway`
**Depends on:** TASK-BE-MOBILE-001, TASK-BE-MOBILE-002 (`SubscribeRequest.channel`/`device_label` phải tồn tại)
**Status:** 🔲 BLOCKED (cập nhật 2026-09-09) — quyết định A/B đã chốt (Phương
án B: mobile có SSO riêng, xem BE-MOBILE-SOL-002 §1.4), nhưng CR hạ tầng
SSO tiền đề (backend-go's `auth-service` + `frontend`, `desktop/` KHÔNG
đổi) chưa tồn tại — vẫn BLOCKED, lý do đã đổi bản chất (không còn "chưa
biết chọn A hay B", mà là "đã biết B, chờ CR hạ tầng B cần")

---

## ✅ Quyết định A/B đã chốt (2026-09-09) — nhưng vẫn BLOCKED vì lý do khác

**Cập nhật:** phần "Quyết định cần chốt" gốc bên dưới (giữ nguyên để tham
khảo lịch sử) đã được người giao việc trả lời trực tiếp: **chọn Phương án
B — mobile có SSO riêng**, KHÔNG chọn Phương án A (desktop relay).
Xác nhận thêm: phạm vi bổ sung chỉ chạm `backend-go` (nhiều khả năng
`auth-service` + `api-gateway`) và `frontend`; `desktop/`'s cơ chế hiện
tại **không đổi** — loại bỏ hẳn nhánh "desktop cần tự có identity với
backend-go" (§1.3's gap kiến trúc) khỏi bàn.

**Task này vẫn KHÔNG chuyển sang TODO được ngay** — không phải vì thiếu
quyết định A/B nữa, mà vì Phương án B tự nó cần 1 CR hạ tầng mới (luồng
SSO mobile: token issuance, refresh, revocation…) **chưa được viết**. Khi
CR đó tồn tại và đủ chi tiết (đặc biệt: format/claims của JWT mobile sẽ
trình, để viết đúng `AuthMiddleware` biến thể chấp nhận
`Authorization: Bearer <jwt>`), task này mới đủ điều kiện chuyển TODO và
viết lại đúng theo khuôn route thật của Phương án B (xem
BE-MOBILE-SOL-002 §1.4) — không phải khuôn Phương án A còn giữ nguyên văn
bên dưới (chỉ để tham khảo lịch sử, KHÔNG dùng làm khuôn implement).

## ⚠️ Quyết định cần chốt (nội dung gốc, đã trả lời — giữ để tham khảo lịch sử)

Task này **KHÔNG đủ điều kiện để thực thi ngay** — khác với mọi task khác
trong bộ `mobile-companion` — vì còn 1 quyết định thiết kế mở, đã ghi rõ ở
[BE-MOBILE-SOL-002](../solutions/BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md)
§1.2/§1.3/§2: **route mới này xác thực caller bằng cơ chế gì, thay cho cookie
session** (mobile app không có cookie trình duyệt)?

**Cập nhật (2026-09-09) — lý do BLOCKED giờ nghiêm trọng hơn bản nháp trước:**
xác minh sâu hơn (§1.3 của solution) cho thấy `desktop/` (phương án "relay") hiện
**không có bất kỳ kết nối nào tới `backend-go`** — không chỉ là "chưa chọn cơ chế
xác thực cụ thể", mà là "kênh để relay chưa tồn tại". Cả 2 phương án (desktop relay
/ mobile SSO riêng) giờ đều cần 1 CR tiền đề mới về hạ tầng xác thực, không phương
án nào còn là "chỉ cần thêm 1 route nhỏ". Đây là quyết định kiến trúc rộng hơn phạm
vi `mobile-companion` — không nên chốt vội chỉ để unblock 1 task.

- Nếu KHÔNG có bước xác thực nào thay thế → route cho phép bất kỳ ai gửi
  `user_id` tuỳ ý trong body đăng ký push token hộ user khác — **lỗ hổng
  bảo mật**, không được code phần "chấp nhận request" trước khi chốt.
- 2 phương án đã đặc tả ở mức thiết kế trong BE-MOBILE-SOL-002 §2 (desktop
  relay qua RPC + service token, hoặc mobile có SSO riêng) — **người phụ
  trách cần chọn 1 trước khi task này chuyển sang `🔲 TODO`**.

**Không tự chọn 1 giá trị cụ thể (vd. "dùng HMAC với secret X") chỉ để có
gì đó code được** — điều đó khoá nhầm 1 quyết định bảo mật vào code mà
không ai thật sự review. Khi quyết định đã chốt, cập nhật lại
`DeviceTokenValidator`'s interface bên dưới cho khớp cơ chế thật, rồi mới
implement.

## Mục tiêu (khi đã chốt quyết định)

Thêm 1 route mới, KHÔNG nằm trong `authMiddleware`/`CookieSessionValidator`
hiện có (mobile không có cookie), nhận `{user_id, endpoint, channel,
device_label}` và forward tới `notification-service`'s `Subscribe` RPC
(đã hỗ trợ `channel`/`device_label` từ TASK-BE-MOBILE-001).

## Files cần sửa (khung sườn — hoàn thiện theo cơ chế xác thực đã chốt)

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` (MODIFY)
2. `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes_test.go` (MODIFY)
3. File adapter xác thực mới — tên/vị trí phụ thuộc cơ chế đã chốt (vd.
   `internal/adapter/devicetoken/validator.go` nếu chọn service-token) —
   **chưa đặt tên cụ thể trong task này vì phụ thuộc quyết định chưa
   chốt**.

## Khung route (điền `DeviceTokenValidator` thật sau khi chốt)

```go
// mountMobilePushRoutes — route riêng, KHÔNG nằm trong authMiddleware.
// deviceTokenValidator's concrete type/semantics: XEM QUYẾT ĐỊNH CẦN CHỐT
// Ở TRÊN — KHÔNG tự ý định nghĩa interface này trước khi chốt, vì chữ ký
// đúng phụ thuộc cơ chế được chọn (service token / mTLS / HMAC / khác).
func mountMobilePushRoutes(r chi.Router, client notificationv1.NotificationServiceClient, deviceTokenValidator DeviceTokenValidator) {
	r.Post("/api/mobile/push-subscribe", handleMobilePushSubscribe(client, deviceTokenValidator))
}

type mobilePushSubscribeRequestBody struct {
	UserID      string `json:"user_id"`
	Endpoint    string `json:"endpoint"`
	Channel     string `json:"channel"`      // "ios" | "android" — route này không phục vụ "web"
	DeviceLabel string `json:"device_label"`
}

func handleMobilePushSubscribe(client notificationv1.NotificationServiceClient, deviceTokenValidator DeviceTokenValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// BƯỚC BẮT BUỘC, chưa implement được tới khi chốt quyết định:
		// xác thực request THẬT SỰ được phép đăng ký push token cho
		// body.UserID — KHÔNG tin body.UserID mù quáng.
		identity, err := deviceTokenValidator.Validate(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid device token")
			return
		}
		var body mobilePushSubscribeRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.Channel != "ios" && body.Channel != "android" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "channel must be ios or android")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Subscribe(ctx, &notificationv1.SubscribeRequest{
			UserId:      identity.UserID, // từ identity đã xác thực, KHÔNG từ body.UserID trực tiếp — đối chiếu 2 giá trị nếu cơ chế cho phép cả hai tồn tại
			Endpoint:    body.Endpoint,
			Channel:     body.Channel,
			DeviceLabel: body.DeviceLabel,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}
```

## Test cases cần cover (khi đã chốt)

- `TestHandleMobilePushSubscribe_ValidDeviceTokenSucceeds`
- `TestHandleMobilePushSubscribe_InvalidDeviceTokenReturns401`
- `TestHandleMobilePushSubscribe_RejectsWebChannel`
- `TestHandleMobilePushSubscribe_CannotImpersonateOtherUser` — **quan
  trọng nhất**: body chứa `user_id` khác với identity đã xác thực → phải
  bị từ chối hoặc identity đã xác thực luôn thắng (không dùng
  `body.UserID`), không phải 1 test tuỳ chọn

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test ./internal/adapter/httpgateway/... -run TestHandleMobilePushSubscribe -v
```

## gitnexus

`impact({target: "mountNotificationRoutes", direction: "upstream"})`
trước khi thêm route mới cạnh route hiện có — xác nhận thứ tự mount trong
`router.go` (route literal path thắng, theo comment sẵn có trong
`mountNotificationRoutes`) không bị phá bởi route mới.

## Blocking

Không có task nào khác trong bộ `mobile-companion` phụ thuộc ngược task
này — đây là task lá của toàn bộ cây phụ thuộc.
