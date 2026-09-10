# CR-MOBILE-001 — `notification-service`: triển khai thật kênh push APNs/FCM (đang là dead data, không dead code)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MOBILE-001 |
| **Tên** | Nối `ChannelDeliveryPush`/`ChannelIOS`/`ChannelAndroid` (đã có trong domain model) tới một `deliver_push.go` thật gọi APNs/FCM |
| **Loại** | Feature (hoàn thiện thiết kế đã có, chưa build) |
| **Priority** | 🟡 P1 |
| **Effort** | Large (~1–2 tuần: proto + usecase + adapter APNs/FCM + credential storage) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F03 ở lớp backend-go/agent" |
| **Tác động Features** | F03 (Mobile Companion App) |
| **Phụ thuộc** | Không — độc lập, nên làm **trước** CR-MOBILE-002 (CR-MOBILE-002 cần RPC `Subscribe` hỗ trợ `ios`/`android` do CR này thêm) |

---

## Bối cảnh & Vấn đề

### 0. Đính chính lại khung audit trước (`docs/roadmap/feature-completion-matrix.md`)

Dòng F03 của ma trận (`docs/roadmap/feature-completion-matrix.md:41`) viết: *"FE có
pairing/QR/E2E đầy đủ; BG chỉ có push notif, AG chỉ có filename constants — thiếu
pairing/QR/E2E ở backend-go & agent"*, và gap #3 (dòng 119) đặt câu hỏi: *"pairing có
thực sự cần phía backend-go/agent, hay đây là kết nối P2P trực tiếp"*.

Khảo sát trực tiếp code xác nhận **gap #3 nên đóng theo hướng "không cần backend-go/agent
cho pairing"** — đây không phải lỗi cần sửa:

- `desktop/src/main/runtime/runtime-rpc.ts:548-589` (`createPairingOffer`) tự sinh
  `endpoint` từ địa chỉ IP LAN/Tailscale cục bộ (`resolvePairingEndpoint`, không gọi
  bất kỳ backend-go service nào) và `publicKeyB64` từ E2EE keypair sinh tại chỗ
  (`getE2EEPublicKey()`), nhúng thẳng vào `pairingUrl` (base64 JSON) — không có
  request nào ra khỏi máy desktop trong bước tạo QR.
- `desktop/src/main/ipc/mobile.ts:93-135` (`mobile:getPairingQR`) gọi `createPairingOffer`
  với `scope: 'mobile'` (dòng 115), sinh mã QR bằng thư viện `qrcode` — thuần cục bộ.
- `mobile/src/transport/pairing.ts:8-18` (`decodePairingUrl`) giải mã URL
  `orca://pair?code=...` **trực tiếp trên điện thoại**, không có bước gọi API nào đến
  server; kết quả decode thẳng ra `endpoint`/`deviceToken`/`publicKeyB64` để mobile tự
  mở WebSocket tới đúng địa chỉ đó.
- `desktop/src/main/runtime/device-registry.ts` (`DeviceRegistry`, dòng 24+) lưu
  `scope: DeviceScope` (mặc định `'mobile'`, dòng 37/57/71) **trong bộ nhớ tiến trình
  Electron main + file cục bộ** (`orca-devices.json`, theo
  `agent/src/main/runtime/mobile-pairing-files.ts:1-2` — 2 dòng, chỉ khai tên file
  dùng cho migrate userdata, không có logic pairing thật) — không phải bảng trong
  Postgres của bất kỳ service backend-go nào.
- `agent/` (Dev Server Agent) không tham gia luồng pairing ở bất kỳ bước nào — nó chỉ
  nhận relay `terminal.send` **sau khi** mobile đã pair xong với desktop, khi target là
  1 dev server từ xa (không liên quan tới việc pair chính nó).

→ **Kết luận: pairing/QR/E2E đúng như spec gốc F03 — "peer-to-peer qua local network,
không có server trung gian" (`docs/features/F03-mobile-companion.md:57`) — và đã
triển khai đúng như vậy.** Không có CR nào trong bộ này đề xuất thêm backend-go/agent
vào luồng pairing. Gap thật của F03 nằm ở **Push Notifications**, không phải pairing.

