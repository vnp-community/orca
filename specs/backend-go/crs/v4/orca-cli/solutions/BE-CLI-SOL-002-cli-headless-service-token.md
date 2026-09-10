# BE-CLI-SOL-002: Credential headless cho CLI — tái dùng `IssueServiceToken` đã có, vá lỗ hổng authorization đã biết trước khi expose

> **🔲 Designed — chưa implement. ✅ Quyết định thiết kế bảo mật đã chốt (2026-09-09,
> xem mục "Quyết định" bên dưới) — sẵn sàng code. ⚠️ Vẫn cần security review độc lập
> trước khi MERGE (không phải trước khi bắt đầu).** Phụ thuộc MỀM
> [BE-CLI-SOL-001](./BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) (cần
> `wscompat` chấp nhận bearer JWT thì token mint ở đây mới dùng được cho CLI).

**CR:** [CR-CLI-002](../../../../../../docs/crs/v4/orca-cli/CR-CLI-002-headless-credential-for-cli.md)
**Service:** `auth-service`, `api-gateway` (route mới)
**TDD tham chiếu:** [`auth-service.md`](../../../../tdd/services/auth-service.md) §9 Security notes, §3 API surface

---

## 1. Trạng thái hiện tại — quan trọng, thay đổi thiết kế so với CR gốc

### 1a. Cơ chế mint JWT headless **đã tồn tại thật** — CR-CLI-002's audit bỏ sót vì grep sai từ khoá

CR-CLI-002's "Bối cảnh & Vấn đề" kết luận *"hiện không có cách nào để lấy 1 bearer JWT
hợp lệ mà không đi qua luồng login tương tác"*, dựa trên grep
`api.?key|personal access token|pat_` → rỗng. Re-verify bằng đọc trực tiếp
`auth-service`'s proto/usecase (không chỉ grep tên) phát hiện **RPC này đã tồn tại,
implement đầy đủ, có test**:

```go
// backend-go/services/auth-service/internal/usecase/issue_service_token.go:29-49
// IssueServiceToken mints a real RS256 JWT (signed via Vault Transit,
// TokenSigner) for an existing user. Per this task's design, "sub" is the
// requested user_id (after verifying it exists), "tenant_id" comes from
// that user's own record, and "aud" is the request's audience verbatim.
type IssueServiceToken struct {
	users  UserRepository
	signer TokenSigner
	clock  Clock
	ttl    time.Duration
}
func NewIssueServiceToken(users UserRepository, signer TokenSigner, clock Clock, ttl time.Duration) *IssueServiceToken
```

`AuthServiceServer.IssueServiceToken` (proto) đã wire tới usecase này trong
`adapter/grpc/server.go:140-152`, ký JWT thật qua `TokenSigner` (Vault Transit RSA
key `"jwt-signing"`, **cùng key** `GetJWKS`/`JWKSClient` dùng để verify) — nghĩa là
1 JWT mint ra từ đây **xác thực được ngay** qua `usecase.AuthValidator.Validate`
(BE-CLI-SOL-001) mà không cần đổi gì thêm ở phía verify, đúng tinh thần CR-CLI-002 §"Không tự chế cơ chế xác thực mới" — chỉ khác là cơ chế "không tự chế" ấy **đã có sẵn**, không cần thiết kế domain `CliToken` mới từ đầu như CR-CLI-002 §"Changes Required" đề xuất.

Grep trực tiếp xác nhận **KHÔNG có route HTTP nào ở `api-gateway` gọi RPC này** —
2 kết quả duy nhất trùng "IssueServiceToken" trong `httpgateway/` là
`fakeAdminAuthServiceClient`/`fakeSSOAuthServiceClient`, 2 test double chỉ implement
đủ interface `AuthServiceClient` để compile, không phải route thật. RPC này **hiện
chỉ gọi được nội bộ qua gRPC** (service mesh), chưa có đường nào từ bên ngoài.

### 1b. LỖ HỔNG BẢO MẬT ĐÃ CÓ SẴN TRONG CODE HIỆN TẠI, CHƯA ĐƯỢC VÁ — bắt buộc xử lý trước khi expose bất kỳ route nào

Chính doc comment của usecase tự thừa nhận:

