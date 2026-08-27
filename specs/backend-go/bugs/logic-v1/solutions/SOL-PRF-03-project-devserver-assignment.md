# SOL-PRF-03: Dev-server binding at project creation, health/repoPath validation, audit + member notification on rebind, and RBAC-filtered project listing

**Resolves:** [BUG-PRF-03](../BUG-PRF-03-project-devserver-assignment-partial.md)
**Service:** `project-service` (+ additive `infra-fleet-service` proto/schema field, + a `notification-service` consumer-list entry)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto` (edit: `CreateProjectRequest` gains `dev_server_id`/`repo_path`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (edit: `DevServer` gains `repeated string tags`)
- `backend-go/services/project-service/internal/usecase/create_project.go` (edit: binding + health + repoPath validation)
- `backend-go/services/project-service/internal/usecase/rebind_dev_server.go` (edit: health check + audit + notify)
- `backend-go/services/project-service/internal/usecase/list_projects.go` (edit: membership scope + RBAC tag filter)
- `backend-go/services/project-service/internal/usecase/ports.go` (extend: `DevServerHealthChecker`, `AuditPublisher`, `MemberNotifier`, `ProfileResolver`)
- `backend-go/services/project-service/internal/adapter/grpcclient/` (new: `dev_server_health_checker.go`, `profile_resolver.go`; edit `infra_fleet_dev_server_lister.go` if tags are exposed there too)
- `backend-go/services/project-service/internal/adapter/eventbus/` (new package: `publisher.go` — project-service currently has none)
- `backend-go/services/project-service/internal/adapter/grpc/server.go` (edit: `CreateProject` handler forwards new fields)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go` (edit: `project.create` forwards `devServerId`/`repoPath`, currently silently dropped)
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (edit: add `{StreamName: "PROJECT", Subject: "orca.project.devserver.changed"}`)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`project-service.md` §2's "Boundary decision" table already names every
outbound call this fix needs and settles their ownership:

- **repoPath existence check** — "Which host a worktree resolves to |
  Provides the pointer (`dev_server_id`) | `connectionId` resolution +
  relay: `git-gateway-service` + `infra-fleet-service`." This solution
  doesn't invent a new relay pathway: `usecase.SetupExistingFolder`
  (`setup_existing_folder.go:61-79`) already implements exactly this — call
  `DevServerRelay.CreateConnection` then `DevServerRelay.Relay(ctx, connID,
  "fs.checkPath", {"path": ...})`, decoding `{exists, isDir}`. This
  solution reuses that proven method name and port (`ports.go:181-190`'s
  `DevServerRelay`) verbatim for `CreateProject`, rather than inventing a
  second filesystem-check mechanism.
- **devServerId existence** — `DevServerLister.Exists`
  (`ports.go:210-217`, `adapter/grpcclient/infra_fleet_dev_server_lister.go:40-51`)
  already implements this against `infra-fleet-service.ListDevServers` and
  is already used by `CreateHostSetup` (`create_host_setup.go:38-44`).
  Reused as-is.
- **devServerId online/health** — genuinely absent everywhere in
  `project-service` today (confirmed: zero `Health`/`online`/`reachable`
  hits in `internal/usecase/`). `infra-fleet-service`'s own proto already
  has the primitive this needs and it's unused by any caller:
  `GetFleetHealth(GetFleetHealthRequest) returns (GetFleetHealthResponse)`
  (`infrafleet.proto:17`), returning `DevServerHealth{dev_server_id,
  reachable, cpu_percent, ram_percent, disk_percent, latency_ms}`
  (`infrafleet.proto:226-233`) — exactly `infra-fleet-service.md`'s "Fleet
  health monitoring — periodic polling of CPU/RAM/disk/latency" (§1) and
  matches `project-service.md` §7's already-documented dependency edge
  (`project-service` calls `infra-fleet-service` "to validate a `devServerId`
  exists on create/rebind"). This solution adds a thin
  `DevServerHealthChecker` port around the already-existing RPC — no
  infra-fleet-service proto change needed for health.
- **Audit log + WS notification on rebind** — `project-service.md` §6's
  package layout already names the target: `adapter/eventbus/ # outbox:
  project.created, project.rebound, member.added, ...` — this directory
  doesn't exist yet (confirmed: `find .../project-service/internal/adapter`
  lists only `postgres/grpc/opaclient/grpcclient`), so this solution builds
  it for the first time, following `tenant-service`'s already-implemented
  `adapter/eventbus` shape (`internal/adapter/eventbus/publisher.go`) and
  `05-data-architecture.md`'s "Transactional outbox + async events
  (default)" pattern, not a synchronous call to any notification/audit
  service (there is none to call — see SOL-PRF-01's audit-emission
  rationale for the identical cross-database argument, which applies here
  unchanged).
