# BUG-PRF-03: Project creation has no dev-server binding, health check, repoPath check, audit log, or member notification

**Business Logic:** [BL-PRF-03](../../../../docs/logic/profile/BL-PRF-03-project-server-assignment.md) — Project-Dev Server Assignment
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** An Admin/Lead creating a new project cannot bind it to a Dev Server at creation time at all — `project.create` accepts no `devServerId`/`repoPath`, so every new project starts with an empty `dev_server_id` until a *separate* rebind call is made later. Even that later rebind never validates the target server exists/is online, never checks the repo path exists on the server, and never notifies other project members or writes an audit trail — despite the spec's explicit "Confirm → ... → Notify all project members (WebSocket push event) → audit_log(...)" flow.

---

## Spec summary

Admin/Lead creates a project by supplying `name`, `repoUrl`, `defaultBranch`, `devServerId` (binding), and `repoPath`, with the server validating the server exists/is online and the repo path exists on it before persisting. Changing the binding later requires a confirm dialog, an audit log entry, and a WebSocket notification to all project members. Membership (add/remove members with role) is separate. `getProjectsForUser()` filters visible projects by membership AND per-user dev-server RBAC (`developer` role limited to servers matching `allowedServerTags` in their resolved profile; `lead`/`admin` unrestricted).

## What backend-go has

- `domain.Project` does carry a `DevServerID` field (`backend-go/services/project-service/internal/domain/project.go:75`), and a dedicated safety-hardened rebind path exists: `RebindDevServer` (`backend-go/services/project-service/internal/usecase/rebind_dev_server.go:22-70`) enforces owner-only OPA authorization (`requireProjectAccess`, `authorization.go:54-84`) and fails closed if `workflow-service`/`task-service` report active executions on the project (`rebind_dev_server.go:53-63`) — this exceeds what the spec asks for on the safety-check front (the spec only asks for a confirm dialog).
- Membership CRUD is real: `add_member.go`, `remove_member.go`, `update_member_role.go`, `list_members.go` in `backend-go/services/project-service/internal/usecase/`, wired to wscompat's `project.*` channels (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:172-345`).
- `CreateProject` persists a project row for real (`backend-go/services/project-service/internal/usecase/create_project.go:38-75`), with tenant/creator identity pulled from context, not the request body.

## What's missing

- **No dev-server binding at project creation.** `CreateProjectInput` (`create_project.go:13-21`) has no `DevServerID`/`RepoPath` field, and `domain.NewProject` (`project.go:92-115`, doc comment at line 92: "Deliberately has no DevServerID field") constructs a project with an empty binding. The spec's flow creates the project WITH the binding in one step; here, `dev_server_id` stays empty until a separate, later `RebindDevServer` call — a real behavioral gap, not just a naming difference.
- **No devServerId existence/online-health validation**, at either creation or rebind time. `grep -rn "healthy\|degraded\|unreachable\|HealthCheck"` over `backend-go/services/project-service/internal/usecase/` returns no hits; `RebindDevServer.Execute` (`rebind_dev_server.go:36-70`) only checks active workflow/task executions, never whether `NewDevServerID` exists in `infra-fleet-service` or is online.
- **No repoPath existence validation via relay** (`relay.call('fs.exists', repoPath)` in the spec) anywhere in `project-service`.
- **No audit logging.** `grep -rln "audit_log\|AuditLog"` over `backend-go/services/project-service/internal/` returns zero matches — `project.devserver.changed` is never recorded.
- **No WebSocket push notification to project members** on a dev-server rebind. `grep -rln "Notify\|WebSocket\|websocket"` over the same tree returns zero matches.
- **No `allowedServerTags`-based project-visibility filtering.** `ListProjects` (`backend-go/services/project-service/internal/usecase/list_projects.go:28-43`) returns every project for the tenant with no membership or resolved-profile-based filtering at all — it isn't even scoped to "projects this caller is a member of," let alone additionally narrowed by `fleet.allowedServerTags` per BL-PRF-02's (also-missing, see BUG-PRF-02) intersect rule. Per-action authorization exists (`requireProjectAccess`) but only gates individual mutating RPCs like `RebindDevServer`, not a `getProjectsForUser`-shaped list filter.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-PRF-02-profile-inheritance-approvedmodels-servertags-missing.md` — `fleet.allowedServerTags` isn't resolved anywhere in `tenant-service` either, so `project-service` has nothing to filter against even if it wanted to.

## References

- `backend-go/services/project-service/internal/domain/project.go:71-126` — `Project` struct, `NewProject`, `Rebind` (no creation-time `DevServerID`)
- `backend-go/services/project-service/internal/usecase/create_project.go:13-75` — `CreateProjectInput`/`CreateProject.Execute` (no server/repoPath binding fields)
- `backend-go/services/project-service/internal/usecase/rebind_dev_server.go:22-70` — `RebindDevServer` (safety checks present; server-health/repoPath checks absent)
- `backend-go/services/project-service/internal/usecase/authorization.go:1-84` — `requireProjectAccess` (per-action, not a list filter)
- `backend-go/services/project-service/internal/usecase/list_projects.go:28-43` — unfiltered tenant-wide list
- `docs/logic/profile/BL-PRF-03-project-server-assignment.md:19-56,74-94` — creation+binding flow, rebind flow, visibility rules
