# BE-CLI-SOL-001: Bearer-JWT identity cho `wscompat` (transport backend-go của CLI)

> **🔲 Designed — chưa implement.** Không phụ thuộc solution nào khác trong bộ này.

**CR:** [CR-CLI-001](../../../../../../docs/crs/v4/orca-cli/CR-CLI-001-headless-transport-to-backend-go.md)
**Service:** `api-gateway` (`internal/adapter/wscompat`, `internal/usecase`)
**Phụ thuộc:** Cần token headless thật để test end-to-end (BE-CLI-SOL-002) — nhưng bản thân code sửa ở đây làm được và có giá trị độc lập, dùng ngay được với 1 bearer JWT bất kỳ do `AuthValidator` verify được (kể cả JWT phiên đăng nhập tương tác hiện có).
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §9 Security notes

---

## 1. Trạng thái hiện tại — quan trọng, thay đổi thiết kế so với CR gốc

Re-verify trực tiếp mã nguồn (không chỉ đọc lại CR) phát hiện **2 điểm CR-CLI-001 ghi sai/ghi thiếu** về phía backend-go — cả hai đều đủ nghiêm trọng để chặn cứng mục tiêu "CLI nói chuyện được với `api-gateway` qua bearer JWT":

### 1a. "Verify path" CR-CLI-001 nêu (`SessionValidator.ValidateToken`) **không xác thực JWT**

CR-CLI-001's "Giải pháp đề xuất" viết: *"verify path phía server là `SessionValidator.ValidateToken`
(`session_validator.go:51`, JWKS qua `JWKSClient`, `jwks_client.go:37`)"*. Đọc lại
`session_validator.go` xác nhận đây là mô tả sai:

```go
// session_validator.go:1-6
// Package authclient implements real (non-placeholder) session validation
// against auth-service, for the two callers that need actual identity
// resolution ... both of which see the browser's real orca_session cookie
// value (a raw, high-entropy token — never a JWT), not a bearer token.
```

`SessionValidator.ValidateToken` (dòng 51) gọi thẳng `auth-service`'s `ValidateSession`
RPC — **tra cứu session token thô (hash lookup)**, không liên quan JWT, không dùng
`JWKSClient` ở đâu cả. Component thật xác thực bearer JWT bằng JWKS là
**`usecase.AuthValidator.Validate`** (`internal/usecase/validate_identity.go:77`) —
resolve `kid` → `JWKSClient.PublicKey` → `jwtauth.VerifyWithKey` → check
`tenant_id`/`sub` claim. Đây là 2 component khác nhau, implement 2 interface khác
nhau (`CookieValidator`/`SessionValidator` vs `AuthValidator`), **không thể dùng lẫn
cho nhau**.

### 1b. `wscompat.Handler` — nơi CR-CLI-001 định tuyến frame CLI tới — hiện **chỉ chấp nhận cookie, không hề đọc `authToken`**

CR-CLI-001 kết luận "Không cần đổi gì phía backend-go (`envelope.go`, `session_dialect.go`) —
`normalizeInboundMessage` đã tự nhận dạng dialect này" — đúng cho việc **parse** frame
`{id, authToken, method, params}`, nhưng **sai cho việc xác thực**: đọc
`normalizeInboundMessage` (`session_dialect.go:57-69`) xác nhận hàm này chỉ dùng
`msg.Method`/`msg.Params` để dựng lại msg dạng `invoke` nội bộ — **không hề đọc
`msg.AuthToken` ở đâu cả**. Danh tính request thật sự được resolve **một lần duy
nhất, tại thời điểm nâng cấp WS**, trong `wscompat.Handler.ServeHTTP`:

```go
// handler.go:74 (nguyên văn)
identity, err := h.Auth.ValidateCookie(r.Context(), r)
```

`h.Auth` có kiểu `SessionValidator` (interface chỉ có 1 method `ValidateCookie`, đọc
cookie `orca_session` trên chính HTTP request nâng cấp WS) — **không có nhánh nào
đọc header `Authorization: Bearer` hay field `authToken` trong body JSON**. Nghĩa là:
field `authToken` mà `sendRequest()`/`sendBackendGoRequest()` (CR-CLI-001) gửi trong
mỗi frame **bị bỏ qua hoàn toàn** ở phía server hiện tại; 1 client không trình duyệt
(không cookie) kết nối `wscompat` hôm nay luôn nhận `401` bất kể `authToken` gửi gì.

**Kết luận: cần code backend-go mới thật sự**, không phải "chỉ verify bằng test" như
CR-CLI-001 giả định. May mắn là pattern đã có sẵn đúng chỗ cần mirror —
`wsbridge.Handler.resolveIdentity` (dùng cho `/v1/notifications/stream`, một endpoint
WS khác của chính api-gateway) đã làm chính xác việc này:

```go
// wsbridge/handler.go:123-134 (nguyên văn, khuôn mẫu để mirror)
// resolveIdentity tries the real cookie-session validator first (same
// cookie-then-JWT fallback order as httpgateway.authMiddleware), falling
// back to h.Auth's bearer/cookie JWT verification only if Cookie is nil or
// the cookie doesn't validate.
func (h *Handler) resolveIdentity(r *http.Request) (usecase.Identity, error) {
	if h.Cookie != nil {
		if id, err := h.Cookie.ValidateCookie(r.Context(), r); err == nil {
			return usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}, nil
		}
	}
	return h.Auth.Validate(r)
}
```