- **`allowedServerTags`-based visibility filter** — depends on
  [SOL-PRF-02](./SOL-PRF-02-approvedmodels-servertags-merge.md) existing
  first (`resolved.fleet.allowedServerTags` has no producer without it) and
  needs `project-service` to call `tenant-service.GetResolvedProfile` — a
  **new** outbound edge, since `02-microservices-decomposition.md`'s
  dependency graph (`:148`) only shows `proj --> tenant` for
  `CreateProject`'s existing `ValidateTenant`-shaped calls, not profile
  resolution specifically, but `tenant-service.md` §3's RPC surface exposes
  `GetResolvedProfile` generally to any caller and §7's "Who calls this
  service" doesn't restrict it to `task-service`/`workflow-service` —
  reusing the same client type (`tenantv1.TenantServiceClient`)
  `project-service` would dial for is a natural, low-risk extension of an
  edge that already exists in the graph, not a new service dependency.
- **`fleet.allowedServerTags` needs something to compare against on the
  server side** — and here there's a genuine, unflagged-elsewhere model
  gap: `infra-fleet-service`'s `DevServer` message
  (`infrafleet.proto:112-123`) carries no `tags` field at all (`id,
  tenant_id, host, mode, ssh_target_id` only), so "developer: server must
  match `allowedServerTags`" (`docs/logic/profile/BL-PRF-03-project-server-assignment.md:79-81`)
  has nothing to match against today. This solution proposes adding
  `repeated string tags = 6;` to `DevServer` and a matching `tags TEXT[]
  NOT NULL DEFAULT '{}'` column to `infra-fleet-service`'s `dev_servers`
  table (`infra-fleet-service.md` §5) — a small, additive schema change,
  flagged explicitly here since it's a genuine TDD gap this bug's fix can't
  route around, not something already specified. `RegisterDevServer`/
  `UpdateDevServer`'s request messages gain the same field so tags are
  settable; out of scope here is building any UI/RPC for *managing* tags
  beyond passing them through — this solution only needs them to exist and
  be readable via `ListDevServers`.

---

## Design — proto additions

```protobuf
// project.proto
message CreateProjectRequest {
  string tenant_id = 1;
  string name = 2;
  string description = 3;
  string default_branch = 4;
  string visibility = 5;
  string dev_server_id = 6; // NEW — binds at creation time, per BL-PRF-03's flow
  string repo_path = 7;     // NEW — absolute path on dev_server_id; becomes the project's first Repo
}
```

```protobuf
// infrafleet.proto
message DevServer {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  ConnectionMode mode = 4;
  string ssh_target_id = 5;
  repeated string tags = 6; // NEW — BL-PRF-03's allowedServerTags match target
}
// RegisterDevServerRequest / UpdateDevServerRequest gain the same field.
```

---

## Design — `usecase/` layer

### `CreateProject` (edit)

