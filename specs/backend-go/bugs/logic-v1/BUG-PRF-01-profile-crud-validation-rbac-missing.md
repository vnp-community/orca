# BUG-PRF-01: Profile CRUD wiring is complete but RBAC, field validation, and audit logging are missing

**Business Logic:** [BL-PRF-01](../../../../docs/logic/profile/BL-PRF-01-profile-crud.md) — Tạo và Cập nhật Profile (Company/Department/User)
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** Any authenticated user (not just an Admin) can call `profile.updateCompany` or `profile.updateDept` and it succeeds — there is no server-side check that the caller holds the `admin`/`lead` role. A Department admin can also silently set `security.*` fields on a department profile: the write succeeds (no 400), and the value is merely discarded later at resolve time instead of being rejected up front as the spec's error table requires. Company `approved_models`/`session_timeout_hours` accept any value, including out-of-range numbers or unsupported model names — no 400 is ever returned.

---

## Spec summary

Admin/Lead/User can create and update the three profile layers (Company/Department/User). Company edits require `role === 'admin'`; Department edits require `role === 'admin'` OR (`role === 'lead'` AND the lead's own department); User edits are self-service. Writes must reject `security.*` at the Department/User layer (400 "Security settings can only be set at company level"), validate `approved_models ⊆ SUPPORTED_MODELS` and `session_timeout_hours ∈ [1, 168]` at the Company layer, enforce department-name uniqueness within a company, and emit `audit_log(...)` events for company/department profile changes (user personal-pref updates are explicitly exempt from audit per the spec).

## What backend-go has

End-to-end wiring for all 6 `profile.*` methods is real and live, contradicting the earlier `specs/backend-go/bugs/missing-v1/BUG-019` finding (see "See also" below — that report is now stale):

- wscompat: `registerProfileChannels` registers all 6 methods — `profile.getResolved`, `profile.getUserProfile`, `profile.listDepts`, `profile.updateCompany`, `profile.updateDept`, `profile.updateUser` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:51-166`), and is actually invoked from `RegisterRealChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:125`) — not merely defined-but-unwired as that file's own stale doc comment (`channels_tenant_project.go:9-11`) still claims.
- gRPC: `tenantv1.TenantServiceClient` has all matching RPCs (`CreateCompany`, `CreateDepartment`, `UpdateCompany`, `UpdateDepartment`, `UpdateUserProfile`, `GetUserProfile`, `ListDepartments`) implemented in `backend-go/services/tenant-service/internal/adapter/grpc/server.go:85-274` and wired in `backend-go/services/tenant-service/cmd/server/main.go:132-138`.
- Usecases persist for real against Postgres: `backend-go/services/tenant-service/internal/usecase/update_company.go`, `update_department.go`, `update_user_profile.go`, `create_department.go`, `set_user_department.go`.
- Cache invalidation scoping matches the spec's table exactly: `UpdateCompany` invalidates every user in the company (`update_company.go:39-48`), `UpdateDepartment` invalidates every user in that department (`update_department.go:47-56`), `UpdateUserProfile`/`SetUserDepartment` invalidate only the one user (`update_user_profile.go:65-71`, `set_user_department.go:69-76`) — plus a cross-replica NATS `PublishProfileInvalidated` broadcast the spec doesn't even ask for.

## What's missing

- **No RBAC/role enforcement anywhere in the chain.** `tenant-service` has no `internal/adapter/opaclient/` and no `internal/usecase/authorization.go` (confirmed absent — compare to `project-service`'s `internal/usecase/authorization.go:1-84`, which does gate `RebindDevServer` via OPA). `UpdateCompany`/`UpdateDepartment`/`UpdateUserProfile`'s usecases (`update_company.go:30`, `update_department.go:34`, `update_user_profile.go:37`) take no role/actor-permission input at all — any caller who can reach the RPC can update any company or department's profile. `channels_tenant_project.go:99-142`'s wscompat handlers likewise perform no role check before calling the gRPC client.
- **No `security.*` rejection at write time.** The domain only discards non-company `security` values during *resolution* (`profile_resolution.go:171-186`'s `lockSecurity`) — there is no equivalent check in `UpdateDepartment`/`UpdateUserProfile` that returns a 400 when the caller's patch includes a `security` key, as the spec's Error Cases table requires ("Dept setting security field → 400").
- **No Company-layer field validation.** No check anywhere in `tenant-service` that `approved_models ⊆ SUPPORTED_MODELS` or `session_timeout_hours ∈ [1, 168]` — confirmed via `grep -rn "approvedModel\|allowedServerTags"` returning zero hits in `backend-go/services/tenant-service/`. `UpdateCompany`/`CreateCompany`'s only validation is `apperrors` for missing id/name (`domain/company.go:29-37`).
- **No department-name-uniqueness check.** `CreateDepartment` (`create_department.go:32-52`) only checks the parent company exists; it never queries for an existing department with the same name within that company.
- **No audit logging.** `grep -rln "audit_log\|AuditLog"` over `backend-go/services/tenant-service/internal/` returns zero non-test matches — none of `company.profile.updated`, `department.created`, `department.profile.updated` are ever emitted.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-019-profile-channels-not-implemented.md` — now stale: its "5 of 6 `profile.*` methods have no backing RPC" finding no longer holds; all 6 RPCs and wscompat wrappers exist and are wired as of this audit.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:38-166` — `registerTenantProjectChannels`/`registerProfileChannels`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:123-125` — call sites proving both `registerTeamChannels` and `registerTenantProjectChannels` are live in `RegisterRealChannels`
- `backend-go/services/tenant-service/internal/adapter/grpc/server.go:85-274` — all `TenantService` RPC handlers
- `backend-go/services/tenant-service/internal/usecase/update_company.go`, `update_department.go`, `update_user_profile.go`, `create_department.go` — no role/patch-content validation
- `backend-go/services/tenant-service/internal/domain/company.go:29-37`, `department.go:16-29`, `user_profile.go:20-33` — full validation surface (id/name non-empty only)
- `backend-go/services/tenant-service/internal/domain/profile_resolution.go:171-186` — `lockSecurity` (resolve-time discard only, not a write-time rejection)
- `backend-go/services/project-service/internal/usecase/authorization.go:1-84` — the OPA-gated pattern `tenant-service` lacks an equivalent of
- `docs/logic/profile/BL-PRF-01-profile-crud.md:101-124` — Validation Rules / Error Cases tables
