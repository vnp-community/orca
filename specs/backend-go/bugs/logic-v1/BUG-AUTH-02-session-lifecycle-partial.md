# BUG-AUTH-02: Session CRUD is real, but RENEW/EXPIRE lifecycle stages and per-process routing don't exist

**Business Logic:** [BL-AUTH-02](../../../../docs/logic/auth/BL-AUTH-02-session-management.md) — Session Management & Isolation
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A session's `last_seen_at` never advances (there's no such column), so nothing in the admin console can show "when was this session last active," and expired/revoked session rows are never cleaned up — they just accumulate in `auth.sessions` forever. The WebSocket path validates identity correctly, but it never routes a connection to a dedicated per-user backend process, because no such process exists (see BUG-AUTH-03).

---

## Spec summary

Sessions go through CREATE → VALIDATE → RENEW (sliding `last_seen_at`) → REVOKE → EXPIRE (background cleanup). `requireAuth()` middleware validates the cookie on every HTTP/WS request and injects `userId`/`userRole`. A WS upgrade additionally routes the connection to a per-user forked child process over a Unix socket.

## What backend-go has

- **CREATE**: `Login.Execute` (`backend-go/services/auth-service/internal/usecase/login.go:76-82`) inserts a `domain.Session` via `Repository.CreateSession` (`backend-go/services/auth-service/internal/adapter/postgres/session_repository.go:15-24`).
- **VALIDATE**: `ValidateSession.Execute` (`backend-go/services/auth-service/internal/usecase/validate_session.go:31-54`) looks up by SHA-256 token hash, checks `session.IsValid(now)` (not revoked, not expired — `backend-go/services/auth-service/internal/domain/session.go:67-72`) and `user.IsActive`.
- **REVOKE**: `Logout.Execute` (self-revoke, `backend-go/services/auth-service/internal/usecase/logout.go:26-49`) and `RevokeSession.Execute` (admin force-revoke, `backend-go/services/auth-service/internal/usecase/revoke_session.go:27-54`) both call `Repository.RevokeSession` (`session_repository.go:44-56`), which sets `revoked_at`.
- **HTTP middleware equivalent**: `authMiddleware` (`backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:50-64`) validates the `orca_session` cookie via `CookieSessionValidator.ValidateCookie` and injects `usecase.Identity{TenantID, UserID}` into the request context on every `/v1/*` route.
- **WS identity injection**: the same `ValidateCookie` path is used for the WS upgrade (`backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:54-55`, wired at `router.go:108-109`) — a WS connection does get a validated `userId`/`tenantId` before any channel handler runs, matching the spec's "inject userId into every WS connection" intent.

## What's missing

- **RENEW never happens.** `domain.Session` (`backend-go/services/auth-service/internal/domain/session.go:29-36`) has no `LastSeenAt` field, `auth.sessions` (per `session_repository.go`'s column list) has no `last_seen_at` column, and no usecase ever updates one. `ValidateSession.Execute` is a pure read (`validate_session.go:31-54`) — it never writes back a "touched" timestamp. This also means the admin-console session list (`ListSessionsForUser`) can never show a real `last_seen_at`, even though the proto's `Session` message declares the field (`backend-go/proto/orca/auth/v1/auth.proto` `message Session { ... last_seen_at = 5; ... }`) — it's simply never populated (`backend-go/services/auth-service/internal/domain/session.go` has no such field to populate from).
- **No IP/User-Agent tracked on a session**, despite the `Session` proto message declaring both (`ip = 6`, `user_agent = 7`) — `domain.Session` has neither field, and nothing in the login path captures them (see BUG-AUTH-01).
- **EXPIRE background cleanup job does not exist.** `grep -rli "reaper\|DeleteExpired\|PurgeExpired" backend-go/services/auth-service` matches only doc comments (`domain/audit.go`, `usecase/ports.go`), not an actual implementation; `auth-service/cmd/server/main.go` starts only the gRPC server, no background ticker/cron. Expired/revoked rows are never deleted from `auth.sessions` — they accumulate indefinitely. This is low functional impact (expiry is still correctly enforced at read-time by `Session.IsValid`), but it is a real, unaddressed spec item and a long-term storage/observability issue (an admin's `ListSessionsForUser` view will show years of dead rows).
- **No per-user process routing.** The spec's "WsSessionRouter.route(userId, ws) → SessionManager.getOrCreate(userId) → fork process → pipe WS↔Unix socket" step has no backend-go equivalent at all — see BUG-AUTH-03 for the full accounting. A validated WS connection in backend-go talks directly to stateless, shared gRPC services; there is no per-user child process to route to.

## See also

- BUG-AUTH-03 (per-user process sandbox) — the "Per-User Process Isolation" table in this same spec doc overlaps entirely with BL-AUTH-03; not re-litigated in detail here.

## References

- `backend-go/services/auth-service/internal/domain/session.go:29-72`
- `backend-go/services/auth-service/internal/usecase/login.go:76-82`
- `backend-go/services/auth-service/internal/usecase/validate_session.go:31-54`
- `backend-go/services/auth-service/internal/usecase/logout.go:26-49`
- `backend-go/services/auth-service/internal/usecase/revoke_session.go:27-54`
- `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go:15-113`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/middleware.go:11-64`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:108-109`
- `backend-go/proto/orca/auth/v1/auth.proto` — `message Session`
- `docs/logic/auth/BL-AUTH-02-session-management.md`
