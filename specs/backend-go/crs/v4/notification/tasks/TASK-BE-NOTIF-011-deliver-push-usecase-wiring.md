# TASK-BE-NOTIF-011: Usecase `DeliverPush` + wire vào `HandleIncomingEvent`

**Solution:** BE-NOTIF-SOL-002 §2.C, §2.D | **CR:** CR-NOTIF-002 (mục A, Changes Required)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-009, TASK-BE-NOTIF-010
**Status:** ✅ DONE (2026-09-09) — kèm 1 sửa kiến trúc so với sketch gốc

> **⚠️ Lỗi kiến trúc phát hiện khi implement: sketch gốc của task này (và của
> TASK-010) đặt `BuildVapidAuthHeader` trong package `internal/adapter/external/webpush`,
> rồi cho `deliver_push.go` (usecase layer) `import "...adapter/external/webpush"`
> để gọi nó — VI PHẠM dependency inversion mà `notification-service` (và toàn
> `backend-go`) tuân thủ nghiêm ngặt: xác nhận bằng grep toàn service, `internal/usecase/`
> KHÔNG BAO GIỜ import `internal/adapter/*` trong code thật (chỉ trong `_test.go`
> cho integration test). Tệ hơn: `webpush` package đã import ngược `usecase` (để
> lấy type `VaultSigner`) — nếu giữ nguyên sketch, `usecase` import `webpush` sẽ
> tạo **import cycle thật**, không build được.
>
> **Đã sửa**: di chuyển toàn bộ `vapid.go` (`BuildVapidAuthHeader`,
> `derECDSASignatureToRawJWS`, `stripVaultVersionPrefix`, `asn1ECDSASignature`,
> `base64url`) từ `internal/adapter/external/webpush/vapid.go` sang
> `internal/usecase/vapid.go` — hợp lý vì hàm này chỉ phụ thuộc `VaultSigner`
> (port đã có sẵn trong `usecase`) và stdlib crypto, không có I/O thật nào cần
> vai trò "adapter" (khác `sender.go`/`encrypt.go`, thật sự làm HTTP + mã hoá,
> đúng là adapter). `vapid_test.go` di chuyển theo. Đã cập nhật lại "Kết quả
> thực tế" của TASK-BE-NOTIF-010 để không còn liệt `vapid.go` là file của
> `webpush` package.
>
> **Kết quả thực tế còn lại (đúng sketch)**:
> - `deliver_push.go`: implement đúng `DeliverPush`/`Execute`/`deliverOne`, thêm
>   helper `originOf` (lấy scheme+host từ endpoint, trả lỗi rõ ràng nếu endpoint
>   không hợp lệ — task sketch dùng `originOf(sub.Endpoint)` như thể không lỗi
>   được, sửa lại có xử lý lỗi).
> - `config.go`: thêm `VAPID_CONTACT_URI` (mặc định `mailto:support@orca.dev`)
>   theo đúng yêu cầu "không hard-code, RFC 8292 cần contact URI thật" — `sub`
>   claim lấy từ đây, không phải placeholder cố định trong code.
> - `handle_incoming_event.go`: thêm field `deliverPush *DeliverPush`, gọi
>   `Execute` SAU `Broadcast`, lỗi chỉ log không fail. `impact({target:
>   "HandleIncomingEvent"})` trước khi sửa: LOW risk, đúng 1 caller trực tiếp
>   (`NewHandleIncomingEvent` → `main.go`'s `run`).
> - `main.go`: wire `webpush.New(nil)` + `NewDeliverPush(repo, repo, signer,
>   webpushSender, cfg.VapidContactURI, logger)`, truyền vào
>   `NewHandleIncomingEvent`.
> - 8 test case yêu cầu đều viết + PASS, cộng test mức `HandleIncomingEvent`
>   xác nhận lỗi push không làm fail `Execute` — dùng subject thật
>   `"orca.task.task.completed"` (subject trong sketch, `"orca.task.v1.assigned"`,
>   không tồn tại trong `subjectRules`, đã sửa lại).
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/usecase/... -run 'TestDeliverPush|TestHandleIncomingEvent' -v`
> — **16/16 PASS** (8 test mới TASK-011 + 8 test `HandleIncomingEvent` hồi quy,
> không cái nào vỡ). `go test ./...` toàn `notification-service` PASS 100%.
>
> **Xác nhận phạm vi thay đổi** (thay `detect_changes()` — bị nhiễu bởi agent
> khác chạy song song cùng thư mục): `git status --porcelain` giới hạn đúng
> `notification-service/` + `credential-broker-service/` (fix bug TASK-010) +
> `api-gateway/.../notification_routes.go` (TASK-008) — không đụng file service
> nào khác ngoài dự kiến của cả bộ Track 1+2.

## Mục tiêu

Usecase `DeliverPush`: với 1 `NotificationEvent` có `Channels` chứa `push`, gửi Web Push tới mọi subscription `active` kênh `web` của mỗi recipient; wire vào `HandleIncomingEvent.Execute` sau `Broadcast`, không chặn lỗi.

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/deliver_push.go` (MỚI)
2. `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` (MODIFY)
3. `backend-go/services/notification-service/internal/adapter/grpc/frame.go` hoặc file domain liên quan — KHÔNG cần sửa (không có thay đổi wire format ở task này)
4. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY)
5. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY nếu cần — xác nhận `VapidKeyRepository.GetPublicKey` đã đủ để lấy `k=` param cho `BuildVapidAuthHeader`, không cần port mới)

