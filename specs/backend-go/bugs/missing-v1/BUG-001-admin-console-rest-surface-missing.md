# BUG-001: `/admin/api/*` admin console REST surface does not exist in backend-go

**Service:** `api-gateway` (would proxy to `auth-service`, possibly others)
**File:** `internal/adapter/httpgateway/router.go` (missing entirely — no `mountAdminRoutes`)
**Severity:** High — the entire admin console UI (`frontend/src/renderer/src/components/admin/*`) is non-functional against backend-go
**Symptom:** Every `adminFetch()` call from the frontend's admin console 404s (chi's default `NotFound`, not even a structured `501` body) — there is no route registered under `/admin/api/*` at all
**Status:** ❌ Open

---

## Description

`specs/frontend/api/http-endpoints.md` documents 11 routes under `/admin/api/*`
that `frontend/src/renderer/src/components/admin/admin-api-client.ts`'s
`adminFetch()` calls (always `credentials: 'include'`, same `orca_session`
cookie as the main app). None of them exist in `backend-go`.

`router.go`'s `NewRouter` only mounts three unauthenticated groups
(`/auth/*` via `mountAuthRoutes`, `GET /ws`, `GET /agent` +
`/api/agent-token`) and one authenticated group under `authMiddleware`,
which only ever registers `/v1/*` prefixes (`mountUsageRoutes`,
`mountAuthAdminRoutes`, `mountAnnotationRoutes`, …, `mountStubRoutes`). No
call to anything mounting `/admin/api` exists anywhere in
`services/api-gateway`:

```
$ grep -rn "admin/api" backend-go/services --include="*.go"
(no matches)
```

There **is** a superficially-related admin surface — `mountAuthAdminRoutes`
(`internal/adapter/httpgateway/auth_admin_routes.go:28`) registers user/session/
audit-log management, but under a completely different prefix and shape
(`/v1/auth/*`, not `/admin/api/*`), with narrower method coverage than the
frontend admin console needs (see table below). It does not help
`adminFetch()` calls, which hit the literal `/admin/api/...` paths — a
different prefix is a 404, not "close enough."

---

## Missing / mismatched routes

| Frontend expects (`http-endpoints.md`) | Caller | backend-go today |
|---|---|---|
| `GET /admin/api/stats` | `AdminApp` dashboard | **Nothing.** No stats/summary RPC exists on any service's proto that this could proxy to either. |
| `GET /admin/api/users` | `UsersPage` | Different shape exists at `GET /v1/auth/users` (`auth_admin_routes.go:31`, → `auth-service.ListUsers`) — right data, wrong path. |
| `POST /admin/api/users` | `UserForm` (create) | Different shape exists at `POST /v1/auth/users` (`auth_admin_routes.go:30`, → `auth-service.CreateUser`) — wrong path. |
| `PATCH /admin/api/users/:id` | `UserForm` (edit) | **Narrower substitute** at `PUT /v1/auth/users/{id}/role` (`auth_admin_routes.go:32`) — only updates `role`; `auth-service`'s `UpdateUserRole` RPC has no path to change `email`/`name`. Wrong path, wrong method, and the underlying RPC can't do a full user edit even if the route existed. |
| `DELETE /admin/api/users/:id` | `UsersPage` (Deactivate) | **No equivalent at all.** No soft-delete/deactivate RPC found on `auth-service`'s proto (`proto/orca/auth/v1/`) — this is a real gap in `auth-service`, not just a missing route. |
| `GET /admin/api/sessions` | `SessionsPage` | **No "list all sessions" RPC/route.** `auth-service` only exposes single-session revoke (see below); nothing lists active sessions across users. |
| `DELETE /admin/api/sessions/:sessionId` | `SessionsPage` (kill session) | Different shape exists at `POST /v1/auth/sessions/{id}/revoke` (`auth_admin_routes.go:33`) — wrong path and wrong HTTP method (`POST` vs `DELETE`). |
| `GET /admin/api/policies` | `PoliciesPage` | **No policy CRUD route or RPC anywhere.** `docs/execution-plan.md`'s Epic E (OPA policy bundle) is marked done, but that's the *authorization* policy bundle consumed internally by services — it has no admin-console-facing list/create/update/delete surface. |
| `POST /admin/api/policies` | `PoliciesPage` (create) | Same — missing. |
| `PUT /admin/api/policies/:id` | `PoliciesPage` (edit) | Same — missing. |
| `DELETE /admin/api/policies/:id` | `PoliciesPage` (delete) | Same — missing. |
| `GET /admin/api/audit` | `AuditPage` | Different shape exists at `GET /v1/auth/audit-log` (`auth_admin_routes.go:34`, → `auth-service.QueryAuditLog`) — right data, wrong path. |
| `DELETE /admin/api/users/:userId/sessions` (kill-all-for-user) | no frontend caller found even in the old spec | **No equivalent.** Also absent from `auth-service`'s proto. |

Summary: **0 of 12** documented `/admin/api/*` routes exist verbatim. 4 have
a same-data-different-shape substitute already reachable at `/v1/auth/*`
(users list/create, sessions revoke, audit log); the rest (`stats`,
deactivate-user, list-sessions, all of `policies`, kill-all-sessions) have
**no backing RPC on any service today**, not just a missing route.

---

## Why the `/v1/auth/*` substitute doesn't fix this

`mountAuthAdminRoutes` was built for a *different* consumer (a REST-first
admin API, per its own doc comment) — it was not written against
`http-endpoints.md`'s contract at all. Even for the 4 routes with a
same-data substitute, `admin-api-client.ts` calls the literal
`/admin/api/...` path and expects `requireAdmin`-gated, `/admin/api`-rooted
responses; it does not know about `/v1/auth/*`. Either:

1. Add real `/admin/api/*` routes (new `mountAdminRoutes` in
   `httpgateway`, admin-authorization-gated the same way
   `auth-service.requireAdminActor` gates `/v1/auth/*` today) that proxy to
   the same `auth-service` RPCs `mountAuthAdminRoutes` already calls for
   the 4 overlapping ones, plus new RPCs (`auth-service` and/or a new
   policy-console surface) for `stats`/deactivate/list-sessions/`policies`/
   kill-all-sessions; or
2. Update the frontend's `admin-api-client.ts` to call `/v1/auth/*` instead
   (out of scope for this backend-only audit, and doesn't solve the RPCs
   that don't exist at all yet either way).

---

## References

- `specs/frontend/api/http-endpoints.md` — `## Admin console (/admin/api/*)`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` — `NewRouter` (no admin mount)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go` — `mountAuthAdminRoutes` (the near-miss substitute)
- `backend-go/services/api-gateway/internal/domain/registry.go` — `NewDefaultServiceRegistry` (confirms `/v1/auth` is `RouteWired`, nothing under `/admin`)
- `backend-go/proto/orca/auth/v1/` — `auth-service`'s proto surface (no deactivate-user, no list-sessions, no policy CRUD RPCs)
