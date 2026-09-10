# CR-NOTIF-002 — Triển khai `DeliverPush` usecase: kênh `push` được gán nhưng chưa từng gửi

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-NOTIF-002 |
| **Tên** | Thêm usecase `DeliverPush` — gửi Web Push/mobile push thật khi `NotificationEvent.Channels` chứa `push` |
| **Loại** | Feature (hoàn thiện 1 nhánh RPC surface đã thiết kế nhưng chưa xây) |
| **Priority** | 🟠 P1 — không chặn acceptance criteria chính của F11 (desktop notification khi app đang mở), nhưng chặn "Notification hoạt động khi app không focus"/đóng tab |
| **Effort** | Medium (3–4 ngày, phần khó nhất — ký VAPID JWT qua Vault — đã có sẵn) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn `backend-go/services/notification-service` theo yêu cầu hoàn thiện F11 |
| **Tác động Features** | F11 (Notifications & Unread State) |
| **Phụ thuộc** | Nên làm SAU [CR-NOTIF-001](./CR-NOTIF-001-unread-state-and-persistence.md) — dùng lại bảng `notification.notification_events` để tránh gửi trùng push khi service restart (xem "Giải pháp đề xuất" mục C) |

---

## Bối cảnh & Vấn đề

### 1. `NotificationEvent.Channels` gán `push` cho hầu hết mọi loại thông báo — nhưng không có code nào đọc field đó để gửi

`backend-go/services/notification-service/internal/domain/notification_event.go`'s `subjectRules` (dòng 97-137) gán `Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush}` cho 5/6 subject đã định nghĩa (dòng 100, 104, 108, 112, 119, 123 — chỉ trừ `star_nag_visibility` ở dòng 133-136 là WS-only có chủ đích). Nhưng:

```bash
grep -rn "\.Channels\b" backend-go/services/notification-service --include="*.go" | grep -v _test.go
# => chỉ 1 kết quả: notification_event.go:184 (nơi field được GÁN, trong TranslateEvent)
```

Không có bất kỳ file nào trong `internal/usecase/` hay `internal/adapter/` **đọc** `event.Channels` để quyết định có gửi Web Push hay không. `HandleIncomingEvent.Execute` (`backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:62-90`) chỉ gọi `uc.broadcaster.Broadcast(ctx, event)` (dòng 88) — tức là **mọi notification chỉ đi qua kênh WS**, bất kể `Channels` nói gì. Field `Channels` hiện là dữ liệu chết (dead data): được tính toán, không bao giờ được dùng.

### 2. `VaultSigner`/`SignVapidPayload` đã wire xong nhưng chưa có caller nào

- `internal/usecase/ports.go:39-50` định nghĩa interface `VaultSigner` với method `SignVapidPayload`.
- `internal/adapter/vaultsigner/` implement interface này thật, gọi `credential-broker-service`'s `SignVapidPayload` RPC (README dòng 52-58, 75-92 xác nhận đây là kết nối thật, không phải stub — Epic B, 2026-08-17).
- `internal/adapter/grpc/server.go:22-45`: struct `Server` có field `signer usecase.VaultSigner` (dòng 34), comment tại dòng 30-34 tự nhận: *"signer is wired for the future DeliverPush usecase... not yet called from any RPC path in this scaffold."*
- `mcp__gitnexus__context` xác nhận: `VaultSigner` chỉ có 4 import (main.go, server.go, consumer.go, 1 test file) và đúng 1 field kiểu `VaultSigner` (`Server.signer`) — **0 call site nào gọi `SignVapidPayload`** trong toàn bộ service.
- README's "Known gaps" (`backend-go/services/notification-service/README.md:145-150`) đã tự ghi nhận đúng gap này: *"No `DeliverPush` usecase in this slice... aren't implemented — this slice covers subscription CRUD, VAPID public-key distribution, event consumption/translation, and WS fan-out only."*

### 3. Hệ quả với F11's acceptance criteria

`docs/features/F11-notifications.md:63` yêu cầu: *"Notification hoạt động khi app không focus."* Với kiến trúc hiện tại (chỉ WS qua `StreamNotifications`), một client mất kết nối WS (đóng tab, mất mạng, máy sleep) sẽ **không nhận được bất kỳ notification nào phát sinh trong lúc đó** — đúng như thiết kế "no offline WS replay queue" đã ghi ở `specs/backend-go/tdd/services/notification-service.md:43-46`, nhưng phần bù trừ dự kiến của thiết kế đó (*"pushdeliver -- 'VAPID-signed Web Push' --> apns"*, xem mermaid diagram §2 dòng 52-85 và §7 dòng 249-254 "Mobile push is a separate path entirely... `deliver_push.go` calls APNs/FCM directly") **chưa tồn tại**. Người dùng đăng ký push subscription qua `POST /v1/notifications/subscribe` (`backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go:66-89`) nhưng **never receives anything** qua kênh đó — subscription chỉ được lưu (`Repository.Save`, `repository.go:39-62`), không bao giờ được dùng để gửi.

