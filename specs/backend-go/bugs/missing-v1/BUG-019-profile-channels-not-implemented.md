# BUG-019: `profile.*` channels not implemented in backend-go

**Service:** `tenant-service` (best fit — owns companies/departments/user profiles per `proto/orca/tenant/v1/tenant.proto:7`'s doc comment; `auth-service` checked too and does not fit better, see below)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** High — `WorkspaceContext.tsx` calls `profile.getResolved` on bootstrap; this likely blocks basic app load
**Symptom:** Every `profile.*` call from `WorkspaceContext.tsx`, `useProfile.ts`, `CompanyProfileAdmin.tsx`, `DeptProfileAdmin.tsx` times out with `channel "profile.X" is not yet implemented in backend-go`
**Status:** ✅ Resolved — see TASK-126–130 (5 task(s), all DONE) for implementation evidence.

---

## Description

None of the 6 `profile.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"profile\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

`tenant-service` is the plausible owner — its proto doc comment reads
"TenantService owns companies, departments, user profiles (3-layer
resolution), teams" (`backend-go/proto/orca/tenant/v1/tenant.proto:7`) — and
it is already `RouteWired` at `/v1/tenants`
(`backend-go/services/api-gateway/internal/domain/registry.go:87`), with a
real REST proxy in `backend-go/services/api-gateway/internal/adapter/httpgateway/tenant_routes.go`.
`auth-service` was also checked (`backend-go/proto/orca/auth/v1/auth.proto`)
since user-profile fields sometimes live there instead — its `User` message
(`auth.proto:36-44`) only carries `id/tenant_id/email/name/role/is_active/
created_at`, and its only mutation RPC is `UpdateUserRole` (role only,
`auth.proto:25`). `auth-service` is not a better fit; `tenant-service`
remains the right owner for this namespace.

Only ONE of the 6 methods has a matching RPC already implemented:

- `profile.getResolved` → `TenantService.GetResolvedProfile`
  (`tenant.proto:14`), backed by
  `backend-go/services/tenant-service/internal/usecase/get_resolved_profile.go`
  (and a caching wrapper, `cached_get_resolved_profile.go`), and already
  REST-wired at `GET /v1/tenants/profile` via `handleGetResolvedProfile`
  (`backend-go/services/api-gateway/internal/adapter/httpgateway/tenant_routes.go:154-177`).
  This one is the "just needs a wscompat wrapper" case.

The other 5 have **no backing RPC or usecase at all** in `tenant-service`:

- `profile.getUserProfile` — no `GetUserProfile` (or similarly named) RPC in
  `tenant.proto`; `TenantService`'s full RPC list
  (`tenant.proto:9-18`) is `CreateCompany, ValidateTenant,
  CreateDepartment, SetUserDepartment, GetResolvedProfile, CreateTeam,
  AddTeamMember, ListTeamMembers` — nothing returns a raw (non-resolved)
  user profile.
- `profile.listDepts` — no `ListDepartments` RPC; only `CreateDepartment`
  exists (`tenant.proto:12`, usecase
  `backend-go/services/tenant-service/internal/usecase/create_department.go`).
  No list usecase file exists in
  `backend-go/services/tenant-service/internal/usecase/`.
- `profile.updateCompany` — no `UpdateCompany` RPC; only `CreateCompany`
  exists (`tenant.proto:10`, usecase `create_company.go`). No update usecase.
- `profile.updateDept` — no `UpdateDepartment` RPC; only `CreateDepartment`
  exists, same as above. No update usecase.
- `profile.updateUser` — no `UpdateUser` RPC in `tenant-service`. The
  closest thing anywhere is `auth-service`'s `UpdateUserRole`
  (`auth.proto:25`), which only changes `Role`, not general profile fields
  — not a match for a general user-profile update.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `profile.getResolved` | `frontend/src/renderer/src/hooks/useProfile.ts:29,65`, `frontend/src/renderer/src/context/WorkspaceContext.tsx:131` | Backing RPC exists (`GetResolvedProfile`) and is already REST-wired — just needs a `wscompat` registration |
| `profile.getUserProfile` | `frontend/src/renderer/src/hooks/useProfile.ts:30` | No matching RPC in `tenant-service`; needs new RPC + usecase |
| `profile.listDepts` | `frontend/src/renderer/src/components/profile/CompanyProfileAdmin.tsx:15`, `frontend/src/renderer/src/components/profile/DeptProfileAdmin.tsx:17` | No `ListDepartments` RPC; needs new RPC + usecase |
| `profile.updateCompany` | `frontend/src/renderer/src/hooks/useProfile.ts:71` | No `UpdateCompany` RPC; needs new RPC + usecase |
| `profile.updateDept` | `frontend/src/renderer/src/hooks/useProfile.ts:74` | No `UpdateDepartment` RPC; needs new RPC + usecase |
| `profile.updateUser` | `frontend/src/renderer/src/hooks/useProfile.ts:64` | No `UpdateUser` RPC in `tenant-service`; `auth-service.UpdateUserRole` is role-only, not a match |

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:134`: 🟢 pure
Postgres relational — `orca_companies`/`orca_departments`/
`orca_user_profiles`/`orca_users`. No relay anywhere. This means all 6
methods are straightforward CRUD once the missing 5 RPCs/usecases exist —
no Dev Server Agent or SSH relay involvement.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `profile.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:87` — `/v1/tenants` → `tenant-service`, `RouteWired`
- `backend-go/proto/orca/tenant/v1/tenant.proto:7-18` — `TenantService`'s full RPC surface
- `backend-go/proto/orca/auth/v1/auth.proto:9-28,36-44` — checked `auth-service`, not a better fit
- `backend-go/services/tenant-service/internal/usecase/` — `get_resolved_profile.go`/`cached_get_resolved_profile.go` exist; no list/update-department/update-company/update-user/get-user-profile usecases
- `backend-go/services/api-gateway/internal/adapter/httpgateway/tenant_routes.go:28-39,154-177` — `mountTenantRoutes`, `handleGetResolvedProfile`
- `specs/frontend/api/backend-agent-execution-boundary.md:134` — `profile.*` 🟢 dispatch classification
- `specs/frontend/api/rpc-catalog.md:357-366` — `profile.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