```go
type CreateProjectInput struct {
	Name          string
	Description   string
	DefaultBranch string
	Visibility    string
	DevServerID   string // "" = create unbound, existing behavior preserved
	RepoPath      string // required iff DevServerID is set
}

type CreateProject struct {
	repo       ProjectRepository
	repos      RepoRepository
	devServers DevServerLister        // existing port, reused
	health     DevServerHealthChecker // new port
	relay      DevServerRelay         // existing port, reused (SetupExistingFolder's pattern)
}

func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	// ... existing tenantID/userID/visibility validation unchanged ...

	devServerID := in.DevServerID
	if devServerID != "" {
		exists, err := uc.devServers.Exists(ctx, tenantID, devServerID)
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
		}
		if !exists {
			return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
		}
		reachable, err := uc.health.IsReachable(ctx, tenantID, devServerID)
		if err != nil {
			// Fail closed, same posture as RebindDevServer's active-execution
			// checks (project-service.md §3/§8) — a health-check outage must
			// never silently bind to a server that might be down.
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED", "failed to verify dev server is online, failing closed", err)
		}
		if !reachable {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE", "dev server is not online", nil)
		}
		if in.RepoPath == "" {
			return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_PATH_REQUIRED", "repo_path is required when dev_server_id is set", nil)
		}
	}

	project, err := domain.NewProject(uuid.NewString(), tenantID, in.Name, devServerID) // domain.NewProject already accepts a devServerID param — no domain change needed
	// ... existing description/defaultBranch/visibility/createdBy assignment unchanged ...

	created, err := uc.repo.Create(ctx, project)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to persist project", err)
	}

	if devServerID != "" {
		// repoPath existence check — SetupExistingFolder's exact pattern
		// (setup_existing_folder.go:61-79), reused not reinvented.
		connID, err := uc.relay.CreateConnection(ctx, devServerID, in.RepoPath, "")
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
		}
		params, _ := json.Marshal(map[string]string{"path": in.RepoPath})
		resultJSON, err := uc.relay.Relay(ctx, connID, "fs.checkPath", params)
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_FAILED", "failed to validate repo path on dev server", err)
		}
		var check struct{ Exists, IsDir bool }
		if err := json.Unmarshal(resultJSON, &check); err != nil || !check.Exists || !check.IsDir {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_REPO_PATH_NOT_FOUND", "repo_path does not exist on dev server", nil)
		}
		repo, err := domain.NewRepo(uuid.NewString(), created.ID, in.RepoPath, in.Name)
		if err == nil {
			_, _ = uc.repos.AddRepo(ctx, repo) // best-effort attach, matching SetupExistingFolder's own non-fatal-repo-attach posture is NOT followed here deliberately — see Test plan's regression note
		}
	}
	return created, nil
}
```

**Ordering note, flagged deliberately**: the dev-server existence/health/
repoPath checks run **before** `uc.repo.Create` (fail before any row is
written); the repoPath filesystem check specifically runs **after**
`Create` in the sketch above only because it needs `created.ID` for the
`Repo` row — an implementation should instead generate the project ID
up front (`uuid.NewString()` already available before `Create`) and run
the repoPath check before persisting the project row too, so a
repo-path failure never leaves an orphaned bound-but-repo-less project.
This is called out explicitly as a correctness requirement for the real
implementation, not left implicit in the sketch's ordering.

### `RebindDevServer` (edit)