### 1. Gap thật: "BG chỉ có push notif" — sai một nửa quan trọng

`backend-go/services/notification-service` **có tồn tại và chạy được** một pipeline
Web Push (RFC 8030, VAPID) hoàn chỉnh — nhưng nó phục vụ **Web Client (trình duyệt)**,
không phải app Mobile Companion (React Native) mà F03 mô tả:

- `frontend/src/renderer/src/hooks/useWebPushSubscription.ts:36-49` gọi
  `navigator.serviceWorker` + `PushManager.subscribe()` (API chỉ tồn tại trong
  browser/PWA) rồi `POST /api/push-subscribe` → `api-gateway`'s
  `notification_routes.go:66-90` (`handleSubscribe`) → gRPC `Subscribe` của
  `notification-service`.
- Hook này chỉ được dùng ở `frontend/src/renderer/src/components/onboarding/NotificationStep.tsx:24`
  (bước onboarding bật thông báo trình duyệt, có nhánh `isWebClientLocation()`) —
  **không liên quan gì tới `mobile/` (app React Native) hay luồng QR pairing**
  (`scope: 'mobile'`). `webClientUrl` mà `createPairingOffer` sinh ra chỉ dùng khi
  `scope === 'runtime'` (`runtime-rpc.ts:587`), một luồng "Web Connect" khác hoàn
  toàn với Mobile Companion.
- `mobile/src/notifications/mobile-notifications.ts` (app RN thật) **không hề gọi**
  `PushManager`/`serviceWorker`/bất kỳ endpoint `/api/push-*` nào — grep xác nhận 0
  kết quả.

Tức là: backend-go's `notification-service` có push thật, nhưng cho **một client
khác** (trình duyệt truy cập qua Web Client URL), không phải cho app Mobile Companion
mà F03 nhắm tới. Với riêng app Mobile Companion, cơ chế "push" hiện tại là:

- `desktop/src/main/runtime/orca-runtime-connection-subscription-notify.ts:155`
  (`dispatchMobileNotification`) gửi `{ type: 'notification', title, body, ... }`
  **qua đúng kênh WebSocket E2EE đã pair** (gọi từ
  `desktop/src/main/ipc/notifications.ts:460`, khi có notification desktop nào đó
  cần forward cho "paired mobile clients").
  - Nguồn gốc lời gọi: `desktop/src/main/ipc/notifications.ts:456-465`, comment tại
    chỗ: *"paired mobile clients should follow the same user-facing notification
    gates as desktop delivery"* — nghĩa là thiết kế cố ý coi mobile như một "listener
    thêm" của luồng notification desktop, không phải một kênh delivery độc lập.
- `mobile/src/notifications/mobile-notifications.ts:108-131` (`showLocalNotification`)
  nhận event này qua RPC client đang mở, rồi gọi
  `Notifications.scheduleNotificationAsync()` của `expo-notifications` — đây là
  **local notification** (lên lịch ngay trên máy, không phải push từ xa qua
  APNs/FCM).

**Hệ quả: thông báo mobile chỉ tới được khi kết nối WebSocket đã pair giữa mobile ↔
desktop còn sống.** Đây là đúng cơ chế cho lúc app đang mở/foreground hoặc mới vừa
background, nhưng **không đáp ứng được chính tiêu chí chấp nhận của F03**
(`docs/features/F03-mobile-companion.md:98`: *"Notification hoạt động khi app mobile
ở background"*, dòng 47: delivery < 5 giây) khi hệ điều hành đã đình chỉ tiến trình
JS/socket của app (điều xảy ra thường xuyên trên iOS sau khoảng ~30 giây background,
và trên Android sau một khoảng thời gian tùy nhà sản xuất) — lúc đó không có cách nào
"đánh thức" app để hiện thông báo nếu không qua APNs/FCM thật.

Bằng chứng app chưa từng cấu hình push thật: `mobile/app.json`'s `expo.plugins` (kiểm
tra trực tiếp bằng script parse JSON) liệt kê `expo-router`,
`android-respect-rotation-lock`, `expo-splash-screen`, `expo-camera`,
`expo-image-picker`, `expo-build-properties` — **không có `expo-notifications`**
trong danh sách plugin (dù có trong `dependencies`,
`mobile/package.json:42`), và `expo.ios.infoPlist` không có khóa
`UIBackgroundModes` (cần `remote-notification` để nhận APNs khi backgrounded). Không
tìm thấy `googleServicesFile`/cấu hình FCM nào cho Android.