```go
// issue_service_token.go:34-39
// KNOWN GAP (see this service's README "Known gaps"): the generated
// IssueServiceTokenRequest carries no caller-identity field, so there is no
// check that the *requester* of a token is itself authorized to mint one
// for the given user_id — this usecase only verifies the target user
// exists, not who is asking.
```

Tức là: **request chỉ cần biết `user_id` + `audience` bất kỳ, không cần chứng minh
gì về danh tính người gọi**, usecase sẽ ký JWT hợp lệ, đầy đủ quyền của user đó.
Hiện tại gap này "vô hại" chỉ vì chưa có route bên ngoài nào gọi tới nó — nhưng đây
chính xác là điều CR-CLI-002 đang định làm (thêm 1 route công khai để mint token).
**Nếu route mới ở CR-CLI-002 chỉ đơn thuần forward `user_id` từ request body sang
`IssueServiceToken` như CR gốc gợi ý ("Endpoint mint token: `POST
/v1/auth/cli-tokens`"), đây sẽ là lỗ hổng leo thang đặc quyền tức thì**: bất kỳ user
đã đăng nhập nào cũng có thể tự mint JWT cho user_id của người khác (kể cả admin)
chỉ bằng cách đổi 1 field trong request body.

→ Đây là điểm bắt buộc phải sửa TRƯỚC khi route được merge, đúng đúng nguyên tắc
bảo mật xuyên suốt của repo (xem README §"Nguyên tắc bảo mật"): **`user_id` của
route mint token KHÔNG BAO GIỜ được lấy từ request body — luôn ép bằng identity đã
xác thực của caller** (giống hệt cách mọi handler khác trong `tenant-service`/
`automation-service` đã làm với `companyID`/`tenantID`). Việc này tự nó đóng gap ở
mức "route công khai" (không ai mint được token cho user khác qua route này), nhưng
**KHÔNG đóng gap ở mức usecase/RPC gRPC nội bộ** — bất kỳ service nội bộ nào khác
gọi thẳng `IssueServiceToken` (không qua route mới) vẫn có thể mint hộ user bất kỳ.
Xem TASK tương ứng (mục "cần security review") cho 2 lựa chọn vá gap này ở tầng
usecase.

### 1c. Không có persistence → không có revocation — xác nhận đúng lo ngại của CR, nhưng vì lý do khác

`IssueServiceToken.Execute` (đọc toàn bộ function) không ghi bất kỳ dòng nào xuống
DB — chỉ sinh `jti` ngẫu nhiên rồi ký, không lưu `jti` ở đâu để sau này đối chiếu.
Nghĩa là: khác với session token (`RevokeSession`/`ForceRevokeAllSessionsForUser` có
tác dụng ngay vì tra DB mỗi request), **1 JWT mint từ `IssueServiceToken` không thể
bị thu hồi trước khi hết hạn bằng bất kỳ cách nào hiện có** — đúng như lo ngại gốc
của CR-CLI-002 ("cơ chế revoke"), nhưng nguyên nhân chính xác hơn CR mô tả: không
phải vì "chưa có domain mint" mà vì "domain mint có rồi nhưng stateless by design".

### 1d. `AuditEntry` domain đã có sẵn — dùng được ngay

`domain/audit.go`'s `NewAuditEntry`/`AuditEntry` đã tồn tại, đang dùng bởi
`QueryAuditLog` usecase — đúng như CR-CLI-002 §"Changes Required" giả định, khả thi
để audit mint/revoke qua domain này mà không cần tạo audit trail riêng.

## 2. Giải pháp

### A. Route mint — mount trong authed group (`router.go:121`), ép `user_id` từ identity đã xác thực, KHÔNG nhận từ body

Route mới đi cùng chỗ với `mountAuthAdminRoutes` (không phải `mountAuthRoutes` ở
`auth_routes.go`, vốn cố tình unauthenticated cho login/SSO — xem `router.go:104-105`
so với `router.go:121-136`) — phải nằm TRONG `r.Group(func(authed chi.Router) {...})`
để `authMiddleware`/`identityFromContext` có hiệu lực, mirror đúng
`mountAuthAdminRoutes`'s pattern thật (`auth_admin_routes.go:28-35, 47-69`):

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/auth_cli_token_routes.go (mới)
// mountCliTokenRoutes wires POST/GET/DELETE /v1/auth/cli-tokens — mounted in
// router.go's authed group (NOT mountAuthRoutes's unauthenticated group),
// same convention as mountAuthAdminRoutes.
func mountCliTokenRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/v1/auth/cli-tokens", func(sub chi.Router) {
		sub.Post("/", handleIssueCliToken(client))
		sub.Get("/", handleListCliTokens(client))
		sub.Delete("/{id}", handleRevokeCliToken(client))
	})
}

// user_id KHÔNG có trong request body — mirror createUserRequestBody's doc
// comment ("tenant_id deliberately absent... comes from the validated
// Identity"); ở đây áp dụng cho user_id thay vì tenant_id.
func handleIssueCliToken(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context()) // đã set bởi authMiddleware
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.IssueServiceToken(ctx, &authv1.IssueServiceTokenRequest{
			UserId:   identity.UserID, // luôn = caller — route không có field nào nhận override
			Audience: "orca-cli",      // cố định, không nhận từ client
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"jwt": resp.GetJwt(), "expires_at": resp.GetExpiresAt().AsTime()})
	}
}
```

`router.go:133-136`'s `if deps.AuthClient != nil { mountAuthAdminRoutes(authed,
deps.AuthClient); mountAdminRoutes(authed, deps.AuthClient) }` — thêm
`mountCliTokenRoutes(authed, deps.AuthClient)` cùng khối này.

### B. Vá KNOWN GAP ở tầng usecase (defense-in-depth, không chỉ chặn ở route)

Thêm field `CallerUserID` vào `IssueServiceTokenInput` (usecase, không phải proto —
tối giản, tương tự cách `tenant-service` giữ `companyID` ở usecase signature thay vì
proto field), usecase so khớp `CallerUserID == UserID` khi audience là `"orca-cli"`
(self-service mint), trả `PermissionDenied` nếu khác — đóng gap ngay ở lớp domain,
không chỉ dựa vào route không cho override. **Đây là quyết định thiết kế bảo mật còn
mở** — xem "Cần security review" bên dưới.

### C. Revocation — domain mới, tối thiểu

```go
// backend-go/services/auth-service/internal/domain/cli_token.go (MỚI)
// RevokedServiceToken ghi lại jti đã bị thu hồi — bảng riêng, KHÔNG tái dùng
// bảng `sessions` (JWT không phải session, không có "hết hạn khi logout").
type RevokedServiceToken struct {
	JTI       string
	UserID    string
	RevokedAt time.Time
}
```

`IssueServiceToken.Execute` phải insert `(jti, userID)` vào 1 bảng
`issued_service_tokens` (KHÔNG chỉ revoked — cần biết token nào đang tồn tại để
`ListCliTokens` liệt kê được) khi mint; `RevokeCliToken` set `revoked_at`.
**Verify path phải được sửa để check jti này** — đây là thay đổi chạm tới
`usecase.AuthValidator.Validate` (BE-CLI-SOL-001 dùng) hoặc 1 lớp mới sau nó, cần
thêm 1 lookup (cache ngắn hạn, tương tự `JWKSClient`'s `jwksCacheTTL`, để không query
DB mỗi request) — **effort này lớn hơn CR gốc ước lượng** (CR gốc coi effort tổng là
"Medium", riêng phần revocation-check-mỗi-request này tương đương độ phức tạp của
`JWKSClient` đã có, không phải việc nhỏ).

### D. Audit log — dùng `AuditEntry` có sẵn

`IssueServiceToken`/`RevokeCliToken` gọi `NewAuditEntry` với action
`"cli_token.issued"`/`"cli_token.revoked"` — mirror đúng cách usecase khác
(`UpdateUserRole`, ...) đã ghi audit, không tạo audit trail riêng.

### ✅ Quyết định (2026-09-09) — đã chốt, không còn "chờ security review" để BẮT ĐẦU code

Product Owner đã chốt 2 điểm mở ở mục 2B, dựa trên khuyến nghị an toàn nhất (nguyên
tắc: mặc định hẹp nhất, dễ nới sau bằng 1 CR riêng nếu phát sinh nhu cầu thật, còn
hơn mở sẵn 1 lỗ hổng chờ nhu cầu):

1. **Vá gap NGAY ở tầng usecase (Phương án B của TASK-BE-CLI-004 cũ), không chỉ chặn
   ở route.** Lý do: RPC `IssueServiceToken` là gRPC nội bộ, "chỉ gọi được qua service
   mesh" không phải lý do đủ để hoãn — 1 service khác (cố ý hoặc do bug) gọi thẳng RPC
   này vẫn mint được JWT toàn quyền cho bất kỳ `user_id` nào, và không có gì phát hiện
   việc đó ngoài audit log (đọc được, không ngăn được). Vá tận gốc rẻ hơn nhiều so với
   dọn hậu quả nếu gap này bị khai thác.
2. **KHÔNG cho phép mint hộ user khác trong v1 (không có `target_user_id`/admin-override
   nào cả)** — `CallerUserID` phải LUÔN bằng `UserID` được mint, không có ngoại lệ, không
   có cờ `allowMintForOtherUser`. Nếu sau này có nhu cầu thật (vd. tự động hoá cấp
   service-account token cho CI), đó là 1 CR bảo mật riêng, tự nó phải qua review — không
   mở sẵn cửa cho khả năng đó vào code hôm nay.

Hệ quả thiết kế (khác với "2 phương án" đã trình bày trước đây ở TASK-BE-CLI-004 cũ):

- **`CallerUserID` lấy từ gRPC context đã xác thực, KHÔNG phải 1 field trong proto
  request** — mirror đúng cách `companyID`/`tenantID` không bao giờ nằm trong proto
  request mà luôn qua `tenant.RequireTenantID(ctx)`. `api-gateway` (caller nội bộ hợp lệ
  duy nhất hôm nay của RPC này) phải propagate identity đã xác thực của HTTP caller
  (`identity.UserID`) vào gRPC context/metadata trước khi gọi `auth-service`;
  `auth-service`'s gRPC server adapter đọc giá trị đó từ context (không phải từ
  `IssueServiceTokenRequest`), populate vào `IssueServiceTokenInput.CallerUserID`. →
  **Không có thay đổi breaking nào ở `IssueServiceTokenRequest` proto** — khác với
  giả định "thêm field `caller_user_id` vào proto, breaking cho mọi caller" của bản
  nháp task trước đây.
- **⚠️ Điều kiện tiên quyết cần verify trước khi code** (chưa xác nhận trong solution
  này — task tương ứng phải verify trước, không giả định có sẵn): cơ chế propagate 1
  giá trị identity tuỳ ý (khác `tenantID`) từ `api-gateway` xuống service khác qua gRPC
  context/metadata **có tồn tại sẵn chưa**? [CR-RBAC-002](../../../../../../docs/crs/v4/team-rbac/CR-RBAC-002-unify-role-model-and-propagate-claims.md)
  đã ghi nhận 1 gap tương tự cho `role` claim (`callerGlobalRole` hard-code rỗng vì
  "no role claim propagates from api-gateway into a service's request context yet").
  Nếu cùng gap đó áp dụng cho trường hợp này, task này cần tự thêm 1 cơ chế propagate
  nhỏ (mirror `tenant.WithRole`/`tenant.Role` một khi CR-RBAC-002 làm xong, hoặc độc
  lập nếu cần gấp hơn) — KHÔNG phát minh 1 cơ chế khác nếu `common/tenant` đã có sẵn
  chỗ cắm vào.
3. **TTL mặc định**: chọn 90 ngày (`ORCA_CLI_TOKEN_TTL`, cấu hình được qua env, không
   hardcode) làm điểm khởi đầu hợp lý cho long-lived CI/headless token — đây KHÔNG phải
   quyết định bảo mật chặn được việc bắt đầu code (chỉ là 1 default value đổi được sau
   qua config, không đổi code), khác với 2 điểm ở trên.

Vẫn giữ nguyên yêu cầu **security review trước khi MERGE** (không phải trước khi bắt
đầu code) cho toàn bộ nhánh việc này — đây là code cấp quyền xác thực, luôn cần 1 người
review độc lập xác nhận implementation đúng ý quyết định ở trên trước khi vào `main`,
theo thông lệ chuẩn cho security-sensitive code, không phải vì thiết kế còn mở nữa.

## Changes Required (cập nhật so với bảng của CR gốc)

| File | Thay đổi | Khác gì so với CR gốc |
|---|---|---|
| `backend-go/services/auth-service/internal/usecase/issue_service_token.go` | Thêm `CallerUserID` vào input, check quyền (mục 2B) | CR gốc không biết usecase này đã tồn tại, đề xuất viết `MintCliToken` mới hoàn toàn — **không viết mới, sửa cái có sẵn** |
| `backend-go/services/auth-service/internal/domain/cli_token.go` (mới) | `RevokedServiceToken`/bảng `issued_service_tokens` | Đúng như CR gốc dự kiến (domain mới), nhưng phạm vi hẹp hơn — chỉ lo revocation, không lo phần mint (đã có) |
| `backend-go/services/auth-service/migrations/000X_issued_service_tokens.up.sql` (mới) | Bảng mới | — |
| `backend-go/services/auth-service/internal/usecase/revoke_cli_token.go`, `list_cli_tokens.go` (mới) | `RevokeCliToken`, `ListCliTokens` | Giữ như CR gốc |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` | `POST /v1/auth/cli-tokens`, `GET`, `DELETE /v1/auth/cli-tokens/{id}` | Route body **không có field `user_id`** (khác CR gốc — CR gốc không nói rõ điều này, đây là điểm phải khoá cứng) |
| Verify path (`AuthValidator.Validate` hoặc lớp bọc mới) | Thêm check jti-not-revoked | **Không có trong bảng "Changes Required" của CR gốc** — CR gốc viết "Không đổi gì ở `session_validator.go`" mà không nhắc gì tới việc JWT cần check revocation ở đâu; đây là gap CR gốc bỏ sót hoàn toàn |

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Route mint không ép self-identity đúng cách | **CAO** | Leo thang đặc quyền tức thì nếu làm sai — xem mục 1b. Bắt buộc test riêng `TestIssueCliToken_CannotMintForOtherUser` |
| ~~Quyết định "admin mint hộ user khác" (mục 2B)~~ | ✅ Đã chốt | Không cho phép trong v1 (xem "Quyết định 2026-09-09") — không còn rủi ro mở |
| Revocation check thêm 1 lookup mỗi request xác thực bearer JWT | Trung bình | Ảnh hưởng latency/throughput của MỌI request bearer-JWT (không chỉ CLI) nếu implement sai (không cache) — mirror `JWKSClient`'s TTL-cache pattern |
| Phụ thuộc mềm BE-CLI-SOL-001 | Thấp | Route mint tự nó không cần SOL-001 (chỉ dùng bearer JWT hiện có của phiên đăng nhập để gọi), nhưng CLI chỉ *dùng được* token mint ra nếu `wscompat` đã nhận bearer JWT (SOL-001) |

