# CR-NOTIF-001 — Persist `NotificationEvent` + unread state (`is_read`/`read_at`) + `ListNotifications`/`MarkAsRead`/`GetUnreadCount`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-NOTIF-001 |
| **Tên** | Thêm bảng lưu trữ notification + trạng thái đã đọc/chưa đọc + API list/mark-as-read/unread-count |
| **Loại** | Feature (mở rộng phạm vi thiết kế gốc — xem "Bối cảnh & Vấn đề" mục 3) |
| **Priority** | 🔴 P0 — chặn toàn bộ acceptance criteria "In-app Notification Center" và "Unread State" của F11 |
| **Effort** | Large (4–6 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn `backend-go/services/notification-service` theo yêu cầu hoàn thiện F11 |
| **Tác động Features** | F11 (Notifications & Unread State) |
| **Phụ thuộc** | Không — độc lập với CR-NOTIF-002, nên làm **trước** vì CR-NOTIF-002 dùng lại cùng bảng lưu trữ |

---

## Bối cảnh & Vấn đề

### 1. `notification-service` hiện tại là dịch vụ **stateless-về-nội-dung** — không có bảng nào lưu chính notification

Khảo sát toàn bộ `backend-go/services/notification-service/`:

- Domain model `NotificationEvent` (`backend-go/services/notification-service/internal/domain/notification_event.go:68-81`) chỉ tồn tại **trong bộ nhớ** — được tạo bởi `TranslateEvent` (dòng 153-187) và tiêu thụ ngay bởi `Broadcaster.Broadcast` (`backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go:91-104`), một registry **in-process, không persist** (comment tự nhận thức tại dòng 8-17: *"This registry still lives entirely in one process's memory"*).
- `HandleIncomingEvent.Execute` (`backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:62-90`) chỉ làm 3 việc: dedup (`MarkProcessed`), translate, broadcast — **không có bước lưu xuống DB nào** giữa translate và broadcast.
- Migration thật của service (`backend-go/services/notification-service/migrations/0001_init.up.sql`, `0002_processed_events.up.sql`) chỉ tạo 3 bảng: `notification.push_subscriptions`, `notification.vapid_key_metadata`, `notification.processed_events` — **không có bảng `notification.notification_events` hay tương đương**. `processed_events` (dòng 6-10 của `0002`) chỉ lưu `event_id`/`subject`/`processed_at` để dedup JetStream redelivery, **không phải audit log** (comment dòng 3-5 của chính migration: *"Short-retention operational table, not an audit log"*).
- Đây **không phải bug ngẫu nhiên** — thiết kế gốc `specs/backend-go/tdd/services/notification-service.md:43-46` nói rõ: *"In-flight WS fan-out routing is transient, not persisted... no offline WS replay queue in this design; flag to product if guaranteed in-app delivery becomes a requirement."* F11 (`docs/features/F11-notifications.md:38-51`, "Đã phát hành") yêu cầu đúng "guaranteed in-app delivery" này — Notification Center hiển thị lịch sử đầy đủ, Unread State persist qua restart — nên đây là điểm quyết định kiến trúc gốc cần được đảo ngược một cách tường minh, không phải một khiếm khuyết cần vá nhỏ.

### 2. Không tồn tại field/API nào cho "đã đọc/chưa đọc" ở bất kỳ tầng nào

- `NotificationEvent` struct (dòng 68-81) không có field `IsRead`/`ReadAt`.
- Proto `backend-go/proto/orca/notification/v1/notification.proto:11-49`: RPC surface chỉ có `Subscribe`, `UnregisterPushSubscription`, `GetVapidPublicKey`, `StreamNotifications` (dòng 12-17) — **không có `ListNotifications`, `MarkAsRead`, `MarkAllAsRead`, hay `GetUnreadCount`**.
- gRPC server thật (`backend-go/services/notification-service/internal/adapter/grpc/server.go:22-117`) implement đúng 4 RPC trên, không hơn.
- `api-gateway`'s REST mount (`backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go:21-26`) chỉ expose `POST /v1/notifications/subscribe` và `GET /v1/notifications/vapid-public-key` — đây là **đăng ký Web Push subscription**, không phải "list notification history". `router.go`'s comment (dòng 15-20 của `notification_routes.go`) xác nhận `StreamNotifications` chỉ là 1 kênh WS sống (`GET /v1/notifications/stream`), không có REST nào để lấy lại lịch sử sau khi kênh đóng.
- WS bridge phía `api-gateway` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:39-68`, kênh `notifications.subscribe`) chỉ pipe từng frame `StreamNotifications` trực tiếp xuống browser (dòng 46-67) — không có buffer, không replay, không "mark as read" gửi ngược.
- README của chính service (`backend-go/services/notification-service/README.md:164-166`) liệt "No per-user notification-preference filtering" là gap đã biết, nhưng **không liệt kê thiếu unread-state/persistence** như một gap riêng — do thiết kế gốc coi đây là chủ đích ("ephemeral by design"), không phải thiếu sót được ghi nhận. CR này là quyết định mở rộng phạm vi, cần review kiến trúc trước khi triển khai.

### 3. Frontend còn chưa tiêu thụ cả stream ephemeral hiện có — đây là gap 2 chiều, không chỉ backend

Khảo sát `frontend/src` xác nhận: kênh `notifications.subscribe`/`notifications.event` (đã mô tả ở mục 1-2) được liệt trong danh sách dọn dẹp subscription chung
(`frontend/src/shared/remote-runtime-shared-control-subscriptions.ts:129-136`, `remote-runtime-shared-control-protocol.ts:70-71`) nhưng **không hề có handler nào tiêu thụ event `notifications.event`** — `grep "notifications\.event"` trên toàn bộ `frontend/src` cho 0 kết quả. `frontend/src/renderer/src/runtime/runtime-notifications-client.ts:1-73` chỉ wrap RPC **desktop OS notification** (`notifications.dispatch`, `notifications.getPermissionStatus`...), không liên quan tới notification-service's stream.

Toàn bộ unread-state phía frontend hiện tại (`isUnread` trên `WorktreeCard`, `unreadTerminalTabs`/`unreadAgentCompletionPanes` trên tab, `useActivityUnreadCount.ts`'s `acknowledgedAgentsByPaneKey`) là **state cục bộ, tính từ agent/terminal status trong phiên hiện tại** — không đọc từ `notification-service` và không persist qua restart (dữ liệu nằm trong Zustand store, không phải Postgres). Không có component nào tên "NotificationCenter"/"NotificationPanel" — component gần nhất là `frontend/src/renderer/src/components/activity/ActivityPrototypePage.tsx` (tên "Prototype", không filter theo severity như spec F11 yêu cầu), và không có `markAsRead`/`markAllAsRead` nào trong toàn bộ `frontend/src`.

**Kết luận:** đây là điểm nghẽn **2 chiều** — backend chưa có gì để list/persist, và frontend cũng chưa gọi API/stream nào của `notification-service` cho mục đích notification-center (F11). CR này chỉ đóng vế backend-go theo đúng phạm vi yêu cầu; phần frontend (thay `ActivityPrototypePage` hoặc thêm Notification Center panel thật, tiêu thụ `ListNotifications`/`MarkAsRead`/kênh `notifications.subscribe`) cần 1 CR frontend riêng, xem "Không thuộc phạm vi CR này".

### 4. Đây là mở rộng phạm vi thiết kế gốc, không phải bug fix

`specs/backend-go/tdd/services/notification-service.md` (TDD spec, §2, dòng 43-50) đặt "no offline WS replay queue" và "no per-user preference" là quyết định kiến trúc tường minh (*"flag to product if guaranteed in-app delivery becomes a requirement"*). F11's acceptance criteria đã publish với trạng thái "✅ Đã phát hành" nhưng dựa trên yêu cầu kỹ thuật cũ ở tầng Electron (`src/main/app-icon.ts`, `src/main/dock/`, `src/main/tray/` — xem `docs/features/F11-notifications.md:70-78`), **không phải** dựa trên `backend-go`'s `notification-service` hiện tại. `feature-completion-matrix.md:49` xác nhận đúng gap này ở tầng backend-go: *"BG có stream/subscribe nhưng thiếu 'unread state' tường minh."* CR này là bước thực thi F11 tường minh ở lớp `backend-go`, đồng thời chính thức đảo ngược quyết định "ephemeral, no replay" ghi trong TDD spec — cần cập nhật spec đó khi CR merge (xem "Changes Required").

---

## Giải pháp đề xuất

### A. Bảng lưu trữ mới: `notification.notification_events`

Thêm migration `0003_notification_events` trong `backend-go/services/notification-service/migrations/`, theo đúng convention RLS 2 migration trước (`0001_init.up.sql:28-33`):

```sql
CREATE TABLE notification.notification_events (
    id                 UUID PRIMARY KEY,             -- cùng giá trị NotificationEvent.ID (uuid.NewString() ở HandleIncomingEvent)
    tenant_id          UUID NOT NULL,
    recipient_user_id  UUID NOT NULL,                -- 1 row / recipient — RecipientUserIDs "xoè" ra nhiều row để mỗi user có unread state độc lập
    source_event_id    TEXT NOT NULL,
    source_subject     TEXT NOT NULL,
    type               TEXT NOT NULL,
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    deep_link          TEXT,
    severity           TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
    is_read            BOOLEAN NOT NULL DEFAULT false,
    read_at            TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_events_recipient_unread
    ON notification.notification_events (tenant_id, recipient_user_id, is_read, created_at DESC);

ALTER TABLE notification.notification_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.notification_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

- Một row / recipient (không phải 1 row / event với mảng recipient) để "mark as read" của user A không đụng trạng thái của user B — đúng model `push_subscriptions` đã dùng (`tenant_id, user_id` là đơn vị scope, `0001_init.up.sql:26`).
- Không thêm retention/pruning job trong CR này (xem "Không thuộc phạm vi").

### B. `HandleIncomingEvent` lưu trước khi broadcast

Sửa `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:62-90`: thêm bước `notificationRepo.Save(ctx, event)` giữa `TranslateEvent` (dòng 78) và `Broadcast` (dòng 88) — lưu **trước** broadcast để một replica crash giữa 2 bước vẫn để lại record persist (mất live-push, không mất lịch sử — đúng nguyên tắc "at-least-persisted" tương tự dedup hiện có). Thêm interface `NotificationRepository` vào `ports.go` cạnh `SubscriptionRepository`.

### C. RPC mới trên `NotificationService` (proto + usecase + adapter)

Thêm vào `backend-go/proto/orca/notification/v1/notification.proto`:

```protobuf
rpc ListNotifications(ListNotificationsRequest) returns (ListNotificationsResponse);
rpc MarkAsRead(MarkAsReadRequest) returns (google.protobuf.Empty);
rpc MarkAllAsRead(MarkAllAsReadRequest) returns (google.protobuf.Empty);
rpc GetUnreadCount(GetUnreadCountRequest) returns (GetUnreadCountResponse);
```

`ListNotificationsRequest{ user_id, cursor, limit, unread_only }` — phân trang kiểu cursor giống các list RPC khác trong repo (không tự phát minh convention mới). `MarkAsReadRequest{ user_id, notification_id }`. `GetUnreadCountResponse{ count }`.

4 usecase mới tương ứng (`list_notifications.go`, `mark_as_read.go`, `mark_all_as_read.go`, `get_unread_count.go`) trong `internal/usecase/`, theo đúng pattern 1-usecase-1-file hiện có (`subscribe.go`, `get_vapid_public_key.go`, `unregister_push_subscription.go`).

### D. Propagate trạng thái đã đọc qua WS — không chỉ REST

`StreamNotifications`'s frame (`backend-go/services/notification-service/internal/adapter/grpc/frame.go:15-20`) hiện chỉ mang `title/body/deep_link/severity`. Thêm `is_read bool` vào `framePayload` — khi `MarkAsRead` chạy trên 1 tab, các tab/thiết bị khác của cùng user cần thấy badge giảm ngay: `MarkAsRead` usecase gọi `broadcaster.Broadcast` với 1 "read receipt" event tối thiểu (tái dùng đúng `Broadcaster.Subscribe` registry đã có ở `broadcaster.go:56-82`, không xây kênh mới) mang `type: "notification_read"` + `id` của notification vừa đọc, để mọi client subscribe cùng user_id cập nhật unread count real-time. Đây là điểm bắt buộc cho use case multi-device/multi-tab của SSH/remote workflow (nhiều phiên trình duyệt cùng 1 user) — xem AGENTS.md's "SSH Use Case".

### E. `api-gateway` REST + WS channel mới

- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go`: thêm `GET /v1/notifications`, `POST /v1/notifications/{id}/read`, `POST /v1/notifications/read-all`, `GET /v1/notifications/unread-count` vào `mountNotificationRoutes` (dòng 21-26), theo đúng pattern `resolveSoftIdentity`/`identityFromContext` các handler hiện có đang dùng (dòng 52-64).
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go`: không cần kênh WS mới cho mark-as-read (dùng REST — mark-as-read là hành động rời rạc, không cần độ trễ real-time của WS), nhưng `registerNotificationStreamChannel` (dòng 45-68) tự động mang `is_read` mới qua vì nó chỉ forward `item` nguyên trạng (dòng 60).

### F. Cập nhật TDD spec

`specs/backend-go/tdd/services/notification-service.md` §2 (dòng 43-50) và §5 (bảng schema, dòng 151-198): cập nhật để phản ánh quyết định mới — notification giờ có 1 system-of-record thứ 3 (`notification_events`, cạnh `push_subscriptions`/`vapid_key_metadata`), và "no offline WS replay queue" chỉ còn đúng cho **live WS push khi offline**, không còn đúng cho việc xem lại lịch sử sau khi client reconnect (client gọi `ListNotifications` khi mount).

---

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/notification-service/migrations/0003_notification_events.{up,down}.sql` | [NEW] Bảng `notification.notification_events` + index + RLS |
| `backend-go/services/notification-service/internal/domain/notification_event.go` | Không đổi struct đã export thêm field (giữ `NotificationEvent` thuần); thêm struct mới `PersistedNotification` (embeds `NotificationEvent` + `IsRead`/`ReadAt`) nếu cần tách domain đọc-lại khỏi domain phát-sinh |
| `backend-go/services/notification-service/internal/usecase/ports.go` | [NEW] Interface `NotificationRepository` (`Save`, `ListByRecipient`, `MarkAsRead`, `MarkAllAsRead`, `CountUnread`) |
| `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` | Gọi `notificationRepo.Save` trước `Broadcast` (dòng ~88) |
| `backend-go/services/notification-service/internal/usecase/list_notifications.go`, `mark_as_read.go`, `mark_all_as_read.go`, `get_unread_count.go` | [NEW] 4 usecase |
| `backend-go/services/notification-service/internal/adapter/postgres/repository.go` | [NEW] Implement `NotificationRepository` (thêm cạnh `SubscriptionRepository`/`VapidKeyRepository` đã có) |
| `backend-go/services/notification-service/internal/adapter/grpc/server.go`, `frame.go` | Wire 4 usecase mới thành RPC; thêm `is_read` vào `framePayload` (dòng 15-20) |
| `backend-go/proto/orca/notification/v1/notification.proto` | [NEW] 4 RPC + message tương ứng |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` | [NEW] 4 REST endpoint trong `mountNotificationRoutes` |
| `specs/backend-go/tdd/services/notification-service.md` | Cập nhật §2, §3 (API surface), §5 (schema) phản ánh quyết định persist |
| `docs/features/F11-notifications.md` | Đánh dấu rõ acceptance criteria nào giờ có backing thật ở `backend-go` (sau khi CR merge) |

---

## Không thuộc phạm vi CR này

- **Retention/pruning job cho `notification_events`** — bảng sẽ tăng vô hạn nếu không dọn; cần CR riêng khi có SLA lưu trữ rõ ràng từ product (tương tự gap `processed_events` chưa có pruning job, `backend-go/docs/execution-plan.md:670`).
- **Per-user notification preference/mute** — README hiện ghi rõ "explicitly out of scope" (`README.md:164-166`); nếu cần, làm CR riêng vì đụng schema mới (bảng preference) không nằm trong CR này.
- **DeliverPush usecase thật (mobile/web push gửi khi offline)** — xem CR-NOTIF-002, phụ thuộc cùng bảng `notification_events` nhưng là luồng riêng.
- **Frontend UI (Notification Center panel thật, tiêu thụ `notifications.event`/`ListNotifications`/`MarkAsRead`)** — xác nhận qua khảo sát `frontend/src`: không có component "NotificationCenter", không có `markAsRead` nào, và kênh `notifications.subscribe` hiện có (`frontend/src/shared/remote-runtime-shared-control-subscriptions.ts:129-136`) không được tiêu thụ cho mục đích này (xem mục "Bối cảnh" #3). Cần 1 CR frontend riêng, phụ thuộc CR này. Bản thân CR này chỉ đảm bảo backend-go có API thật để frontend gọi.
- **Desktop/native OS notification (macOS Notification Center, dock badge)** — thuộc `desktop/src/main/` (Electron), không thuộc `backend-go`, ngoài phạm vi CR này theo đúng yêu cầu "thực thi F11 ở lớp backend-go".

---

## Tiêu chí chấp nhận

- [ ] `notification.notification_events` tồn tại, mỗi `NotificationEvent` được ghi 1 row/recipient trước khi broadcast (test: kill broadcaster mid-flight, row vẫn tồn tại).
- [ ] `ListNotifications` trả về lịch sử đúng thứ tự `created_at DESC`, hỗ trợ `unread_only=true` và phân trang cursor.
- [ ] `MarkAsRead`/`MarkAllAsRead` set `is_read=true, read_at=now()`, idempotent (gọi lại không lỗi).
- [ ] `GetUnreadCount` trả về đúng số `is_read=false` của user, khớp với `ListNotifications(unread_only=true)`'s tổng số phần tử.
- [ ] Một `MarkAsRead` trên tab A phản ánh qua WS tới tab B (cùng user, 2 session) trong < 1 giây — test 2 subscriber giả lập trên `Broadcaster` (tái dùng pattern `broadcaster_test.go`).
- [ ] Unread state persist qua restart service (đọc từ Postgres, không phụ thuộc `Broadcaster` in-memory) — khớp đúng acceptance criterion "Mark as unread persist qua restart" của `F11-notifications.md:64`.
- [ ] `gitnexus detect_changes({scope:"compare", base_ref:"main"})` chạy sạch trước khi commit, không có symbol HIGH/CRITICAL ngoài dự kiến.

---

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted |
|---|---|---|---|
| `NotificationEvent` (`notification-service/internal/domain/notification_event.go:68`) | upstream | — (chưa chạy `impact`, chỉ `context`) | 5 incoming refs: 4 test file trong `adapter/broadcaster`, 1 caller `TranslateEvent` |
| `HandleIncomingEvent` (`notification-service/internal/usecase/handle_incoming_event.go:34`) | upstream | LOW | 3 (1 direct, module `Usecase`) |
| `Broadcaster` (`notification-service/internal/adapter/broadcaster/broadcaster.go:42`) | upstream | LOW | 3 (1 direct, module `Usecase`) |
| `Server` (`notification-service/internal/adapter/grpc/server.go:23`) | upstream | LOW | 3 (1 direct, module `Usecase`) |
| `TranslateEvent` (`notification-service/internal/domain/notification_event.go:153`) | upstream | LOW | 1 (module `Domain`) |

Toàn bộ symbol sửa trong CR này (`HandleIncomingEvent`, `Broadcaster`, `Server`, `TranslateEvent`) đều **LOW risk**, blast radius nhỏ (1-3 symbol phụ thuộc, không process nào bị ảnh hưởng theo gitnexus) — phù hợp với việc đây là service quy mô nhỏ, ít dependent (README tự mô tả "Phase 1 — pilot tier, low risk, few dependents"). Chạy lại `impact()` cho từng usecase MỚI (`list_notifications.go` v.v.) ngay trước khi implement — các symbol này chưa tồn tại nên không thể đo impact ở bước khảo sát này. Chạy `detect_changes({scope:"compare", base_ref:"main"})` trước khi commit theo đúng quy tắc bắt buộc của repo.

---

## Liên quan

- [F11-notifications.md](../../../features/F11-notifications.md)
- [feature-completion-matrix.md](../../../roadmap/feature-completion-matrix.md) (dòng F11)
- `specs/backend-go/tdd/services/notification-service.md` (§2, §3, §5 — thiết kế gốc "ephemeral by design" cần cập nhật)
- `backend-go/services/notification-service/README.md` ("Known gaps / follow-ups")
- [CR-NOTIF-002](./CR-NOTIF-002-deliver-push-usecase.md) (dùng chung bảng `notification_events` cho audit trạng thái push đã gửi)
- `docs/crs/v1/onboarding/CR-OB-008-notification-server.md` (bối cảnh thiết kế Web Push cũ ở tầng Electron/TS — không phải backend-go, nhưng cùng domain concept)
