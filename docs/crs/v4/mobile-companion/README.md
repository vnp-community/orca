# Mobile Companion (F03) — Change Requests (v4)

> **Bối cảnh:** Yêu cầu "thực thi đầy đủ F03 — Mobile Companion ở lớp `backend-go` (và
> `agent` nếu liên quan)", dựa trên audit trước
> ([`docs/roadmap/feature-completion-matrix.md`](../../../roadmap/feature-completion-matrix.md),
> dòng F03): *"FE có pairing/QR/E2E đầy đủ; BG chỉ có push notif, AG chỉ có filename
> constants — thiếu pairing/QR/E2E ở backend-go & agent"*, và gap #3 đặt câu hỏi kiến
> trúc: *"pairing có thực sự cần phía backend-go/agent, hay đây là kết nối P2P trực
> tiếp?"*. Khảo sát trực tiếp code (grep + đọc trực tiếp + GitNexus `context()`/`impact()`)
> trả lời dứt điểm câu hỏi đó và tìm ra gap thật khác với gap ma trận nêu.

## Kết luận khảo sát — đính chính khung câu hỏi ban đầu

| Câu hỏi ban đầu | Trả lời (bằng chứng chi tiết trong CR-MOBILE-001 §0) |
|---|---|
| Pairing/QR/E2E có cần backend-go/agent không? | **Không.** Xác nhận đây đúng là P2P thuần: `desktop/src/main/runtime/runtime-rpc.ts:548` (`createPairingOffer`) tự sinh endpoint LAN + E2EE public key tại chỗ, không gọi backend-go; `mobile/src/transport/pairing.ts` giải mã QR và mở WS thẳng tới desktop. Đúng như spec gốc F03 (`docs/features/F03-mobile-companion.md:57`: "không có server trung gian"). **Không cần CR nào cho phần này.** |
| `agent/` có logic mobile thật không? | **Không, và không cần có.** `agent/src/main/runtime/mobile-pairing-files.ts` chỉ 7 dòng khai tên file cho việc migrate userdata — đúng như ma trận mô tả ("filename constants"). Dev Server Agent không tham gia pairing hay push; vai trò duy nhất liên quan là relay `terminal.send` xuống dev server từ xa **sau khi** mobile đã pair xong với desktop — không phải một gap. |
| "BG chỉ có push notif" — đúng chưa? | **Đúng nhưng gây hiểu lầm quan trọng.** Push thật ở backend-go (`notification-service`, Web Push RFC 8030 + VAPID) phục vụ **Web Client (trình duyệt)** qua `useWebPushSubscription.ts`/`NotificationStep.tsx` — **không phải** app Mobile Companion (React Native, `mobile/`) mà F03 mô tả. App Mobile Companion thật hiện chỉ nhận "local notification qua kênh WebSocket đã pair" (`dispatchMobileNotification`), không phải push APNs/FCM thật — sẽ không hoạt động khi OS đã đình chỉ app (không thoả tiêu chí "hoạt động khi app ở background" của F03). Đây là gap thật. |

## Gap thật đã xác nhận

| Gap | Trạng thái thật | CR |
|-----|-----------------|-----|
| Pairing/QR/E2E P2P (desktop ↔ mobile) | ✅ Đã hoàn chỉnh, đúng thiết kế gốc — không cần sửa | — |
| `agent/` tham gia pairing | ✅ Đúng là không cần — không phải gap | — |
| `notification-service`'s domain/schema hỗ trợ push APNs/FCM (`ChannelIOS`/`ChannelAndroid`, `ChannelDeliveryPush`) | 🟡 Đã thiết kế trong domain/schema/design-doc, **chưa có adapter thật gửi APNs/FCM** (`deliver_push.go` chưa tồn tại) | CR-MOBILE-001 |
| Web Push cho Web Client (trình duyệt) | ✅ Hoạt động thật, không liên quan tới app Mobile Companion — không đụng tới | — |
| App Mobile Companion đăng ký push token (APNs/FCM/Expo) | ❌ Chưa từng lấy hay gửi token nào — `mobile/app.json` thiếu plugin `expo-notifications`, thiếu `UIBackgroundModes` | CR-MOBILE-002 |
| `desktop/src/main/mobile/MobileCompanionService.ts` (Web Push SQLite riêng trong Electron main) | ❌ Dead code — `context()` xác nhận 0 caller, trùng chức năng với `notification-service` | CR-MOBILE-002 |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-MOBILE-001](./CR-MOBILE-001-notification-service-native-push-delivery.md) | `notification-service` thiếu adapter APNs/FCM thật dù domain/schema đã sẵn sàng | 🟡 P1 | Large | 🔲 Chưa triển khai |
| [CR-MOBILE-002](./CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md) | App Mobile Companion chưa đăng ký push token; `MobileCompanionService.ts` là dead code cần dọn | 🟡 P1 | Medium | 🔲 Chưa triển khai |

