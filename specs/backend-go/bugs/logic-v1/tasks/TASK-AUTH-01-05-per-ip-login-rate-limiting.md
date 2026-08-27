# TASK-AUTH-01-05: Per-IP rate limiting on `/auth/local` (10/min, burst 10)

**From Solution:** SOL-AUTH-01
**Priority:** P1
**Service:** `api-gateway` (httpgateway)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`
**Depends on:** TASK-AUTH-01-04
**Status:** `[ ]` TODO

---

## Context

`/auth/local` has no throttling today, so an attacker can brute-force passwords at unlimited request rate. `usecase.RateLimiter` (`services/api-gateway/internal/usecase/rate_limit.go`) is already a generic per-key token-bucket limiter used for the per-tenant `authed` route group — this task adds a second, independently-keyed instance, keyed by client IP, dedicated to `/auth/local`.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, alongside the existing per-tenant `RateLimiter` construction and the `mountAuthRoutes` call site:

```go
// 10 attempts/min per IP, burst 10 — spec's literal figure
// (docs/logic/auth/BL-AUTH-01-local-login.md).
loginRateLimiter := usecase.NewRateLimiter(10.0/60.0, 10)
mountAuthRoutes(r, deps.AuthClient, deps.CookieValidator, loginRateLimiter)
```

(replacing the existing `mountAuthRoutes(r, deps.AuthClient, deps.CookieValidator)` call.)

In `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`, change `mountAuthRoutes`'s signature and add the throttle check at the top of the `/auth/local` handler:

```go
func mountAuthRoutes(mux chi.Router, authClient authv1.AuthServiceClient, cookieValidator CookieSessionValidator, loginLimiter *usecase.RateLimiter) {
	mux.Post("/auth/local", func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !loginLimiter.Allow(ip) {
			writeJSONError(w, http.StatusTooManyRequests, "too_many_attempts", "too many login attempts, try again later")
			return
		}

		var body loginRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
			return
		}
		// ... existing Login call + status-code switch from TASK-AUTH-01-04 unchanged
	})
	// ... /auth/me, /auth/logout, /auth/config, /auth/sso/{provider} unchanged
}
```

Add the `usecase` import to `auth_routes.go` if not already present (`github.com/stablyai/orca-go/services/api-gateway/internal/usecase`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAuthRoutes -v
```

Expected: an 11th `/auth/local` request within a rolling minute from the same simulated IP returns `429 {"error":"too_many_attempts"}` without the fake `authClient.Login` being called (assert call count ≤10); two different `X-Forwarded-For` IPs each get their own independent budget (mirror `TestRateLimiter_TracksTenantsIndependently` in `rate_limit_test.go`).
