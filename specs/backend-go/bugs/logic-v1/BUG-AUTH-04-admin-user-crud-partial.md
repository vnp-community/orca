# BUG-AUTH-04: Admin console REST surface exists, but admin-created users can never log in and cross-user session listing is stubbed out

**Business Logic:** [BL-AUTH-04](../../../../docs/logic/auth/BL-AUTH-04-admin-user-crud.md) — Admin User CRUD & Session Kill
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** An admin who creates a new user via `POST /admin/api/users` gets back a `201` with a user record, but that account is permanently unusable — the password is a random 24-character string generated server-side, hashed, and then discarded forever; there is no invite/reset-link flow to hand the real user a working credential, so the new account can never actually log in. Separately, `GET /admin/api/sessions` (the "active sessions across all users" dashboard view the spec documents) 400s unless the caller passes `?user_id=`, because the backing RPC can only list one user's sessions at a time.

---

## Spec summary

Admin creates/lists/updates/deactivates users and kills sessions via `/admin/api/*`. `POST /admin/api/users` takes `{email, name, role, password}` and creates a working account. Deactivating a user also kicks all their sessions and kills their process. `GET /admin/api/sessions` lists active sessions across all users (`userId, last_seen_at, IP`). A first-run bootstrap auto-creates an `admin@localhost` account with a generated password printed to stdout.

## What backend-go has

The literal `/admin/api/*` surface the spec documents **does exist** — `mountAdminRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:27-43`) registers all of: `GET/POST /admin/api/users`, `GET /admin/api/stats`, `PATCH /admin/api/users/{id}`, `DELETE /admin/api/users/{id}`, `GET /admin/api/sessions`, `DELETE /admin/api/sessions/{sessionId}`, `DELETE /admin/api/users/{userId}/sessions`, and full `/admin/api/policies` CRUD — mounted behind `authMiddleware` + `rateLimitMiddleware` (`backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:118-130`). Each handler proxies to a real `auth-service` RPC with server-side admin enforcement via `requireAdminActor` (`backend-go/services/auth-service/internal/usecase/authorization.go:19`), gRPC `PermissionDenied` mapped to HTTP 403 by `writeGRPCError`. This supersedes the older finding in `specs/backend-go/bugs/missing-v1/BUG-001-admin-console-rest-surface-missing.md`, which reported 0/12 routes existing — that gap has since been substantially closed; see "See also" below for what's still genuinely missing.

- **Create user**: `handleCreateUser` (`backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:48-71`, reused by `admin_routes.go:31`) → `CreateUser.Execute` (`backend-go/services/auth-service/internal/usecase/create_user.go:37-82`) → audit entry `user.created` (`create_user.go:77`).
- **Deactivate**: `handleDeactivateUser` (`admin_routes.go:62-75`) → `DeactivateUser.Execute` (`backend-go/services/auth-service/internal/usecase/deactivate_user.go:28-53`), sets `is_active=false`, audit entry `user.deactivated` (`deactivate_user.go:49`).
- **Reactivate** (RPC exists, no REST wired — see below): `ReactivateUser.Execute` (`backend-go/services/auth-service/internal/usecase/reactivate_user.go:26-51`).
- **Kill one session**: `handleRevokeSession` (`auth_admin_routes.go:132-145`, reused by `admin_routes.go:35`) → `RevokeSession.Execute` (`backend-go/services/auth-service/internal/usecase/revoke_session.go:27-54`), audit entry `session.revoked`.
- **Kill all sessions for a user**: `handleForceRevokeAllSessions` (`admin_routes.go:106-119`) → `ForceRevokeAllSessionsForUser.Execute` (`backend-go/services/auth-service/internal/usecase/force_revoke_all_sessions.go:27-51`), audit entry `session.force_revoke_all`.
- **Stats**: `handleAdminStats` (`admin_routes.go:45-56`) → `GetAdminStats.Execute` (`backend-go/services/auth-service/internal/usecase/get_admin_stats.go:26-49`) — total users, active sessions, total policies.
- **First-run bootstrap**: `Bootstrap.EnsureAdmin` (`backend-go/services/auth-service/internal/usecase/bootstrap.go:52-100`) — runs once at service startup (not via RPC, intentionally not client-reachable per its own doc comment), auto-generates a 16-char password when none is configured and returns it for `main.go` to log once (`bootstrap.go:96-98`), matching the spec's "print to stdout, never stored" contract.
- **"Kicks the session" on deactivate — functionally equivalent, even though the spec's literal mechanism isn't used**: `DeactivateUser.Execute` never deletes/revokes the user's `auth.sessions` rows, but `ValidateSession.Execute` re-checks `user.IsActive` on every single request (`backend-go/services/auth-service/internal/usecase/validate_session.go:49-50`) — so a deactivated user is locked out on their very next request regardless of a still-valid session row. The net security effect (immediate lockout) is preserved; only the spec's literal "DELETE FROM sessions" step is skipped.