## Thứ tự thực thi

```
CR-MOBILE-001 (notification-service: adapter APNs/FCM thật)
                    │
                    ▼
CR-MOBILE-002 (mobile app đăng ký token + dọn dead code)
   phụ thuộc cứng — cần RPC Subscribe hỗ trợ channel=ios/android trước
```

Không có CR nào sửa pairing/QR/E2E hay `agent/` — audit xác nhận cả hai đều không có
gap thật (xem bảng ở trên).

## Quyết định kiến trúc còn treo (không tự chốt trong bộ CR này)

CR-MOBILE-002's mục "Quyết định kiến trúc cần chốt trước khi code" nêu 2 phương án
chưa chốt: dùng **Expo Push Service** làm lớp trung gian (rẻ hơn, ít việc tích hợp
hơn) hay **tự tích hợp APNs/FCM trực tiếp** (đúng như CR-MOBILE-001's thiết kế mặc
định). Chọn phương án nào **thay đổi đáng kể phạm vi Changes Required của
CR-MOBILE-001's mục C** — cần chốt với người phụ trách trước khi bắt đầu implement,
không phải quyết định kỹ thuật đơn thuần khi code.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol sửa | Risk | Impacted count | CR |
|---|---|---|---|
| `HandleIncomingEvent` (`notification-service/internal/usecase/handle_incoming_event.go`) | LOW | 3 (1 direct, module `Usecase`) | CR-MOBILE-001 |
| `Broadcaster` (`notification-service/internal/adapter/broadcaster/broadcaster.go`) | LOW | 3 (1 direct, module `Usecase`) | CR-MOBILE-001 (tham chiếu, không sửa) |
| `Subscribe` (`notification-service/internal/usecase/subscribe.go`) | LOW | 3 (1 direct) | CR-MOBILE-001 |
| `MobileCompanionService` (`desktop/src/main/mobile/MobileCompanionService.ts`) | `context()`: 0 caller thật (`incoming: {}`); `impact()`: báo CRITICAL/5164 — **hai kết quả mâu thuẫn nhau, xem CR-MOBILE-002 §2 để biết cách xử lý** | — | CR-MOBILE-002 |

Symbol mới sẽ tạo (`DeliverPush`, `PushSender`, adapter APNs/FCM/Expo, hàm lấy push
token phía `mobile/`) **chưa tồn tại** nên chưa chạy được `impact()`/`context()` ở
bước khảo sát này — mỗi CR đã ghi rõ cần chạy lại ngay trước khi implement. Không CR
nào trong bộ này được thực thi (code) trong lần rà soát này — cả 2 file trên là tài
liệu đặc tả, chưa có thay đổi code nào.

## Việc chưa làm ngoài bộ CR này

- Xác nhận với product owner phương án Expo Push Service vs APNs/FCM trực tiếp
  (CR-MOBILE-002's mục kiến trúc treo) trước khi bắt đầu implement CR-MOBILE-001's
  mục C.
- Điều tra xem `mobile/` (app React Native) có luồng đăng nhập/SSO độc lập với
  desktop hay không — cần thiết để thiết kế endpoint xác thực cho việc đăng ký push
  token (CR-MOBILE-002 mục B) nhưng nằm ngoài phạm vi khảo sát của bộ CR này.
- Audit bảng `orca_push_subscriptions` (SQLite, Electron main) trước khi xoá migration
  liên quan — xem "Không thuộc phạm vi" của CR-MOBILE-002.
- Cập nhật `docs/roadmap/feature-completion-matrix.md`'s dòng F03 và gap #3 để phản
  ánh đúng kết luận khảo sát này (pairing không cần backend-go; gap thật là push
  native, không phải pairing) — chưa sửa file đó trong lần rà soát này vì nằm ngoài
  phạm vi thư mục `docs/crs/v4/mobile-companion/` được giao.