## `deliver_push.go`

```go
package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type pushPayload struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link,omitempty"`
}

// DeliverPush sends event as a VAPID-signed Web Push message to every
// active `web`-channel subscription of event.RecipientUserIDs, when
// event.Channels contains ChannelDeliveryPush. Mobile (ios/android)
// channels are out of scope (see solutions/BE-NOTIF-SOL-002's "Không
// thuộc phạm vi"). One subscription's failure (expired endpoint, signing
// error, transient network error) must not abort delivery to other
// subscriptions/recipients.
type DeliverPush struct {
	subscriptions SubscriptionRepository
	vapidKeys     VapidKeyRepository
	signer        VaultSigner
	sender        WebPushSender
	logger        *slog.Logger
}

func NewDeliverPush(subscriptions SubscriptionRepository, vapidKeys VapidKeyRepository, signer VaultSigner, sender WebPushSender, logger *slog.Logger) *DeliverPush {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliverPush{subscriptions: subscriptions, vapidKeys: vapidKeys, signer: signer, sender: sender, logger: logger}
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	if !slices.Contains(event.Channels, domain.ChannelDeliveryPush) {
		return nil
	}

	key, err := uc.vapidKeys.GetPublicKey(ctx, event.TenantID)
	if err != nil {
		// No active VAPID key for this tenant — nothing can be signed;
		// this is not a caller error (WS already delivered via Broadcast),
		// log and no-op rather than failing the whole event.
		uc.logger.WarnContext(ctx, "no active vapid key, skipping push delivery", slog.String("tenant_id", event.TenantID), slog.Any("error", err))
		return nil
	}

	payload, err := json.Marshal(pushPayload{ID: event.ID, Type: event.Type, Title: event.Title, Body: event.Body, DeepLink: event.DeepLink})
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_PUSH_PAYLOAD_MARSHAL_FAILED", "failed to marshal push payload", err)
	}

	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			uc.logger.ErrorContext(ctx, "failed to list push subscriptions", slog.String("user_id", userID), slog.Any("error", err))
			continue // 1 recipient's lookup failure must not abort the others
		}
		for _, sub := range subs {
			if sub.Channel != domain.ChannelWeb {
				continue
			}
			uc.deliverOne(ctx, event.TenantID, key.PublicKey, sub, payload)
		}
	}
	return nil
}

func (uc *DeliverPush) deliverOne(ctx context.Context, tenantID, publicKey string, sub domain.PushSubscription, payload []byte) {
	authHeader, err := webpush.BuildVapidAuthHeader(ctx, uc.signer, tenantID, originOf(sub.Endpoint), "mailto:support@example.com")
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to build vapid auth header", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
		return
	}
	expired, err := uc.sender.Send(ctx, sub, authHeader, payload)
	if expired {
		if markErr := uc.subscriptions.MarkExpired(ctx, sub.Endpoint); markErr != nil {
			uc.logger.ErrorContext(ctx, "failed to mark subscription expired", slog.String("endpoint", sub.Endpoint), slog.Any("error", markErr))
		}
		return
	}
	if err != nil {
		uc.logger.ErrorContext(ctx, "web push delivery failed", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
	}
}
```

**Điểm cần xác nhận khi implement** (không bịa): `subject`/`aud` claim thật của VAPID JWT (`"mailto:support@example.com"` ở trên là placeholder) — đọc `VapidKeyMetadata`/config service xem có sẵn 1 contact URI cấu hình theo tenant hay theo service chưa; nếu chưa có, thêm vào `internal/config/config.go` (biến môi trường mới, ví dụ `VAPID_CONTACT_URI`) thay vì hard-code — RFC 8292 yêu cầu `sub` là 1 `mailto:`/`https:` URI liên hệ được của publisher, không phải giá trị tuỳ ý. `originOf(endpoint)` (lấy scheme+host từ URL) cần 1 helper nhỏ dùng `net/url` — viết trong cùng file hoặc `vapid.go` của TASK-BE-NOTIF-010, không thêm dependency mới cho việc này.

## `handle_incoming_event.go` — wire `DeliverPush`

```go
type HandleIncomingEvent struct {
	broadcaster     NotificationBroadcaster
	processedEvents ProcessedEventRepository
	notifications   NotificationRepository // nếu TASK-BE-NOTIF-003 đã merge; nếu chưa, bỏ field này và chỉ thêm deliverPush
	deliverPush     *DeliverPush
	logger          *slog.Logger
}
```

```go
	uc.broadcaster.Broadcast(ctx, event)

	// Push delivery failure must not NAK the JetStream message — WS
	// fan-out already succeeded, which is enough to consider the event
	// "handled"; a push failure is logged, not retried via redelivery.
	if err := uc.deliverPush.Execute(ctx, event); err != nil {
		uc.logger.ErrorContext(ctx, "deliver push failed", slog.String("event_id", in.EventID), slog.Any("error", err))
	}
	return nil
