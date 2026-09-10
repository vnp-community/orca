# BE-NOTIF-SOL-002: Usecase `DeliverPush` — Web Push thật khi `Channels` chứa `push`

> **🔲 Designed — chưa implement.** Không phụ thuộc cứng kỹ thuật vào BE-NOTIF-SOL-001; khuyến nghị làm SAU (xem [README](./README.md)'s "Quan hệ phụ thuộc") để tránh 2 PR cùng sửa `HandleIncomingEvent.Execute` và để tái dùng `NotificationEvent.ID` đã persist.

**CR:** [CR-NOTIF-002](../../../../../../docs/crs/v4/notification/CR-NOTIF-002-deliver-push-usecase.md)
**Service:** `notification-service`
**TDD tham chiếu:** [`notification-service.md`](../../../../tdd/services/notification-service.md) §6 (package layout — `adapter/external/webpush/`, `deliver_push.go` đã dự kiến sẵn tên/vị trí), §7 ("Mobile push is a separate path entirely"), §9 (security — Vault Transit signing)

---

## 1. Trạng thái hiện tại (re-verify — xem [README](./README.md) bảng đối chiếu)

Xác nhận lại đúng CR gốc: `VaultSigner.SignVapidPayload` có implementation thật (`adapter/vaultsigner/signer.go:46`, gọi `credential-broker-service`'s `SignVapidPayload` RPC thật — xác nhận bằng `codegraph_explore`, thấy cả phía server `credential-broker-service/internal/adapter/grpc/server.go:195` implement RPC này), nhưng **0 call site** nào trong `notification-service` gọi `signer.SignVapidPayload(...)` — field `Server.signer` (`server.go:34`) chỉ được gán ở constructor, không đọc ở bất kỳ handler nào trong 4 RPC hiện có. `push_subscriptions.status` đã có sẵn giá trị `expired` trong CHECK constraint (`0001_init.up.sql:17`, `domain/push_subscription.go:38` `SubscriptionExpired`) nhưng không có method `MarkExpired` nào trong `SubscriptionRepository`/`Repository` — khớp đúng CR's mục C.

### Quyết định khác/thêm so với CR gốc

1. **CR gốc mục B nói "dùng thư viện Web Push chuẩn của Go ecosystem"** nhưng không chốt tên thư viện, và **không giải quyết một xung đột thật**: mọi thư viện Web Push Go phổ biến (ví dụ `webpush-go`) tự ký VAPID JWT bằng private key ECDSA cục bộ truyền vào qua option — nhưng thiết kế bảo mật của service này (§9) bắt buộc **không private key nào được phép vào process này**, chữ ký phải qua `VaultSigner.SignVapidPayload` (Vault Transit, remote call). Hai giả định này xung đột trực tiếp: không thể "chỉ gọi thư viện" mà vẫn giữ đúng §9. **Quyết định ở đây: không dùng API ký-tích-hợp-sẵn của bất kỳ thư viện Web Push nào** — implement trực tiếp 2 phần tách rời bằng thư viện chuẩn/stdlib Go, không tự chế thuật toán mật mã:
   - **Message encryption (RFC 8291)**: dùng `crypto/ecdh` (stdlib từ Go 1.20+, service đã ở Go 1.25) cho ECDH P-256 + `golang.org/x/crypto/hkdf` (đã là dependency gián tiếp có sẵn trong `go.sum`, xem `go.mod`) cho HKDF key derivation + `crypto/aes`/`crypto/cipher` (stdlib) cho AES-128-GCM — đây là đúng bộ nguyên thủy RFC 8291 yêu cầu, không phải tự nghĩ ra thuật toán mới.
   - **VAPID JWT (RFC 8292)**: tự build `header.claims` (base64url, `ES256`/`P-256`), rồi gọi `VaultSigner.SignVapidPayload(ctx, tenantID, signingInput)` để lấy chữ ký (Vault Transit ký hộ, không phải tự ký cục bộ), rồi ghép `header.claims.signature` — đây chính xác là mô hình "Vault: sign call, not read secret, then sign locally" mà `ports.go:39-47`'s comment đã mô tả cho `SignVapidPayload`.
   - Rủi ro của quyết định này: nhiều code hơn là gọi 1 hàm thư viện — chấp nhận được vì đây là ~150-200 dòng thuật toán đã chuẩn hoá (RFC), không phải logic nghiệp vụ dễ sai, và tránh việc phải vá/fork 1 thư viện third-party để tiêm luồng ký qua Vault vào giữa nó.
2. **CR gốc mục D để "khuyến nghị"** dùng `NotificationEvent.ID` đã persist khi BE-NOTIF-SOL-001 merge trước. Solution này **không tự tạo ID riêng cho push** trong mọi trường hợp — `DeliverPush.Execute` nhận thẳng `domain.NotificationEvent` (đã có `ID` từ `HandleIncomingEvent`, dù BE-NOTIF-SOL-001 đã merge hay chưa, vì `TranslateEvent` luôn tự sinh `ID` qua `uuid.NewString()` bất kể có persist hay không) — nghĩa là **không cần chờ BE-NOTIF-SOL-001** để có 1 ID ổn định; điểm phụ thuộc mềm chỉ còn đúng lý do "tránh gửi trùng khi restart", không phải "thiếu ID".

## 2. Giải pháp

### A. `ports.go` — thêm `WebPushSender`, mở rộng `SubscriptionRepository`

```go
// WebPushSender sends 1 already RFC-8291-encrypted Web Push message to 1
// subscription's endpoint. Implemented by internal/adapter/external/webpush.
type WebPushSender interface {
    // Send returns (expired=true, nil) when the push service responded
    // 404/410 — the caller (DeliverPush) must not treat this as an error,
    // it means "subscription is dead", not "delivery failed transiently".
    Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (expired bool, err error)
}
```

`SubscriptionRepository` thêm `MarkExpired(ctx context.Context, endpoint string) error` — cùng khuôn idempotent-by-design như `DeleteByEndpoint` (affect 0 rows không phải lỗi, vì có thể đã bị unregister song song).

### B. `internal/adapter/external/webpush/` — encrypt + VAPID JWT + gửi

```go
// Package webpush implements usecase.WebPushSender: RFC 8291 message
// encryption + RFC 8292 VAPID auth, sent as an HTTP POST to the
// subscription's push-service endpoint. VAPID JWT signing never happens
// locally — see Sender.buildVapidHeader, which delegates the actual
// signature to usecase.VaultSigner (Vault Transit), matching §9's rule
// that no VAPID private key material ever enters this process.
package webpush

type Sender struct {
    signer usecase.VaultSigner
    http   *http.Client
}

func (s *Sender) Send(ctx context.Context, sub domain.PushSubscription, tenantID string, payload []byte) (expired bool, err error) {
    encrypted, salt, serverPub, err := encrypt(payload, sub.P256dhKey, sub.AuthKey) // RFC 8291 — crypto/ecdh + hkdf + aes-gcm
    if err != nil { return false, fmt.Errorf("webpush: encrypt: %w", err) }

    authHeader, err := s.buildVapidHeader(ctx, tenantID, sub.Endpoint) // RFC 8292 — signs via s.signer.SignVapidPayload
    if err != nil { return false, fmt.Errorf("webpush: vapid header: %w", err) }

    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(encrypted))
    req.Header.Set("Content-Encoding", "aes128gcm")
    req.Header.Set("TTL", "86400")
    req.Header.Set("Authorization", authHeader)
    resp, err := s.http.Do(req)
    if err != nil { return false, fmt.Errorf("webpush: post: %w", err) }
    defer resp.Body.Close()

    switch resp.StatusCode {
    case http.StatusCreated, http.StatusOK, http.StatusAccepted:
        return false, nil
    case http.StatusNotFound, http.StatusGone:
        return true, nil // expired — caller marks the subscription, does not retry
    default:
        return false, fmt.Errorf("webpush: push service returned %d", resp.StatusCode)
    }
}
```

`encrypt`/`buildVapidHeader`'s nội dung chi tiết (byte layout RFC 8291, JWT claims RFC 8292) đọc trực tiếp 2 RFC khi implement — solution này chỉ khoá kiến trúc (đầu vào/đầu ra, ai ký, ai gọi), không chép lại toàn bộ đặc tả mật mã vào tài liệu. `salt`/`serverPub` mỗi lần gửi PHẢI random mới (RFC 8291 §3.1 — không tái dùng salt giữa các lần gửi, kể cả cùng subscription) — điểm dễ sai nhất khi implement, ghi rõ trong test case (TASK-BE-NOTIF-010).

### C. `DeliverPush` usecase

```go
// internal/usecase/deliver_push.go
type DeliverPush struct {
    subscriptions SubscriptionRepository
    sender        WebPushSender
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
    hasPush := slices.Contains(event.Channels, domain.ChannelDeliveryPush)
    if !hasPush {
        return nil
    }
    for _, userID := range event.RecipientUserIDs {
        subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
        if err != nil {
            return apperrors.New(apperrors.KindInternal, "NOTIFICATION_PUSH_LIST_SUBS_FAILED", "failed to list push subscriptions", err)
        }
        payload, _ := json.Marshal(pushPayload{ID: event.ID, Type: event.Type, Title: event.Title, Body: event.Body, DeepLink: event.DeepLink})
        for _, sub := range subs {
            if sub.Channel != domain.ChannelWeb {
                continue // mobile (ios/android) ngoài phạm vi CR này — xem "Không thuộc phạm vi"
            }
            expired, err := uc.sender.Send(ctx, sub, event.TenantID, payload)
            if expired {
                _ = uc.subscriptions.MarkExpired(ctx, sub.Endpoint) // best-effort — không abort các subscription khác nếu lỗi ở đây
                continue
            }
            if err != nil {
                slog.ErrorContext(ctx, "web push delivery failed", slog.String("endpoint", sub.Endpoint), slog.Any("error", err))
                continue // đúng doc comment CR: 1 subscription lỗi không được chặn subscription/recipient khác
            }
        }
    }
    return nil
}
```

`ListByUser` đã lọc `status = 'active'` sẵn (xem `repository.go`'s `ListByUser` query) — `DeliverPush` không cần tự lọc status, chỉ lọc theo `Channel == web` (mobile ngoài phạm vi).

### D. Wire vào `HandleIncomingEvent.Execute`

Gọi ngay sau `Broadcast` (dòng ~88), không chặn nhau — lỗi `DeliverPush` KHÔNG được làm `Execute` trả lỗi (không NAK JetStream chỉ vì 1 push gửi lỗi, WS đã broadcast thành công là đủ để coi event "đã xử lý"):

```go
uc.broadcaster.Broadcast(ctx, event)
if err := uc.deliverPush.Execute(ctx, event); err != nil {
    uc.logger.ErrorContext(ctx, "deliver push failed", slog.Any("error", err))
    // không return err — xem lý do ở trên
}
return nil
```

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Tự implement RFC 8291/8292 thay vì dùng thư viện có sẵn | Trung bình | Xem "Quyết định khác" mục 1 — cân nhắc lại nếu team tìm được thư viện Go hỗ trợ "external signer callback" cho VAPID (không tự ký local); nếu có, ưu tiên dùng thay vì tự viết |
| Chung `HandleIncomingEvent.Execute` với BE-NOTIF-SOL-001 | Trung bình | Làm sau BE-NOTIF-SOL-001, xem README |
| `credential-broker-service` down khi push đang gửi | Thấp | `SignVapidPayload` trả lỗi rõ ràng (`vaultsigner: credential-broker-service SignVapidPayload: %w`) — `DeliverPush` log và bỏ qua subscription đó, không crash consumer loop |
| Salt/ephemeral key tái sử dụng giữa các lần gửi (lỗi mật mã nghiêm trọng nếu implement sai) | Cao nếu implement sai | Bắt buộc test riêng xác nhận 2 lần gọi liên tiếp cho cùng subscription tạo ra ciphertext khác nhau — xem TASK-BE-NOTIF-010 |

## Không thuộc phạm vi solution này

- Mobile push thật (APNs/FCM) — `Channel != web` bị skip có chủ đích trong `DeliverPush.Execute`.
- Per-user preference lọc channel.
- Retry/backoff queue cho lỗi tạm thời (không phải 410/404).
- Bảng `notification_events` của BE-NOTIF-SOL-001 — không phụ thuộc cứng để chạy được.

## Liên quan

- `backend-go/services/notification-service/internal/usecase/ports.go:39-50` (`VaultSigner`)
- `backend-go/services/notification-service/internal/adapter/vaultsigner/signer.go` (implementation thật, khuôn mẫu gọi `credential-broker-service`)
- `backend-go/services/notification-service/internal/adapter/grpc/server.go:22-45` (`Server.signer` đã wire sẵn)
- `backend-go/services/notification-service/internal/domain/push_subscription.go` (`SubscriptionExpired`)
- `backend-go/services/notification-service/internal/adapter/postgres/repository.go` (`ListByUser`, khuôn mẫu thêm `MarkExpired`)
- [BE-NOTIF-SOL-001](./BE-NOTIF-SOL-001-persist-unread-state.md) (phụ thuộc mềm, xem README)
