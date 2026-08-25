# BUG-028: `team.*` channels not implemented in backend-go

**Service:** `tenant-service` (via `api-gateway`)
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium — admin-only feature (`TeamAdmin.tsx`); no channel is on the app bootstrap path
**Symptom:** Every call the Team Admin screen makes (`team.list`, `team.listMembers`, `team.create`, `team.addMember`, `team.removeMember`) falls through to `notImplementedHandler` and times out client-side
**Status: ❌ Open**

---

## Description

`specs/frontend/api/rpc-catalog.md` lists 5 `team.*` methods, all called from
`frontend/src/renderer/src/components/admin/TeamAdmin.tsx`:

```
grep -n '"team\.' services/api-gateway/internal/adapter/wscompat/channels.go
```

returns **zero matches** — no `team.*` channel is registered in
`RegisterRealChannels` (`channels.go:79-89`). Every call reaches
`registry.go`'s `notImplementedHandler` (`registry.go:59`).

`tenant-service` is the correct owning service — it already has real
`Team`/`TeamMember` domain types, a Postgres-backed `TeamRepository`, and 3 of
the 5 RPCs `team.*` needs:

- `rpc CreateTeam` — `proto/orca/tenant/v1/tenant.proto:15`
- `rpc AddTeamMember` — `proto/orca/tenant/v1/tenant.proto:16`
- `rpc ListTeamMembers` — `proto/orca/tenant/v1/tenant.proto:17`

implemented for real against `tenant.teams`/`tenant.team_members`
(`services/tenant-service/internal/adapter/postgres/team_repository.go:26-90`,
`internal/usecase/ports.go:48-59`).

`auth-service` (`proto/orca/auth/v1/auth.proto`) has no team-shaped RPCs at
all — its surface is `Login`/`Logout`/`ValidateSession`/`IssueServiceToken`/
`GetJWKS`/`CreateUser`/`ListUsers`/`UpdateUserRole`/`RevokeSession`/
`QueryAuditLog` (lines 12-27). It manages users/sessions/roles, not team
membership — `tenant-service` wins as the owner, consistent with it already
owning company/department/team hierarchy per its README.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `team.create` | `TeamAdmin.tsx:108` | Backing RPC exists: `CreateTeam` (`tenant.proto:15`), `usecase/create_team.go`. Just needs a wscompat wrapper. |
| `team.addMember` | `TeamAdmin.tsx:129` | Backing RPC exists: `AddTeamMember` (`tenant.proto:16`), `usecase/add_team_member.go`. Note: proto has no `role` field, only `priority` — `AddTeamMemberRequest` (`tenant.proto:90-94`); `role` defaults to `'member'` server-side (README "Known gaps"). The frontend call passes `role` (`TeamAdmin.tsx:129`) which has nowhere to go today. |
| `team.listMembers` | `TeamAdmin.tsx:79` | Backing RPC exists: `ListTeamMembers` (`tenant.proto:17`), `usecase/list_team_members.go`. Just needs a wscompat wrapper. |
| `team.list` | `TeamAdmin.tsx:62` | **No backing RPC.** `tenant.proto` has no `ListTeams` (list-all-teams-for-a-company) RPC, and `TeamRepository` (`usecase/ports.go:48-59`) has no `List`/`ListByCompany` method — only `Create`, `Get`, `AddMember`, `ListMembers`, `ListUserTeamLayers`. |
| `team.removeMember` | `TeamAdmin.tsx:146` | **No backing RPC.** Explicitly documented as a known gap: `services/tenant-service/README.md:99-101` lists `RemoveTeamMember` among the RPCs not yet in `tenant.proto`'s "reduced subset" surface. No `RemoveMember` method on `TeamRepository` either. |

3 of 5 methods (`create`, `addMember`, `listMembers`) are "just needs a
wscompat wrapper" — real usecases and Postgres persistence already exist.
`list` and `removeMember` need new `tenant-service` RPCs (`ListTeams`,
`RemoveTeamMember`) plus repository methods before a wscompat wrapper is
possible.

---

## Dispatch model

🟢 Pure Postgres relational, no relay anywhere — the old TS backend stored
teams and membership in `orca_teams`/`orca_team_members` tables and served
every `team.*` call directly from Postgres. backend-go's `tenant-service`
follows the same shape (`tenant.teams`/`tenant.team_members`, see
`services/tenant-service/internal/adapter/postgres/team_repository.go`).
There is no Dev Server Agent relay, no SSH provider dispatch, and no
async/background execution involved in this namespace — every `team.*` RPC
is a synchronous read or write against `tenant-service`'s own database.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go:79-89` — `RegisterRealChannels` (no `team.*` registration)
- `services/api-gateway/internal/adapter/wscompat/registry.go:59` — `notImplementedHandler`
- `proto/orca/tenant/v1/tenant.proto:9-18` — `TenantService` RPC surface
- `services/tenant-service/internal/usecase/ports.go:48-59` — `TeamRepository` interface
- `services/tenant-service/internal/adapter/postgres/team_repository.go` — `TeamRepository` implementation
- `services/tenant-service/README.md:99-101` — documented gap: no `RemoveTeamMember`/`UpdateTeam` RPC yet
- `frontend/src/renderer/src/components/admin/TeamAdmin.tsx:62,79,108,129,146` — all 5 call sites
- `specs/frontend/api/rpc-catalog.md:447-455` — `team.*` catalog entries
