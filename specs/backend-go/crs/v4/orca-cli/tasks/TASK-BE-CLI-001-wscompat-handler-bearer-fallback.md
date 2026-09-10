# TASK-BE-CLI-001: `wscompat.Handler` — thêm fallback bearer-JWT vào `resolveIdentity`

**Solution:** [BE-CLI-SOL-001](../solutions/BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) | **CR:** CR-CLI-001
**Service:** `api-gateway`
**Depends on:** Không
**Status:** ✅ DONE

> **Kết quả thực tế:** Code thật khớp mô tả task gần như 100% (dòng 54-61,
> 74 đúng như trích dẫn). Đã thêm field `BearerAuth *usecase.AuthValidator`,
> đổi `New(...)` thêm tham số, thêm method `resolveIdentity` y hệt bản thiết
> kế, và thay `h.Auth.ValidateCookie(...)` trong `ServeHTTP` bằng
> `h.resolveIdentity(r)`. `"fmt"` chưa có sẵn trong `handler.go` (task đoán
> "rất có thể đã có sẵn" — SAI, phải thêm import mới) — đã thêm cùng import
> `usecase` package (không có import cycle: `usecase` không import
> `wscompat`, xác nhận bằng grep). Cập nhật 1 call site có sẵn trong
> `handler_test.go` (`newTestHandlerServer`) truyền `nil` cho `BearerAuth`.
> Thêm 4 test case yêu cầu (`TestHandler_ResolveIdentity_PrefersCookieOverBearerJWT`,
> `..._FallsBackToBearerJWTWhenCookieFails`,
> `..._BearerAuthNilFallsThroughToError`, `..._BothFail`) — copy pattern
> `fakeJWKSClient`/`newBearerAuthValidator` từ `wsbridge/handler_test.go`
> (không import chéo, định nghĩa lại trong package `wscompat`), thêm 1 fake
> mới `fakeCookieSessionValidator` (có `err`/`calls`) vì `fakeSessionValidator`
> có sẵn trong file không hỗ trợ giả lập lỗi cookie.
>
> **Build/test thật**: `go build ./...` sạch. `go test
> ./internal/adapter/wscompat/... -run TestHandler_ResolveIdentity -v` —
> **4/4 PASS**. `gofmt -l` sạch trên `handler.go`+`handler_test.go`. Đã chạy
> thêm full suite `go test ./internal/adapter/wscompat/...` — PASS (24.6s,
> không có test nào regress).

---

## Mục tiêu

`wscompat.Handler.ServeHTTP` hiện chỉ xác thực bằng cookie (`h.Auth.ValidateCookie`),
không có nhánh nào cho bearer JWT — thêm fallback, mirror đúng
`wsbridge.Handler.resolveIdentity` đã có, để 1 client không trình duyệt (CLI, gửi
`Authorization: Bearer <jwt>`) kết nối `wscompat` được.

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go` (MODIFY)

## Nội dung sửa

Struct `Handler` hiện tại (đọc trước khi sửa — dòng 56-60):

```go
type Handler struct {
	Logger   *slog.Logger
	Auth     SessionValidator
	Registry *Registry
}

func New(logger *slog.Logger, auth SessionValidator, registry *Registry) *Handler {
	return &Handler{Logger: logger, Auth: auth, Registry: registry}
}
```

Thêm field `BearerAuth *usecase.AuthValidator` (import
`"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"` — xác nhận
import path chính xác bằng `head -20 handler.go` trước khi thêm, package
`wscompat` hiện chưa import `usecase` nên cần thêm dòng import mới):

```go
type Handler struct {
	Logger     *slog.Logger
	Auth       SessionValidator
	BearerAuth *usecase.AuthValidator // MỚI — nil-tolerant, fallback khi Auth.ValidateCookie thất bại
	Registry   *Registry
}

func New(logger *slog.Logger, auth SessionValidator, bearerAuth *usecase.AuthValidator, registry *Registry) *Handler {
	return &Handler{Logger: logger, Auth: auth, BearerAuth: bearerAuth, Registry: registry}
}
```

Thêm method `resolveIdentity`, thay dòng `identity, err := h.Auth.ValidateCookie(r.Context(), r)`
(dòng 74 hiện tại) bằng `identity, err := h.resolveIdentity(r)`:

```go
// resolveIdentity tries the cookie session validator first (unchanged
// behavior for every existing browser client), falling back to bearer-JWT
// verification only when BearerAuth is configured and the cookie doesn't
// validate — mirrors wsbridge.Handler.resolveIdentity's exact fallback
// order (see that function's doc comment for why cookie must win first:
// a cookie-authenticated session has no bearer JWT to present).
func (h *Handler) resolveIdentity(r *http.Request) (Identity, error) {
	if id, err := h.Auth.ValidateCookie(r.Context(), r); err == nil {
		return id, nil
	}
	if h.BearerAuth == nil {
		return Identity{}, fmt.Errorf("wscompat: no valid session cookie present")
	}
	uid, err := h.BearerAuth.Validate(r)
	if err != nil {
		return Identity{}, err
	}
	return Identity{TenantID: uid.TenantID, UserID: uid.UserID}, nil
}
```

Xác nhận `"fmt"` đã import trong `handler.go` trước khi dùng `fmt.Errorf` (rất có thể
đã có sẵn — file này dùng nhiều error wrapping; kiểm tra bằng đọc phần import thật,
không giả định).

## Test cases cần cover

Thêm vào `backend-go/services/api-gateway/internal/adapter/wscompat/handler_test.go`
(mirror chính xác 4 test đã có ở `wsbridge/handler_test.go` cho cùng logic):

- `TestHandler_ResolveIdentity_PrefersCookieOverBearerJWT`
- `TestHandler_ResolveIdentity_FallsBackToBearerJWTWhenCookieFails`
- `TestHandler_ResolveIdentity_BearerAuthNilFallsThroughToError` (khác 1 test so
  với `wsbridge` — ở đó `Cookie` là field nil-tolerant, ở đây `BearerAuth` là field
  nil-tolerant, hướng ngược lại; cần test riêng cho trường hợp `BearerAuth == nil`)
- `TestHandler_ResolveIdentity_BothFail`

Dùng lại đúng `fakeJWKSClient`/`newBearerAuthValidator`-style helper đã có ở
`wsbridge/handler_test.go` (copy pattern, không import chéo package test) để tạo 1
JWT hợp lệ ký test.

## Verify

```bash
cd backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/wscompat/... -run TestHandler_ResolveIdentity -v
gofmt -l internal/adapter/wscompat/handler.go
```

## gitnexus

`impact({target: "Handler", direction: "upstream"})` trên package `wscompat` —
**bắt buộc trước khi sửa struct** (đổi signature `New(...)` là breaking change cho
mọi call site). Ghi nhận rủi ro theo README của bộ tasks: rà `cmd/server/main.go`
(1 call site thật) + mọi test double `wscompat.New(...)` trong
`*_test.go` cùng package trước khi merge.

## Blocking

TASK-BE-CLI-002 (wiring `main.go`) phụ thuộc field mới `BearerAuth`/signature mới
của `New(...)` ở task này.
