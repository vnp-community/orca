# TASK-BE-CLI-003: Route `POST /v1/auth/cli-tokens` — mint, ép `user_id` = caller

**Solution:** [BE-CLI-SOL-002](../solutions/BE-CLI-SOL-002-cli-headless-service-token.md) | **CR:** CR-CLI-002
**Service:** `api-gateway`
**Depends on:** Không (route tự nó gọi RPC `IssueServiceToken` đã tồn tại thật)
**Status:** ✅ DONE — TASK-BE-CLI-004 đã merge xong trước (self-mint-only enforced
ở tầng usecase), nên ràng buộc "không merge trước 004" ở dòng Status gốc đã thoả

> **Kết quả thực tế:** Code khớp gần như 100% mô tả task. Tạo mới
> `auth_cli_token_routes.go` (route `POST /v1/auth/cli-tokens`, `user_id`
> luôn = `identity.UserID` từ context đã xác thực, `audience` cố định
> `"orca-cli"`, không đọc field nào từ body). Wire `mountCliTokenRoutes` vào
> `router.go`'s authed group, sau `mountAdminRoutes`, đúng vị trí task mô tả
> (dòng ~133-136 thực tế). Tạo `auth_cli_token_routes_test.go` với
> `fakeCliTokenAuthServiceClient` (implement đủ 20 method của
> `authv1.AuthServiceClient` hiện tại — nhiều hơn số method task doc liệt kê
> vì 1 agent khác đang thêm RPC `AppendAuditEntry` song song, xem ghi chú
> môi trường bên dưới) + 4 test case yêu cầu.
>
> **Ghi chú môi trường quan trọng (ảnh hưởng cách verify, không phải lỗi của
> task này)**: tại thời điểm verify, package `httpgateway` KHÔNG build được
> bằng lệnh `go build`/`go test` trực tiếp trên working directory thật — 1
> agent khác đang sửa dở `auth.proto`/`proto/gen/go/orca/auth/v1/*.pb.go`/
> `httpgateway/middleware.go`/`automation_routes.go` song song (thêm RPC
> `AppendAuditEntry`), khiến 2 fake client CÓ SẴN từ trước
> (`fakeAdminAuthServiceClient`, `fakeSSOAuthServiceClient`) thiếu method mới
> đó và fail build — KHÔNG liên quan tới thay đổi của task này. Để verify
> code CỦA TASK NÀY thật sự đúng mà không đụng/sửa file của agent khác: copy
> toàn bộ `backend-go/` sang thư mục scratch cô lập, patch tạm 2 method còn
> thiếu CHỈ trong bản copy đó (không đụng repo thật), build+test ở đó, rồi
> xoá bản copy — bản thật trong repo không hề bị sửa ngoài phạm vi.
>
> **Build/test thật (chạy trong scratch copy cô lập, xác nhận đúng code thật
> của task này)**: `go build ./...` sạch, `go vet
> ./internal/adapter/httpgateway/...` sạch. `go test
> ./internal/adapter/httpgateway/... -run TestIssueCliToken -v` —
> **4/4 PASS** (`TestIssueCliToken_Success`,
> `TestIssueCliToken_CannotMintForOtherUser` — **test bảo mật quan trọng
> nhất, PASS**, `TestIssueCliToken_RequiresAuthentication`,
> `TestIssueCliToken_PropagatesGRPCError`). `go test
> ./internal/adapter/httpgateway/...` (toàn bộ package) — PASS, không
> regress route nào khác. `gofmt -l` sạch trên cả 2 file mới +
> `router.go` (chạy trên repo thật, không phải scratch).

---

## Mục tiêu

