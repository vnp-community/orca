# backend-go Solutions — Orca CLI / F09 (v4)

**CRs:** [docs/crs/v4/orca-cli/](../../../../../../docs/crs/v4/orca-cli/README.md)
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md), [`auth-service.md`](../../../../tdd/services/auth-service.md)

## Đánh giá trạng thái hiện tại (bắt buộc trước khi thiết kế — theo yêu cầu)

CLI thật (F09) sống ở `desktop/src/cli/` — không phải `backend-go` (xem CR-CLI-001).
Phần **backend-go** của bộ CR này KHÔNG phải xây service mới, mà là 3 việc hẹp,
độc lập trên `api-gateway`/`auth-service` đã tồn tại thật:

1. **`wscompat` (WS RPC endpoint CLI sẽ nói chuyện qua)** hiện chỉ xác thực bằng
   cookie trình duyệt (`ServeHTTP` gọi thẳng `h.Auth.ValidateCookie`), **không có
   nhánh bearer-JWT nào** — dù CR-CLI-001 giả định "không cần đổi gì". Re-verify
   (xem [BE-CLI-SOL-001](./BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) mục 1)
   xác nhận đây là code mới thật sự cần viết, nhưng nhỏ và có khuôn mẫu sẵn để
   mirror (`wsbridge.Handler.resolveIdentity`).
2. **Cơ chế mint JWT headless mà CR-CLI-002 tưởng chưa tồn tại — đã tồn tại**:
   `auth-service`'s `IssueServiceToken` RPC (`internal/usecase/issue_service_token.go`)
   đã ký JWT thật qua Vault Transit, có test, nhưng (a) chưa có route HTTP nào expose
   nó, và (b) **tự thân code đã ghi nhận 1 lỗ hổng authorization chưa vá** (không
   check ai đang gọi có quyền mint token cho `user_id` nào) — xem
   [BE-CLI-SOL-002](./BE-CLI-SOL-002-cli-headless-service-token.md) mục 1b, đây là
   điểm **bắt buộc security review** trước khi bất kỳ route nào expose RPC này.
3. **`channels_cli.go`/`registerCliChannels`** — đúng 100% với mô tả của CR-CLI-003,
   không phát hiện lệch nào; chore rename nhỏ, rủi ro thấp.

## Solutions

| Solution | CR | Service | Status |
|---|---|---|---|
| [BE-CLI-SOL-001](./BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) | CR-CLI-001 | `api-gateway` | 🔲 Designed — chưa implement |
| [BE-CLI-SOL-002](./BE-CLI-SOL-002-cli-headless-service-token.md) | CR-CLI-002 | `auth-service`, `api-gateway` | 🔲 Designed — **⚠️ cần security review trước khi implement** |
| [BE-CLI-SOL-003](./BE-CLI-SOL-003-rename-cli-installer-channels.md) | CR-CLI-003 | `api-gateway` | 🔲 Designed — chưa implement |

Phần backend-go của CR-CLI-001 (`desktop/src/cli/runtime/backend-go-transport.ts`,
`client.ts`, `launch.ts`, `environments.ts`) là code TypeScript ở `desktop/`, ngoài
phạm vi `specs/backend-go/` — không có solution riêng ở đây cho phần đó.

## Thứ tự implement

```
BE-CLI-SOL-003 → độc lập hoàn toàn, làm bất kỳ lúc nào — nên làm sớm (Small, rủi ro thấp)

BE-CLI-SOL-001 → độc lập với 002/003 — tự nó đã có giá trị: bất kỳ bearer JWT hợp lệ
                  nào (kể cả JWT phiên đăng nhập tương tác hiện có, verify được qua
                  AuthValidator/JWKS ngay hôm nay) đều dùng gọi wscompat được sau khi
                  solution này xong, không cần chờ 002

BE-CLI-SOL-002 → phụ thuộc MỀM vào 001 (token mint ra chỉ *dùng được* cho CLI sau khi
                  wscompat nhận bearer JWT) — nhưng bản thân route mint + revocation
                  domain có thể thiết kế/implement song song với 001, miễn security
                  review xong trước khi merge phần route
```

001 và 003 có thể làm song song (khác file, khác mục tiêu). 002 nên bắt đầu thiết kế
sớm (vì cần security review trước — thời gian chờ review không nên nằm trên
critical path) nhưng code thật của 002 có thể merge sau 001.

## Nguyên tắc bảo mật xuyên suốt (đặc biệt quan trọng cho BE-CLI-SOL-002)

`tenantID`/`userID`/`user_id` cho MỌI route/usecase mới ở bộ CR này **luôn lấy từ
identity đã xác thực của caller** (`identityFromContext(ctx)` ở `httpgateway`,
`usecase.Identity` sau khi `AuthValidator.Validate`/`SessionValidator.ValidateCookie`
chạy) — **không bao giờ từ request body/query param do client gửi**, đúng quy tắc đã
kiểm chứng ở `BE-SOL-STORAGE-001/002`, `BE-AUTO-SOL-*`, và `auth-service.md`/
`api-gateway.md` §9.

Nguyên tắc này quan trọng hơn bình thường ở đây vì **BE-CLI-SOL-002 sửa đúng chỗ
CR-CLI-002 định thêm route cho — 1 RPC (`IssueServiceToken`) mà chính code hiện tại
đã tự ghi nhận là "no check that the requester is authorized to mint a token for the
given user_id"**. Route mới BẮT BUỘC ép `user_id` = identity của caller, không nhận
field đó (hay bất kỳ field tương đương nào) từ request body — xem
[BE-CLI-SOL-002](./BE-CLI-SOL-002-cli-headless-service-token.md) mục 1b/2A để biết
chi tiết vì sao làm sai điểm này tạo lỗ hổng leo thang đặc quyền tức thì, và mục "Cần
security review" cho quyết định còn mở (có cho phép mint hộ user khác không).
