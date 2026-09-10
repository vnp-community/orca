# CR-MOBILE-002 — Mobile app đăng ký device push token thật + dọn code chết (`MobileCompanionService`)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MOBILE-002 |
| **Tên** | Đăng ký APNs/FCM device token sau khi pairing thành công (`mobile/`) + retire `desktop/src/main/mobile/MobileCompanionService.ts` (dead code) |
| **Loại** | Feature + Cleanup |
| **Priority** | 🟡 P1 |
| **Effort** | Medium (~3–5 ngày: client-side registration flow + xoá dead code + wiring `channel`/`device_label`) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F03 ở lớp backend-go/agent" |
| **Tác động Features** | F03 (Mobile Companion App) |
| **Phụ thuộc** | **Cứng vào [CR-MOBILE-001](./CR-MOBILE-001-notification-service-native-push-delivery.md)** — cần RPC `Subscribe` hỗ trợ `channel = ios/android` trước khi mobile có gì để gọi |

---

## Bối cảnh & Vấn đề

CR-MOBILE-001 đã xác nhận: `backend-go`'s `notification-service` có schema/domain
sẵn sàng cho push APNs/FCM nhưng chưa có adapter gửi thật. Kể cả sau khi CR-MOBILE-001
xong, **không có client nào gọi tới RPC `Subscribe` với `channel=ios/android`** — vì
2 lý do cụ thể tìm thấy trong code:

### 1. App Mobile Companion (`mobile/`) chưa từng lấy/đăng ký push token

- `mobile/src/notifications/mobile-notifications.ts` chỉ có
  `getNotificationPermissionState`/`ensureNotificationPermissions` (dòng 68-95, xin
  quyền hiển thị noti) và `showLocalNotification` (dòng 108-131, lên lịch local
  notification) — grep toàn bộ `mobile/src` cho `getExpoPushTokenAsync`/
  `getDevicePushTokenAsync`/`registerForPushNotifications` cho **0 kết quả**: app
  chưa từng gọi API lấy push token của Expo/APNs/FCM.
- `mobile/app.json`'s `expo.plugins` (parse trực tiếp JSON) không có mục
  `expo-notifications` — theo tài liệu Expo, thiếu bước này thì
  `getExpoPushTokenAsync()` không hoạt động đúng trên build production (thiếu
  entitlement `aps-environment` trên iOS, thiếu channel cấu hình trên Android).
  `expo.ios.infoPlist` không có `UIBackgroundModes: ["remote-notification"]`.
- Không có RPC method nào trong `mobile/src/transport/rpc-client.ts` hay HTTP call
  nào gửi token lên backend-go sau khi pairing xong — token, nếu có lấy được, cũng
  không có nơi để gửi tới hôm nay.

### 2. `desktop/src/main/mobile/MobileCompanionService.ts` — code chết, không nên giữ

File này (186 dòng, comment tự nhận `TASK-MB-001`, xem
`specs/backend/bugs/mobile-companion/tasks/TASK-MB-001-implement-web-push.md`) implement
lại **một bản Web Push RFC 8030 riêng, dùng SQLite** (`INSERT OR REPLACE INTO
orca_push_subscriptions`, dòng 84-95 — cú pháp `INSERT OR REPLACE` chỉ có ở SQLite,
xác nhận đây là DB cục bộ của Electron main, không phải Postgres của backend-go) —
**hoàn toàn độc lập và trùng lặp chức năng** với
`backend-go/services/notification-service`.

Xác minh bằng 2 cách độc lập rằng class này **chưa từng được khởi tạo**:

- `grep -rn "new MobileCompanionService" desktop/src frontend/src backend-go` → 0 kết quả.
- `gitnexus context({name: "MobileCompanionService"})` trả về `"incoming": {}` —
  **0 cạnh gọi đến** từ bất kỳ symbol nào khác trong toàn bộ codebase đã index.

