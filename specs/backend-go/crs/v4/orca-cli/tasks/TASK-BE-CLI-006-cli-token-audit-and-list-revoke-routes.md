# TASK-BE-CLI-006: Audit log mint/revoke qua `AuditEntry` có sẵn + route list/revoke

**Solution:** [BE-CLI-SOL-002](../solutions/BE-CLI-SOL-002-cli-headless-service-token.md) | **CR:** CR-CLI-002
**Service:** `auth-service`, `api-gateway`
**Depends on:** TASK-BE-CLI-003 (route mint), TASK-BE-CLI-005 (revocation usecase)
**Status:** ✅ DONE

> **Kết quả thực tế:** `NewAuditEntry`'s chữ ký thật đã đổi so với giả định
> của task (1 agent khác thêm `outcome Outcome, ipAddress string` — migration
> `0004_audit_outcome_and_ip` cùng batch làm task này gặp phải ở
> TASK-BE-CLI-005) — chữ ký thật:
> `NewAuditEntry(id, tenantID, actorID, action, target string, outcome Outcome, ipAddress string, occurredAt time.Time)`.
> Đọc `update_user_role.go`'s call site thật (không đoán) để lấy đúng
> convention: `outcome = domain.OutcomeAllowed`, `ipAddress = ""` (chưa có
> usecase nào trong batch này populate `tenant.ClientIP(ctx)`, giữ nguyên
> tiền lệ). `issue_service_token.go` ghi audit action `"cli_token.issued"`,
> actor = `user.ID` (self-mint-only nên actor luôn = user được mint, đúng
> đúng như task dự trù cho trường hợp KHÔNG cho phép mint hộ), target =
> `jti`. `revoke_cli_token.go` ghi `"cli_token.revoked"`, target = jti bị
> revoke; cần thêm `UserRepository` vào struct (trước đó `revoke_cli_token.go`
> không cần `users` — audit entry cần `tenantID` mà `IssuedServiceToken` domain
> type không lưu, nên phải `GetUserByID` để lấy). Cả 2 nơi audit đều
> best-effort (`_ = uc.audit.Append(...)`, không fail nghiệp vụ chính nếu
> audit lỗi) — đúng convention mọi usecase khác trong service.
>
> **Route list/revoke**: thêm `GET`/`DELETE /v1/auth/cli-tokens{/id}` vào
> `auth_cli_token_routes.go` — `handleListCliTokens` gửi `UserId:
> identity.UserID` (không đọc query/path nào cho user), `handleRevokeCliToken`
> gửi `Jti` từ `{id}` + `UserId: identity.UserID` (KHÔNG tự check ownership ở
> route, uỷ quyền hoàn toàn cho usecase — đúng thiết kế task).
>
> **Proto**: `IsServiceTokenRevoked` (TASK-BE-CLI-005) + `ListCliTokens`/
> `RevokeCliToken` (task này) — cả 3 RPC + message liên quan (`CliToken`,
> `ListCliTokensRequest/Response`, `RevokeCliTokenRequest`,
> `IsServiceTokenRevokedRequest/Response`) đã thêm CHUNG 1 lần vào
> `auth.proto` và chạy `buf generate` 1 lần duy nhất ở bước TASK-BE-CLI-005
> (gộp cả 2 task's proto need để tránh chạy `buf generate` 2 lần riêng —
> ghi rõ ở đó). Xác nhận `git diff --stat proto/gen/go/` chỉ có
> `auth.pb.go`/`auth_grpc.pb.go` tăng thêm đúng phần mới — không đụng
> `infrafleet` (dù `infrafleet.proto`/`infrafleet.pb.go` cũng đổi trong
> cùng lúc, đã xác nhận đó là do 1 agent khác sửa `.proto` đó song song, KHÔNG
> phải do `buf generate` của task này gây ra — `infra-fleet-service` vẫn
> `go build` sạch sau đó).
>
> **Test case yêu cầu**: `TestIssueServiceToken_RecordsAuditEntry`,
> `TestRevokeCliToken_RecordsAuditEntry` (auth-service, usecase level) —
> PASS. `TestListCliToken_ReturnsOnlyCallerOwnTokens` (route level, khác
> `TestListCliTokens_ScopedToCallerUser` ở usecase level đã làm ở
> TASK-BE-CLI-005 — đúng "2 lớp test khác nhau" task yêu cầu) — PASS.
> `TestRevokeCliToken_CannotRevokeAnotherUsersToken` — **test bảo mật quan
> trọng thứ 2 của bộ CR-CLI-002** (route level: xác nhận response KHÔNG
> BAO GIỜ là 200/204 khi usecase từ chối cross-user revoke, và `UserId` gửi
> lên RPC luôn là danh tính caller thật, không suy ra từ path `{id}`) — PASS.
>
> **Build/test thật**: `buf generate` chạy sạch (xem TASK-BE-CLI-005).
> `cd auth-service && go build ./... && go test ./...` — **TOÀN BỘ PASS**.
> `cd ../api-gateway && go build ./...` sạch. `go test
> ./internal/adapter/httpgateway/... -run
> "TestIssueCliToken|TestListCliToken|TestRevokeCliToken" -v` — **8/8 PASS**
> (chạy trong bản scratch-copy cô lập do package `httpgateway` thật vẫn
> đang bị agent khác chặn build — xem ghi chú môi trường ở TASK-BE-CLI-003/005;
> lần này patch tạm bằng 1 FILE MỚI trong bản copy thay vì sửa nội dung
> file có sẵn, tránh lỗi cú pháp gặp phải ở lần patch trước). `gofmt -l`
> sạch trên mọi file mới/sửa.