```go
type RebindDevServer struct {
	repo            ProjectRepository
	workflowChecker WorkflowExecutionChecker
	taskChecker     TaskExecutionChecker
	opa             OPAClient
	devServers      DevServerLister        // new dependency
	health          DevServerHealthChecker // new dependency
	audit           AuditPublisher         // new dependency
	notifier        MemberNotifier         // new dependency
}

func (uc *RebindDevServer) Execute(ctx context.Context, in RebindDevServerInput) (domain.Project, error) {
	// ... existing tenantID/NewDevServerID-non-empty/requireProjectAccess checks unchanged ...

	exists, err := uc.devServers.Exists(ctx, tenantID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate new dev server", err)
	}
	if !exists {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "new dev server does not exist", nil)
	}
	reachable, err := uc.health.IsReachable(ctx, tenantID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED", "failed to verify new dev server is online, failing closed", err)
	}
	if !reachable {
		return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE", "new dev server is not online", nil)
	}

	// ... existing workflowChecker/taskChecker active-execution guard unchanged ...

	before, err := uc.repo.Get(ctx, tenantID, in.ProjectID) // for oldId in the audit/notify payload
	updated, err := uc.repo.UpdateDevServerID(ctx, tenantID, in.ProjectID, in.NewDevServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_REBIND_FAILED", "failed to update dev server binding", err)
	}

	actorID, _ := tenant.UserID(ctx)
	if uc.audit != nil {
		_ = uc.audit.PublishAuditEvent(ctx, tenantID, actorID, "project.devserver.changed", in.ProjectID)
	}
	if uc.notifier != nil {
		members, err := uc.repo.ListMembers(ctx, in.ProjectID)
		if err == nil {
			ids := make([]string, 0, len(members))
			for _, m := range members {
				ids = append(ids, m.UserID)
			}
			_ = uc.notifier.NotifyDevServerChanged(ctx, tenantID, ids, in.ProjectID, before.DevServerID, in.NewDevServerID)
		}
	}
	return updated, nil
}
```