### 2. Domain model backend-go đã "thiết kế sẵn" cho push thật — chỉ chưa nối dây

Đây là phần quan trọng nhất để CR này không phải "viết lại từ đầu": schema, domain
model, và cả tài liệu thiết kế của `notification-service` **đã có sẵn khái niệm
APNs/FCM từ trước**, chỉ là chưa có adapter thật gọi ra ngoài:

- `backend-go/services/notification-service/internal/domain/push_subscription.go:14-20`:
  `type Channel string` với 3 giá trị `ChannelWeb`, `ChannelIOS`, `ChannelAndroid` —
  comment tại chỗ: *"ios/android are device-token channels for APNs/FCM"*.
- `backend-go/services/notification-service/migrations/0001_init.up.sql:12`:
  cột `channel TEXT NOT NULL CHECK (channel IN ('web','ios','android'))` —
  **schema Postgres đã hỗ trợ `ios`/`android` từ migration đầu tiên**, không cần
  migration mới cho CR này.
- `backend-go/services/notification-service/internal/domain/notification_event.go:10-17`:
  `type DeliveryChannel string` với `ChannelDeliveryWS`/`ChannelDeliveryPush`, comment:
  *"mobile push (APNs/FCM), the two distinct delivery mechanisms
  notification-service.md keeps separate"*.