## Không thuộc phạm vi solution này

- Scope token theo project/repo cụ thể — v1 toàn quyền user, chờ security review (giữ nguyên từ CR).
- UI quản lý token trong Admin console — chờ CR-RBAC-001 cutover.
- SSO/SAML machine-to-machine — CR riêng nếu cần.
- Phần CLI-side (`orca login --print-token`, đọc `ORCA_API_TOKEN`) — thuộc `desktop/`, ngoài phạm vi `specs/backend-go/`.
- Vá KNOWN GAP cho MỌI caller nội bộ của `IssueServiceToken` (không chỉ route CLI) — nếu security review quyết định cần vá tận gốc ở usecase (mục 2B), phạm vi đó áp dụng cho toàn bộ RPC, không riêng CLI; nếu chỉ cần vá cho route CLI, có thể làm hẹp hơn (check ngay tại route, không sửa usecase) — quyết định này chờ security review.

## Liên quan

- `backend-go/services/auth-service/internal/usecase/issue_service_token.go` (đã có, tái dùng)
- `backend-go/services/auth-service/internal/adapter/grpc/server.go:140-152` (`IssueServiceToken` handler, đã có)
- `backend-go/services/auth-service/internal/domain/audit.go` (`AuditEntry`, tái dùng)
- `backend-go/services/auth-service/README.md` §"Known gaps" (tự ghi nhận KNOWN GAP)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes_test.go`, `auth_routes_test.go` (nơi duy nhất `IssueServiceToken` được tham chiếu ở `api-gateway` hiện tại — chỉ là test fake, không phải route thật)
- [BE-CLI-SOL-001](./BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) (phụ thuộc mềm)
- CR-CLI-002 (nguồn), CR-RBAC-005 (audit log — dùng chung `AuditEntry`), CR-AUTO-008 (gap tương tự ở domain khác)
