# Notifications — Change Requests (v4)

> **Bối cảnh:** Yêu cầu "viết CR để thực thi đầy đủ [F11](../../../features/F11-notifications.md) ở lớp
> `backend-go`" — theo [`feature-completion-matrix.md`](../../../roadmap/feature-completion-matrix.md)
> dòng F11: *"BG có stream/subscribe nhưng thiếu 'unread state' tường minh."* Khảo sát trực tiếp
> `backend-go/services/notification-service/` xác nhận đúng và **sâu hơn** nhận định đó: service này
> hiện là một **fan-out ephemeral thuần túy** — không có bảng nào lưu chính nội dung notification, không
> có khái niệm đã đọc/chưa đọc ở bất kỳ tầng nào, và nhánh mobile/Web Push (`Channels: push`) được gán
> trong domain model nhưng chưa từng có code gửi. Đây là **quyết định kiến trúc tường minh** của thiết kế
> gốc (`specs/backend-go/tdd/services/notification-service.md` §2: *"no offline WS replay queue... flag
> to product if guaranteed in-app delivery becomes a requirement"*), không phải lỗi ngẫu nhiên — 2 CR dưới
> đây là quyết định đảo ngược có chủ đích để khớp với F11's acceptance criteria đã publish.

## Tổng quan gap đã xác nhận (bằng chứng chi tiết ở từng CR)

| Gap | Trạng thái thật | CR |
|-----|-----------------|-----|
| Bảng lưu `NotificationEvent` (lịch sử notification) | ❌ Không tồn tại — chỉ có `push_subscriptions`, `vapid_key_metadata`, `processed_events` (dedup, không phải audit log) | CR-NOTIF-001 |
| Trạng thái đã đọc/chưa đọc (`is_read`/`read_at`) | ❌ Không có field, không có RPC (`MarkAsRead`), không có REST endpoint | CR-NOTIF-001 |
| `ListNotifications`/`GetUnreadCount` API | ❌ Không có trong proto lẫn implementation — RPC surface thật chỉ có `Subscribe`, `UnregisterPushSubscription`, `GetVapidPublicKey`, `StreamNotifications` | CR-NOTIF-001 |
| Frontend tiêu thụ notification history/unread từ backend | ❌ Không — mọi unread-state ở `frontend/` (`isUnread` trên worktree, `unreadTerminalTabs`, `useActivityUnreadCount`) là state cục bộ tính từ agent/terminal status, không đọc từ `notification-service`; kênh `notifications.subscribe` tồn tại nhưng không handler nào tiêu thụ `notifications.event` | Ngoài phạm vi (frontend CR riêng) — ghi nhận ở CR-NOTIF-001 |
| `DeliverPush` (Web Push/mobile push thật) | ❌ `NotificationEvent.Channels` gán `push` cho hầu hết subject nhưng 0 call site nào đọc field đó để gửi; `VaultSigner`/`SignVapidPayload` đã wire xong nhưng chưa từng được gọi | CR-NOTIF-002 |
| Per-user notification preference/mute | ❌ 0% — README's "Known gaps" tự ghi "explicitly out of scope" | Không có CR (chưa đủ evidence là yêu cầu thật, xem "Việc chưa làm" bên dưới) |
| WS fan-out cross-replica | ✅ Đã đúng (Epic F, `SubscribeEphemeral`) — không cần CR | — |
| Consumer-side dedup (JetStream redelivery) | ✅ Đã có (`processed_events`, Epic đã đóng) — không cần CR | — |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-NOTIF-001](./CR-NOTIF-001-unread-state-and-persistence.md) | Notification hoàn toàn ephemeral — không bảng, không unread state, không list/mark-as-read API | 🔴 P0 | Large | 🔲 Chưa triển khai |
| [CR-NOTIF-002](./CR-NOTIF-002-deliver-push-usecase.md) | `Channels: push` là dead data — `DeliverPush` chưa từng được xây dù `VaultSigner` đã wire xong | 🟠 P1 | Medium | 🔲 Chưa triển khai |

## Thứ tự thực thi

```
CR-NOTIF-001 (persist + unread state)  ──▶  CR-NOTIF-002 (DeliverPush)
```

