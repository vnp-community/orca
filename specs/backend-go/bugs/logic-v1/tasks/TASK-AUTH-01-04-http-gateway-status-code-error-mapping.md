# TASK-AUTH-01-04: `/auth/local` status-code-aware error mapping + client-IP/UA capture

**From Solution:** SOL-AUTH-01
**Priority:** P0
**Service:** `api-gateway` (httpgateway)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`
**Depends on:** TASK-AUTH-01-01, TASK-AUTH-01-03
**Status:** `[x]` DONE — `auth_routes.go` now maps `codes.PermissionDenied` to 403 `account_inactive` and everything else to 401 `invalid_credentials`, resolves `clientIP`/`User-Agent` into `LoginRequest`; new tests in `auth_routes_test.go` (`TestClientIP_*`, `TestAuthRoutes_Login_*`) pass.

---

## Context

`/auth/local`'s handler currently collapses every non-nil `Login` error to a hardcoded `401 invalid_credentials`, silently changing a deactivated account's real `403` into a generic `401`. This task replaces that with a gRPC-status-code switch and resolves/forwards the caller's real IP and `User-Agent`.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`, add imports (`net`, `strings`, `google.golang.org/grpc/codes`, `google.golang.org/grpc/status`) and a `clientIP` helper:

```go
// clientIP prefers the first X-Forwarded-For hop (this gateway may sit
// behind a reverse proxy/load balancer — SSH/tunnel deployments are a
// first-class use case per AGENTS.md, not just direct-connect), falling
// back to r.RemoteAddr. Trusting X-Forwarded-For without a configured
// trusted-proxy allowlist is a known, narrow spoofing surface for the
// per-IP rate limiter only (TASK-AUTH-01-05) — acceptable for a
// brute-force *throttle* (defense in depth, not the sole control) but
// flagged so a future trusted-proxy-list config isn't mistaken for out of
// scope.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

Replace the `/auth/local` handler body's Login call and error branch:

```go
resp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
	Email:     body.Email,
	Password:  body.Password,
	Ip:        clientIP(r),
	UserAgent: r.UserAgent(),
})
if err != nil {
	st, _ := status.FromError(err)
	switch st.Code() {
	case codes.PermissionDenied:
		writeJSONError(w, http.StatusForbidden, "account_inactive", "account is deactivated")
	default:
		// Deliberately generic for every other failure kind — do not leak
		// "user not found" vs "wrong password" vs "malformed input"
		// distinctions to the client.
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	}
	return
}
```

Leave `setSessionCookie`/`writeJSON` success-path lines unchanged below it.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAuthRoutes -v
```

Expected: add/update `auth_routes_test.go` cases — deactivated-account gRPC error (`codes.PermissionDenied`) → HTTP `403 {"error":"account_inactive"}`; wrong-password/unknown-email (`codes.Unauthenticated`) → `401 {"error":"invalid_credentials"}`; `clientIP` unit test: `X-Forwarded-For: 1.2.3.4, 10.0.0.1` → `"1.2.3.4"`, no header → falls back to `RemoteAddr`'s host; existing `TestAuthRoutes_LoginSetsSessionCookie`-equivalent test still passes with a fake `AuthServiceClient` now receiving non-empty `Ip` when `X-Forwarded-For` is set.