`wscompat.Handler` cần đúng khuôn này: cookie trước (giữ nguyên hành vi browser hiện
tại), fallback bearer JWT (`Authorization: Bearer <token>` header) khi không có
cookie hoặc cookie không hợp lệ — **đây chính là con đường CLI (không cookie, chỉ có
JWT) sẽ đi qua**.

## 2. Giải pháp

### `wscompat.Handler` thêm field `Auth *usecase.AuthValidator` + đổi `ServeHTTP`

```go
// handler.go — Handler struct, thêm field mới
type Handler struct {
	Logger   *slog.Logger
	Auth     SessionValidator      // cookie-only, giữ nguyên tên/vai trò hiện có
	BearerAuth *usecase.AuthValidator // MỚI — fallback bearer JWT, nil-tolerant như wsbridge.Handler.Cookie
	Registry *Registry
}

func New(logger *slog.Logger, auth SessionValidator, bearerAuth *usecase.AuthValidator, registry *Registry) *Handler {
	return &Handler{Logger: logger, Auth: auth, BearerAuth: bearerAuth, Registry: registry}
}
```

```go
// handler.go:74 — thay 1 dòng bằng 1 hàm resolveIdentity mirror wsbridge.Handler
func (h *Handler) resolveIdentity(r *http.Request) (Identity, error) {
	if id, err := h.Auth.ValidateCookie(r.Context(), r); err == nil {
		return id, nil
	}
	if h.BearerAuth == nil {
		return Identity{}, fmt.Errorf("wscompat: no valid session cookie and no bearer auth configured")
	}
	uid, err := h.BearerAuth.Validate(r)
	if err != nil {
		return Identity{}, err
	}
	// usecase.Identity không có Role — wscompat.Identity thì có (dùng cho
	// accounts/RBAC channel khác); JWT bearer path chưa mang role claim
	// (xem validate_identity.go dòng 18-21's chính comment), giữ Role rỗng,
	// đúng hành vi hiện tại của AuthValidator ở mọi call site khác.
	return Identity{TenantID: uid.TenantID, UserID: uid.UserID}, nil
}
```

Gọi `resolveIdentity(r)` thay cho dòng `h.Auth.ValidateCookie(...)` cũ tại
`ServeHTTP`. **Thứ tự ưu tiên cookie-trước-JWT-sau giữ nguyên 100% hành vi cho mọi
client hiện có** (browser luôn có cookie, luôn thắng ở nhánh đầu) — chỉ mở thêm
đường cho client không cookie.

### `cmd/server/main.go` wiring

`api-gateway`'s composition root đã dựng `*usecase.AuthValidator` sẵn cho
`wsbridge.New(...)` — truyền cùng instance đó vào `wscompat.New(...)` (không tạo
`JWKSClient`/`AuthValidator` thứ hai — dùng chung 1 instance, đúng cách
`wsbridge`/`httpgateway.authMiddleware` đã chia sẻ).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Đổi field `SessionValidator` field name trong `Handler` struct | Thấp-Trung bình | `impact({target: "Handler", direction: "upstream"})` trên package `wscompat` bắt buộc trước khi sửa — cần rà mọi call site `wscompat.New(...)` (test doubles, `main.go`) |
| Bearer JWT không mang role claim | Thấp | Ghi nhận đúng hạn chế đã có sẵn ở `validate_identity.go` dòng 18-21 — CLI qua backend-go transport tạm thời không có `Role` trong `Identity`; nếu channel nào yêu cầu role (RBAC) sẽ fail giống hệt mọi caller bearer-JWT khác hiện tại, không phải regression riêng của CR này |
| CR-CLI-001's "Không cần đổi gì phía backend-go" | Đã sai — sửa ở solution này | Task mới bắt buộc, không thể bỏ qua bằng "chỉ verify bằng test" như CR gốc viết |

## Không thuộc phạm vi solution này

- Cấp bearer JWT headless cho CI/CD (không có phiên đăng nhập trước) — xem
  [BE-CLI-SOL-002](./BE-CLI-SOL-002-cli-headless-service-token.md). Solution này chỉ
  làm cho `wscompat` CHẤP NHẬN được 1 bearer JWT hợp lệ bất kỳ (kể cả JWT phiên đăng
  nhập tương tác hiện có) — không tự tạo ra JWT mới.
- Thêm channel `orchestration.run`/`orchestration.dispatch`/`automation.runNow` —
  gap có sẵn, ngoài phạm vi bộ CR này (đã ghi rõ trong CR-CLI-001).
- Phần transport TypeScript ở `desktop/src/cli/runtime/` (`backend-go-transport.ts`,
  `client.ts`, `launch.ts`, `environments.ts`) — thuộc `desktop/`, không phải
  `backend-go`, xem tài liệu tương ứng bên frontend/desktop track (ngoài phạm vi bộ
  spec `specs/backend-go/` này).

## Liên quan

- `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go:74` (sẽ sửa)
- `backend-go/services/api-gateway/internal/adapter/wscompat/session_dialect.go:57-69` (đọc, không sửa)
- `backend-go/services/api-gateway/internal/usecase/validate_identity.go:61-105` (`AuthValidator`, tái dùng nguyên vẹn)
- `backend-go/services/api-gateway/internal/adapter/wsbridge/handler.go:123-134` (khuôn mẫu `resolveIdentity`)
- `backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go:1-6` (comment xác nhận session token "never a JWT")
- CR-CLI-001 (nguồn), CR-RBAC-002 (chuẩn hoá bearer-JWT toàn `api-gateway` — cùng hướng)