(Lưu ý: `gitnexus impact({target: "MobileCompanionService", direction: "upstream"})`
báo `impactedCount: 5164, risk: CRITICAL` — con số này **mâu thuẫn** với `context()`'s
`incoming: {}` và với việc grep xác nhận 0 lời gọi thật. Đây nhiều khả năng là một
false-positive của thuật toán `impact()` (có thể do duyệt qua cạnh loại khác ngoài
CALLS thật, hoặc một hiệu ứng module-level). Trước khi xoá file, **phải chạy lại
`impact()` bằng `target_uid` chính xác** (`Class:desktop/src/main/mobile/MobileCompanionService.ts:MobileCompanionService`)
kèm `mode` khác để đối chiếu, không tin số liệu 5164 này khi lập kế hoạch xoá —
nhưng bằng chứng `context()` + grep độc lập cho thấy về mặt runtime, class này
không được gọi.)

`webpush` package (npm) vẫn còn trong `desktop/package.json` chỉ để phục vụ file
chết này.

## Giải pháp đề xuất

### A. `mobile/`: lấy push token sau khi pairing, gửi lên backend-go qua `api-gateway`

```
1. Sau khi mobile pair thành công (WS connect + E2EE handshake OK, cùng thời điểm
   mobile/src/transport/pairing.ts's decode xong và app đã có deviceToken hợp lệ):
   - Cấu hình expo.plugins thêm ["expo-notifications", { ... }] trong mobile/app.json
   - Thêm expo.ios.infoPlist.UIBackgroundModes: ["remote-notification"]
   - Gọi Notifications.getExpoPushTokenAsync() (Expo Push Service — không cần tự
     quản lý APNs/FCM credential phía app; Expo Push Service của Expo sẽ relay tới
     APNs/FCM thật) HOẶC getDevicePushTokenAsync() nếu quyết định gọi thẳng APNs/FCM
     từ backend-go (xem "Quyết định kiến trúc" bên dưới) — 1 trong 2, không làm cả 2.
2. Gửi token lên backend-go: mobile không có session cookie như web client, nên
   endpoint đăng ký token phải xác thực bằng deviceToken đã pair (JWT ngắn hạn cấp
   khi pairing, hoặc endpoint riêng nhận kèm deviceToken E2EE) — CẦN quyết định
   thiết kế cụ thể khi implement (xem mục "Quyết định kiến trúc cần chốt trước khi
   code" bên dưới, đây không phải chi tiết nhỏ).
3. api-gateway forward vào notification-service's Subscribe RPC (CR-MOBILE-001) với
   channel = 'ios' | 'android', device_label = tên thiết bị hiển thị trong
   MobilePairedDevicesSection.tsx.
```

### Quyết định kiến trúc cần chốt trước khi code (không tự quyết trong CR này)

F03 gốc nhấn mạnh "không có server trung gian — kết nối peer-to-peer"
(`docs/features/F03-mobile-companion.md:57`) cho pairing, nhưng **push thật về bản
chất bắt buộc phải có một server trung gian** (APNs/FCM luôn là bên thứ ba giữa
server-gửi và thiết bị — không có cách nào push tới app đã bị OS kill mà không qua
Apple/Google). Điều này đúng ngay cả với thiết kế đã có sẵn của
`notification-service` cho Web Push (VAPID cũng đi qua browser's push service, không
P2P). Vậy CR này cần review/quyết định (không tự ý chọn khi viết CR):

1. **Dùng Expo Push Service làm lớp trung gian** (mobile gọi
   `getExpoPushTokenAsync()`, backend-go gửi tới `exp.host/--/api/v2/push/send` thay
   vì tự ký APNs/FCM) — đơn giản hơn nhiều (không cần tự quản lý APNs `.p8` key/FCM
   service account), nhưng thêm phụ thuộc vào hạ tầng Expo và giới hạn theo chính
   sách rate-limit của Expo. Nếu chọn hướng này, **CR-MOBILE-001's mục C (APNs/FCM
   adapter) cần đổi thành 1 adapter `expo_push_sender.go`** — ít việc hơn nhiều so
   với tự tích hợp APNs/FCM.
