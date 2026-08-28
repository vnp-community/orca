# SOL-AUTH-01: Honest 403/401/429 mapping, per-IP login throttling, and `login.fail` audit for `/auth/local`

**Resolves:** [BUG-AUTH-01](../BUG-AUTH-01-local-login-partial.md)
**Service:** `api-gateway` (HTTP surface, rate limiting) + `auth-service` (usecase, proto)
**Affected files (proposed):**
- `backend-go/proto/orca/auth/v1/auth.proto` — `LoginRequest` gains `ip`/`user_agent`
- `backend-go/services/auth-service/internal/usecase/login.go` — `LoginInput`, failure-path audit, minimal format validation
- `backend-go/services/auth-service/internal/adapter/grpc/server.go` — thread `req.GetIp()`/`req.GetUserAgent()` into `LoginInput`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` — `/auth/local` handler: status-code-aware error mapping, IP/UA capture, pre-auth rate limiting
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` — construct and pass a login-specific `*usecase.RateLimiter`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes_test.go`
- `backend-go/services/auth-service/internal/usecase/login_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- **The Kind distinction the fix needs already exists** — `Login.Execute` returns `apperrors.KindPermissionDenied`/`AUTH_ACCOUNT_DEACTIVATED` for a deactivated account vs. `apperrors.KindUnauthenticated`/`AUTH_INVALID_CREDENTIALS` for bad credentials (`backend-go/services/auth-service/internal/usecase/login.go:63-65,57-61,66-68`), and `apperrors.ToGRPCStatus(err)` already turns that `Kind` into a gRPC status code at `backend-go/services/auth-service/internal/adapter/grpc/server.go:98` (`s.login.Execute` → `apperrors.ToGRPCStatus(err)`). The gap is entirely in `api-gateway`'s `/auth/local` handler collapsing every non-nil error to one hardcoded 401 (`backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go:68-74`) — the fix is a status-code switch, not new business logic.
- **Rate limiting belongs in `api-gateway`, per-replica in-memory, same shape as the existing limiter** — `auth-service.md` §9 ("Brute-force mitigation") describes the target end-state as an OPA-governed, `access_policies`-backed decision; that is a larger lift (a new `AccessPolicy` `kind: rate-tier` row, an OPA input document, a Rego rule) that does not exist anywhere in the codebase yet. Given this bug's severity (Medium) and the spec's literal ask ("10 attempts/min per IP, 429 `too_many_attempts`"), this solution closes the gap the same way `usecase.RateLimiter` already closes per-tenant limiting on the `authed` route group (`backend-go/services/api-gateway/internal/usecase/rate_limit.go:17-33`, wired at `router.go:118`) — a second, independently-keyed instance of the same token-bucket type, keyed by client IP instead of tenant ID. `RateLimiter.Allow(key string)` (`rate_limit.go:37-39`) is already generic over its key's meaning (nothing in its implementation assumes "tenant"), so no new type is needed. The OPA/`access_policies`-governed version stays a flagged future extension, not silently dropped.
- **IP/User-Agent must be resolved server-side, never trusted from the browser** — `07-security-architecture.md`'s multi-tenancy layer 4 ("OPA policy input always includes the resolved tenant ID from the validated JWT/session — never trusted from request body") establishes the general principle this reuses: `api-gateway` is the only service with an external-facing listener (`07-security-architecture.md` "Service-to-service transport security"), so it is the only place that can see the real client IP/`User-Agent`; `auth-service` receives them as fields on `LoginRequest`, populated exclusively by `api-gateway`'s internal (mTLS-authenticated) gRPC call, the same trust boundary `AttachIdentity`-based tenant propagation already relies on elsewhere in this handler file.
- **`login.fail` audit metadata is intentionally shallow here** — full `{ip, email}` *metadata* on an audit entry requires `domain.AuditEntry` to grow a metadata field, which is [BUG-AUTH-05](../BUG-AUTH-05-audit-log-partial.md)'s scope (see [SOL-AUTH-05](./SOL-AUTH-05-audit-log-schema-and-export.md)). This solution writes the `login.fail` entry using whatever `AuditEntry` shape exists at merge time — `Target`-only if SOL-AUTH-05 hasn't landed yet, `TargetType`/`TargetID`/`Metadata` once it has — so BUG-AUTH-01 is not blocked on BUG-AUTH-05, but the two should land together for the entry to carry real IP data, per `auth-service.md` §9's audit-integrity framing.

## Design — proto

```protobuf
// auth.proto
message LoginRequest {
  string email = 1;
  string password = 2;
  // ip/user_agent are populated by api-gateway from the terminating HTTP
  // request (real client IP behind any reverse proxy, User-Agent header) —
  // never trusted from an external caller, since Login is only ever called
  // internally by api-gateway over mTLS. See auth-service.md's "who calls
  // whom" contract (§7) — no other service calls Login.
  string ip = 3;
  string user_agent = 4;
}
```

