# TASK-CLI-01-05: `api-gateway` — `POST /auth/cli-token` route

**From Solution:** SOL-CLI-01
**Priority:** P1 — required before `orca-cli` (TASK-CLI-01-06/07) can authenticate, but independent of the worktree-idempotency chain
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`
**Depends on:** none (`auth-service`'s `IssueServiceToken` RPC already exists and is wired at `backend-go/services/auth-service/internal/adapter/grpc/server.go:127`)
**Status:** `[ ]` TODO

---

## Context

CLI/CI callers need a bearer JWT, not the browser session cookie `/auth/local` issues (`setSessionCookie` sets an `HttpOnly` cookie a non-browser client can't use). `auth-service`'s `IssueServiceToken(user_id, audience) -> {jwt, expires_at}` RPC is real and already wired server-side, but no route calls it. This adds that route.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go`, inside `mountAuthRoutes` (after the existing `/auth/local` block, same function):

```go
mux.Post("/auth/cli-token", func(w http.ResponseWriter, r *http.Request) {
	var body loginRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	loginResp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
		Email: body.Email, Password: body.Password,
	})
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}

	tokenResp, err := authClient.IssueServiceToken(r.Context(), &authv1.IssueServiceTokenRequest{
		UserId: loginResp.GetUser().GetId(), Audience: "cli",
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// No cookie set here — deliberate. A CLI/CI caller stores the JWT
	// itself (orca-cli writes it to ~/.config/orca/credentials.json,
	// 0600); a cookie would be silently dropped by any non-browser client.
	writeJSON(w, http.StatusOK, map[string]any{
		"jwt": tokenResp.GetJwt(), "expires_at": tokenResp.GetExpiresAt(),
		"user": toAuthUserResponse(loginResp.GetUser()),
	})
})
```

Reuses the existing `loginRequestBody` type (already defined in this file) and `toAuthUserResponse` helper — no new types needed. Not wrapped in `authMiddleware` (same as `/auth/local`) — this route *establishes* identity.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAuthRoutes -v
```

Expected new cases in `auth_routes_test.go`: valid credentials -> `200` with `{jwt, expires_at, user}` and **no** `Set-Cookie` header (regression guard distinguishing this from `/auth/local`); invalid credentials -> `401`, `IssueServiceToken` never called (assert on the fake `AuthServiceClient`).
