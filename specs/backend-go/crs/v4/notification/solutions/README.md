# backend-go Solutions — Notifications (v4)

**CRs:** [docs/crs/v4/notification/](../../../../../../docs/crs/v4/notification/README.md)
**TDD tham chiếu:** [`notification-service.md`](../../../../tdd/services/notification-service.md) §2 (bounded context), §5 (schema), §6 (package layout), §7 (dependencies), §9 (security)

## Re-verify trước khi thiết kế (bắt buộc — theo yêu cầu, xác nhận bằng Read/grep/`codegraph_explore` trực tiếp trên `backend-go/services/notification-service`, không suy đoán từ CR)

| Khẳng định của CR nguồn | Kết quả re-verify | Lệch? |
|---|---|---|
| Migration thật chỉ tạo 3 bảng: `notification.push_subscriptions`, `notification.vapid_key_metadata`, `notification.processed_events` — không có `notification.notification_events` | Đúng — đọc trực tiếp `migrations/0001_init.up.sql` + `0002_processed_events.up.sql`, đúng 3 bảng, đúng tên cột (`status`, `p256dh_key`, `auth_key`, `vault_key_ref`...) | Không lệch |
| Proto RPC surface thật chỉ có `Subscribe`, `UnregisterPushSubscription`, `GetVapidPublicKey`, `StreamNotifications` | Đúng — đọc trực tiếp `notification.proto`, đúng 4 rpc, không có `ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount` | Không lệch |
| `HandleIncomingEvent.Execute` chỉ: `MarkProcessed` → `DecodePayload` → `TranslateEvent` → `Broadcast`, không có bước lưu DB | Đúng — đọc trực tiếp `handle_incoming_event.go`, đúng 4 bước, không có `Save`/repository call nào giữa `TranslateEvent` và `Broadcast` | Không lệch |
| `VaultSigner.SignVapidPayload` đã wire (`Server.signer`) nhưng **0 call site** gọi nó trong `notification-service` | Đúng — `codegraph_explore`/`grep -rn "SignVapidPayload"` xác nhận: định nghĩa interface (`ports.go:49`), implement thật (`adapter/vaultsigner/signer.go:46`, gọi `credential-broker-service`), field `Server.signer` (`server.go:34`) — không có usecase nào gọi `signer.SignVapidPayload(...)`. `server.go:30-34`'s comment tự xác nhận: *"wired for the future DeliverPush usecase... not yet called from any RPC path"* | Không lệch |
| `NotificationEvent.Channels` gán `push` cho 5/6 subject rule nhưng không đọc ở đâu để gửi | Đúng — đọc trực tiếp `notification_event.go:97-144` (`subjectRules`), đúng 5/6 rule gán `ChannelDeliveryPush`; `grep "\.Channels\b"` toàn service chỉ ra field được gán ở dòng 184, không có nơi đọc | Không lệch |
| `StreamNotifications`'s frame chỉ mang `title/body/deep_link/severity` | Đúng — đọc trực tiếp `frame.go`'s `framePayload` struct, đúng 4 field, không có `is_read` | Không lệch |
| `api-gateway`'s REST mount chỉ có `POST /v1/notifications/subscribe` + `GET /v1/notifications/vapid-public-key` (`mountNotificationRoutes`) | Đúng — đọc trực tiếp `notification_routes.go`, đúng 2 route trong `mountNotificationRoutes`; có thêm 1 mount unauth riêng (`mountPushRoutes`, `/api/vapid-public-key`, `/api/push-subscribe`, `/api/push-unsubscribe`) không được CR nhắc tới nhưng không ảnh hưởng thiết kế (mount khác, cho use case unauthenticated service-worker) | Không lệch, có 1 chi tiết CR không nhắc (mount unauth song song) — ghi nhận, không đổi thiết kế |

**1 điểm cần làm rõ** (không phải lệch của CR, mà là cách diễn đạt trong yêu cầu gốc khi giao việc): tên bảng VAPID đúng trong code và trong CR là **`vapid_key_metadata`** (không phải "vault_key_metadata") — xác nhận lại bằng chính `0001_init.up.sql`. Cả 2 CR nguồn đều dùng đúng tên này, không cần sửa CR.

