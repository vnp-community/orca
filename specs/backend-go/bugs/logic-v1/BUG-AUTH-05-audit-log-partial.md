# BUG-AUTH-05: Audit log captures actions, but with a reduced schema (no metadata/IP) and no export

**Business Logic:** [BL-AUTH-05](../../../../docs/logic/auth/BL-AUTH-05-audit-log.md) — Audit Log Ghi nhận Action
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** An admin viewing `GET /admin/api/audit` sees who did what and when, but never sees the IP address a login came from, the `{from, to}` of a role change, or any other contextual metadata the spec requires — the `metadata` column doesn't exist. There is also no `login.fail` entry when someone mistypes their password, and no CSV export button will ever work because `GET /admin/api/audit/export` isn't implemented.

---

## Spec summary

Every security-relevant action (`login.success`, `login.fail`, `user.create`, `user.deactivate`, `user.role_change`, `session.kill`, `ssh.connect`) is appended to an immutable `orca_audit_log` table with `actor_id`, `action`, `target_type`, `target_id`, `metadata` (JSON), `ip_address`, `created_at`. Admin can query/filter it via `GET /admin/api/audit` and export it as CSV.

## What backend-go has

- Real, append-only storage: `AuditRepository.Append`/`Query` (referenced by every usecase below), backed by `audit_repository.go` (`backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go`). `domain.AuditEntry` (`backend-go/services/auth-service/internal/domain/audit.go:23-56`) has no update/delete method — its doc comment states this explicitly ("there is deliberately no usecase method that updates or deletes an AuditEntry, only Append and Query").
- Several of the spec's events are genuinely written, just under backend-go's own action-name convention (not a bug — internal naming, not a documented external contract):
  - Login → `user.login` (`backend-go/services/auth-service/internal/usecase/login.go:94`)
  - Logout → `user.logout` (`backend-go/services/auth-service/internal/usecase/logout.go:45`)
  - Create user → `user.created` (`backend-go/services/auth-service/internal/usecase/create_user.go:77`)
  - Deactivate → `user.deactivated` (`backend-go/services/auth-service/internal/usecase/deactivate_user.go:49`)
  - Reactivate → `user.reactivated` (`backend-go/services/auth-service/internal/usecase/reactivate_user.go:47`) — not in the spec's event table at all, an addition
  - Role change → `user.role_updated` (`backend-go/services/auth-service/internal/usecase/update_user_role.go:43`) — matches spec's `user.role_change` in intent
  - Revoke one session → `session.revoked` (`backend-go/services/auth-service/internal/usecase/revoke_session.go:50`)
  - Revoke all sessions for a user → `session.force_revoke_all` (`backend-go/services/auth-service/internal/usecase/force_revoke_all_sessions.go:47`)
  - First-run bootstrap → `user.bootstrap_created` (`backend-go/services/auth-service/internal/usecase/bootstrap.go:90`) — not in spec, an addition
- Query API: `QueryAuditLog.Execute` (`backend-go/services/auth-service/internal/usecase/query_audit_log.go:36-51`), admin-gated via `requireAdminActor`, paginated (default/cap 50/200), exposed at both `GET /v1/auth/audit-log` (`backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:147-184`) and the spec's literal `GET /admin/api/audit` (`backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:41`), supporting `since`/`page_token`/`page_size` filters.

## What's missing

- **`AuditEntry` has no `metadata` field.** `domain.AuditEntry` (`backend-go/services/auth-service/internal/domain/audit.go:23-30`) is `{ID, TenantID, ActorID, Action, Target, OccurredAt}` — no JSON metadata blob. The spec's per-event metadata (`{ip, userAgent}` for logins, `{from, to}` for role changes, `{targetEmail, role}` for user creation, etc.) is entirely absent; every entry only records a single opaque `Target` string.
- **No `ip_address` column/field anywhere.** Confirmed no login/logout/admin-action path captures a request IP (see BUG-AUTH-01's finding that the gRPC `Login` method never reads peer/metadata for this). The spec's audit schema explicitly calls out `ip_address` as a top-level column, not just embedded in `metadata`.
- **`target_type` and `target_id` are collapsed into a single `Target` field.** The spec's schema separates `target_type` (`"user"`, `"session"`, `"ssh_host"`) from `target_id`; `domain.AuditEntry` only has `Target string` (`audit.go:26`), so a query/filter UI can't distinguish "target was a user" from "target was a session" without parsing the action name itself.
- **No `login.fail` entries** — see BUG-AUTH-01; `Login.Execute` never audits a failed attempt.
- **No `ssh.connect` audit event** — `grep -rn "ssh.connect" backend-go/services/auth-service backend-go/services/api-gateway` returns no matches; nothing in `scm_routes.go` or any SSH-connection usecase appends an audit entry for this event.
- **No CSV export endpoint.** `grep -rn "export\|csv\|CSV" backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go backend-go/services/auth-service/internal/usecase/query_audit_log.go` returns no matches; `GET /admin/api/audit/export?format=csv` does not exist in `admin_routes.go` either (its route table at `admin_routes.go:27-43` has no `/audit/export`).
- **Filtering is narrower than the spec's example** (`?action=login.fail&userId=xxx&from=...&to=...`) — `QueryAuditLogRequest` only supports `tenant_id`/`since`/`page_token`/`page_size` (`backend-go/proto/orca/auth/v1/auth.proto` `message QueryAuditLogRequest`); there is no `action`, `userId`/`actor_id`, or `to` (upper-bound date) filter.

## See also

- BUG-AUTH-01 (local login) — the missing IP/User-Agent capture and missing `login.fail` audit entry are rooted in the same login-path gap and are not re-detailed here.

## References

- `backend-go/services/auth-service/internal/domain/audit.go:23-56`
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go`
- `backend-go/services/auth-service/internal/usecase/query_audit_log.go:11-51`
- `backend-go/services/auth-service/internal/usecase/login.go:93-99`, `logout.go:45`, `create_user.go:77`, `deactivate_user.go:49`, `reactivate_user.go:47`, `update_user_role.go:43`, `revoke_session.go:50`, `force_revoke_all_sessions.go:47`, `bootstrap.go:90`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:147-184`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go:27-43`
- `backend-go/proto/orca/auth/v1/auth.proto` — `message AuditEntry`, `message QueryAuditLogRequest`
- `docs/logic/auth/BL-AUTH-05-audit-log.md`
