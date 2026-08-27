# SOL-022: `projectHostSetup.*` — settle ownership as `project-service`, add a `project_host_setups` wizard table + 5 RPCs

**Resolves:** [BUG-022](../BUG-022-projecthostsetup-channels-not-implemented.md)
**Service:** `project-service` (new table, 5 new RPCs) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto`
- `backend-go/services/project-service/internal/domain/host_setup.go` (new)
- `backend-go/services/project-service/migrations/000X_project_host_setups.up.sql` / `.down.sql` (new)
- `backend-go/services/project-service/internal/usecase/ports.go` (new `HostSetupRepository`, `DevServerRelay` reused from SOL-021)
- `backend-go/services/project-service/internal/usecase/create_host_setup.go` / `list_host_setups.go` / `update_host_setup.go` / `delete_host_setup.go` / `setup_existing_folder.go` (new)
- `backend-go/services/project-service/internal/adapter/postgres/host_setup_repository.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerProjectHostSetupChannels`)
**Status:** 📋 Proposed — not yet implemented

---

## Settling ownership: `project-service`, not `infra-fleet-service`

BUG-022 checked both services and correctly found neither owns a "bind
project to dev-server host" concept today. Deciding between them:

- **`infra-fleet-service`** owns *reachability* — dev servers, SSH targets,
  connections (`infra-fleet-service.md` §1: "everything needed to reach a
  dev server and know which one owns a given piece of work"). It has no
  concept of a *project* at all, and per its own §2 boundary table it
  never owns coordination metadata that belongs to another service's
  domain — adding a project-shaped wizard record here would duplicate
  `project-service`'s job of owning "workspace organization metadata"
  (`project-service.md` §1).
- **`project-service`** already owns the adjacent, structurally similar
  concepts: `Project.dev_server_id` (a project↔host binding, §4), `Repo`
  (metadata for an on-disk folder — `path`, `remote_url`, §4), and the
  `RebindDevServer` saga that validates a `devServerId` against
  `infra-fleet-service` before committing a binding (§3). `rpc-catalog.md`
  describes `projectHostSetup.*` as "binding a project to a dev-server host
  (legacy repo/host-setup model)" and `setupExistingFolder` as "point an
  existing on-disk folder at a host and create a project from it" — both
  are project-creation-adjacent operations, not fleet-reachability
  operations.

**Decision: `project-service` owns `projectHostSetup.*`**, referencing
`infra-fleet-service`'s `dev_server_id` as a **logical FK, ID-only,
validated via gRPC call** — the exact cross-service reference convention
`05-data-architecture.md` specifies ("no cross-database FK ... a service
that needs data another service owns calls that service's API") and the
same pattern `project-service.md` §5 already uses for `projects.dev_server_id`
itself. No new convention is introduced; this is the established one,
applied to a new table.

---

## Design — domain model & schema

`projectHostSetup` models the **pre-project wizard step**: a user names a
dev server + an existing absolute folder path on it, the backend validates
that path (relayed to the Dev Server Agent — see below, never checked
against `project-service`'s own host, closing the same "legacy
desktop-app assumption" BUG-021/BUG-022 both flag), and on
`setupExistingFolder` finalizes it into a real `Project` + `Repo`. `create`/
`list`/`update`/`delete` manage the wizard record itself (a user may start,
revisit, or abandon a setup before finalizing).

```sql
-- New table in project-service's own database (schema `project`,
-- 05-data-architecture.md's database-per-service rule — no new database).
CREATE TABLE project_host_setups (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL,          -- logical FK -> infra-fleet-service.dev_servers,
                                          -- validated via gRPC at create/setupExistingFolder time, never joined in SQL
  folder_path    TEXT NOT NULL,          -- absolute path on that dev server
  display_name   TEXT,
  status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'validated', 'completed', 'failed')),
  project_id     UUID REFERENCES projects (id) ON DELETE SET NULL, -- set once setupExistingFolder finalizes it
  created_by     UUID NOT NULL,          -- logical FK -> tenant-service user id
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_project_host_setups_tenant ON project_host_setups (tenant_id);
```

RLS enabled, `tenant_id`-scoped, matching every other table in this schema
(`05-data-architecture.md`, `project-service.md` §5's closing paragraph).

```go
// internal/domain/host_setup.go
type HostSetupStatus string

const (
    HostSetupPending   HostSetupStatus = "pending"
    HostSetupValidated HostSetupStatus = "validated"
    HostSetupCompleted HostSetupStatus = "completed"
    HostSetupFailed    HostSetupStatus = "failed"
)

// HostSetup is the pre-project wizard record projectHostSetup.* manages.
// project_id is empty until SetupExistingFolder finalizes it into a real
// Project — see project-service.md-style "logical FK" convention applied
// to infra-fleet-service.dev_servers below.
type HostSetup struct {
    ID           string
    TenantID     string
    DevServerID  string // logical FK -> infra-fleet-service, ID-only, validated via gRPC (05-data-architecture.md)
    FolderPath   string
    DisplayName  string
    Status       HostSetupStatus
    ProjectID    string // empty until Status == HostSetupCompleted
    CreatedBy    string
}
```

---

## Design — Proto additions (`project.proto`)

```protobuf
message HostSetup {
  string id = 1;
  string tenant_id = 2;
  string dev_server_id = 3;
  string folder_path = 4;
  string display_name = 5;
  string status = 6; // pending | validated | completed | failed
  string project_id = 7; // empty until completed
}