**Kết luận:** cả 2 CR nguồn (`CR-NOTIF-001`, `CR-NOTIF-002`) mô tả đúng và vẫn khớp 100% với code hiện tại tại thời điểm viết solution này (2026-09-09) — không có drift nào cần cảnh báo. Thiết kế bên dưới dựa thẳng trên "Giải pháp đề xuất" của từng CR, chỉ khoá cụ thể hơn vài quyết định mà CR cố ý để mở (xem từng solution's mục "Quyết định khác/thêm so với CR gốc").

## Solutions

| Solution | CR | Service | Effort | Status |
|---|---|---|---|---|
| [BE-NOTIF-SOL-001](./BE-NOTIF-SOL-001-persist-unread-state.md) | CR-NOTIF-001 | `notification-service`, `api-gateway` | Large | 🔲 Designed — chưa implement |
| [BE-NOTIF-SOL-002](./BE-NOTIF-SOL-002-deliver-push-usecase.md) | CR-NOTIF-002 | `notification-service` | Medium | 🔲 Designed — chưa implement |

## Quan hệ phụ thuộc giữa 2 solution — đọc kỹ CR, không giả định

`docs/crs/v4/notification/README.md`'s "Thứ tự thực thi" và CR-NOTIF-002's header ("Phụ thuộc") nói rõ:

- **Không phụ thuộc cứng về mặt kỹ thuật.** `DeliverPush` (BE-NOTIF-SOL-002) chỉ cần `event.Channels`/`SubscriptionRepository`/`VaultSigner` — cả 3 đã tồn tại hôm nay, không cần bảng `notification_events` của BE-NOTIF-SOL-001 để chạy được. Có thể implement BE-NOTIF-SOL-002 trước, độc lập, nếu ưu tiên nghiệp vụ đảo ngược.
- **Phụ thuộc MỀM, khuyến nghị làm BE-NOTIF-SOL-001 trước**, vì 2 lý do cụ thể CR nêu:
  1. CR-NOTIF-002's mục D: nếu BE-NOTIF-SOL-001 đã merge, `DeliverPush` nên dùng lại `NotificationEvent.ID` đã persist để giảm rủi ro gửi push trùng khi service restart giữa lúc lưu và lúc gửi — không bắt buộc transaction 2 pha, chỉ là tận dụng ID đã ổn định thay vì tự sinh riêng.
  2. **Cả 2 CR cùng sửa `HandleIncomingEvent.Execute`** (BE-NOTIF-SOL-001 thêm bước `Save`, BE-NOTIF-SOL-002 thêm bước gọi `DeliverPush`) — làm tuần tự (001 trước) và review gộp tránh 2 PR chạm cùng 1 hàm, đúng khuyến nghị của `CR-NOTIF-002`'s "Impact analysis" mục cuối.

**Thứ tự implement khuyến nghị:** `BE-NOTIF-SOL-001` → `BE-NOTIF-SOL-002`. Nếu 2 người làm song song, BE-NOTIF-SOL-002 nên bắt đầu từ `internal/adapter/external/webpush/` + `ports.go` (không chạm `HandleIncomingEvent.Execute`) và để bước wire vào `HandleIncomingEvent.Execute` (Changes Required's dòng 2) làm cuối cùng, sau khi BE-NOTIF-SOL-001's bước tương tự đã merge.

## Nguyên tắc bảo mật xuyên suốt (kế thừa từ nhóm CR-STORAGE/CR-EVM/CR-AUTO)

`tenantID` cho mọi usecase mới ở đây **luôn lấy từ `tenant.RequireTenantID(ctx)`** — xác nhận đây đúng là pattern hiện có của service (`subscribe.go:41`, `get_vapid_public_key.go:24` đều gọi `tenant.RequireTenantID(ctx)` trước khi chạm repository, không có field `tenant_id` nào trên bất kỳ proto request nào). `userID` cho các RPC mới (`ListNotifications`, `MarkAsRead`, `MarkAllAsRead`, `GetUnreadCount`) tiếp tục theo đúng pattern `Subscribe`/`StreamNotifications` hiện có: `user_id` là 1 field trên request (không phải lấy từ metadata như `tenantID`), nhưng ở tầng `api-gateway`, `user_id` gửi lên gRPC **phải luôn là `identity.UserID`** đã xác thực qua `resolveSoftIdentity`/`identityFromContext` (đúng khuôn `handleSubscribe`'s `UserId: identity.UserID` — **không bao giờ đọc `user_id` từ request body/query của client**, kể cả khi REST route thêm path param `{id}` cho `MarkAsRead` — path param đó là ID của **notification**, không phải user). Mọi usecase mới ở `internal/usecase/` PHẢI tự đối chiếu lại: repository query luôn `WHERE tenant_id = $1 AND recipient_user_id = $2` (2 điều kiện, không chỉ 1) để 1 user không thể mark-as-read hay đọc lịch sử của user khác cùng tenant bằng cách đoán `notification_id`.
