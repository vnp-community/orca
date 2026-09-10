# backend-go Tasks — Notifications

**Solutions:** [../solutions/](../solutions/README.md)

## Track 1 — Persist + unread state (BE-NOTIF-SOL-001)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-NOTIF-001](./TASK-BE-NOTIF-001-notification-events-migration.md) — migration `0003_notification_events` | Không | ✅ DONE |
| [TASK-BE-NOTIF-002](./TASK-BE-NOTIF-002-notification-repository-port-and-postgres.md) — domain field + `NotificationRepository` port + Postgres impl | TASK-BE-NOTIF-001 | ✅ DONE |
| [TASK-BE-NOTIF-003](./TASK-BE-NOTIF-003-handle-incoming-event-persist.md) — `HandleIncomingEvent` gọi `Save` trước `Broadcast` | TASK-BE-NOTIF-002 | ✅ DONE |
| [TASK-BE-NOTIF-004](./TASK-BE-NOTIF-004-proto-list-mark-unread-rpc.md) — proto 4 RPC mới + `buf generate` | Không (độc lập code, có thể song song 002/003) | ✅ DONE |
| [TASK-BE-NOTIF-005](./TASK-BE-NOTIF-005-list-mark-unreadcount-usecases.md) — 4 usecase CRUD (`ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount`) | TASK-BE-NOTIF-002 | ✅ DONE |
| [TASK-BE-NOTIF-006](./TASK-BE-NOTIF-006-grpc-handlers-and-frame-isread.md) — gRPC handler wiring + `frame.go`'s `is_read` | TASK-BE-NOTIF-004, TASK-BE-NOTIF-005 | ✅ DONE |
| [TASK-BE-NOTIF-007](./TASK-BE-NOTIF-007-ws-read-receipt-propagation.md) — WS read-receipt real-time (đa tab/thiết bị) | TASK-BE-NOTIF-005 | ✅ DONE |
| [TASK-BE-NOTIF-008](./TASK-BE-NOTIF-008-rest-endpoints-api-gateway.md) — REST `api-gateway` (4 route) | TASK-BE-NOTIF-006 | ✅ DONE |

## Track 2 — `DeliverPush` (BE-NOTIF-SOL-002)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-NOTIF-009](./TASK-BE-NOTIF-009-webpush-sender-port-and-mark-expired.md) — `WebPushSender` port + `SubscriptionRepository.MarkExpired` | Không | ✅ DONE |
| [TASK-BE-NOTIF-010](./TASK-BE-NOTIF-010-webpush-adapter-rfc8291.md) — `adapter/external/webpush/` (RFC 8291 encrypt + RFC 8292 VAPID JWT qua Vault) | TASK-BE-NOTIF-009 | ✅ DONE — ⚠️ kèm fix bug `SignVapidPayload` sai Vault operation ở `credential-broker-service` |
| [TASK-BE-NOTIF-011](./TASK-BE-NOTIF-011-deliver-push-usecase-wiring.md) — `DeliverPush` usecase + wire vào `HandleIncomingEvent` | TASK-BE-NOTIF-009, TASK-BE-NOTIF-010 | ✅ DONE — phát hiện + sửa lỗi kiến trúc (usecase import adapter), di chuyển `vapid.go` sang `usecase/` |

**Cả 11/11 task DONE (2026-09-09).** `go build`/`go vet`/`gofmt`/`go test ./...` sạch 100% toàn `notification-service`. Xem "Kết quả thực tế" trong từng file task cho chi tiết đầy đủ (bug đã phát hiện, sai khác so với sketch, kết quả test thật).

## Thứ tự thực thi

```
Track 1: 001 → 002 → 003                          (tuyến tính — cùng chạm HandleIncomingEvent)
              002 ┐
         004 (song song, độc lập code) ┤→ 006 → 008
              005 ┘
              005 → 007                            (WS read-receipt, tách riêng khỏi CRUD usecase)

Track 2: 009 → 010 ┐
              009 ─┴→ 011
```

**Khuyến nghị thứ tự chạy 2 track** (theo [solutions/README.md](../solutions/README.md)'s "Quan hệ phụ thuộc"): hoàn thành hết Track 1 (đặc biệt TASK-BE-NOTIF-003, sửa `HandleIncomingEvent.Execute`) trước khi bắt đầu TASK-BE-NOTIF-011 (cũng sửa hàm đó) — TASK-BE-NOTIF-009/010 (port + adapter webpush, không chạm `HandleIncomingEvent`) có thể làm song song với Track 1 bất cứ lúc nào.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore` trước khi sửa symbol** — mỗi task đã ghi rõ symbol cần kiểm tra ở mục "gitnexus", đây là yêu cầu bắt buộc chung theo `CLAUDE.md`/`AGENTS.md`, không chỉ khi task nhắc tới. `HandleIncomingEvent` đặc biệt nhạy — bị sửa bởi CẢ TASK-BE-NOTIF-003 và TASK-BE-NOTIF-011; chạy lại `impact({target: "HandleIncomingEvent", direction: "upstream"})` ngay trước khi sửa nếu task kia đã merge trước.
- **`tenantID` luôn lấy từ `tenant.RequireTenantID(ctx)`, không bao giờ từ field request** — đúng pattern `subscribe.go`/`get_vapid_public_key.go` đã có. **`userID`** trên request PHẢI được `api-gateway` gán từ `identity.UserID` đã xác thực (không phải giá trị client tự gửi) — xem TASK-BE-NOTIF-008.
- **Mọi query trên bảng `notification_events` PHẢI lọc cả `tenant_id` VÀ `recipient_user_id`** (2 điều kiện, không chỉ 1) — RLS là lớp phòng thủ thứ 2, không phải lớp duy nhất; application-layer filter vẫn bắt buộc, đúng `architecture/05-data-architecture.md`.
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê; phát hiện gap khác thì ghi nhận, không sửa luôn nếu ngoài phạm vi task.
- **`buf generate` sau bất kỳ thay đổi `.proto` nào** — kiểm tra `git diff --stat proto/gen/go/` chỉ đổi file của `notification.proto`, không đụng service khác dùng chung `proto/gen/go`.
- **Test trước, không giả định pass** — mọi lệnh `go test` trong mục "Verify" phải thực sự chạy và thấy kết quả.
- **`gitnexus detect_changes({scope:"compare", base_ref:"main"})` trước khi commit** mỗi track — xác nhận không có symbol HIGH/CRITICAL ngoài dự kiến, đúng yêu cầu bắt buộc của cả 2 CR nguồn.
