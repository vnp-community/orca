# BE-MOBILE-SOL-002: `api-gateway` — endpoint xác thực cho mobile đăng ký push token

> **🔲 Designed — chưa implement.** Phụ thuộc cứng BE-MOBILE-SOL-001 (cần
> `SubscribeRequest.channel`/`device_label` tồn tại trước). Quyết định
> kiến trúc mục 1.2/2 **đã chốt (2026-09-09) — xem mục 1.4**: chọn
> Phương án B (mobile có SSO riêng). Phương án A (desktop relay) bị loại
> hẳn, không phải "vẫn mở". Implementation vẫn chờ 1 CR hạ tầng mới (SSO
> cho `mobile/`) chưa được viết — chưa đủ điều kiện để chuyển
> TASK-BE-MOBILE-010 sang TODO.

**CR:** [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md)
**Service:** `api-gateway`
**TDD tham chiếu:** [`notification-service.md`](../../../../tdd/services/notification-service.md) §7 (`api-gateway` là client `StreamNotifications`)

---

## 1. Trạng thái hiện tại — quan trọng, thay đổi thiết kế so với CR gốc

### 1.1. Phạm vi backend-go thật của CR-MOBILE-002 mỏng hơn nhiều so với toàn bộ CR

CR-MOBILE-002's "Changes Required" liệt kê 7 file; audit trực tiếp xác nhận
**chỉ 1 trong 7 file đó là backend-go**:
`backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go`.
6 file còn lại (`mobile/app.json`, `mobile/src/notifications/mobile-notifications.ts`,
`mobile/src/transport/pairing.ts`, `desktop/src/main/mobile/MobileCompanionService.ts`,
`desktop/src/main/db/migrations/0012_port_forwards_push.ts`,
`desktop/package.json`, `frontend/.../MobilePairedDevicesSection.tsx`) là
React Native (`mobile/`), Electron main (`desktop/`), hoặc renderer
(`frontend/`) — **ngoài phạm vi `specs/backend-go/`**, không có task nào cho
chúng trong thư mục này (xem README "Phạm vi").

Xác nhận `notification_routes.go` hiện tại (đọc trực tiếp file, không phải
suy đoán từ CR): `mountPushRoutes`/`handleSubscribe`/`resolveSoftIdentity`
xác thực bằng `CookieSessionValidator` (cookie `orca_session`,
`api-gateway/internal/adapter/authclient/session_validator.go:23`) — đúng
như CR mô tả, mobile app không có cookie trình duyệt nên không gọi được
route này hôm nay.

### 1.2. Quyết định kiến trúc CHƯA CHỐT — CR gốc để ngỏ, solution này KHÔNG tự chốt

CR-MOBILE-002 nêu 2 phương án cho §B ("Endpoint xác thực") nhưng minh bạch
ghi "cần quyết định thiết kế cụ thể khi implement". Audit thêm khi viết
solution này xác nhận **cả 2 tiền đề của CR vẫn đúng, chưa có gì mới để tự
chốt thay**:

- `grep -rn "orca_session" backend-go desktop/src` xác nhận `orca_session`
  là cookie của `api-gateway` (`auth-service`/`api-gateway`'s
  `auth_routes.go`) — **desktop không có code nào tự đặt/đọc cookie này**
  (`grep` trong `desktop/src/main` cho 0 kết quả ngoài các file tự định
  nghĩa khái niệm `orca_session` **khác**, cho phiên local server riêng của
  Electron main — 2 khái niệm trùng tên, khác hệ thống). Tức là **desktop
  không có sẵn 1 session đã xác thực với `api-gateway`** để "relay hộ"
  mobile như phương án 1 của CR §B gợi ý — phương án đó cần thêm hạ tầng
  (desktop tự đăng nhập `api-gateway`) không tồn tại hôm nay, tốn hơn CR
  ước lượng.
