# backend-go Tasks — Mobile Companion (F03, v4)

**Solutions:** [../solutions/](../solutions/README.md)

## Track 1 — `notification-service` push delivery (BE-MOBILE-SOL-001)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-MOBILE-001](./TASK-BE-MOBILE-001-subscribe-channel-field.md) — proto `channel`/`device_label` + `Subscribe` usecase + gRPC handler | Không | ✅ DONE |
| [TASK-BE-MOBILE-002](./TASK-BE-MOBILE-002-api-gateway-subscribe-passthrough.md) — `api-gateway`'s `handleSubscribe` passthrough | 001 | ✅ DONE |
| [TASK-BE-MOBILE-003](./TASK-BE-MOBILE-003-subscription-repository-mark-expired.md) — `SubscriptionRepository.MarkExpired` | Không | ✅ DONE — **regression phát hiện + vá lại 2026-09-11**: bị merge `41442c8e7` xoá mất sau lần DONE gốc, nay đã thêm lại + wire thật vào `deliver_push.go` + 3 integration test PASS trên Postgres thật (xem addendum trong file task) |
| [TASK-BE-MOBILE-004](./TASK-BE-MOBILE-004-push-credential-resolver.md) — `PushCredentialResolver` port + `credentialbroker` client | Không | ✅ DONE |
| [TASK-BE-MOBILE-005](./TASK-BE-MOBILE-005-pushgateway-apns-sender.md) — `PushSender` port + `APNsSender` | 004 | ✅ DONE |
| [TASK-BE-MOBILE-006](./TASK-BE-MOBILE-006-pushgateway-fcm-sender.md) — `FCMSender` | 004, 005 | ✅ DONE |
| [TASK-BE-MOBILE-007](./TASK-BE-MOBILE-007-deliver-push-usecase.md) — `DeliverMobilePush` usecase (đổi tên từ `DeliverPush` — trùng tên với TASK-BE-NOTIF-011) | 003, 005, 006 | ✅ DONE |
| [TASK-BE-MOBILE-008](./TASK-BE-MOBILE-008-handle-incoming-event-wiring.md) — `HandleIncomingEvent` wiring | 007 | ✅ DONE |
| [TASK-BE-MOBILE-009](./TASK-BE-MOBILE-009-main-wiring.md) — `cmd/server/main.go` composition root | 004, 005, 006, 007, 008 | ✅ DONE |

**Track 1 hoàn thành 9/9 task (2026-09-09).** Track 2 (TASK-BE-MOBILE-010) vẫn `BLOCKED` — quyết định A/B đã chốt (Phương án B: mobile SSO riêng), nhưng chờ 1 CR hạ tầng SSO mới (backend-go + frontend, desktop không đổi) chưa được viết. Không tự động unblock.

## Track 2 — `api-gateway` mobile auth endpoint (BE-MOBILE-SOL-002)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-MOBILE-010](./TASK-BE-MOBILE-010-mobile-push-subscribe-route.md) — route `POST /api/mobile/push-subscribe` | 001, 002 | 🔲 **BLOCKED** — A/B đã chốt (Phương án B), chờ CR hạ tầng SSO mobile tiền đề (xem task's "Quyết định A/B đã chốt", BE-MOBILE-SOL-002 §1.4) |

Track 2 chỉ có 1 task vì phần backend-go thật của CR-MOBILE-002 chỉ có 1
file (xem BE-MOBILE-SOL-002's README §"Phạm vi") — mọi phần còn lại của CR
đó (`mobile/`, `desktop/`) không phải backend-go, không có task ở đây.

## Thứ tự thực thi

```
Track 1:
  001 → 002                          (proto/usecase trước, rồi api-gateway passthrough)
  003 ┐
  004 → 005 → 006 ┐
                  ├→ 007 → 008 → 009
  003 ────────────┘
  (003, 004 độc lập nhau, có thể làm song song; 005 phụ thuộc 004;
   006 phụ thuộc 004+005 vì chia sẻ interface PushSender định nghĩa ở 005;
   007 cần cả 003+005+006 xong; 008 cần 007; 009 là điểm nối cuối, cần
   004,005,006,007,008 đều xong)

Track 2:
  010 phụ thuộc 001+002, NHƯNG bị BLOCKED bởi quyết định kiến trúc riêng —
  không tự động unblock chỉ vì Track 1 xong.
```

2 track độc lập nhau về service (`notification-service` vs `api-gateway`)
nhưng Track 2's task 010 cần `SubscribeRequest` đã có field mới từ Track
1's 001/002 để build được — vẫn phải đợi 2 task đó xong dù khác track.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore` trước khi sửa symbol** — mỗi
  task đã ghi rõ symbol cần kiểm tra ở mục "gitnexus", nhưng đây là yêu cầu
  bắt buộc chung theo `CLAUDE.md`/`AGENTS.md`, không chỉ khi task nhắc tới.
  Với symbol MỚI (chưa tồn tại lúc viết task, vd. `DeliverPush`,
  `PushSender`), `impact()` sẽ trả rỗng trước khi task đó chạy — điều đó
  KHÔNG có nghĩa được bỏ qua bước này, chạy lại ngay sau khi tạo symbol để
  xác nhận không có consumer/tên trùng bất ngờ.
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê ở
  "Files cần sửa". Nếu phát hiện gap khác lúc làm (vd. 1 dependency Go còn
  thiếu ngoài dự kiến), ghi nhận lại trong "Ghi chú thực thi" của task đó,
  không tự sửa file ngoài phạm vi nếu không thật sự cần thiết để task build
  được.
- **`buf generate` sau bất kỳ thay đổi `.proto` nào** (TASK-BE-MOBILE-001)
  — kiểm tra `git diff --stat proto/gen/go/` không phá code-gen của service
  khác dùng chung `proto/gen/go` trước khi commit.
- **Test thật, không giả định pass** — mọi lệnh `go test`/`go build` trong
  mục "Verify" của từng task phải thực sự chạy và thấy kết quả (PASS/FAIL
  cụ thể), không suy đoán hay báo cáo "chắc sẽ pass".
- **TASK-BE-MOBILE-010 không được thực thi khi còn trạng thái `BLOCKED`** —
  nếu 1 AI agent được giao task này mà quyết định kiến trúc chưa được ai
  chốt, DỪNG LẠI và báo lại, không tự chọn 1 phương án để có gì đó code
  được (task đã giải thích rõ lý do trong mục riêng của nó).
- **Thêm dependency Go mới** (TASK-BE-MOBILE-005's `golang.org/x/net/http2`,
  TASK-BE-MOBILE-006's `golang.org/x/oauth2/google`) — chạy `go build
  ./...` từ `backend-go/` (workspace gốc) sau `go mod tidy`, không chỉ từ
  thư mục service, để xác nhận `go.work.sum` không bị phá cho service
  khác.
- **Không nhầm `VaultSigner`/`SignVapidPayload` với credential APNs/FCM** —
  2 port khác nhau, phục vụ 2 giao thức khác nhau hoàn toàn (xem
  BE-MOBILE-SOL-001 §1.2). Nếu 1 task tương lai ngoài bộ này định "tái sử
  dụng `signer` cho push mobile", đó là dấu hiệu sai thiết kế, không phải
  tối ưu.
