# TASK-BE-CLI-005: ⚠️ Domain revocation cho CLI/service token — persist `jti` + check ở verify path

> **⚠️ SECURITY-SENSITIVE TASK.** Thời hạn/TTL mặc định của token là quyết định cần
> security review (CR-CLI-002's "Cần security review trước khi triển khai") — task
> này implement CƠ CHẾ revoke, không tự chọn TTL mặc định (dùng placeholder cấu hình
> được, không hardcode 1 con số cụ thể vào code mà không qua review).

**Solution:** [BE-CLI-SOL-002](../solutions/BE-CLI-SOL-002-cli-headless-service-token.md) | **CR:** CR-CLI-002
**Service:** `auth-service`, `api-gateway`
**Depends on:** Không bắt buộc (kỹ thuật độc lập với TASK-BE-CLI-004), nhưng nên làm
SAU vì migration số thứ tự — xem ghi chú dưới
**Status:** ✅ DONE

> **Kết quả thực tế:** Migration đánh số lại `0005_issued_service_tokens`
> (không phải `0004` như bản nháp task) — `ls migrations/` cho thấy `0004`
> đã bị 1 agent khác dùng song song (`0004_audit_outcome_and_ip`, audit
> log's outcome/ip_address) trước khi task này chạy; xác nhận đúng
> `auth.users.id` là `UUID` bằng đọc `0001_init.up.sql` thật (khớp giả định
> của task). Không thêm `tenant_id`/RLS vào bảng mới dù `auth.sessions`/
> `auth.sso_identities` có tiền lệ đó — giữ đúng schema task đã cho, không
> tự mở rộng.
>
> `domain/service_token.go`, `usecase/ports.go` (`ServiceTokenRepository`
> — method `ListForUser` phải đổi tên thành `ListServiceTokensForUser` vì
> `*postgres.Repository` đã có `ListForUser` cho `SessionRepository`, Go
> không cho 2 method cùng tên khác signature trên cùng receiver — phát hiện
> bằng `go build` thật, không đoán), `adapter/postgres/service_token_repository.go`
> (mới, mirror `session_repository.go`) đều theo đúng mô tả. `issue_service_token.go`
> ghi `RecordIssuedToken` sau `Sign` thành công.
>
> **`revoke_cli_token.go`/`list_cli_tokens.go`**: thêm 1 lớp phòng thủ NGOÀI
> những gì task mô tả — không chỉ ép `UserID` từ route, còn có `CallerUserID`
> (từ `tenant.UserID(ctx)`, cùng cơ chế TASK-BE-CLI-004 đã xác nhận có sẵn)
> và so khớp `CallerUserID == UserID` trước khi cho revoke/list — cùng lớp
> lỗ hổng "tin field request làm identity" mà TASK-BE-CLI-004 vừa vá cho
> `IssueServiceToken`, áp dụng nhất quán cho 2 RPC anh em mới tạo trong
> cùng batch này (quyết định tự đưa ra, ghi rõ ở đây theo đúng yêu cầu
> "không tự ý mở rộng phạm vi... nhưng ghi nhận lại" — đây được coi là mở
> rộng hợp lý vì cùng class bảo mật, cùng PR-equivalent, áp dụng cho code
> MỚI chưa có caller nào phụ thuộc hành vi cũ).
>
> **Verify path (api-gateway)**: `usecase/ports.go` thêm `RevocationChecker`;
> `AuthValidator` thêm field EXPORTED `Revocation` (không phải `revocation`
> chữ thường như task gợi ý — set sau khi construct, nil-tolerant, tránh đổi
> chữ ký `NewAuthValidator` phá mọi call site có sẵn, đúng gợi ý "field
> optional set sau" ở mục gitnexus của task). `Validate` fail-closed khi
> `IsRevoked` lỗi (coi lỗi = revoked) — quyết định tự đưa ra, ghi rõ lý do
> trong code comment (transient outage không được phép fallback về "trust
> token"). `authclient/revocation_client.go` (mới) — cache theo từng jti,
> TTL 5 phút giống `jwksCacheTTL`, gọi RPC mới `IsServiceTokenRevoked`.
>
> **Proto**: cần RPC mới `IsServiceTokenRevoked` (task 005) — gộp CHUNG 1
> lần sửa `.proto` + 1 lần `buf generate` với 2 RPC của TASK-BE-CLI-006
> (`ListCliTokens`, `RevokeCliToken`) để tránh chạy `buf generate` 2 lần
> riêng rẽ (rủi ro với `proto/gen/go` dùng chung đã ghi trong README) — xem
> chi tiết ở TASK-BE-CLI-006's "Kết quả thực tế".
>
> **Test case đổi tên do trùng lệ thuộc `AuthValidator`**:
> `TestAuthValidator_CachesRevocationCheckWithinTTL` đặt trong package
> `usecase` (api-gateway) theo đúng tên task yêu cầu, nhưng test THẬT cho
> hành vi cache TTL nằm ở `authclient.TestRevocationClient_CachesWithinTTL`
> — lý do: `usecase` không thể import `authclient` (cycle: `authclient` ->
> `wscompat` -> `usecase`, do `wscompat.Handler.BearerAuth *usecase.AuthValidator`
> từ TASK-BE-CLI-001). Test trong `usecase` package xác nhận
> `AuthValidator.Validate` gọi `IsRevoked` đúng 1 lần/request (tiền đề để
> cache ở `authclient` có tác dụng); test trong `authclient` package xác
> nhận cache TTL thật hoạt động (3 lần gọi liên tiếp trong TTL → RPC chỉ
> chạy 1 lần).
>
> **Build/test thật**: `auth-service`: `go build ./...` sạch, `go test
> ./...` — **TOÀN BỘ PASS** (bao gồm `TestRevokeCliToken_TokenRejectedAfterRevoke`,
> `TestListCliTokens_ScopedToCallerUser`, và các test bảo mật bổ sung tự
> thêm). `api-gateway`: `go build ./...` sạch. `go test
> ./internal/usecase/...` — PASS, bao gồm 4/4 test yêu cầu
> (`TestAuthValidator_RevocationCheckerNilSkipsCheck`,
> `TestAuthValidator_CachesRevocationCheckWithinTTL`, cộng
> `TestAuthValidator_RevokedTokenRejected`). `go test
> ./internal/adapter/authclient/...` — PASS
> (`TestRevocationClient_CachesWithinTTL` và 3 test khác). `gofmt -l` sạch
> trên mọi file mới/sửa.
>
> **Ghi chú môi trường**: `go build ./...` của `auth-service` ban đầu FAIL ở
> `cmd/server/main.go` (2 dòng `NewCreateAccessPolicy`/`NewDeleteAccessPolicy`
> thiếu tham số `policyPublisher`) — xác nhận bằng `git diff` đây là do 1
> agent khác đang sửa dở `create_access_policy.go`/`delete_access_policy.go`
> song song (không phải do task này). Vì cùng 1 file `main.go` mà task này
> cũng phải sửa (thêm wiring `IsServiceTokenRevoked`/`ListCliTokens`/
> `RevokeCliToken`), đã tạm vá đúng 2 dòng đó (thêm `policyPublisher` đúng
> vị trí tham số `NewUpdateAccessPolicy` đã dùng) để `go build`/`go test`
> chạy được thật như yêu cầu — agent kia sau đó tự hội tụ về đúng cách sửa
> tương đương (đã xác nhận lại bằng đọc `main.go` sau đó, không xung đột).
> `httpgateway` package (api-gateway) vẫn build FAIL độc lập với task này —
> xem "Kết quả thực tế" của TASK-BE-CLI-003 (agent khác thêm `AppendAuditEntry`
> chưa xong); đã verify code CỦA TASK NÀY qua bản scratch-copy cô lập
> (patch tạm 3 method còn thiếu chỉ trong bản copy) — build/test sạch.
>
> **Cập nhật (2026-09-09):** TASK-BE-CLI-004 đã chốt quyết định self-mint-only (v1
> không cho phép mint hộ user khác) — bảng `issued_service_tokens` dưới đây **không
> cần cột `issued_by` riêng** (đã cân nhắc ở bản nháp trước của task này), vì `issued_by`
> luôn trùng `user_id` trong mọi trường hợp hợp lệ. Giữ schema đơn giản như bên dưới.

---

## Mục tiêu

`IssueServiceToken` hiện không ghi gì xuống DB (thuần ký JWT, stateless) — nghĩa là
không có cách thu hồi trước khi hết hạn. Thêm bảng `issued_service_tokens` (lưu
`jti` khi mint) + `RevokeCliToken`/`ListCliTokens` usecase + check `jti`-not-revoked
vào verify path.

## Files cần sửa

1. `backend-go/services/auth-service/migrations/0004_issued_service_tokens.up.sql` (MỚI)
2. `backend-go/services/auth-service/migrations/0004_issued_service_tokens.down.sql` (MỚI)
3. `backend-go/services/auth-service/internal/domain/service_token.go` (MỚI)
4. `backend-go/services/auth-service/internal/usecase/issue_service_token.go` (MODIFY — ghi `jti` khi mint)
5. `backend-go/services/auth-service/internal/usecase/revoke_cli_token.go` (MỚI)
6. `backend-go/services/auth-service/internal/usecase/list_cli_tokens.go` (MỚI)
7. `backend-go/services/api-gateway/internal/usecase/validate_identity.go` (MODIFY — check revocation)

## Nội dung

### Migration

```sql
-- 0004_issued_service_tokens.up.sql
-- auth-service's schema is "auth" (see 0001_init.up.sql's "CREATE TABLE
-- auth.users"/"auth.sessions"/"auth.audit_log") — this table follows the
-- same schema-qualified convention, not a bare "users" FK.
CREATE TABLE auth.issued_service_tokens (
    jti TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    audience TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_issued_service_tokens_user_id ON auth.issued_service_tokens(user_id);
```

Xác nhận kiểu dữ liệu chính xác của `auth.users.id` (UUID hay TEXT) bằng đọc
`0001_init.up.sql` đầy đủ trước khi khoá kiểu cột `user_id` ở trên — ví dụ trên giả
định `UUID`, PHẢI verify lại, không suy đoán.

### `issue_service_token.go` — ghi `jti` khi mint

```go
// Execute, sau khi Sign thành công, TRƯỚC khi return:
if err := uc.tokens.RecordIssuedToken(ctx, domain.IssuedServiceToken{
	JTI: jti, UserID: user.ID, Audience: in.Audience, IssuedAt: now, ExpiresAt: expiresAt,
}); err != nil {
	return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_RECORD_FAILED", "failed to record issued token", err)
}
```

Thêm port `ServiceTokenRepository` vào `usecase/ports.go` (`RecordIssuedToken`,
`IsRevoked(ctx, jti) (bool, error)`, `Revoke(ctx, jti) error`,
`ListForUser(ctx, userID) ([]domain.IssuedServiceToken, error)`), inject vào
`IssueServiceToken`/`RevokeCliToken`/`ListCliTokens` qua constructor — mirror cách
`SessionRepository`/`UserRepository` đã inject.

### Verify path — check revocation

`usecase.AuthValidator.Validate` (api-gateway) hiện verify chữ ký + `exp`/`iat`/
claims — KHÔNG check revocation (không có khái niệm này trước task này). Thêm 1
interface mới `RevocationChecker` (mirror `JWKSClient`'s port shape trong
`ports.go`):

```go
// api-gateway/internal/usecase/ports.go
type RevocationChecker interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
```

`AuthValidator` thêm field `revocation RevocationChecker` (nil-tolerant — nếu nil,
bỏ qua check, giữ nguyên hành vi cho JWT không có domain revocation, vd. session-cookie
JWT nếu có); implement thật ở `api-gateway/internal/adapter/authclient/` gọi 1 RPC
mới `auth-service`'s `IsServiceTokenRevoked` — **cache ngắn hạn bắt buộc** (mirror
`JWKSClient`'s `jwksCacheTTL` 5 phút) để không query DB mỗi request bearer-JWT.

**Đây là điểm hiệu năng quan trọng nêu ở BE-CLI-SOL-002 mục 2C** — không được bỏ
qua cache, ảnh hưởng latency mọi request bearer-JWT, không chỉ riêng CLI.

## Test cases cần cover

- `TestRevokeCliToken_TokenRejectedAfterRevoke` — mint → dùng được → revoke → request
  tiếp theo với cùng JWT bị từ chối rõ ràng (không phải lỗi chung chung).
- `TestListCliTokens_ScopedToCallerUser` — user A không thấy token của user B.
- `TestAuthValidator_RevocationCheckerNilSkipsCheck` — regression, JWT không liên
  quan tới domain này (vd. bearer JWT mint qua đường khác, nếu có) không bị ảnh hưởng
  khi `revocation == nil`.
- `TestAuthValidator_CachesRevocationCheckWithinTTL` — không gọi `IsRevoked` quá 1
  lần trong window TTL cho cùng `jti`.

## Verify

```bash
cd backend-go/services/auth-service && go build ./... && go test ./...
cd ../api-gateway && go build ./... && go test ./internal/usecase/...
```

## gitnexus

`impact({target: "AuthValidator", direction: "upstream"})` — bắt buộc trước khi
thêm field mới, xác nhận không phá `NewAuthValidator(...)` call site nào (hiện có
1 ở `main.go`, các test double khác trong `wsbridge`/`httpgateway`/`usecase` test
files — rà đủ trước khi đổi signature nếu chọn thêm tham số bắt buộc thay vì field
optional set sau).

## Blocking

TASK-BE-CLI-006 (audit log revoke) phụ thuộc `RevokeCliToken` usecase ở task này.