## What's missing

- **No password/invite path for admin-created users — the account is unusable.** `CreateUserRequest` (`backend-go/proto/orca/auth/v1/auth.proto` `message CreateUserRequest { email; name; tenant_id; role; }`) has no password field at all. `CreateUser.Execute` generates a random 24-char password, hashes it, and never returns or stores the plaintext anywhere (`create_user.go:47-61`, its own doc comment: "there is no invite/reset-link flow implemented in this scaffold to hand a chosen password to the new user"). `grep -rli "reset.password\|invite\|forgot.password\|password.reset" backend-go/services/auth-service backend-go/services/api-gateway` returns zero matches — confirming there is genuinely no follow-up flow anywhere. Every admin-created user is permanently locked out until this is built.
- **`GET /admin/api/sessions` cannot list sessions across users**, contradicting the spec's dashboard requirement. `handleListAllSessions` (`admin_routes.go:86-104`) requires a `?user_id=` query param and 400s without one (`admin_routes.go:90-93`), because it just proxies to `ListSessionsForUser` (`backend-go/services/auth-service/internal/usecase/list_sessions_for_user.go:23-36`) — `auth-service`'s proto has no cross-user `ListAllSessions`/`ListSessions` RPC (confirmed: `backend-go/proto/orca/auth/v1/auth.proto`'s `AuthService` service block lists only `ListSessionsForUser`, no unscoped variant). The handler's own doc comment (`admin_routes.go:77-85`) acknowledges this is a known, deliberate stopgap.
- **Session rows aren't physically cleaned up on deactivate**, even though the user is functionally locked out (see "What backend-go has" above) — an admin looking at `ListSessionsForUser` for a deactivated user will still see their old sessions listed as unrevoked/unexpired, which is misleading for the audit/observability use case the spec's dashboard implies. Low severity given the actual security enforcement is intact.
- **`PATCH /admin/api/users/:id` is role-only**, not a full edit. `handleUpdateUserRole` (`auth_admin_routes.go:108-130`, reused at `admin_routes.go:32`) only accepts `{role}`; there is no RPC or route path to change a user's `email`/`name`/`is_active` via a single PATCH the way the spec's `Body: { is_active: false }` example implies (deactivation is instead a separate `DELETE`, which does match the spec's actual step-by-step flow — just not its literal `PATCH` example).

## See also

- `specs/backend-go/bugs/missing-v1/BUG-001-admin-console-rest-surface-missing.md` — **stale**: reported 0/12 `/admin/api/*` routes existing; `admin_routes.go` (added since) now wires 12/12 documented paths. Its `stats`/`policies` findings are resolved; only the cross-user sessions-listing gap it flagged survives, in the narrower form described above.
- `specs/backend-go/bugs/missing-v1/BUG-002-auth-sso-route-missing.md` — unrelated to this BL; also stale (the SSO stub route is wired, see BUG-AUTH-01's "See also").

## References

- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:27-222`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:28-195`
- `backend-go/services/auth-service/internal/usecase/create_user.go:13-82`
- `backend-go/services/auth-service/internal/usecase/deactivate_user.go:13-53`
- `backend-go/services/auth-service/internal/usecase/reactivate_user.go:13-51`
- `backend-go/services/auth-service/internal/usecase/revoke_session.go:13-54`
- `backend-go/services/auth-service/internal/usecase/force_revoke_all_sessions.go:12-51`
- `backend-go/services/auth-service/internal/usecase/list_sessions_for_user.go:10-36`
- `backend-go/services/auth-service/internal/usecase/get_admin_stats.go:10-49`
- `backend-go/services/auth-service/internal/usecase/bootstrap.go:13-100`
- `backend-go/services/auth-service/internal/usecase/validate_session.go:31-54`
- `backend-go/proto/orca/auth/v1/auth.proto` — `AuthService` RPC list, `CreateUserRequest`
- `docs/logic/auth/BL-AUTH-04-admin-user-crud.md`