- `grep -rln "login\|SSO\|auth" mobile/src` không tìm thấy luồng đăng nhập
  độc lập nào trong app Mobile Companion — chỉ có `AuthFailedBanner.tsx`
  (báo lỗi xác thực **pairing**, không phải SSO người dùng) — xác nhận
  đúng nhận định của CR: `mobile/` **không có** khái niệm "user đã đăng
  nhập" độc lập với desktop.

→ **Cả 2 phương án CR nêu đều cần thêm hạ tầng chưa tồn tại.** Solution này
không tự chọn — mục 2 dưới đây đặc tả CẢ HAI ở mức đủ chi tiết để chốt
nhanh, và đánh dấu rõ trong README/task nào đang BLOCKED chờ quyết định đó,
theo đúng yêu cầu "không để 1 task cần nhiều quyết định mở mà không ghi rõ".

## 1.3. ⚠️ Phát hiện mới (2026-09-09, verify sâu hơn) — tiền đề của Phương án A KHÔNG đúng như solution này giả định ban đầu

Khảo sát trực tiếp (không suy đoán) xác nhận: **`desktop/` (Electron main) hiện KHÔNG
gọi bất kỳ RPC nào tới `backend-go` cả** — không `wscompat`, không REST `/v1/*`, 0 kết nối.
Cụ thể:

- Không có URL/host nào trong `desktop/src/main` trỏ tới `api-gateway` (không
  `ORCA_API_URL`/`GATEWAY_URL`), không WS client nào mở `ws://.../ws` tới backend-go.
- Consumer thật của `wscompat`'s `clientState.*`/`workspaceSession.*` (và mọi route
  REST `authed` khác) là **web frontend chạy trong TRÌNH DUYỆT**
  (`frontend/src/platform/adapters/web/rpc-client.ts`'s `WebSocketRpcClient`, dựa vào
  cookie `orca_session` trình duyệt tự gửi) — **không phải** Electron main process.
  `desktop/src/platform/adapters/web/rpc-client.ts` tồn tại byte-identical nhưng
  **không có nơi nào trong `desktop/src` import nó**.
- Cái duy nhất trong `desktop/` gọi ra ngoài bằng `Authorization: Bearer` là
  `orca-profiles/profile-cloud-*.ts` — nhưng đó là PKCE OAuth tới **1 sản phẩm SaaS
  "Orca Cloud" hoàn toàn khác** (`ORCA_CLOUD_API_URL`, đồng bộ "local app profile"
  kiểu Chrome profile), xác nhận qua `specs/backend/api/orca-profiles-server-mode-design.md:23`
  — không liên quan `backend-go`, không có route `/v1/desktop/auth/*` nào tồn tại ở đó.

**Hệ quả:** "desktop relay qua kênh RPC đã pair" (Phương án A) không phải "thêm 1 route
nhỏ dùng lại kết nối desktop→backend-go đã có" như solution này giả định ban đầu —
**kết nối đó chưa hề tồn tại**. Xây Phương án A đầy đủ nghĩa là desktop phải có: (1)
1 cách để trở thành "identity đã biết" với `auth-service` (hiện KHÔNG có — desktop
không đăng nhập vào backend-go bằng bất kỳ cơ chế nào), RỒI MỚI (2) mint/dùng được 1
bearer JWT (`IssueServiceToken`, đang được F09/CR-CLI-002 xây) để gọi route mobile-push
mới. Việc (1) là 1 khoảng trống kiến trúc LỚN hơn hẳn phạm vi CR-MOBILE-002 — không có
CR nào trong repo hiện mô tả "desktop tự đăng nhập vào backend-go bằng danh nghĩa gì".

**KHÔNG tự phát minh cơ chế (1) trong solution này** — đây là quyết định kiến trúc vượt
phạm vi mobile-companion, ảnh hưởng tới bất kỳ tính năng nào khác cần desktop nói
chuyện với backend-go trong tương lai (không chỉ mobile push). Task tương ứng
(TASK-BE-MOBILE-010) **vẫn giữ `BLOCKED`** — không phải vì thiếu quyết định nhỏ như
trước, mà vì phụ thuộc 1 gap kiến trúc lớn hơn cần xử lý ở cấp CR riêng trước.