---

## Mục tiêu

Ghi audit log mỗi lần mint/revoke CLI token, dùng ĐÚNG domain `AuditEntry` đã tồn
tại (`internal/domain/audit.go`, `NewAuditEntry`) — không tạo audit trail riêng.
Thêm route `GET`/`DELETE /v1/auth/cli-tokens` (list/revoke) hoàn thiện nốt bộ CRUD
đã bắt đầu ở TASK-BE-CLI-003.

## Files cần sửa

1. `backend-go/services/auth-service/internal/usecase/issue_service_token.go` (MODIFY — ghi audit)
2. `backend-go/services/auth-service/internal/usecase/revoke_cli_token.go` (MODIFY — ghi audit, đã tạo ở TASK-BE-CLI-005)
3. `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_cli_token_routes.go` (MODIFY — thêm 2 route)

## Nội dung

### Audit khi mint (trong `Execute`, sau `RecordIssuedToken` thành công)

Đọc trước `NewAuditEntry`'s chữ ký thật (`domain/audit.go`) — theo pattern đã thấy ở
`domain/audit_test.go`: `NewAuditEntry(id, tenantID, actorID, action, targetID, occurredAt)`.
Xác nhận đúng thứ tự tham số + cách usecase khác (`UpdateUserRole`, ...) gọi
`NewAuditEntry` thật trước khi copy — không đoán chữ ký từ tên biến trong test.

```go
entry, err := domain.NewAuditEntry(
	generateRandomToken(...), // id — mirror cách usecase khác sinh audit id
	user.TenantID,
	user.ID,          // actor = chính user (self-mint) — nếu TASK-BE-CLI-004 chọn
	                  // phương án cho phép mint hộ, actor phải là CallerUserID,
	                  // không phải UserID — cập nhật lại theo kết luận của 004
	"cli_token.issued",
	jti,              // target = jti vừa mint, không phải user_id (đã có ở actor)
	now,
)
if err == nil {
	_ = uc.auditLog.Record(ctx, entry) // best-effort — không fail toàn bộ mint vì audit lỗi, mirror convention xử lý audit ở usecase khác
}
```

### Audit khi revoke — tương tự, action `"cli_token.revoked"`, target = `jti` bị revoke.

### Route list/revoke

```go
// auth_cli_token_routes.go — bổ sung vào mountCliTokenRoutes
r.Route("/v1/auth/cli-tokens", func(sub chi.Router) {
	sub.Post("/", handleIssueCliToken(client))
	sub.Get("/", handleListCliTokens(client))       // MỚI
	sub.Delete("/{id}", handleRevokeCliToken(client)) // MỚI — {id} = jti
})

func handleListCliTokens(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListCliTokens(ctx, &authv1.ListCliTokensRequest{UserId: identity.UserID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp) // shape cụ thể tuỳ proto — xem TASK-BE-CLI-005's usecase output
	}
}

func handleRevokeCliToken(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		jti := chi.URLParam(r, "id")
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		// auth-service's RevokeCliToken usecase PHẢI tự verify jti thuộc về
		// identity.UserID trước khi revoke — route này không tự check, mirror
		// cách RevokeSession hiện có uỷ quyền toàn bộ business rule cho usecase.
		if _, err := client.RevokeCliToken(ctx, &authv1.RevokeCliTokenRequest{Jti: jti, UserId: identity.UserID}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
```

Cần thêm 3 message + 2 RPC (`ListCliTokens`, `RevokeCliToken`) vào
`auth.proto` — chạy `buf generate` sau khi sửa, kiểm tra không phá `proto/gen/go`
dùng chung service khác trước khi commit (rủi ro đã ghi nhận ở nhiều solution khác
trong repo, ví dụ BE-SOL-STORAGE-001 §7).

## Test cases cần cover

- `TestIssueServiceToken_RecordsAuditEntry`
- `TestRevokeCliToken_RecordsAuditEntry`
- `TestListCliToken_ReturnsOnlyCallerOwnTokens` (route-level, mirror
  `TestListCliTokens_ScopedToCallerUser` ở usecase level — 2 lớp test khác nhau)
- `TestRevokeCliToken_CannotRevokeAnotherUsersToken` — user A gọi
  `DELETE /v1/auth/cli-tokens/{jti-cua-B}` → lỗi rõ ràng (403/404, không phải 200
  im lặng) — test bảo mật quan trọng thứ 2 của bộ CR-CLI-002 (sau
  `TestIssueCliToken_CannotMintForOtherUser` ở TASK-BE-CLI-003).

## Verify

```bash
cd backend-go && buf generate
cd services/auth-service && go build ./... && go test ./...
cd ../api-gateway && go build ./... && go test ./internal/adapter/httpgateway/...
gofmt -l $(git diff --name-only | grep '\.go$')
```

## gitnexus

`impact({target: "AuthServiceServer", direction: "upstream"})` sau khi thêm 2 RPC
mới vào interface — `UnimplementedAuthServiceServer` cần 2 method mới để mọi struct
embed nó (kể cả test double) vẫn compile; xác nhận không phá build ở service khác
dùng chung `authv1` package.

## Blocking

Đây là task cuối của BE-CLI-SOL-002's chuỗi phụ thuộc trong bộ này — sau task này,
solution coi như hoàn tất về mặt code (còn chờ kết luận TASK-BE-CLI-004 trước khi
được coi là "an toàn để dùng production", theo đúng cảnh báo ở đầu README).