2. **Tự tích hợp APNs/FCM trực tiếp** (đúng như CR-MOBILE-001 mô tả) — không phụ
   thuộc Expo runtime, nhưng cần tự quản lý credential + tuân thủ 2 giao thức khác
   nhau.

CR này giả định **phương án 1 (Expo Push Service)** là điểm khởi đầu rẻ hơn vì
`mobile/` đã là app Expo (`mobile/package.json`), nhưng **đội triển khai phải xác
nhận lại với product owner trước khi khoá vào CR-MOBILE-001's thiết kế APNs/FCM
trực tiếp** — đổi hướng ở đây làm thay đổi đáng kể "Changes Required" của
CR-MOBILE-001's mục C.

### B. Endpoint xác thực cho mobile đăng ký token (không dùng cookie session)

`api-gateway/internal/adapter/httpgateway/notification_routes.go:66-90`
(`handleSubscribe`) hiện xác thực bằng `CookieSessionValidator` — mobile app không có
cookie trình duyệt. Cần 1 trong 2:

- Route riêng `POST /api/mobile/push-subscribe`, xác thực bằng chính deviceToken đã
  cấp lúc pairing (tra `DeviceRegistry` — nhưng `DeviceRegistry` sống trong tiến
  trình desktop, không phải api-gateway/notification-service, nên **cần một bước
  liên lạc userId thật** — desktop biết `userId` nào đang chạy Electron, forward
  request kèm `userId` khi relay yêu cầu subscribe hộ mobile qua kênh RPC đã pair,
  thay vì mobile tự gọi thẳng `api-gateway`).
- Hoặc: mobile tự có JWT của riêng user (nếu người dùng đã đăng nhập SSO trong app
  Mobile Companion) — cần xác nhận `mobile/` có luồng auth độc lập với desktop hay
  không (ngoài phạm vi khảo sát của CR này, cần điều tra thêm khi implement).

### C. Xoá `MobileCompanionService.ts` và phụ thuộc liên quan

- Xoá `desktop/src/main/mobile/MobileCompanionService.ts`.
- Xoá migration `desktop/src/main/db/migrations/0012_port_forwards_push.ts` **chỉ
  nếu** xác nhận bảng `orca_push_subscriptions` không được đọc bởi bất kỳ nơi nào
  khác (migration này cần audit riêng khi implement — không xoá migration đã chạy
  trên máy người dùng thật mà không có kế hoạch dọn bảng, chỉ ngừng dùng bảng đó).
- Xoá dependency `web-push` khỏi `desktop/package.json` nếu không còn nơi nào import
  (chỉ `MobileCompanionService.ts` import `webpush` theo grep hiện tại).
- **Không đụng tới** `useWebPushSubscription.ts`/`notification_routes.go` (Web Client
  push cho trình duyệt) — đây là tính năng khác, đang hoạt động, ngoài phạm vi CR
  này.

## Changes Required