CR-NOTIF-002 không phụ thuộc cứng vào CR-NOTIF-001 để chạy được (nhánh Web Push độc lập với nhánh
persist/unread ở mức code), nhưng nên làm sau để dùng chung `notification.notification_events` làm điểm
tham chiếu chống gửi-trùng khi service restart giữa lúc lưu và lúc gửi push (xem CR-NOTIF-002's "Giải
pháp đề xuất" mục D). Cả 2 CR cùng sửa `HandleIncomingEvent.Execute` — khuyến nghị review gộp thay vì 2
PR tách rời chạm cùng 1 hàm.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol sửa | Risk | Impacted count | CR |
|---|---|---|---|
| `HandleIncomingEvent` (`notification-service/internal/usecase/handle_incoming_event.go:34`) | LOW | 3 (1 direct, module `Usecase`) | CR-NOTIF-001, CR-NOTIF-002 |
| `NotificationEvent` (`notification-service/internal/domain/notification_event.go:68`) | — (chỉ chạy `context`, chưa chạy `impact`) | 5 incoming refs (4 test file + `TranslateEvent`) | CR-NOTIF-001 |
| `Broadcaster` (`notification-service/internal/adapter/broadcaster/broadcaster.go:42`) | LOW | 3 (1 direct, module `Usecase`) | CR-NOTIF-001 |
| `Server` (`notification-service/internal/adapter/grpc/server.go:23`) | LOW | 3 (1 direct, module `Usecase`) | CR-NOTIF-001 |
| `TranslateEvent` (`notification-service/internal/domain/notification_event.go:153`) | LOW | 1 (module `Domain`) | CR-NOTIF-001 |
| `Subscribe` (`notification-service/internal/usecase/subscribe.go`) | LOW | 3 (1 direct, module `Usecase`) | CR-NOTIF-002 |
| `VaultSigner` (`notification-service/internal/usecase/ports.go:47`) | — (context only) | 4 import site, 1 typed property, 0 call site tới `SignVapidPayload` | CR-NOTIF-002 |

Toàn bộ symbol đã đo đều **LOW risk**, blast radius nhỏ (`notification-service` là "Phase 1, pilot tier —
low risk, few dependents" theo chính README/spec của service) — không có process nào trong gitnexus's
process index bị ảnh hưởng bởi các thay đổi này ở thời điểm khảo sát. Symbol MỚI mà mỗi CR thêm vào
(`ListNotifications`, `MarkAsRead`, `DeliverPush`, v.v.) chưa tồn tại nên không đo được impact ở bước khảo
sát này — mỗi CR đã ghi rõ cần chạy lại `impact()` ngay trước khi implement, theo đúng quy tắc bắt buộc
của repo. Không CR nào trong bộ này được thực thi (code) trong lần rà soát này — cả 2 file là tài liệu
đặc tả, chưa có thay đổi code nào.

## Việc chưa làm ngoài bộ CR này

- **Frontend Notification Center thật** (panel filter theo agent/worktree/severity, tiêu thụ
  `ListNotifications`/`MarkAsRead`/kênh `notifications.subscribe`) — xác nhận không tồn tại trong
  `frontend/src` (xem CR-NOTIF-001's "Bối cảnh" mục 3). Cần CR frontend riêng, phụ thuộc CR-NOTIF-001 đã
  merge để có API thật để gọi. Component gần nhất hiện có (`ActivityPrototypePage.tsx`) là 1 trang khác
  mục đích (agent activity feed cục bộ theo phiên, không phải notification history persist).
- **Per-user notification preference/mute** — README của `notification-service` tự ghi "explicitly out of
  scope"; không đủ bằng chứng đây là yêu cầu sản phẩm thật (chưa thấy trong F11's acceptance criteria hay
  bất kỳ doc nào khác) nên không mở CR — chỉ ghi nhận là khả năng mở rộng nếu product xác nhận nhu cầu.
- **Retention/pruning cho `notification.notification_events`** (bảng mới của CR-NOTIF-001) và
  **`notification.processed_events`** (đã tồn tại, chưa có pruning job — gap cũ, không thuộc F11) — cả 2
  cần CR vận hành riêng khi có SLA lưu trữ rõ ràng.
- **Mobile push thật (APNs/FCM)** — CR-NOTIF-002 chỉ làm Web Push (`channel = 'web'`); schema đã hỗ trợ
  `ios`/`android` nhưng client thật cho 2 kênh đó ngoài phạm vi 2 CR này.
- **Desktop/native OS notification** (macOS Notification Center, dock badge) — thuộc `desktop/src/main/`
  (Electron), không thuộc `backend-go`, ngoài phạm vi bộ CR này theo đúng yêu cầu gốc.

## Liên quan

- [F11-notifications.md](../../../features/F11-notifications.md)
- [feature-completion-matrix.md](../../../roadmap/feature-completion-matrix.md)
- `specs/backend-go/tdd/services/notification-service.md`
- `backend-go/services/notification-service/README.md`
- `docs/crs/v1/onboarding/CR-OB-008-notification-server.md` (thiết kế Web Push cũ ở tầng Electron/TS — bối cảnh lịch sử, không phải backend-go)
