# TASK-BE-CLI-002: Wire `authValidator` có sẵn vào `wscompat.New(...)` ở `main.go`

**Solution:** [BE-CLI-SOL-001](../solutions/BE-CLI-SOL-001-wscompat-bearer-jwt-identity.md) | **CR:** CR-CLI-001
**Service:** `api-gateway`
**Depends on:** TASK-BE-CLI-001
**Status:** ✅ DONE

> **Kết quả thực tế:** Code thật khớp mô tả (call site + doc comment đúng
> gần vị trí đã ghi, dòng 236/238-244/262 thực tế nằm ở 236/238-244/262 sau
> khi rà lại — sai khác không đáng kể do các đợt sửa trước). Đổi
> `wscompat.New(logger, sessionValidator, wsCompatRegistry)` thành
> `wscompat.New(logger, sessionValidator, authValidator, wsCompatRegistry)`
> — dùng đúng `authValidator` đã dựng sẵn ở dòng 209, không tạo
> `JWKSClient`/`AuthValidator` thứ hai. Cập nhật lại doc comment phía trên
> theo đúng nội dung task đã cho (mô tả cookie-first + bearer-JWT fallback).
> Xác nhận `cmd/server/main_test.go` KHÔNG tồn tại (`ls cmd/server/` không
> có file test nào) → không cần bổ sung/viết test tích hợp mới, đúng nhánh
> "chưa có suite" mà task đã dự trù.
>
> **Build/test thật**: `go build ./...` sạch. `go vet ./...` sạch. `gofmt -l
> cmd/server/main.go` sạch.

---

## Mục tiêu

`cmd/server/main.go` đã dựng sẵn `authValidator := usecase.NewAuthValidator(jwksClient)`
(dòng 209) và truyền cho `wsbridge.New(...)` (dòng 236) — truyền CÙNG instance đó vào
`wscompat.New(...)` (dòng 262) sau khi TASK-BE-CLI-001 đổi signature. Không tạo
`JWKSClient`/`AuthValidator` thứ hai.

## Files cần sửa

1. `backend-go/services/api-gateway/cmd/server/main.go` (MODIFY, 2 điểm)

## Nội dung sửa

Dòng 262 hiện tại:

```go
wsCompatHandler := wscompat.New(logger, sessionValidator, wsCompatRegistry)
```

Sửa thành:

```go
wsCompatHandler := wscompat.New(logger, sessionValidator, authValidator, wsCompatRegistry)
```

Doc comment ngay phía trên (dòng 238-244) hiện viết sai theo thiết kế mới — PHẢI sửa
lại, không để nguyên (comment cũ khẳng định "not usecase.AuthValidator's JWT
verification path", điều này không còn đúng sau TASK-BE-CLI-001):

```go
// wscompat: the legacy channel-based RPC transport the deployed
// frontend/ actually speaks over /ws (see internal/adapter/wscompat's
// package doc and docs/execution-plan.md's frontend-compatibility-layer
// section). Session auth: cookie first via authclient.SessionValidator (a
// REAL auth-service.ValidateSession call — the browser's orca_session
// cookie holds a raw session token, never a JWT), falling back to
// authValidator's bearer-JWT verification (CR-CLI-001/BE-CLI-SOL-001) for
// non-browser callers (Orca CLI over ORCA_SERVER_URL) that present
// Authorization: Bearer <jwt> instead of a cookie.
```

## Test cases cần cover

Không cần unit test mới ở `main.go` (composition root, không unit-test theo convention
repo) — nhưng bắt buộc 1 test tích hợp xác nhận end-to-end (mirror cách
TASK-BE-STORAGE's integration tests xác nhận wiring qua `go build` + smoke test có
sẵn, nếu `api-gateway` có sẵn 1 integration test suite khởi động `NewRouter`/`main`
thật — kiểm tra `cmd/server/main_test.go` hoặc tương đương có tồn tại không trước khi
quyết định viết mới hay bổ sung).

## Verify

```bash
cd backend-go/services/api-gateway
go build ./...
go vet ./...
gofmt -l cmd/server/main.go
```

## gitnexus

`impact({target: "wscompat.New", direction: "upstream"})` — xác nhận đây là call
site DUY NHẤT trong `main.go` cần sửa (không có call site nào khác của
`wscompat.New` ngoài composition root + test).

## Blocking

Không có task nào phụ thuộc trực tiếp — đây là bước wiring cuối của
BE-CLI-SOL-001, sau đó solution coi như hoàn tất về mặt code (còn lại là test tích
hợp thủ công theo tiêu chí chấp nhận của CR-CLI-001, thuộc phạm vi `desktop/`,
ngoài `specs/backend-go/`).