| File | Thay đổi |
|------|---------|
| `mobile/app.json` | Thêm plugin `expo-notifications` vào `expo.plugins`; thêm `expo.ios.infoPlist.UIBackgroundModes: ["remote-notification"]` |
| `mobile/src/notifications/mobile-notifications.ts` | Thêm hàm lấy push token (`getExpoPushTokenAsync` hoặc `getDevicePushTokenAsync` theo quyết định kiến trúc ở trên) sau khi pairing thành công |
| `mobile/src/transport/pairing.ts` hoặc file gọi sau khi pair xong | Trigger gửi token đăng ký ngay sau khi WS pair thành công lần đầu |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` | Thêm route/handler xác thực theo deviceToken (không dùng `CookieSessionValidator`) cho mobile subscribe |
| `desktop/src/main/mobile/MobileCompanionService.ts` | **Xoá file** |
| `desktop/src/main/db/migrations/0012_port_forwards_push.ts` | Audit rồi xoá nếu bảng `orca_push_subscriptions` không còn được đọc ở đâu khác |
| `desktop/package.json` | Xoá dependency `web-push` nếu không còn import nào |
| `frontend/src/renderer/src/components/settings/MobilePairedDevicesSection.tsx` | (nếu cần) hiển thị trạng thái "push token registered" cho từng thiết bị đã pair |

## Không thuộc phạm vi CR này

- Adapter APNs/FCM/Expo Push thật ở backend-go — xem CR-MOBILE-001 (CR này chỉ lo
  phía client + endpoint xác thực).
- Sửa luồng pairing/QR/E2E — không đổi.
- Auth độc lập cho `mobile/` (SSO trong app) nếu chưa có — nếu khảo sát khi
  implement cho thấy `mobile/` hoàn toàn không có khái niệm "user đăng nhập" độc
  lập với desktop, đây trở thành một CR tiền đề riêng, không giải quyết trong
  CR-MOBILE-002.

## Tiêu chí chấp nhận

- [ ] App Mobile Companion lấy được push token thật (Expo Push token hoặc device
      token APNs/FCM tuỳ quyết định kiến trúc) ngay sau khi pairing lần đầu thành
      công
- [ ] Token được gửi thành công tới `notification-service`'s `Subscribe` RPC với
      `channel` đúng (`ios`/`android`), không dùng cookie session
- [ ] Agent hoàn thành task khi app Mobile Companion **đã bị OS kill hoàn toàn**
      (không chỉ backgrounded) vẫn nhận được thông báo — test thủ công trên thiết bị
      thật iOS + Android
- [ ] `desktop/src/main/mobile/MobileCompanionService.ts` bị xoá, không còn import
      `web-push` nào orphan trong `desktop/`
- [ ] Không có regression cho Web Client push (`useWebPushSubscription.ts`) — vẫn
      hoạt động y hệt trước CR này

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `MobileCompanionService` (`desktop/src/main/mobile/MobileCompanionService.ts`) | upstream | context() cho `incoming: {}` (0 caller thật); `impact()` báo CRITICAL/5164 — **mâu thuẫn, xem giải thích ở "Bối cảnh" §2** | — | Chạy lại `impact()` bằng `target_uid` cụ thể + đối chiếu thủ công (grep `new MobileCompanionService`) trước khi xoá, không tin một trong hai con số một cách mù quáng |

Các symbol phía client mới (hàm lấy push token trong `mobile/`, handler route mới ở
`api-gateway`) **chưa tồn tại** nên chưa chạy được `impact()`/`context()` — phải chạy
ngay trước khi implement, đúng quy tắc bắt buộc của repo. `detect_changes({scope:
"compare", base_ref: "main"})` bắt buộc trước khi commit, đặc biệt vì CR này xoá file
(`MobileCompanionService.ts`) — cần xác nhận `detect_changes` không báo bất kỳ
execution flow nào bị ảnh hưởng bởi việc xoá đó.

## Liên quan

- [CR-MOBILE-001](./CR-MOBILE-001-notification-service-native-push-delivery.md) (phụ thuộc cứng)
- [F03-mobile-companion.md](../../../features/F03-mobile-companion.md)
- `specs/backend/bugs/mobile-companion/tasks/TASK-MB-001-implement-web-push.md` (nguồn gốc `MobileCompanionService.ts`, xác nhận đây là nỗ lực cũ, riêng biệt với `notification-service`)
- `mobile/src/notifications/mobile-notifications.ts`, `mobile/app.json`
- `desktop/src/main/runtime/orca-runtime-connection-subscription-notify.ts:155` (`dispatchMobileNotification` — cơ chế local-notification-qua-WS hiện tại, vẫn giữ làm fallback khi app đang mở foreground)
- `frontend/src/renderer/src/hooks/useWebPushSubscription.ts` (Web Client push — không đụng tới, chỉ để đối chiếu không nhầm 2 luồng)
