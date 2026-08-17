# HTTP Endpoints

Plain `fetch()`/`adminFetch()` calls — session cookies, not the RPC protocol.
All confirmed present on the backend (no gaps found in this surface).

## Auth (`/auth/*`)

Backend: `backend/src/main/auth/auth-router.ts`, mounted directly on the app
(not under `/admin/api`). Session identity is an `orca_session` HttpOnly
cookie, not a bearer token.

| Method | Path | Frontend caller | Purpose |
|---|---|---|---|
| `GET` | `/auth/config` | `frontend/src/renderer/src/web/main.tsx` | Boot-time probe: 200 with `{providers: [...]}` (currently always `[]` — SSO is a stub) means multi-user web mode; 404 means fall back to the legacy pair-code app. |
| `POST` | `/auth/local` | `frontend/src/renderer/src/web/login/LoginPage.tsx` (`LoginForm`) | Email+password login. Sets the session cookie on success. |
| `POST` | `/auth/logout` | web app logout action | Revokes the session, clears the cookie. |
| `GET` | `/auth/me` | web app bootstrap (`requireAuth`-gated) | Current session's user identity. |
| `GET` | `/auth/sso/:provider` | `SsoButton` (`LoginPage.tsx`) | SSO login kick-off — **stub**: backend returns `501` unconditionally today (see `docs/ui/pages/login.md`). |

## Admin console (`/admin/api/*`)

Backend: `backend/src/main/admin/admin-router.ts`, mounted at `/admin/api` in
`server/http-server.ts`, gated by `requireAdmin` middleware on every route
(`router.use(requireAdmin)`). Frontend: `frontend/src/renderer/src/components/
admin/admin-api-client.ts`'s `adminFetch()` (always `credentials: 'include'`,
same `orca_session` cookie as the main app — see `docs/ui/pages/admin-*.md` for
the client-side auth-guard gap this doesn't fix).

| Method | Path | Frontend caller | Purpose |
|---|---|---|---|
| `GET` | `/admin/api/stats` | `AdminApp` dashboard | Summary counts for the admin landing view. |
| `GET` | `/admin/api/users` | `UsersPage` | List all user accounts. |
| `POST` | `/admin/api/users` | `UserForm` (create mode) | Create a user account. |
| `PATCH` | `/admin/api/users/:id` | `UserForm` (edit mode) | Update a user account. |
| `DELETE` | `/admin/api/users/:id` | `UsersPage` (Deactivate) | Soft-delete (`is_active = 0`) — see `docs/ui/pages/admin-users.md` for why there's no matching "reactivate" UI action despite the field supporting it. |
| `GET` | `/admin/api/sessions` | `SessionsPage` | List active sessions across all users. |
| `DELETE` | `/admin/api/sessions/:sessionId` | `SessionsPage` (kill session) | Force-revoke one session. |
| `GET` | `/admin/api/policies` | `PoliciesPage` | List access/rate policies. |
| `POST` | `/admin/api/policies` | `PoliciesPage` (create) | Create a policy. |
| `PUT` | `/admin/api/policies/:id` | `PoliciesPage` (edit) | Update a policy. |
| `DELETE` | `/admin/api/policies/:id` | `PoliciesPage` (delete) | Remove a policy. |
| `GET` | `/admin/api/audit` | `AuditPage` | Audit log entries, optional query-string filters. |

Backend also registers `DELETE /admin/api/users/:userId/sessions` (kill every
session for one user) — no frontend caller found for it in this pass; either
dead backend capability or reached through a path this grep missed.

## Web push (`/api/push-*`)

Backend: `backend/src/server/push-api-routes.ts` (hand-rolled, not an Express
router — matches `req.method`/`url` directly). No `requireAdmin`/session gate
noted at this layer; check the handler if that matters for your use case.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/vapid-public-key` | Fetch the VAPID public key needed to create a browser `PushSubscription`. |
| `POST` | `/api/push-subscribe` | Register a browser's push subscription. |
| `POST` | `/api/push-unsubscribe` | Remove a push subscription. |