Đây khác về bản chất so với CR-NOTIF-001: không phải thiếu 1 khái niệm (unread state chưa từng được thiết kế), mà là **1 nhánh đã thiết kế đầy đủ (mermaid diagram, RPC ports, security notes §9) nhưng dừng giữa chừng ở lớp implementation** — README's "Known gaps" đã tự flag rõ, không phải phát hiện mới của audit này.

---

## Giải pháp đề xuất

### A. Usecase `DeliverPush` mới — đúng vị trí design doc đã chỉ định

Thêm `backend-go/services/notification-service/internal/usecase/deliver_push.go`, đúng tên file design doc đã dự kiến (`specs/backend-go/tdd/services/notification-service.md:218`):

```go
// DeliverPush sends event as a VAPID-signed Web Push message to every
// active push subscription of event.RecipientUserIDs, when
// event.Channels contains ChannelDeliveryPush. Failure to deliver to one
// subscription (expired endpoint, signing error) must not abort delivery
// to other subscriptions/recipients — each Web Push send is independent.
type DeliverPush struct {
    subscriptions SubscriptionRepository
    signer        VaultSigner
    sender        WebPushSender // new port — internal/adapter/external/webpush implements it
}
```

Gọi từ `HandleIncomingEvent.Execute` (`handle_incoming_event.go:88`, ngay sau `Broadcast`): nếu `event.Channels` chứa `ChannelDeliveryPush`, gọi `deliverPush.Execute(ctx, event)` — song song hoặc tuần tự sau `Broadcast` (không chặn WS fan-out nếu push chậm/lỗi).

### B. Adapter `internal/adapter/external/webpush/` — đúng gói design doc đã chỉ định

`specs/backend-go/tdd/services/notification-service.md:227` đã chỉ định `adapter/external/webpush/` cho "APNs/FCM clients, Web Push protocol framing". Implement Web Push protocol (RFC 8291 message encryption dùng `p256dh_key`/`auth_key` đã lưu sẵn ở `push_subscriptions`, VAPID JWT ký qua `VaultSigner.SignVapidPayload` đã có) — không tự chế lại RFC 8291, dùng thư viện Web Push chuẩn của Go ecosystem tương đương thư viện `web-push` npm mà `CR-OB-008-notification-server.md` (thiết kế TS cũ, §Backend) từng dùng.

### C. Xử lý subscription hết hạn — vòng phản hồi về `push_subscriptions.status`

Web Push trả `410 Gone`/`404` khi endpoint hết hạn (browser đã unsubscribe phía client mà server chưa biết). `DeliverPush` phải map các mã lỗi này thành `SubscriptionRepository`'s update `status = 'expired'` (cột đã có sẵn, `0001_init.up.sql:17`) — tránh retry vô ích vào 1 endpoint chết mỗi lần có event mới. Cần thêm method `MarkExpired(ctx, endpoint)` vào `SubscriptionRepository` interface (`ports.go:17-29`) và implement trong `postgres/repository.go`.

### D. Không gửi trùng khi CR-NOTIF-001 đã persist event

Nếu CR-NOTIF-001 đã merge, `DeliverPush` nên nhận `NotificationEvent` đã có `ID` persist thay vì gọi lại logic riêng — tránh trường hợp service restart giữa `Save` và `DeliverPush` khiến 1 event gửi push 2 lần. Không bắt buộc transaction 2-phase; ghi log rõ ràng khi push gửi trùng (idempotency ở mức Web Push endpoint không đảm bảo được từ phía server, nhưng dedup ở mức tần suất chấp nhận được cho notification, không phải giao dịch tài chính).