- Cùng file, `subjectRules` (dòng 98-136) gán `Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush}`
  cho đúng các subject mà F03 cần: `orca.task.task.completed` (dòng 98-101),
  `orca.workflow.execution.completed/failed` (102-109), `orca.automation.run.completed`
  (110-113), `orca.orchestration.decision_gate.opened` (121-124, khớp "notification khi
  agent chờ user input"). Dữ liệu `Channels` này **được tính toán nhưng chưa từng được
  đọc** ở bất kỳ đâu khác trong service.
- `specs/backend-go/tdd/services/notification-service.md` mô tả kiến trúc đích tường
  minh: dòng 83-84 (sơ đồ mermaid `pushdeliver -- "VAPID-signed Web Push" --> apns["APNs / FCM"]`),
  dòng 218 (`deliver_push.go # NotificationEvent -> VAPID-signed Web Push -> APNs/FCM`),
  dòng 227 (`external/webpush/ # APNs/FCM clients, Web Push protocol framing`), dòng 250
  (*"`deliver_push.go` calls APNs/FCM directly via the Web Push..."*).

**Nhưng trong code thật:**

- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:88`:
  `Execute()` chỉ gọi **duy nhất** `uc.broadcaster.Broadcast(ctx, event)` — bỏ hoàn
  toàn trường `event.Channels` đã tính ở bước `TranslateEvent`, không có nhánh nào xử
  lý `ChannelDeliveryPush`.
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go`
  (`Broadcaster.Broadcast`, dòng ~78-92) chỉ fan-out tới các channel Go trong bộ nhớ
  process (kênh WS `StreamNotifications`) — không có logic APNs/FCM nào.
- File `deliver_push.go` và thư mục `internal/adapter/external/webpush/` mà tài liệu
  thiết kế mô tả **không tồn tại** (`find backend-go/services/notification-service -name "*.go"`
  xác nhận danh sách 15 file, không có 2 mục này).
- `backend-go/services/notification-service/internal/usecase/subscribe.go:18` (comment)
  + dòng 50: usecase `Subscribe` **hard-code** `domain.ChannelWeb` khi tạo
  `PushSubscription` — không có tham số `channel` nào truyền vào để đăng ký
  `ios`/`android`.
- Proto `backend-go/proto/orca/notification/v1/notification.proto:20-25`
  (`message SubscribeRequest`) chỉ có field Web Push (`endpoint`, `p256dh_key`,
  `auth_key`) — không có field nào cho device token APNs/FCM hay `channel`.
- `internal/usecase/ports.go`'s `NotificationBroadcaster` interface (dòng ~74-92) chỉ
  khai báo `Subscribe`/`Broadcast` cho kênh WS — không có phương thức nào kiểu
  `DeliverPush(ctx, event, subscription)`.

Tóm lại: **schema + domain model + design doc đã "chốt" push APNs/FCM là một phần của
`notification-service` từ đầu, nhưng phần thực thi (usecase mới, adapter gọi
APNs/FCM, mở rộng proto) chưa từng được viết.** Đây là "dead data" (field tồn tại,
không ai đọc) chứ không phải thiếu ý tưởng thiết kế.

## Giải pháp đề xuất

### A. Mở rộng proto + usecase `Subscribe` để nhận đăng ký `ios`/`android`

```protobuf
// backend-go/proto/orca/notification/v1/notification.proto
message SubscribeRequest {
  string user_id = 1;
  string endpoint = 2;      // Web Push endpoint (channel=web) HOẶC device token (ios/android)
  string p256dh_key = 3;    // chỉ dùng khi channel=web
  string auth_key = 4;      // chỉ dùng khi channel=web
  string channel = 5;       // "web" | "ios" | "android" — mặc định "web" nếu rỗng, giữ tương thích ngược
  string device_label = 6;  // optional, hiển thị trong UI "Paired Devices"
}
```

`usecase.Subscribe.Execute` đọc `in.Channel` (mặc định `ChannelWeb` nếu rỗng — giữ
nguyên hành vi cũ cho caller hiện tại là `useWebPushSubscription.ts`), validate theo
đúng invariant đã có sẵn trong `domain.NewPushSubscription` (dòng 84-107 của
`push_subscription.go` — không cần sửa domain, chỉ cần usecase truyền đúng tham số).

### B. `deliver_push.go` — usecase mới, đúng tên/vị trí design doc đã chỉ định

```go
// backend-go/services/notification-service/internal/usecase/deliver_push.go
type DeliverPush struct {
    subscriptions SubscriptionRepository
    sender        PushSender // port mới — implement bởi adapter APNs/FCM
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
    // Chỉ chạy khi event.Channels chứa ChannelDeliveryPush (subjectRules đã tính sẵn).
    // Với mỗi recipient: lấy subscription theo channel ios/android (KHÔNG lấy web —
    // web đã có đường riêng qua VAPID/Broadcaster), gọi sender.Send(...).
    // Lỗi "device token hết hạn" (APNs BadDeviceToken / FCM UNREGISTERED) →
    // đánh dấu subscription domain.SubscriptionExpired, không throw.
}
```

`HandleIncomingEvent.Execute` (`handle_incoming_event.go:88`) gọi thêm
`uc.deliverPush.Execute(ctx, event)` song song với `uc.broadcaster.Broadcast(ctx,
event)`, chỉ khi `slices.Contains(event.Channels, domain.ChannelDeliveryPush)` — lỗi ở
nhánh push không được làm fail toàn bộ event (WS vẫn phải đi qua).

### C. Adapter thật gọi APNs/FCM — `internal/adapter/pushgateway/`

Đúng vị trí `external/webpush/` mà design doc đề xuất (đặt tên `pushgateway` để tránh
nhầm với thư mục `webpush` hiện chưa tồn tại và tránh xung đột với khái niệm "Web
Push" vốn đã dùng cho kênh `web`):

| Adapter | Giao thức | Credential |
|---|---|---|
| `apns_sender.go` | HTTP/2 tới APNs (`api.push.apple.com`), JWT ES256 ký bằng APNs Auth Key (.p8) | Team ID, Key ID, `.p8` key — lưu qua Vault Transit giống cách `VaultSigner` hiện tại xử lý VAPID (`internal/adapter/vaultsigner/signer.go`), **không** lưu key thô trong DB/env, theo đúng nguyên tắc §9 của `notification-service.md` mà `vaultsigner` đã tuân thủ cho VAPID |
| `fcm_sender.go` | HTTP v1 API (`fcm.googleapis.com/v1/projects/{project}/messages:send`), OAuth2 service-account | Service account JSON — cùng nguyên tắc lưu qua Vault, không qua biến môi trường trần |

`PushSender` interface (port, khai trong `ports.go`) trừu tượng hoá 2 sender này sau
1 method `Send(ctx, subscription, event) error`, `DeliverPush.Execute` chọn sender
theo `subscription.Channel`.

### D. Cấu hình / triển khai — tuân thủ SSH & cross-platform

CR này không có yêu cầu riêng gì về SSH hay đa nền tảng (đây là service backend chạy
trên server, không chạy trên máy người dùng) — nhưng cần lưu ý theo AGENTS.md: agent
chạy trên host của người dùng (Linux/macOS/Windows, kể cả qua SSH) hoàn toàn không
tham gia luồng này, nên không có ràng buộc cross-platform bổ sung ngoài việc credential
APNs/FCM là cấu hình **per-tenant** (giống `vapid_key_metadata` đã per-tenant,
`0001_init.up.sql:34`), không phải biến môi trường toàn cục.

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/proto/orca/notification/v1/notification.proto` | `SubscribeRequest` thêm `channel`, `device_label`; regenerate `proto/gen/go/orca/notification/v1/*.pb.go` |
| `backend-go/services/notification-service/internal/usecase/subscribe.go` | Đọc `in.Channel`/`in.DeviceLabel` thay vì hard-code `domain.ChannelWeb`; validate `p256dh_key`/`auth_key` chỉ bắt buộc khi `channel == web` (domain đã validate, chỉ cần truyền đúng) |
| `backend-go/services/notification-service/internal/usecase/deliver_push.go` (mới) | Usecase `DeliverPush.Execute` — lọc subscription theo `ChannelIOS`/`ChannelAndroid`, gọi `PushSender`, xử lý expired-token |
| `backend-go/services/notification-service/internal/usecase/ports.go` | Thêm interface `PushSender` |
| `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` | Gọi thêm `deliverPush.Execute` khi `event.Channels` chứa `ChannelDeliveryPush`, độc lập lỗi với nhánh WS |
| `backend-go/services/notification-service/internal/adapter/pushgateway/apns_sender.go` (mới) | HTTP/2 + JWT ES256 gọi APNs |
| `backend-go/services/notification-service/internal/adapter/pushgateway/fcm_sender.go` (mới) | HTTP v1 + OAuth2 service-account gọi FCM |
| `backend-go/services/notification-service/internal/adapter/grpc/server.go` | `Subscribe` RPC handler truyền `req.Channel`/`req.DeviceLabel` vào `SubscribeInput` |
| `backend-go/services/notification-service/cmd/server/main.go` | Wire `DeliverPush` + `PushSender` (APNs/FCM) vào `HandleIncomingEvent`, đọc credential qua Vault (giống cách `VaultSigner` được wire hiện tại) |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` | `handleSubscribe` truyền thêm `channel`/`device_label` từ request body (mở route mới `/api/mobile-push-subscribe` hoặc mở rộng field body hiện có — quyết định cụ thể thuộc CR-MOBILE-002 vì đó là nơi client mobile gọi vào) |

## Không thuộc phạm vi CR này

- Client mobile (`mobile/`, `desktop/`) đăng ký device token thật, cấu hình
  `expo-notifications` plugin/`UIBackgroundModes`/build FCM & APNs credentials cho
  app — xem **CR-MOBILE-002** (phụ thuộc CR này).
- Xoá `desktop/src/main/mobile/MobileCompanionService.ts` (Web Push RFC 8030 dead
  code trong Electron main) — xem CR-MOBILE-002.
- Thay đổi luồng pairing/QR/E2E — đã xác nhận không cần sửa (mục "Bối cảnh" §0).
- UI "Paired Devices" hiển thị trạng thái push (`MobilePairedDevicesSection.tsx`) —
  cải tiến UI nằm trong CR-MOBILE-002 nếu cần.
- Retry/dead-letter cho push gửi lỗi không phải do token hết hạn (vd. APNs rate-limit)
  — v1 log lỗi, không cần queue retry riêng.

## Tiêu chí chấp nhận

- [ ] `Subscribe` RPC nhận `channel = "ios"` hoặc `"android"` kèm device token, lưu
      đúng `domain.PushSubscription` (không hard-code `ChannelWeb` nữa)
- [ ] Một event với `Channels` chứa `ChannelDeliveryPush` (vd. `orca.task.task.completed`)
      gọi được tới `PushSender.Send` cho mọi subscription `ios`/`android` đang
      `active` của đúng user
- [ ] APNs auth key / FCM service-account không nằm trong biến môi trường trần hay
      cột DB — đi qua Vault, cùng cơ chế `vaultsigner` đã dùng cho VAPID
    private key
- [ ] Token hết hạn (APNs `BadDeviceToken` / FCM `UNREGISTERED`) tự động chuyển
      subscription sang `SubscriptionExpired`, không crash toàn bộ `HandleIncomingEvent`
- [ ] Lỗi ở nhánh push không chặn nhánh WS hiện có (2 nhánh độc lập lỗi)
- [ ] `go build`/`go vet`/`go test` sạch cho `notification-service` và `api-gateway`
      (do đổi proto dùng chung)

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `HandleIncomingEvent` (`notification-service/internal/usecase/handle_incoming_event.go`) | upstream | LOW | 3 (1 direct, module `Usecase`) | Điểm sẽ thêm nhánh gọi `DeliverPush` |
| `Broadcaster` (`notification-service/internal/adapter/broadcaster/broadcaster.go`) | upstream | LOW | 3 (1 direct, module `Usecase`) | Không sửa, chỉ tham chiếu để đối chiếu hành vi WS hiện có |
| `Subscribe` (`notification-service/internal/usecase/subscribe.go`) | upstream | LOW | 3 (1 direct) | Điểm sẽ bỏ hard-code `ChannelWeb` |

`deliver_push.go`, `PushSender`, `apns_sender.go`, `fcm_sender.go` là symbol **chưa
tồn tại** nên không thể chạy `impact()` cho chúng ở bước khảo sát này — phải chạy
`impact({target: "DeliverPush", direction: "upstream"})` (và cho `Subscribe`,
`SubscribeRequest` proto message sau khi regenerate) **ngay trước khi** sửa
`handle_incoming_event.go`/`subscribe.go` thật, theo đúng quy tắc bắt buộc của repo.
`detect_changes({scope: "compare", base_ref: "main"})` bắt buộc trước khi commit.

## Liên quan

- [F03-mobile-companion.md](../../../features/F03-mobile-companion.md)
- [feature-completion-matrix.md](../../../roadmap/feature-completion-matrix.md) (dòng 41, 119 — gap #3 đã đính chính ở mục "Bối cảnh" §0 của CR này)
- [specs/backend-go/tdd/services/notification-service.md](../../../../specs/backend-go/tdd/services/notification-service.md) (§4, §6, §9 — thiết kế gốc của `deliver_push.go`/`external/webpush/`)
- `backend-go/services/notification-service/internal/domain/notification_event.go`, `push_subscription.go`
- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go`, `subscribe.go`
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go`, `vaultsigner/signer.go`
- `backend-go/proto/orca/notification/v1/notification.proto`
- [CR-MOBILE-002](./CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md) (phụ thuộc CR này)