Both new outbound calls (`audit`, `notifier`) are best-effort after the
authoritative write succeeds — matches BL-PRF-03's flow order ("Confirm →
UPDATE ... → Notify ... → audit_log(...)"; the UPDATE is the fact of
record, notification/audit are downstream of it succeeding) and
`tenant-service`'s own `PublishProfileInvalidated` best-effort convention
(`ports.go:101-114`'s doc comment: "A nil `CacheInvalidationPublisher`...
is valid — callers must nil-check"), reused here for both new ports rather
than inventing a stricter contract this bug doesn't ask for.

### `ListProjects` → `getProjectsForUser` (edit)

```go
type ListProjects struct {
	repo     ProjectRepository
	profiles ProfileResolver // new port -> tenant-service.GetResolvedProfile
}

func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	// Membership scope: BL-PRF-03's getProjectsForUser starts from
	// "userMemberships = db.projectMembers.findByUser(userId)" — the
	// current List(tenantID, ...) has NO membership filter at all (returns
	// every tenant project regardless of caller), a gap this fix closes
	// alongside the RBAC filter, since a visibility filter over an
	// unscoped list would be incomplete on its own.
	projects, next, err := uc.repo.ListForMember(ctx, tenantID, userID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}

	role, _ := tenant.Role(ctx) // same known upstream gap as SOL-PRF-01 — "" until claim propagation lands; fails to the SAFER branch below, not the permissive one
	if role == "admin" || role == "lead" {
		return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
	}

	// developer (or unknown/"" role): filter by allowedServerTags per
	// BL-PRF-03's hasServerAccess. Fail-CLOSED posture differs from
	// SOL-PRF-01's authz checks deliberately: an unknown role here means
	// "assume most restrictive membership filter (developer)," not "deny
	// the whole list" — ListProjects has no single yes/no gate to fail
	// closed on, it's a per-item filter, so the safe default is the
	// narrowest one, not an empty result or an error.
	resolved, err := uc.profiles.GetResolvedProfile(ctx, tenantID, userID)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_PROFILE_RESOLVE_FAILED", "failed to resolve caller profile for visibility filtering", err)
	}
	allowedTags, hasRestriction := resolved.AllowedServerTags() // "" key absent -> hasRestriction=false -> no filtering

	filtered := projects[:0]
	for _, p := range projects {
		if !hasRestriction || p.DevServerID == "" {
			filtered = append(filtered, p)
			continue
		}
		tags, err := uc.profiles.DevServerTags(ctx, tenantID, p.DevServerID) // -> infra-fleet-service.ListDevServers
		if err == nil && tagsIntersect(tags, allowedTags) {
			filtered = append(filtered, p)
		}
	}
	return ListProjectsOutput{Projects: filtered, NextPageToken: next}, nil
}
```

`ProjectRepository` (`ports.go`) gains `ListForMember(ctx, tenantID,
userID, pageToken string, pageSize int32) ([]domain.Project, string,
error)` — a join against `project_members` instead of `List`'s bare
`tenant_id` filter; `List` itself is kept (still used by
admin/cross-project tooling that legitimately needs the unfiltered view,
if any exists — flagged as a decision to preserve, not delete, the
existing method).

---

## Design — wiring (wscompat + grpc)

`wscompat`'s `project.create` (`channels_tenant_project.go:173-196`)
already decodes `devServerId` off the wire but **silently drops it** — the
`projectv1.CreateProjectRequest` it builds never sets a `DevServerId`
field because the field didn't exist. Fix:

```go
type createArgs struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DevServerID   string `json:"devServerId"`
	RepoPath      string `json:"repoPath"` // NEW — wasn't decoded at all before
	DefaultBranch string `json:"defaultBranch"`
	Visibility    string `json:"visibility"`
}
// ...
resp, err := client.CreateProject(rpcCtx, &projectv1.CreateProjectRequest{
	TenantId: id.TenantID, Name: in.Name, Description: in.Description,
	DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
	DevServerId: in.DevServerID, RepoPath: in.RepoPath, // NEW
})
```

`project-service`'s `adapter/grpc/server.go`'s `CreateProject` handler
gains `DevServerID: req.GetDevServerId(), RepoPath: req.GetRepoPath()` in
its `usecase.CreateProjectInput` construction (currently reads neither
field, since neither existed).

`internal/adapter/eventbus/publisher.go` (new package, mirrors
`tenant-service`'s): `StreamName = "PROJECT"`, `Subject =
"orca.project.devserver.changed"`, payload `{user_ids: [...],
title, body, deep_link}` — matches `notification-service`'s
generic `EventPayload{UserIDs, Title, Body, DeepLink}` shape
(`notification_event.go:39-45`) exactly, so it translates via
`defaultRule` even without a dedicated `subjectRules` entry; this solution
adds one anyway for a better title/body:

```go
// notification_event.go's subjectRules map (edit, in notification-service):
"orca.project.devserver.changed": {
	Type: "project_devserver_changed", Title: "Dev server changed",
	Body: "This project's dev server binding was changed.",
	Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS},
},
```

`notification-service/internal/adapter/eventbus/consumer.go`'s static
`{StreamName, Subject}` list (`:45-50`) gains
`{StreamName: "PROJECT", Subject: "orca.project.devserver.changed"}` — the
one required cross-service change for the WS push to actually reach
`notification-service`'s consumer at all.

---

## Test plan

- `create_project_test.go`: `DevServerID` empty → unchanged existing
  behavior (unbound project, no relay/health calls at all — regression
  guard). `DevServerID` set + `RepoPath` empty → `KindInvalidArgument`
  before any repo/health call. Fake `DevServerLister.Exists` false →
  `KindInvalidArgument`, no health/relay call. Fake `DevServerHealthChecker`
  returning `reachable=false` → `KindFailedPrecondition`, no relay call.
  Fake `DevServerRelay` returning `{exists:false}` → `KindFailedPrecondition`,
  and (per the ordering note above) no orphaned project row left in the
  fake repository.
- `rebind_dev_server_test.go`: add cases for the two new checks
  (not-found, unreachable) preceding the existing active-execution guard —
  assert `workflowChecker`/`taskChecker` are never called when the new
  dev server fails existence/health (fail fast, cheapest check first).
  Assert `AuditPublisher.PublishAuditEvent` and
  `MemberNotifier.NotifyDevServerChanged` are both called exactly once on
  success, with the correct old/new dev server ids and the full member-id
  list from a fake `ListMembers`. Assert a `nil` `audit`/`notifier` (not
  wired) doesn't panic — same nil-safe convention as
  `CacheInvalidationPublisher`.
- `list_projects_test.go`: fake `ProjectRepository.ListForMember` returns
  only the caller's projects (not the whole tenant) — regression guard for
  the previously-entirely-unscoped `List`. Role `"admin"`/`"lead"` → no
  `ProfileResolver` call at all (short-circuit, cheapest path first). Role
  `"developer"`/`""` → `ProfileResolver.GetResolvedProfile` called once;
  a project whose dev server's tags don't intersect `allowedServerTags` is
  excluded; a project with no `DevServerID` (never bound) always passes
  through regardless of role (nothing to check tags against).
- `internal/domain/profile_resolution_test.go` (tenant-service, cross-
  reference to SOL-PRF-02): confirm `ResolvedProfile.AllowedServerTags()`'s
  present-vs-absent distinction survives JSON round-trip through the gRPC
  `ResolvedProfile` message, since `ListProjects`'s filter depends on being
  able to tell "unrestricted" from "explicitly locked out."
- `adapter/eventbus/publisher_test.go` (new, project-service): payload
  shape matches `notification-service`'s expected `EventPayload` JSON tags
  exactly — a cross-service contract test, since a typo here (e.g.
  `userIds` vs `user_ids`) fails silently (an untranslatable payload just
  produces `ErrNoRecipients`, logged and dropped, not an error either side
  would notice without this test).
- `notification-service/internal/adapter/eventbus/consumer_test.go`: the
  new `{StreamName: "PROJECT", ...}` entry is present in the static
  subscription list (regression guard for the cross-service wiring step).

## References

- `specs/backend-go/tdd/services/project-service.md:26-52` (§2 boundary
  table — every outbound call this solution needs is already named here),
  `:122-161` (`RebindDevServer`'s existing saga, extended not replaced),
  `:268-294` (§6 package layout — `adapter/eventbus` target this solution
  builds), `:296-318` (§7 dependencies — `tenant-service`/`infra-fleet-service`
  edges this solution reuses/extends)
- `specs/backend-go/tdd/services/infra-fleet-service.md:78-138` (§3 API
  surface — `ResolveConnection`/`GetFleetHealth`), `:140-166` (§4 domain
  model — `DevServer`/`FleetHealthSample`)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166`
  (dependency graph — `proj --> tenant`, `proj --> infra` edges this
  solution's new calls extend, not add new nodes to)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98`
  (transactional outbox default pattern — grounds the new
  `adapter/eventbus` package)
- `docs/logic/profile/BL-PRF-03-project-server-assignment.md:19-94`
  (creation+binding flow, rebind flow, `getProjectsForUser`/`hasServerAccess`)
- `backend-go/services/project-service/internal/usecase/setup_existing_folder.go:40-118`
  (the exact `CreateConnection`/`fs.checkPath` pattern this solution reuses
  for `CreateProject`'s repoPath check)
- `backend-go/services/project-service/internal/usecase/create_host_setup.go:28-56`,
  `ports.go:181-217` (`DevServerLister`/`DevServerRelay` ports reused as-is)
- `backend-go/services/project-service/internal/domain/project.go:108-116`
  (`domain.NewProject` already accepts a `devServerID` parameter — no
  domain-layer change needed for creation-time binding)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:112-123,222-237`
  (`DevServer` message this solution's `tags` field extends;
  `GetFleetHealth`/`DevServerHealth` this solution's health checker calls)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:173-196`
  (`project.create`'s existing `devServerId`-decoded-but-dropped bug,
  confirmed by reading the handler)
- `backend-go/services/notification-service/internal/domain/notification_event.go:35-133`
  (`EventPayload`/`subjectRules`/`defaultRule` — the WS-push translation
  mechanism this solution's audit/notify publisher targets)
- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:45-50`
  (static subscription list needing the new `PROJECT` stream entry)
- `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go`
  (the best-effort-publish shape this solution's new project-service
  `adapter/eventbus` package mirrors)
- [SOL-PRF-02](./SOL-PRF-02-approvedmodels-servertags-merge.md) — hard
  prerequisite for this solution's `allowedServerTags` visibility filter