## 2. Giải pháp — 2 phương án, cần chốt 1 trước khi thực thi task tương ứng (Phương án A giờ có thêm 1 tiền đề chưa giải quyết — xem 1.3)

### Phương án A — desktop relay qua kênh RPC đã pair (khuyến nghị ban đầu — nay cần thêm 1 CR tiền đề, xem 1.3)

Vì mobile ↔ desktop đã có kênh RPC E2EE đã pair, đơn giản nhất về mặt bảo
mật là: **thêm 1 RPC method mới trong `desktop/src/main/runtime/rpc/methods/`**
(ví dụ `notifications.registerPushToken`, cùng thư mục
`notifications.subscribe`/`notifications.unsubscribe` đã có) nhận
`{ token, channel, deviceLabel }` từ mobile, rồi desktop tự làm 1 trong 2
việc (cả hai đều **ngoài phạm vi backend-go**, không thuộc task nào ở đây):

- Nếu desktop cuối cùng có 1 session `api-gateway` riêng (cần CR tiền đề
  khác, không có hôm nay) → gọi thẳng `/v1/notifications/subscribe` bằng
  cookie đó.
- Hoặc (rẻ hơn, không cần CR tiền đề): desktop gọi 1 route backend-go
  **mới, xác thực bằng 1 shared secret cấp lúc provisioning** thay vì
  cookie session — route này **là phần backend-go duy nhất** phương án A
  cần.

Route mới (backend-go, `api-gateway`):

```go
// notification_routes.go — route riêng, KHÔNG nằm trong authMiddleware
// (mobile/desktop không có cookie), xác thực bằng service token thay vì
// CookieSessionValidator.
func mountMobilePushRoutes(r chi.Router, client notificationv1.NotificationServiceClient, deviceTokenValidator DeviceTokenValidator) {
	r.Post("/api/mobile/push-subscribe", handleMobilePushSubscribe(client, deviceTokenValidator))
}

type mobilePushSubscribeRequestBody struct {
	UserID      string `json:"user_id"`      // desktop biết userId đang chạy Electron — truyền tường minh, KHÔNG suy ra từ token
	Endpoint    string `json:"endpoint"`     // Expo push token hoặc APNs/FCM device token
	Channel     string `json:"channel"`      // "ios" | "android" — bắt buộc, route này không phục vụ "web"
	DeviceLabel string `json:"device_label"`
}
```

**Bảo mật — điểm mở cần chốt cùng lúc**: `DeviceTokenValidator` xác thực
CÁI GÌ? Route này KHÔNG có cookie, nên không thể dùng
`tenant.RequireTenantID(ctx)` qua middleware hiện có — cần 1 cơ chế khác
xác nhận request THẬT SỰ đến từ 1 desktop instance đã được tenant nào đó
cấp quyền (không phải giả mạo `user_id` tuỳ ý trong body — nếu không có
bước xác thực nào, đây là 1 lỗ hổng cho phép bất kỳ ai đăng ký push token
hộ user khác). Cụ thể cơ chế nào (service token cấp lúc provisioning
desktop, mTLS, hay HMAC theo 1 secret chia sẻ) **là quyết định cần chốt
trước khi viết TASK cho route này** — task tương ứng (xem tasks/README.md)
đánh dấu `🔲 BLOCKED — cần chốt cơ chế xác thực` thay vì `🔲 TODO`.

### Phương án B — mobile có SSO riêng, gọi thẳng `api-gateway`

