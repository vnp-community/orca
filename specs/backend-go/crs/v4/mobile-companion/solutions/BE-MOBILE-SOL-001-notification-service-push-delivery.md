# BE-MOBILE-SOL-001: `notification-service` — triển khai thật `DeliverPush` (APNs/FCM)

> **🔲 Designed — chưa implement.** Độc lập, làm trước BE-MOBILE-SOL-002.

**CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service` (chính), `credential-broker-service` (gọi qua, không sửa), `api-gateway` (1 file nhỏ)
**TDD tham chiếu:** [`notification-service.md`](../../../../tdd/services/notification-service.md) §4, §6, §9

---

## 1. Trạng thái hiện tại — quan trọng, thay đổi thiết kế so với CR gốc

Audit khi viết solution (khác thời điểm viết CR, dùng Read/Grep trực tiếp trên
code hiện tại) xác nhận **đúng** phần lớn khảo sát của CR-MOBILE-001:

- `domain.Channel` đã có `ChannelWeb`/`ChannelIOS`/`ChannelAndroid`
  (`push_subscription.go:17-21`), migration `0001_init.up.sql:12` đã cho phép
  `channel IN ('web','ios','android')` từ đầu — **không cần migration mới**.
- `domain.DeliveryChannel` (`ChannelDeliveryWS`/`ChannelDeliveryPush`,
  `notification_event.go:14-18`) và `subjectRules` (dòng ~98-136) đã gán
  `ChannelDeliveryPush` cho đúng 6 subject F03 cần — **dữ liệu được tính
  nhưng chưa ai đọc**, đúng như CR mô tả.
- `HandleIncomingEvent.Execute` (`handle_incoming_event.go:88`, dòng cuối)
  xác nhận **chỉ** gọi `uc.broadcaster.Broadcast(ctx, event)` — bỏ hoàn toàn
  `event.Channels`. Đúng như CR.
- `Subscribe.Execute` (`subscribe.go:44-61`) xác nhận **hard-code**
  `domain.ChannelWeb` khi gọi `domain.NewPushSubscription(...)` — tham số
  `channel` không tồn tại trong `SubscribeInput`. Đúng như CR.
- `notification.proto`'s `SubscribeRequest` (đọc trực tiếp file) chỉ có 4
  field (`user_id`, `endpoint`, `p256dh_key`, `auth_key`) — không có
  `channel`/`device_label`. Đúng như CR.
- `deliver_push.go`, `internal/adapter/pushgateway/` (hay `external/webpush/`
  design doc cũ đề xuất) **không tồn tại** — `find ... -name "*.go"` liệt kê
  đúng 20 file, không có 2 mục này. Đúng như CR.
- `SubscriptionRepository`/`ports.go` không có phương thức `DeliverPush(ctx,
  event, subscription)` nào. Đúng như CR.

### 1.1. Sai lệch thật — credential storage KHÔNG còn đi qua Vault trực tiếp

CR-MOBILE-001's mục C viết: *"lưu qua Vault Transit giống cách `VaultSigner`
hiện tại xử lý VAPID (`internal/adapter/vaultsigner/signer.go`)"* — đọc thẳng
file này (không đoán từ tên) cho thấy **giả định này đã lỗi thời**:

```go
// internal/adapter/vaultsigner/signer.go — package doc comment
// Package vaultsigner implements usecase.VaultSigner against
// credential-broker-service's SignVapidPayload RPC — Epic B ...
// Before Epic B, this package called common/secrets.TransitEncrypt
// directly against Vault ... Epic B closes that exception for real:
// credential-broker-service now owns the "vapid-signing-<tenant_id>"
// Transit key and this adapter is a thin gRPC client ...
```

`vaultsigner.Signer.SignVapidPayload` ngày nay chỉ là 1 gRPC client gọi
`credentialbrokerv1.CredentialBrokerServiceClient.SignVapidPayload` —
**`notification-service` không còn tự chạm Vault ở đâu cả** kể từ Epic B.
`cmd/server/main.go` (dòng ~85-96) xác nhận service này đã dial sẵn 1 kết nối
gRPC tới `credential-broker-service` (`cfg.CredentialBrokerAddr`, biến qua
`brokerConn`) đúng cho mục đích này.

**Hệ quả cho thiết kế**: adapter APNs/FCM mới **không được** tự implement 1
đường Vault Transit riêng (sẽ tái tạo đúng "exception" mà Epic B vừa đóng) —
phải lấy credential qua `credential-broker-service`, dùng lại `brokerConn` đã
có sẵn trong `main.go`, giống hệt pattern `vaultsigner` đang dùng.

### 1.2. Sai lệch thứ hai — `VaultSigner`/`SignVapidPayload` KHÔNG dùng được cho APNs/FCM

`adapter/grpc/server.go`'s `Server` struct đã có sẵn field này, với comment
tại chỗ:

```go
// signer is wired for the future DeliverPush usecase
// (mobile push delivery via APNs/FCM, notification-service.md §6's
// deliver_push.go) — not yet called from any RPC path in this
// scaffold. See this service's README "Known gaps".
signer usecase.VaultSigner
```

Comment này **tự mâu thuẫn về mặt kỹ thuật**: `SignVapidPayload` ký JWT VAPID
theo RFC 8292 — giao thức chỉ dùng cho **Web Push** (browser). APNs dùng JWT
ES256 ký bằng Apple Auth Key (Team ID + Key ID + `.p8` — không phải VAPID),
FCM dùng OAuth2 service-account JSON — cả hai **không liên quan gì tới VAPID
signing**. `DeliverPush`/`pushgateway` mới **không tái sử dụng** `signer`
field này (nó thuộc về đường Web Push riêng, chưa từng bị đụng tới ở CR này)
— cần 1 port mới (`PushCredentialResolver`, §2.3 dưới đây) lấy credential
thô (bytes) qua `credential-broker-service`, không phải chữ ký.

### 1.3. Phát hiện thêm — `Save()`'s upsert luôn ép `status = 'active'`

`postgres/repository.go`'s `Save` (dòng ~34-58) — SQL `ON CONFLICT (endpoint)
DO UPDATE SET ... status = 'active'` **luôn** ghi đè `status` về `active`,
bất kể `PushSubscription.Status` truyền vào là gì. `DeliverPush`'s yêu cầu
"token hết hạn → đánh dấu `SubscriptionExpired`" (tiêu chí chấp nhận của CR)
**không thể** làm qua `Save()` hiện có — cần 1 phương thức repository mới,
`MarkExpired(ctx, endpoint string) error`, không đụng tới `Save`.

## 2. Giải pháp

### 2.1. Proto + `Subscribe` usecase — nhận `channel`/`device_label`

```protobuf
// backend-go/proto/orca/notification/v1/notification.proto
message SubscribeRequest {
  string user_id = 1;
  string endpoint = 2;
  string p256dh_key = 3;
  string auth_key = 4;
  string channel = 5;       // "web" (mặc định nếu rỗng) | "ios" | "android"
  string device_label = 6;  // optional
}
```

`SubscribeInput` thêm `Channel string`/`DeviceLabel string`; `Execute` map
`in.Channel` rỗng → `domain.ChannelWeb` (giữ nguyên hành vi hiện tại của
`useWebPushSubscription.ts`), khác rỗng → `domain.Channel(in.Channel)`, để
`domain.NewPushSubscription`'s `Channel.Valid()` tự chặn giá trị lạ (đã có
sẵn, không cần sửa domain).

### 2.2. `SubscriptionRepository.MarkExpired` — repo mới, không đụng `Save`

```go
// internal/usecase/ports.go — thêm vào SubscriptionRepository
// MarkExpired transitions the subscription at endpoint to
// SubscriptionExpired. Unlike Save's upsert (which always forces
// status back to 'active'), this never resurrects a row — a device
// token APNs/FCM reports dead stays dead until the client re-subscribes.
MarkExpired(ctx context.Context, endpoint string) error
```

```go
// internal/adapter/postgres/repository.go
func (r *Repository) MarkExpired(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.push_subscriptions
		SET status = 'expired', updated_at = now()
		WHERE endpoint = $1
	`, endpoint)
	if err != nil {
		return fmt.Errorf("postgres: mark subscription expired: %w", err)
	}
	return nil
}
```

### 2.3. `PushCredentialResolver` — port mới, credential-broker-service, KHÔNG Vault trực tiếp

```go
// internal/usecase/ports.go
// PushCredentialResolver fetches this tenant's APNs/FCM credential
// material via credential-broker-service — CREDENTIAL_CATEGORY_SERVICE_SECRET
// (already defined in credentialbroker.proto, handled by that service's
// grpc/server.go, but exercised by no caller yet before this). Distinct
// from VaultSigner: that port signs a VAPID JWT for Web Push; this one
// returns raw credential bytes for a completely different protocol
// (APNs JWT ES256 / FCM OAuth2), so it is its own interface, not an
// overload of VaultSigner.
type PushCredentialResolver interface {
	// Resolve returns the opaque credential blob stored for
	// (tenantID, ownerID) — ownerID is "apns" or "fcm". The blob's shape
	// is adapter-owned: apns_sender.go JSON-decodes it as
	// {team_id, key_id, private_key_pem}; fcm_sender.go treats it as the
	// raw FCM service-account JSON.
	Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error)
}
```

```go
// internal/adapter/credentialbroker/client.go (package mới) — mirror
// scm-integration-service/internal/adapter/credentialbroker/client.go's
// Resolver, cùng RPC (ResolveCredentialByOwner), category khác.
package credentialbroker

type Resolver struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

func New(conn grpc.ClientConnInterface) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

func (r *Resolver) Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET,
		OwnerId:  ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("credentialbroker: resolving %s push credential: %w", ownerID, err)
	}
	return resp.GetValue(), nil
}
```

`main.go` dùng lại đúng `brokerConn` đã dial sẵn cho `vaultsigner.New(...)` —
truyền thêm cho `notificationcredentialbroker.New(brokerConn)`, không dial
kết nối thứ hai.

Việc **ghi** credential (APNs `.p8`/FCM service-account JSON) vào
`credential-broker-service` lần đầu (`WriteCredential`,
category=`SERVICE_SECRET`, owner_id=`"apns"`/`"fcm"`) là thao tác vận hành
một lần (ops/admin), không thuộc phạm vi RPC nào của `notification-service`
— xem "Không thuộc phạm vi" bên dưới.

### 2.4. `PushSender` port + `pushgateway` adapter

```go
// internal/usecase/ports.go
// PushSender delivers event to one ios/android subscription. A
// "device token no longer valid" outcome (APNs BadDeviceToken / FCM
// UNREGISTERED) is reported via ErrDeviceTokenInvalid, not a generic
// error — DeliverPush uses this sentinel to decide whether to call
// MarkExpired.
type PushSender interface {
	Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error
}

var ErrDeviceTokenInvalid = errors.New("usecase: device token no longer valid")
```

```go
// internal/adapter/pushgateway/apns_sender.go
// HTTP/2 POST https://api.push.apple.com/3/device/{token}, header
// "authorization: bearer <JWT ES256 ký bằng credential.private_key_pem>".
// credential lấy qua PushCredentialResolver.Resolve(ctx, tenantID, "apns").
// 410/BadDeviceToken → usecase.ErrDeviceTokenInvalid.
type APNsSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client // http2-enabled Transport
}

func (s *APNsSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error { /* ... */ }
```

```go
// internal/adapter/pushgateway/fcm_sender.go
// HTTP v1 POST https://fcm.googleapis.com/v1/projects/{project}/messages:send,
// OAuth2 access token lấy từ service-account JSON (credential qua
// PushCredentialResolver.Resolve(ctx, tenantID, "fcm")) bằng
// golang.org/x/oauth2/google. UNREGISTERED (404/NOT_FOUND từ FCM) →
// usecase.ErrDeviceTokenInvalid.
type FCMSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client
}
```

`DeliverPush.Execute` chọn sender theo `subscription.Channel` (`ChannelIOS`
→ `APNsSender`, `ChannelAndroid` → `FCMSender`) — port `PushSender` là 1
interface chung, `DeliverPush` giữ 1 map `domain.Channel -> PushSender`
(không phải if/else lặp), theo đúng khuôn `internal/adapter/eventbus`'s
"1 interface, N adapter" pattern đã dùng trong service này.

### 2.5. `DeliverPush` usecase + wiring vào `HandleIncomingEvent`

```go
// internal/usecase/deliver_push.go
type DeliverPush struct {
	subscriptions SubscriptionRepository
	senders       map[domain.Channel]PushSender
	logger        *slog.Logger
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			return fmt.Errorf("deliver_push: listing subscriptions: %w", err)
		}
		for _, sub := range subs {
			sender, ok := uc.senders[sub.Channel]
			if !ok {
				continue // ChannelWeb has its own path, not this usecase's job
			}
			if err := sender.Send(ctx, sub, event); err != nil {
				if errors.Is(err, ErrDeviceTokenInvalid) {
					if markErr := uc.subscriptions.MarkExpired(ctx, sub.Endpoint); markErr != nil {
						uc.logger.ErrorContext(ctx, "failed to mark expired push subscription", slog.Any("error", markErr))
					}
					continue
				}
				uc.logger.ErrorContext(ctx, "push delivery failed", slog.String("channel", string(sub.Channel)), slog.Any("error", err))
				// Không return — 1 subscription lỗi không được chặn các subscription/user khác (tiêu chí chấp nhận CR).
			}
		}
	}
	return nil
}
```

`HandleIncomingEvent.Execute` (`handle_incoming_event.go`), ngay sau dòng
`uc.broadcaster.Broadcast(ctx, event)`, thêm:

```go
if slices.Contains(event.Channels, domain.ChannelDeliveryPush) {
	if err := uc.deliverPush.Execute(ctx, event); err != nil {
		uc.logger.ErrorContext(ctx, "deliver_push failed", slog.Any("error", err))
		// Lỗi push KHÔNG được làm fail toàn bộ Execute — WS đã gửi xong ở
		// nhánh trên; NAK lại (return err) sẽ redeliver cả nhánh WS đã
		// thành công, không đúng theo tiêu chí chấp nhận CR-MOBILE-001.
	}
}
```

`HandleIncomingEvent` struct thêm field `deliverPush *DeliverPush`,
`NewHandleIncomingEvent` thêm tham số tương ứng — `cmd/server/main.go` wire
đủ (`DeliverPush` cần `SubscriptionRepository` đã có (`repo`), và map
sender xây từ `notificationpushgateway.NewAPNsSender(credResolver, ...)`/
`NewFCMSender(credResolver, ...)`).

### 2.6. `api-gateway`'s `handleSubscribe` — passthrough `channel`/`device_label`

Chỉ phần **wire-through cơ học**, độc lập với quyết định kiến trúc mobile
auth của BE-MOBILE-SOL-002 (route/field mới cho mobile là việc của solution
đó — đây chỉ là làm cho 2 route hiện có,
`/v1/notifications/subscribe`/`/api/push-subscribe`, có khả năng nhận
`channel`/`device_label` cho bất kỳ caller nào đã xác thực được — web client
hôm nay luôn gửi `channel` rỗng nên hành vi không đổi):

```go
// notification_routes.go
type subscribeRequestBody struct {
	Endpoint    string `json:"endpoint"`
	P256dhKey   string `json:"p256dh_key"`
	AuthKey     string `json:"auth_key"`
	Channel     string `json:"channel"`      // mới — rỗng = "web" (không đổi hành vi cũ)
	DeviceLabel string `json:"device_label"` // mới
}
// handleSubscribe: thêm Channel: body.Channel, DeviceLabel: body.DeviceLabel
// vào notificationv1.SubscribeRequest{...}
```

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Credential APNs/FCM chưa từng được ghi vào `credential-broker-service` (`CREDENTIAL_CATEGORY_SERVICE_SECRET` tồn tại nhưng 0 caller thật hiện nay) | Trung bình | Cần 1 bước vận hành ghi credential trước khi `DeliverPush` có gì để gửi — xem "Không thuộc phạm vi" |
| APNs/FCM HTTP client thật (mTLS/HTTP2, OAuth2) chưa có thư viện sẵn trong `go.mod` của `notification-service` | Trung bình | Kiểm tra `go.mod` trước khi code — có thể cần thêm `golang.org/x/oauth2/google` |
| `slices.Contains`/Go version | Thấp | Xác nhận Go version của module hỗ trợ `slices` (Go 1.21+) trước khi dùng |
| Nhầm `VaultSigner`/`signer` field có sẵn với credential APNs/FCM | Cao nếu bỏ qua §1.2 | Đã ghi rõ: đây là 2 port khác nhau, không tái sử dụng |

## Không thuộc phạm vi solution này

- Ghi credential APNs (.p8/Team ID/Key ID)/FCM (service-account JSON) vào
  `credential-broker-service` lần đầu cho mỗi tenant — thao tác vận hành
  một lần, dùng `WriteCredential` RPC đã có sẵn qua công cụ/CLI nội bộ hiện
  có cho category khác (không cần RPC mới) — không phải code path của
  `notification-service`.
- Route/field REST mới riêng cho mobile không dùng cookie session — xem
  [BE-MOBILE-SOL-002](./BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md).
- Retry/dead-letter cho lỗi push không phải "token hết hạn" (v1 chỉ log,
  theo đúng CR gốc).
- Mọi thay đổi phía `mobile/` (lấy token, gọi RPC) và dọn
  `MobileCompanionService.ts` (thuần Electron/desktop, không phải
  backend-go) — xem CR-MOBILE-002; các phần backend-go còn lại của CR đó ở
  BE-MOBILE-SOL-002.

## Liên quan

- `backend-go/services/notification-service/internal/domain/{push_subscription,notification_event}.go`
- `backend-go/services/notification-service/internal/usecase/{subscribe,handle_incoming_event,ports}.go`
- `backend-go/services/notification-service/internal/adapter/{postgres/repository.go,vaultsigner/signer.go}`
- `backend-go/services/notification-service/cmd/server/main.go`
- `backend-go/services/scm-integration-service/internal/adapter/credentialbroker/client.go` (khuôn mẫu `ResolveCredentialByOwner`)
- `backend-go/proto/orca/{notification,credentialbroker}/v1/*.proto`
- [BE-MOBILE-SOL-002](./BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md) (phụ thuộc CR, không phụ thuộc code trực tiếp)