rpc CreateHostSetup(CreateHostSetupRequest) returns (CreateHostSetupResponse);
rpc ListHostSetups(ListHostSetupsRequest) returns (ListHostSetupsResponse);
rpc UpdateHostSetup(UpdateHostSetupRequest) returns (UpdateHostSetupResponse);
rpc DeleteHostSetup(DeleteHostSetupRequest) returns (DeleteHostSetupResponse);
rpc SetupExistingFolder(SetupExistingFolderRequest) returns (SetupExistingFolderResponse);

message CreateHostSetupRequest {
  string dev_server_id = 1;
  string folder_path = 2;
  string display_name = 3;
}
message CreateHostSetupResponse {
  HostSetup setup = 1;
}

message ListHostSetupsRequest {}
message ListHostSetupsResponse {
  repeated HostSetup setups = 1;
}

message UpdateHostSetupRequest {
  string id = 1;
  string folder_path = 2;  // empty = no change
  string display_name = 3; // empty = no change
}
message UpdateHostSetupResponse {
  HostSetup setup = 1;
}

message DeleteHostSetupRequest {
  string id = 1;
}
message DeleteHostSetupResponse {}

// SetupExistingFolder validates folder_path exists on dev_server_id (relayed,
// never checked against project-service's own host — see below), then
// creates a real Project + Repo from it and marks the HostSetup completed.
message SetupExistingFolderRequest {
  string id = 1; // the HostSetup being finalized
}
message SetupExistingFolderResponse {
  HostSetup setup = 1; // status now "completed" or "failed"
  Project project = 2; // set only on success
}
```

Additive only — no `buf breaking` risk. `AddRepoRequest` also needs a new
`path` field (it currently only carries `url`/`display_name` — a
remote-clone-URL shape with no room for "this is already a folder on the
dev server, not something to clone"); flagged here as a small, additive
extension `SetupExistingFolder`'s internal `AddRepo` call depends on.

---

## Design — `usecase/` layer

`CreateHostSetup`/`ListHostSetups`/`UpdateHostSetup`/`DeleteHostSetup` are
plain CRUD against `HostSetupRepository`, each validating `dev_server_id`
via `infra-fleet-service.GetDevServer` on create (same saga-step pattern
`project-service.md` §7 already documents for `CreateProject`) — no new
design needed beyond that one validation call.

`SetupExistingFolder` is the one real usecase, and it must **never** stat
the path against `project-service`'s own filesystem — same principle
SOL-021 applies to `scanNested`, reusing the identical `DevServerRelay`
port:

```go
// internal/usecase/setup_existing_folder.go
func (uc *HostSetupUseCase) SetupExistingFolder(ctx context.Context, tenantID, actorID, setupID string) (domain.HostSetup, domain.Project, error) {
    setup, err := uc.repo.Get(ctx, tenantID, setupID)
    if err != nil {
        return domain.HostSetup{}, domain.Project{}, err
    }

    // Validate the path on the DEV SERVER, never locally — the exact
    // "legacy desktop-app assumption" both BUG-021 and BUG-022 flag.
    connID, err := uc.relay.CreateConnection(ctx, setup.DevServerID, setup.FolderPath, "")
    if err != nil {
        return domain.HostSetup{}, domain.Project{}, err
    }
    params, _ := json.Marshal(map[string]string{"path": setup.FolderPath})
    resultJSON, err := uc.relay.Relay(ctx, connID, "fs.checkPath", params)
    if err != nil {
        _ = uc.repo.SetStatus(ctx, tenantID, setupID, domain.HostSetupFailed)
        return domain.HostSetup{}, domain.Project{}, apperrors.Wrap(err, "validate folder on dev server")
    }
    check, err := domain.ParsePathCheckResult(resultJSON) // {exists, isDir, isGitRepo}
    if err != nil || !check.Exists || !check.IsDir {
        _ = uc.repo.SetStatus(ctx, tenantID, setupID, domain.HostSetupFailed)
        return domain.HostSetup{}, domain.Project{}, domain.ErrFolderNotFoundOnHost
    }

    // Finalize: create the real Project, then attach the folder as its
    // first Repo — reuses CreateProject/AddRepo's existing usecases rather
    // than duplicating their validation, in one DB transaction.
    project, err := uc.projects.CreateProject(ctx, tenantID, actorID, domain.ProjectCreateInput{
        Name: setup.DisplayName, DevServerID: setup.DevServerID,
    })
    if err != nil {
        _ = uc.repo.SetStatus(ctx, tenantID, setupID, domain.HostSetupFailed)
        return domain.HostSetup{}, domain.Project{}, err
    }
    if _, err := uc.projects.AddRepo(ctx, tenantID, project.ID, domain.RepoCreateInput{Path: setup.FolderPath}); err != nil {
        return domain.HostSetup{}, domain.Project{}, err
    }
    return uc.repo.Complete(ctx, tenantID, setupID, project.ID)
}
```

**Open dependency, called out explicitly**: `fs.checkPath` is this
solution's proposed JSON-RPC method name relayed via
`infra-fleet-service.Relay` — same caveat as SOL-021's `fs.scanNestedRepos`,
Agent-side support needs its own confirmation, not assumed.

---

## Design — `wscompat` wiring (all 5 channels)

```go
// ── projectHostSetup.* ─────────────────────────────────────────────────