```

**Nếu TASK-BE-NOTIF-003 (Track 1) đã merge trước** (khuyến nghị theo [tasks/README.md](./README.md)): chạy lại `impact({target: "HandleIncomingEvent", direction: "upstream"})` NGAY TRƯỚC khi sửa task này để lấy đúng version mới nhất của hàm (đã có bước `Save`) — chèn `DeliverPush.Execute` SAU `Broadcast`, giữ nguyên bước `Save` đã có ở giữa `TranslateEvent` và `Broadcast`.

## `cmd/server/main.go` — wiring

```go
webpushSender := webpush.New(signer, nil)
deliverPushUC := usecase.NewDeliverPush(repo, repo, signer, webpushSender, logger)
handleIncomingEventUC := usecase.NewHandleIncomingEvent(broadcast, repo, repo, deliverPushUC, logger) // thứ tự tham số khớp field mới thêm — xác nhận lại theo state thật của handle_incoming_event.go tại thời điểm implement
```

## Test cases cần cover

- `TestDeliverPush_ChannelsWithoutPush_NoOp` — event's `Channels` không chứa `push` → không gọi `ListByUser`/`sender.Send` nào (assert qua fake).
- `TestDeliverPush_SendsToEveryActiveWebSubscription` — 3 subscription (2 `web`, 1 `ios`) → chỉ 2 lần gọi `sender.Send`.
- `TestDeliverPush_NoActiveSubscription_NoError` — recipient không có subscription nào → no-op sạch, không lỗi (mirror `Broadcaster`'s "no active subscription" case, đúng tiêu chí chấp nhận CR).
- `TestDeliverPush_OneSubscriptionFails_OthersStillAttempted` — fake sender trả lỗi cho subscription 1, thành công cho subscription 2 → cả 2 đều được gọi (không dừng sớm).
- `TestDeliverPush_ExpiredSubscription_CallsMarkExpired`.
- `TestDeliverPush_NoActiveVapidKey_NoOpNoError`.
- `TestDeliverPush_SignerCalledOncePerSubscription` — fake `VaultSigner` assert số lần gọi khớp số subscription `web` active.
- `TestHandleIncomingEvent_DeliverPushErrorDoesNotFailExecute` — fake `DeliverPush`-equivalent (hoặc fake `WebPushSender`/`VaultSigner` gây lỗi) → `Execute` vẫn trả `nil`.

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/usecase/... -run 'TestDeliverPush|TestHandleIncomingEvent'
go test ./...
gofmt -l internal/usecase/deliver_push.go internal/usecase/handle_incoming_event.go cmd/server/main.go
```

## gitnexus

`impact({target: "HandleIncomingEvent", direction: "upstream"})` bắt buộc trước khi sửa (task này sửa cùng hàm với TASK-BE-NOTIF-003 — xem cảnh báo ở trên). `impact({target: "VaultSigner", direction: "upstream"})` trước khi thêm call site đầu tiên gọi `SignVapidPayload` — theo CR-NOTIF-002's khảo sát, đây sẽ là call site DUY NHẤT trong toàn service tính tới thời điểm merge task này; xác nhận không phá test hiện có nào giả định "0 call site" (nếu có test kiểu `TestVaultSigner_NoCallSitesYet`, không nên có nhưng kiểm tra để chắc).

## Blocking

Không có — đây là task cuối của Track 2. `gitnexus detect_changes({scope:"compare", base_ref:"main"})` trước khi commit toàn bộ Track 2, theo yêu cầu bắt buộc của CR-NOTIF-002's tiêu chí chấp nhận.
