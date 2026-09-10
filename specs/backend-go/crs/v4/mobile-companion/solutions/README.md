# backend-go Solutions — Mobile Companion (F03, v4)

**CRs:** [docs/crs/v4/mobile-companion/](../../../../../../docs/crs/v4/mobile-companion/README.md)

## Đánh giá trạng thái hiện tại (bắt buộc trước khi thiết kế — theo yêu cầu)

Audit trực tiếp code (Read/Grep, không chép nguyên văn CR) khi viết các
solution dưới đây xác nhận: phần lớn khảo sát của CR-MOBILE-001/002 vẫn
đúng — `notification-service`'s schema/domain đã "thiết kế sẵn" `ios`/
`android`/`ChannelDeliveryPush` nhưng chưa có adapter thật gửi APNs/FCM;
`MobileCompanionService.ts` xác nhận lại 0 caller thật. **Nhưng có 1 sai
lệch quan trọng cần sửa lại thiết kế**: CR-MOBILE-001 giả định credential
APNs/FCM sẽ lưu "qua Vault Transit giống `VaultSigner`" — audit
`vaultsigner/signer.go` cho thấy từ Epic B, `VaultSigner` **không còn tự
chạm Vault**, nó là 1 gRPC client tới `credential-broker-service`'s
`SignVapidPayload` RPC. Thiết kế đúng cho credential APNs/FCM là dùng lại
RPC **generic** đã có sẵn của chính `credential-broker-service`
(`ResolveCredentialByOwner`/`WriteCredential`, category
`CREDENTIAL_CATEGORY_SERVICE_SECRET` — đã định nghĩa trong proto, đã xử lý ở
`credential-broker-service`'s `adapter/grpc/server.go`, nhưng **0 caller
thật** trước solution này) — xem BE-MOBILE-SOL-001 §1.1/§1.2 cho bằng chứng
đầy đủ. `VaultSigner`/`SignVapidPayload` (ký JWT VAPID cho Web Push) và
credential APNs/FCM (JWT ES256 Apple / OAuth2 service-account Google) là 2
thứ khác nhau về giao thức — không dùng lẫn được, dù `adapter/grpc/server.go`
có 1 comment có sẵn gợi ý sai điều đó.

## Phạm vi: chỉ phần backend-go thật

CR-MOBILE-001 gần như toàn bộ là backend-go (`notification-service` +
1 file `api-gateway`) — có 1 solution riêng, đầy đủ.

CR-MOBILE-002 phần lớn là `mobile/` (React Native) và dọn dead code ở
`desktop/` (Electron main) — **không phải backend-go**. Audit xác nhận
trong 7 file "Changes Required" của CR-MOBILE-002, chỉ **1 file** là
backend-go thật (`api-gateway`'s `notification_routes.go`, thêm route xác
thực cho mobile). Quyết định: **tách riêng 1 solution nhỏ**
(BE-MOBILE-SOL-002) thay vì gộp vào BE-MOBILE-SOL-001, vì lượng việc
backend-go của CR-MOBILE-002 quá nhỏ để đáng 1 nhóm task riêng nhưng đủ
tách biệt về concern (auth endpoint, không phải push delivery) để không
đáng trộn chung 1 file solution với BE-MOBILE-SOL-001. 6 file còn lại của
CR-MOBILE-002 (`mobile/*`, `desktop/*`, `frontend/*`) không có solution/task
nào ở đây — thuộc phạm vi frontend/mobile/desktop repo khác.

## Solutions

| Solution | CR | Service | Status |
|---|---|---|---|
| [BE-MOBILE-SOL-001](./BE-MOBILE-SOL-001-notification-service-push-delivery.md) | CR-MOBILE-001 | `notification-service`, `api-gateway` | 🔲 Designed — chưa implement |
| [BE-MOBILE-SOL-002](./BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md) | CR-MOBILE-002 | `api-gateway` | 🔲 Designed — chưa implement; chứa 1 quyết định kiến trúc CHƯA CHỐT (xem §1.2 của solution) |

## Thứ tự implement

```
BE-MOBILE-SOL-001 → làm trước, độc lập hoàn toàn — proto + usecase + adapter
                    APNs/FCM + credential qua credential-broker-service
                    (KHÔNG Vault trực tiếp — xem §1.1)
                          │
                          ▼
BE-MOBILE-SOL-002 → phụ thuộc CỨNG vào 001 (cần SubscribeRequest.channel/
                    device_label tồn tại trước). Route mới cho mobile auth
                    BỊ BLOCKED tới khi chốt cơ chế xác thực (§1.2 của
                    solution) — không tự chọn khi implement.
```

## Nguyên tắc bảo mật xuyên suốt (kế thừa từ nhóm CR-STORAGE/CR-EVM/CR-AUTO)

`tenantID` cho mọi usecase mới ở đây **luôn lấy từ
`tenant.RequireTenantID(ctx)` đã xác thực** — không bao giờ từ field
request client gửi. Xác nhận `Subscribe.Execute` hiện có
(`subscribe.go:44`) đã tuân thủ đúng quy tắc này (`tenantID` từ context,
`user_id` từ request nhưng scoping tenant vẫn qua metadata) — mọi thay đổi
ở BE-MOBILE-SOL-001 giữ nguyên pattern, không nới lỏng. Riêng
BE-MOBILE-SOL-002's route mới (mobile không có cookie session để
`tenant.RequireTenantID` tự resolve) là đúng lý do route đó cần 1 cơ chế
xác thực khác thay thế cookie, KHÔNG phải lý do để bỏ qua việc xác thực
tenant/user — xem solution đó §1.2/§2 cho ràng buộc cụ thể.

Credential nhạy cảm (APNs `.p8` key, FCM service-account JSON) **không bao
giờ** nằm trong biến môi trường trần hay cột DB của `notification-service`
— luôn qua `credential-broker-service`, đúng nguyên tắc
"chỉ `credential-broker-service` chạm Vault" mà Epic B đã đóng hoàn toàn
(§1.1 của BE-MOBILE-SOL-001).