func registerProjectHostSetupChannels(r *Registry, client projectv1.ProjectServiceClient) {
    r.Register("projectHostSetup.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            DevServerID string `json:"devServerId"`
            FolderPath  string `json:"folderPath"`
            DisplayName string `json:"displayName"`
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.CreateHostSetup(rpcCtx, &projectv1.CreateHostSetupRequest{
            DevServerId: in.DevServerID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetSetup(), nil
    })

    r.Register("projectHostSetup.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListHostSetups(rpcCtx, &projectv1.ListHostSetupsRequest{})
        if err != nil {
            return nil, err
        }
        return resp.GetSetups(), nil
    })

    r.Register("projectHostSetup.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ID          string `json:"id"`
            FolderPath  string `json:"folderPath"`
            DisplayName string `json:"displayName"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateHostSetup(rpcCtx, &projectv1.UpdateHostSetupRequest{
            Id: in.ID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetSetup(), nil
    })

    r.Register("projectHostSetup.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type deleteArgs struct {
            ID string `json:"id"`
        }
        in, err := decodeArg[deleteArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        if _, err := client.DeleteHostSetup(rpcCtx, &projectv1.DeleteHostSetupRequest{Id: in.ID}); err != nil {
            return nil, err
        }
        return map[string]bool{"ok": true}, nil
    })

    r.Register("projectHostSetup.setupExistingFolder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type setupArgs struct {
            ID string `json:"id"`
        }
        in, err := decodeArg[setupArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        // Same reasoning as SOL-021's projectGroup.scanNested — a remote
        // path check/finalize round-trip can exceed the standard rpcTimeout.
        rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
        resp, err := client.SetupExistingFolder(rpcCtx, &projectv1.SetupExistingFolderRequest{Id: in.ID})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })
}
```

`RegisterRealChannels` gains a `registerProjectHostSetupChannels(r,
projectClient)` call, reusing the same `projectv1.ProjectServiceClient` as
SOL-020/SOL-021.

---

## Test plan

- `services/project-service/internal/domain/host_setup_test.go` — status
  transitions (`pending → validated/failed → completed`); `Complete`
  requires a non-empty `project_id`.
- `services/project-service/internal/usecase/create_host_setup_test.go` —
  `dev_server_id` validated via a fake `infra-fleet-service` client; an
  unknown `dev_server_id` is rejected before any row is written.
- `services/project-service/internal/usecase/setup_existing_folder_test.go`
  — fake `DevServerRelay`: a path-check failure marks the setup `failed`
  and does **not** create a `Project`; a path-check success creates exactly
  one `Project` + one `Repo` and marks the setup `completed` with
  `project_id` set; assert no code path calls anything resembling a local
  `os.Stat` on `folder_path` (the regression this solution exists to
  prevent — grep-based lint or a fake filesystem port that panics if
  touched is one way to enforce this in CI).
- `services/project-service/internal/adapter/postgres/host_setup_repository_test.go`
  — `testcontainers-go`: tenant isolation (a setup from tenant A is
  `NOT_FOUND` for tenant B), `project_id` FK `ON DELETE SET NULL` behavior
  if the finalized project is later deleted.
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — 5
  tests; `setupExistingFolder`'s longer timeout asserted the same way as
  SOL-021's `scanNested` test.

## References

- `specs/backend-go/tdd/services/project-service.md §1,§3,§4,§5,§7` — `Project.dev_server_id`/`Repo` model and the existing `devServerId`-validation saga pattern this solution's ownership decision and schema extend
- `specs/backend-go/tdd/services/infra-fleet-service.md §1,§2` — reachability-only scope, why it does not own this namespace
- `specs/backend-go/tdd/architecture/05-data-architecture.md:34-36` — the "logical reference, validated by calling the owning service" convention applied to `dev_server_id` here
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — standard layout/ports convention this new table follows
- `specs/backend-go/bugs/missing-v1/solutions/SOL-021-projectgroup-channels.md` — shares the `DevServerRelay` port and the dev-server-not-backend-host relay principle
- `backend-go/proto/orca/project/v1/project.proto` — current RPC surface `AddRepoRequest`'s `path`-field gap noted against
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:11,25-31` — `RegisterDevServer`/`Relay`, reused as-is (no new infra-fleet-service RPC needed)
- `specs/backend-go/bugs/missing-v1/BUG-022-projecthostsetup-channels-not-implemented.md` — the bug this resolves; its ownership investigation this solution settles
- `specs/backend-go/bugs/missing-v1/BUG-021-projectgroup-channels-not-implemented.md` — sibling "legacy desktop-app assumption" finding this solution applies identically