---

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/notification-service/internal/usecase/deliver_push.go` | [NEW] Usecase `DeliverPush` |
| `backend-go/services/notification-service/internal/usecase/ports.go` | [NEW] Interface `WebPushSender`; thêm method `MarkExpired` vào `SubscriptionRepository` |
| `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` | Gọi `DeliverPush.Execute` khi `event.Channels` chứa `push` (sau `Broadcast`, dòng ~88) |
| `backend-go/services/notification-service/internal/adapter/external/webpush/` | [NEW] Package implement `WebPushSender` — RFC 8291 message encryption + gửi HTTP POST tới push service endpoint |
| `backend-go/services/notification-service/internal/adapter/postgres/repository.go` | [NEW] `MarkExpired(ctx, endpoint)` |
| `backend-go/services/notification-service/internal/adapter/grpc/server.go` | Truyền `DeliverPush` usecase vào composition (constructor `New`, dòng 37-45) |
| `backend-go/services/notification-service/cmd/server/main.go` | Wire `webpush.Sender` + `DeliverPush` vào composition root |
| `backend-go/services/notification-service/README.md` | Cập nhật "Known gaps" — xoá mục "No `DeliverPush` usecase" sau khi merge |

---

## Không thuộc phạm vi CR này

- **Mobile push thật (APNs/FCM)** — design doc (§7 dòng 249-254) coi Web Push và mobile push là "2 cơ chế phân biệt"; `push_subscriptions.channel` đã hỗ trợ `ios`/`android` trong schema (`0001_init.up.sql:12`) nhưng CR này chỉ làm `web` channel trước (đúng với những gì frontend/`CR-OB-008` đã thiết kế Web Push cho browser) — APNs/FCM client thật là CR riêng khi có thiết bị mobile thật cần hỗ trợ.
- **Per-user preference lọc channel** — README đã ghi "explicitly out of scope" (dòng 164-166); CR này gửi push cho mọi subscription active của recipient, không filter theo preference.
- **Retry/backoff queue cho push gửi lỗi tạm thời (không phải 410/404)** — v1 gửi 1 lần, log lỗi; retry queue là cải tiến sau nếu observability cho thấy tỷ lệ lỗi tạm thời đáng kể.
- **CR-NOTIF-001's bảng `notification_events`** — không bắt buộc CR này phụ thuộc cứng vào đó để chạy được (Broadcast hiện tại không cần bảng đó), chỉ khuyến nghị làm sau để tránh gửi trùng (mục D).

---

## Tiêu chí chấp nhận

- [ ] Một event có `Channels` chứa `push` khiến `DeliverPush` gửi Web Push thật tới mọi subscription `active` của recipient (verify bằng test server giả lập Web Push endpoint, không cần trình duyệt thật).
- [ ] Endpoint trả `410 Gone` → subscription tương ứng chuyển `status = 'expired'` trong `push_subscriptions`, không retry.
- [ ] `VaultSigner.SignVapidPayload` được gọi thật (test bằng fake `VaultSigner`, assert `Execute` được gọi đúng 1 lần / subscription).
- [ ] Một recipient không có subscription `active` nào → `DeliverPush` không lỗi, không gọi Web Push nào (no-op sạch, giống `Broadcast`'s "no active subscription" case ở `broadcaster.go:100-102`).
- [ ] `go test ./...` xanh cho `notification-service`; `gitnexus detect_changes({scope:"compare", base_ref:"main"})` sạch trước khi commit.

---

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted |
|---|---|---|---|
| `HandleIncomingEvent` (`notification-service/internal/usecase/handle_incoming_event.go:34`) | upstream | LOW | 3 (1 direct, module `Usecase`) — symbol này cũng bị CR-NOTIF-001 sửa; nếu làm cả 2 CR, review gộp 1 lần thay vì 2 lần riêng biệt |
| `Subscribe` (`notification-service/internal/usecase/subscribe.go`) | upstream | LOW | 3 (1 direct, module `Usecase`) |
| `VaultSigner` (`notification-service/internal/usecase/ports.go:47`) | upstream (context, chưa chạy impact số liệu) | — | 4 import site thật (`main.go`, `server.go`, `consumer.go`, `broadcaster_test.go`), đúng 1 typed property (`Server.signer`) — xác nhận bằng chứng "0 call site nào gọi `SignVapidPayload`" trong mục Bối cảnh #2 |

`HandleIncomingEvent` bị sửa bởi cả CR-NOTIF-001 (thêm bước `Save`) và CR-NOTIF-002 (thêm bước `DeliverPush`) — khuyến nghị merge tuần tự (NOTIF-001 trước), chạy lại `impact({target: "HandleIncomingEvent", direction: "upstream"})` ngay trước khi sửa CR-NOTIF-002 để bắt số liệu mới nhất sau khi CR-NOTIF-001 đã đổi symbol này.

---

## Liên quan

- [F11-notifications.md](../../../features/F11-notifications.md)
- [feature-completion-matrix.md](../../../roadmap/feature-completion-matrix.md) (dòng F11)
- `specs/backend-go/tdd/services/notification-service.md` (§2 mermaid diagram, §6 package layout dòng 218-227, §7 dòng 249-254, §9 security notes)
- `backend-go/services/notification-service/README.md` ("Known gaps / follow-ups" — mục "No `DeliverPush` usecase")
- [CR-NOTIF-001](./CR-NOTIF-001-unread-state-and-persistence.md) (khuyến nghị merge trước)
- `docs/crs/v1/onboarding/CR-OB-008-notification-server.md` (thiết kế Web Push cũ ở tầng Electron/TS — tham chiếu shape API, không phải backend-go)