Nếu quyết định đầu tư cho `mobile/` một luồng đăng nhập độc lập (JWT/OAuth
riêng người dùng, không qua desktop), route backend-go cần chỉ là: thêm 1
`AuthMiddleware` biến thể chấp nhận `Authorization: Bearer <jwt>` bên cạnh
cookie hiện có (không phải cookie mới, xác thực khác cơ chế), rồi
`/v1/notifications/subscribe` hiện có (đã đủ field sau BE-MOBILE-SOL-001)
dùng được luôn — **không cần route `/api/mobile/push-subscribe` riêng**.
Phương án này rẻ hơn về code backend-go, nhưng tốn hơn nhiều ở việc xây SSO
cho `mobile/` — nằm ngoài phạm vi backend-go hoàn toàn (thuộc `auth-service`
+ `mobile/`, một CR riêng theo đúng ghi chú "ngoài phạm vi" của CR-MOBILE-002
gốc).

**Cập nhật (2026-09-09, sau phát hiện ở mục 1.3): ưu thế chi phí của Phương án A không
còn chắc chắn như đánh giá ban đầu.** Khi viết solution này lần đầu, Phương án A được
coi là "rẻ hơn, không cần CR tiền đề" — điều đó chỉ đúng NẾU desktop đã có sẵn 1 kênh
xác thực tới backend-go để "relay". Mục 1.3 xác nhận kênh đó **không tồn tại** — nghĩa
là Phương án A giờ cũng cần 1 CR tiền đề mới (desktop→backend-go identity), y hệt
Phương án B cần 1 CR tiền đề (mobile SSO). Hai phương án giờ **gần ngang nhau về việc
"cần xây thêm hạ tầng xác thực mới"** — khác biệt còn lại là: A gắn thêm 1 hạ tầng dùng
được cho MỌI tính năng tương lai cần desktop nói chuyện với backend-go (lợi ích rộng
hơn), còn B chỉ giải quyết đúng mobile push (hẹp nhưng đơn giản hơn để reasoning). Đây
là lựa chọn kiến trúc thật sự cần Product Owner/Architecture Owner cân nhắc — **solution
này không tự chọn lại**, giữ nguyên task ở trạng thái BLOCKED, đã cập nhật lý do BLOCKED
cho đúng bản chất mới (xem mục 1.3), không phải chỉ "chưa chọn cơ chế `DeviceTokenValidator`
cụ thể" như bản nháp trước.

## 1.4. ✅ Quyết định đã chốt (2026-09-09) — Phương án B, desktop KHÔNG đổi

Người giao việc đã chọn trực tiếp: **Phương án B (mobile có SSO riêng)**,
không phải Phương án A. Xác nhận thêm 2 điểm quan trọng đi kèm quyết định:

- **Phạm vi bổ sung chỉ chạm `backend-go` và `frontend`** — không phải
  `desktop/`. Điều này loại bỏ hẳn nhánh Phương án A (desktop relay,
  §1.3's gap kiến trúc "desktop chưa có identity với backend-go") khỏi
  bàn — không cần giải quyết gap đó nữa, vì không đi theo hướng đó.
- **`desktop/`'s cơ chế hiện tại giữ nguyên, không đổi** — không có RPC
  method mới (`notifications.registerPushToken`) ở
  `desktop/src/main/runtime/rpc/methods/` như Phương án A từng đề xuất ở
  mục 2 bên dưới. Toàn bộ nội dung "Phương án A" từ đây trở xuống (mục 2's
  phần đầu) được giữ lại **chỉ để tham khảo lịch sử** — không còn là
  hướng triển khai.

### Phạm vi backend-go thật của Phương án B (thu hẹp lại từ mục 2's mô tả gốc)

Route backend-go cần chỉ là: thêm 1 `AuthMiddleware` biến thể chấp nhận
`Authorization: Bearer <jwt>` bên cạnh cookie hiện có (`api-gateway`), rồi
`/v1/notifications/subscribe` hiện có (đã đủ field sau BE-MOBILE-SOL-001)
dùng được luôn — không cần route `/api/mobile/push-subscribe` riêng như
Phương án A đề xuất.

**Điều kiện tiên quyết còn thiếu**: JWT đó phải do 1 luồng SSO mobile thật
phát hành — luồng đó **chưa tồn tại**, cần 1 CR hạ tầng mới (chưa được
viết trong bộ `mobile-companion` này). Theo xác nhận của người giao việc,
CR đó sẽ chạm:

- `backend-go` — nhiều khả năng `auth-service` (phát hành/validate JWT cho
  identity mobile, khác hẳn `IssueServiceToken`'s luồng CLI/service-token
  hiện có ở F09) + `api-gateway`'s `AuthMiddleware` biến thể nói trên.
- `frontend` — phạm vi cụ thể chưa xác định trong quyết định này (có thể
  là UI cấu hình/quản lý SSO, hoặc phần callback/redirect nếu dùng OAuth —
  **chưa được đặc tả**, không tự suy đoán thêm ở đây).

**Không tự viết CR hạ tầng mới này ở đây** — đây là 1 thiết kế SSO đầy đủ
(luồng đăng nhập, token issuance, refresh, revocation…), quy mô lớn hơn 1
solution nhỏ trong bộ `mobile-companion`, cần phân tích riêng trước khi
có task cụ thể. `TASK-BE-MOBILE-010` **vẫn giữ BLOCKED** cho tới khi CR đó
tồn tại và có ít nhất đủ chi tiết để viết lại route thật (không phải vì
thiếu quyết định A/B nữa — quyết định đó đã xong — mà vì CR hạ tầng tiền
đề chưa được viết).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Cơ chế xác thực route mới chưa chốt (§1.2/§2 Phương án A) | Cao | KHÔNG code route này trước khi chốt — nguy cơ route cho phép mạo danh `user_id` |
| Phụ thuộc cứng BE-MOBILE-SOL-001 | Cao | Cần `SubscribeRequest.channel`/`device_label` tồn tại |
| Desktop's RPC method mới (`notifications.registerPushToken`) | Ngoài phạm vi | Việc của CR-MOBILE-002 phía `desktop/`, không phải backend-go — chỉ ghi nhận ở đây để task backend-go biết caller tương lai là ai |

## Không thuộc phạm vi solution này

- Toàn bộ phía `mobile/` (lấy Expo/APNs/FCM token, gọi RPC mới) — React
  Native, không phải backend-go.
- RPC method mới `notifications.registerPushToken` ở `desktop/src/main/runtime/rpc/methods/` — Electron main, không phải backend-go.
- Xoá `desktop/src/main/mobile/MobileCompanionService.ts` + migration
  `0012_port_forwards_push.ts` + dependency `web-push` — thuần
  Electron/desktop, xác nhận lại 0 caller
  (`grep -rn "new MobileCompanionService" desktop/src frontend/src backend-go`
  → 0 kết quả, khớp CR) nhưng việc xoá thuộc `desktop/`, không có task nào
  ở đây.
- UI `MobilePairedDevicesSection.tsx` — `frontend/`, không phải backend-go.
- ~~Phương án B (SSO riêng cho `mobile/`) nếu được chọn thay Phương án A —
  cần viết lại solution này.~~ **Đã chọn (2026-09-09, xem mục 1.4)** —
  không cần viết lại solution này (mục 1.4 đã bổ sung đủ để narrow scope),
  nhưng bản thân luồng SSO cho `mobile/` (auth-service + frontend) vẫn
  ngoài phạm vi *solution này* — cần 1 CR/solution riêng.

## Liên quan

- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go`
- `backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go` (cookie hiện có, đối chiếu)
- [BE-MOBILE-SOL-001](./BE-MOBILE-SOL-001-notification-service-push-delivery.md) (phụ thuộc cứng)
- `desktop/src/main/runtime/rpc/methods/notifications.ts` (caller tương lai phía desktop, ngoài phạm vi)
