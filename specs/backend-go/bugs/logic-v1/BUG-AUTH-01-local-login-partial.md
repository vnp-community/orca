# BUG-AUTH-01: Local login works, but error-code contract and rate limiting are gone

**Business Logic:** [BL-AUTH-01](../../../../docs/logic/auth/BL-AUTH-01-local-login.md) — Local Login (email + password)
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A deactivated user who tries to log in gets the same generic `401 invalid_credentials` as a wrong password (not the documented `403 account_inactive`), and any client (or attacker) can hammer `POST /auth/local` as fast as it likes — there is no per-IP throttling at all, so the spec's brute-force defense doesn't exist.

---

## Spec summary

`POST /auth/local` validates email+password against `orca_users`, bcrypt-compares (12 rounds), creates a session, and sets an `HttpOnly`/`SameSite=Lax` cookie. It must distinguish `account_inactive` (403) from `invalid_credentials` (401), rate-limit to 10 attempts/min per IP (429 `too_many_attempts`), and write a `login.success` audit entry with `{ip, userAgent}` metadata.

## What backend-go has

- Core flow is real and working: `POST /auth/local` → `authv1.AuthServiceClient.Login` → `Login.Execute` (`backend-go/services/auth-service/internal/usecase/login.go:52-87`) looks up the user by email, compares bcrypt hash (`backend-go/services/auth-service/internal/adapter/bcrypt/hasher.go:37-43`, cost floor `MinCost = 12` at `hasher.go:20`), creates a `domain.Session` (SHA-256-hashed token only, never the raw token, persisted — `login.go:76-82`), and returns it.
- REST wiring: `mountAuthRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go:57-78`) sets the cookie via `setSessionCookie` (`auth_routes.go:131-141`): `HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteStrictMode` (spec says `Lax`; `Strict` is stricter, not a functional gap), `MaxAge` 24h.
- "Don't leak which credential was wrong": `Login.Execute` deliberately returns the identical `AUTH_INVALID_CREDENTIALS` error for both "no such user" and "wrong password" (`login.go:57-68`), matching the spec's security note.
- Audit on success: `appendAuditBestEffort` writes a `user.login` entry (`login.go:93-99`) — see BUG-AUTH-05 for why this entry's shape is a reduced subset of the documented schema.

## What's missing

- **No 403/`account_inactive` distinction at the HTTP layer.** `Login.Execute` does correctly return a different error kind for a deactivated account (`apperrors.KindPermissionDenied`, `AUTH_ACCOUNT_DEACTIVATED`, `login.go:63-65`) vs wrong credentials (`apperrors.KindUnauthenticated`) — but `mountAuthRoutes`'s `/auth/local` handler throws away every error uniformly: `if err != nil { writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", ...) }` (`auth_routes.go:68-74`) never calls `writeGRPCError` (the helper other admin routes use to map gRPC codes to HTTP status, e.g. `auth_admin_routes.go:65-68`). A deactivated user gets `401 invalid_credentials`, not the spec's `403 { error: "account_inactive" }`.
- **No rate limiting on `/auth/local` at all**, per-IP or otherwise. `mountAuthRoutes` is called before the `authed` router group is constructed (`backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:100`), and `rateLimitMiddleware(deps.RateLimiter)` is only `.Use()`'d on that later `authed` group (`router.go:118`) — so `/auth/local` runs with zero throttling. Even if it were wired, `usecase.RateLimiter` (`backend-go/services/api-gateway/internal/usecase/rate_limit.go:9-33`) keys its token buckets by **tenant ID** (`Allow(tenantID string)`), not by IP — an unauthenticated login request has no tenant yet, so this limiter can't enforce the spec's "10/min per IP" even in principle. `grep -rli "too_many_attempts" backend-go/services/api-gateway backend-go/services/auth-service` returns no matches.
- **No `login.fail` audit entries.** `Login.Execute` only calls `appendAuditBestEffort` on the success path (`login.go:84`); every early-return error path (missing creds, invalid creds, deactivated account) returns before any audit write. The spec's `login.fail` event (with `{ip, email}` metadata) never happens.
- **No IP/User-Agent capture anywhere in the login path.** `LoginRequest` (proto) carries only `email`/`password` (`backend-go/proto/orca/auth/v1/*.proto:58-61`); the gRPC server's `Login` method (`backend-go/services/auth-service/internal/adapter/grpc/server.go:95`) never reads gRPC peer/metadata for a client IP or forwards a `User-Agent` header. This blocks both the `login.fail` metadata above and the `login.success` metadata the spec requires (see BUG-AUTH-05).
- **No password-format validation** (spec's "Zod schema: email format, password min 8 chars") — `Login.Execute` only checks non-empty (`login.go:53-55`). Low-severity: a malformed password just fails the bcrypt compare instead of being pre-rejected.
- `ORCA_MULTI_USER=0 → 404` has no backend-go equivalent — architectural: backend-go's services are inherently multi-tenant with no single-user mode toggle, so this precondition doesn't map cleanly. Not counted as a gap, noted for completeness.

## See also

- No prior missing-v1/api-v1 bug covers this specific gap (BUG-002 in missing-v1 is about the unrelated `/auth/sso/:provider` stub route, which **is** correctly wired — see `auth_routes.go:126-128` — and should not be re-reported).

## References

- `backend-go/services/auth-service/internal/usecase/login.go:52-99`
- `backend-go/services/auth-service/internal/adapter/bcrypt/hasher.go:20,37-43`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go:57-141`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:100,118`
- `backend-go/services/api-gateway/internal/usecase/rate_limit.go:9-63`
- `backend-go/services/auth-service/internal/adapter/grpc/server.go:95`
- `backend-go/proto/orca/auth/v1/auth.proto` — `LoginRequest`/`LoginResponse`
- `docs/logic/auth/BL-AUTH-01-local-login.md`