Expose `auth-service`'s `IssueServiceToken` RPC (đã tồn tại, đã có test, ký JWT thật
qua Vault Transit) qua 1 route HTTP authenticated, để user tự mint token cho CHÍNH
MÌNH — dùng cho `orca login --print-token` (phía `desktop/`, ngoài phạm vi task này).

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_cli_token_routes.go` (MỚI)
2. `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` (MODIFY — 1 dòng)

## Nội dung

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/auth_cli_token_routes.go
package httpgateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// cliTokenAudience is the fixed `aud` claim for every token minted through
// this route — never accepted from the request, so a caller cannot mint a
// token scoped to an arbitrary other service's audience.
const cliTokenAudience = "orca-cli"

// mountCliTokenRoutes wires POST/GET/DELETE /v1/auth/cli-tokens. Mounted in
// router.go's authed group (NOT mountAuthRoutes's unauthenticated group) —
// same convention as mountAuthAdminRoutes, see that function's doc comment.
func mountCliTokenRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/v1/auth/cli-tokens", func(sub chi.Router) {
		sub.Post("/", handleIssueCliToken(client))
	})
}

// issueCliTokenResponseBody deliberately has no request body counterpart —
// there is nothing a caller may specify; user_id and audience are both
// fixed server-side. See createUserRequestBody's doc comment
// (auth_admin_routes.go) for the same "never trust identity fields from the
// body" convention this mirrors.
type issueCliTokenResponseBody struct {
	JWT       string `json:"jwt"`
	ExpiresAt string `json:"expires_at"`
}

func handleIssueCliToken(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.IssueServiceToken(ctx, &authv1.IssueServiceTokenRequest{
			UserId:   identity.UserID, // ALWAYS the caller — never from body/query
			Audience: cliTokenAudience,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, issueCliTokenResponseBody{
			JWT:       resp.GetJwt(),
			ExpiresAt: resp.GetExpiresAt().AsTime().Format(http.TimeFormat),
		})
	}
}
```

`router.go`'s authed group (dòng 133-136 hiện tại):

```go
if deps.AuthClient != nil {
	mountAuthAdminRoutes(authed, deps.AuthClient)
	mountAdminRoutes(authed, deps.AuthClient)
	mountCliTokenRoutes(authed, deps.AuthClient) // MỚI
}
```

## Test cases cần cover

- `TestIssueCliToken_Success` — identity giả có `UserID`/`TenantID`, xác nhận request
  gửi tới fake `AuthServiceClient` có đúng `UserId == identity.UserID`,
  `Audience == "orca-cli"`.
- `TestIssueCliToken_CannotMintForOtherUser` — **test bảo mật quan trọng nhất của
  task này**: xác nhận route KHÔNG đọc bất kỳ field nào từ request body (gửi body
  `{"user_id": "someone-else"}` và xác nhận request gRPC thực tế đi ra vẫn mang
  `identity.UserID` của caller, không phải giá trị trong body).
- `TestIssueCliToken_RequiresAuthentication` — không có cookie/bearer hợp lệ → 401
  (do `authMiddleware` chặn trước khi vào handler, mirror test tương tự đã có cho
  `mountAuthAdminRoutes`'s routes).
- `TestIssueCliToken_PropagatesGRPCError` — `IssueServiceToken` trả lỗi (vd. user
  không tồn tại — trường hợp lý thuyết vì identity đã xác thực, nhưng vẫn test theo
  đúng convention `writeGRPCError` của mọi route khác trong file).

## Verify

```bash
cd backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/httpgateway/... -run TestIssueCliToken -v
gofmt -l internal/adapter/httpgateway/auth_cli_token_routes.go
```

## gitnexus

`impact({target: "NewRouter", direction: "upstream"})` trước khi sửa `router.go` —
xác nhận không phá cấu trúc `Deps`/thứ tự mount hiện có. Không cần `impact()` cho
`mountCliTokenRoutes` (symbol mới).

## Blocking

- Route này hoạt động độc lập về mặt build/test, nhưng theo ràng buộc bảo mật của
  CR-CLI-002, **không merge vào `main` trước khi có kết luận security review** cho
  TASK-BE-CLI-004 (liệu có cần vá thêm ở tầng usecase hay route-level check này đã
  đủ cho v1).
- TASK-BE-CLI-006 (audit log) nên cùng PR với task này (mint mà không log ngay là
  gap tạm thời không nên tồn tại độc lập lâu).