## Design — `usecase/login.go`

```go
type LoginInput struct {
    Email     string
    Password  string
    IP        string // resolved client IP, see LoginRequest.ip
    UserAgent string
}

func (uc *Login) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
    if in.Email == "" || in.Password == "" {
        return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_CREDENTIALS", "email and password are required", nil)
    }
    // Minimal format pre-check (spec: "Zod schema: email format, password
    // min 8 chars") — low severity, a malformed password would otherwise
    // just fail the bcrypt compare; this only saves a wasted user lookup.
    if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
        uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_FORMAT")
        return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_FORMAT", "invalid email or password format", nil)
    }

    user, passwordHash, err := uc.users.GetUserByEmail(ctx, in.Email)
    if err != nil {
        uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_CREDENTIALS")
        return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
    }
    if !user.IsActive {
        uc.appendFailureAuditBestEffort(ctx, in, "AUTH_ACCOUNT_DEACTIVATED")
        return LoginOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_ACCOUNT_DEACTIVATED", "account is deactivated", nil)
    }
    if err := uc.hasher.Compare(passwordHash, in.Password); err != nil {
        uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_CREDENTIALS")
        return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
    }
    // ... unchanged: token gen, NewSession (now passed in.IP/in.UserAgent —
    // see SOL-AUTH-02 for domain.Session gaining those fields), CreateSession
    uc.appendAuditBestEffort(ctx, user, now) // unchanged success path
    return LoginOutput{SessionToken: rawToken, User: user}, nil
}

// appendFailureAuditBestEffort writes a login.fail entry — mirrors
// appendAuditBestEffort's best-effort pattern (login.go:89-92) so a
// audit-write failure never turns a real auth decision into a 500. ActorID
// is empty (no authenticated user exists on a failed login, per
// domain.AuditEntry's doc comment allowing a nil/empty actor) — the
// attempted email is the only identifying fact, carried as the target until
// SOL-AUTH-05's Metadata field exists to carry {ip, email} together.
func (uc *Login) appendFailureAuditBestEffort(ctx context.Context, in LoginInput, reason string) {
    entry, err := domain.NewAuditEntry(uuid.NewString(), tenantIDOrUnknown(in), "", "login.fail", in.Email, uc.clock.Now())
    if err != nil {
        return
    }
    _ = uc.audit.Append(ctx, entry)
}
```

`tenantIDOrUnknown` is a small helper: a failed login by email alone has no
resolved tenant (matching the existing `GetUserByEmail` lookup's own
tenant-less shape, per `ports.go:46-48`'s doc comment) — use a sentinel
tenant value or skip the audit write entirely for the "no such user" case if
`domain.NewAuditEntry`'s `ErrEmptyTenant` invariant (`domain/audit.go:39-40`)
can't be satisfied; this is a real modeling question SOL-AUTH-05 should
settle (system-wide audit entries with no resolvable tenant), not papered
over here.

## Design — `adapter/grpc/server.go`

```go
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
    out, err := s.login.Execute(ctx, usecase.LoginInput{
        Email: req.GetEmail(), Password: req.GetPassword(),
        IP: req.GetIp(), UserAgent: req.GetUserAgent(),
    })
    // ... unchanged
}
```

## Design — wiring (`api-gateway` REST)

### Error mapping — `/auth/local`

Replaces the blanket branch at `auth_routes.go:68-74` with a status-code
switch on the gRPC error, giving each spec-documented body exactly the shape
`docs/logic/auth/BL-AUTH-01-local-login.md` expects (`403 {error:
"account_inactive"}` / `401 {error: "invalid_credentials"}`):

```go
resp, err := authClient.Login(r.Context(), &authv1.LoginRequest{
    Email: body.Email, Password: body.Password,
    Ip: clientIP(r), UserAgent: r.UserAgent(),
})
if err != nil {
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.PermissionDenied:
        writeJSONError(w, http.StatusForbidden, "account_inactive", "account is deactivated")
    default:
        // Deliberately generic for every other failure kind — do not leak
        // "user not found" vs "wrong password" vs "malformed input"
        // distinctions to the client (login.go:59-60's rationale, unchanged).
        writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
    }
    return
}
```

This is a bespoke mapping, not a reuse of `writeGRPCError` (the admin-route
helper at `auth_admin_routes.go`'s call sites, e.g. `handleCreateUser:66`) —
that helper's generic `{error: "PERMISSION_DENIED", ...}` shape doesn't match
this endpoint's spec-mandated `account_inactive`/`invalid_credentials`
literal error codes, so `/auth/local` needs its own switch the same way it
already has its own bespoke cookie-setting logic.

### Per-IP rate limiting

`router.go` constructs a second `*usecase.RateLimiter`, independent from the
per-tenant one already `.Use()`'d on the `authed` group (`router.go:118`),
and passes it into `mountAuthRoutes`:

```go
// router.go — alongside the existing per-tenant limiter construction
loginRateLimiter := usecase.NewRateLimiter(10.0/60.0, 10) // 10/min, burst 10 — spec's literal figure
mountAuthRoutes(mux, deps.AuthClient, deps.CookieValidator, loginRateLimiter)
```

```go
// auth_routes.go
func mountAuthRoutes(mux chi.Router, authClient authv1.AuthServiceClient, cookieValidator CookieSessionValidator, loginLimiter *usecase.RateLimiter) {
    mux.Post("/auth/local", func(w http.ResponseWriter, r *http.Request) {
        ip := clientIP(r)
        if !loginLimiter.Allow(ip) {
            writeJSONError(w, http.StatusTooManyRequests, "too_many_attempts", "too many login attempts, try again later")
            return
        }
        // ... existing decode + Login call, using the error-mapping switch above
    })
    // ... /auth/me, /auth/logout, /auth/config, /auth/sso/{provider} unchanged
}

// clientIP prefers the first X-Forwarded-For hop (this gateway may sit
// behind a reverse proxy/load balancer — SSH/tunnel deployments are a first-
// class use case per AGENTS.md, not just direct-connect), falling back to
// r.RemoteAddr. Trusting X-Forwarded-For without a configured trusted-proxy
// allowlist is a known, narrow spoofing surface for THIS limiter only (an
// attacker who can set arbitrary headers could evade the per-IP bucket by
// rotating a fake header value) — acceptable for a brute-force *throttle*
// (defense in depth, not the sole control) but flagged so a future
// trusted-proxy-list config isn't mistaken for out of scope.
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

## Test plan

- `login_test.go`: deactivated user → `KindPermissionDenied`/`AUTH_ACCOUNT_DEACTIVATED` (already covered — regression guard only); wrong password / unknown email → identical `KindUnauthenticated` error (no leak); every failure path calls `audit.Append` with `action: "login.fail"` exactly once (fake `AuditRepository` records call count + action string); success path unaffected.
- `auth_routes_test.go`: deactivated-account gRPC error → HTTP `403 {"error":"account_inactive"}`; wrong-password/unknown-email → `401 {"error":"invalid_credentials"}`; 11th request within a rolling minute from the same simulated IP → `429 {"error":"too_many_attempts"}` without ever calling `authClient.Login` (assert the fake client records ≤10 calls); two different `X-Forwarded-For` IPs each get their own independent budget (mirrors `TestRateLimiter_TracksTenantsIndependently`'s existing per-key isolation test at `rate_limit_test.go`).
- `clientIP` unit test: `X-Forwarded-For: 1.2.3.4, 10.0.0.1` → `"1.2.3.4"`; no header → falls back to `RemoteAddr`'s host.
- Regression: `TestAuthRoutes_LoginSetsSessionCookie` (or equivalent existing test) still passes with the new `Ip`/`UserAgent` fields on the request — assert the fake `AuthServiceClient` receives non-empty `Ip` when `X-Forwarded-For` is set.

## References

- `backend-go/services/auth-service/internal/usecase/login.go:52-99` — current `Execute`/`appendAuditBestEffort`
- `backend-go/services/auth-service/internal/adapter/grpc/server.go:95-101` — `Login` RPC handler, `apperrors.ToGRPCStatus`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go:1-149` — `mountAuthRoutes`, `/auth/local` handler, `setSessionCookie`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:100,118` — `mountAuthRoutes` call site, `authed` group's `rateLimitMiddleware` wiring
- `backend-go/services/api-gateway/internal/usecase/rate_limit.go:17-64` — `RateLimiter`/`Allow`/`NewRateLimiter`, the type this solution reuses keyed by IP
- `backend-go/services/api-gateway/internal/usecase/ports.go:20-28` — `RateLimitStore`, the future Redis-backed extension point this doesn't need yet
- `backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-84` — `authMiddleware`/`rateLimitMiddleware`, the per-tenant pattern this mirrors
- `specs/backend-go/tdd/services/auth-service.md:64-127` (§3 RPC surface, no IP/UA fields yet), `:313-322` (§9 "Brute-force mitigation" — the OPA/`access_policies`-governed target this solution flags as future work)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:19-22` (api-gateway is the only externally-reachable service), `:63-66` (never trust request-body-asserted identity — the principle this solution's IP/UA server-side-resolution follows)
- `specs/backend-go/bugs/logic-v1/BUG-AUTH-05-audit-log-partial.md` — the `Metadata`/`ip_address` gap this solution's `login.fail` write is provisional against
