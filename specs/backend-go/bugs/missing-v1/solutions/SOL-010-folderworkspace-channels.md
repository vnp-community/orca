# SOL-010: Add `FolderWorkspace` CRUD to `project-service`

**Resolves:** [BUG-010](../BUG-010-folderworkspace-channels-not-implemented.md)
**Service:** `project-service` (new table + RPCs) + `api-gateway`
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto`
- `backend-go/services/project-service/internal/domain/folder_workspace.go`
- `backend-go/services/project-service/internal/usecase/create_folder_workspace.go`, `update_folder_workspace.go`, `delete_folder_workspace.go`, `list_folder_workspaces.go`, `get_folder_workspace_path_status.go`
- `backend-go/services/project-service/internal/usecase/ports.go` (add `FolderWorkspaceRepository`)
- `backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go`
- `backend-go/services/project-service/migrations/NNNN_create_folder_workspaces.sql`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerFolderWorkspaceChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## `project-service` is the right home, but `FolderWorkspace` is a new table, not a repurposed `ProjectGroup`

BUG-010 already did the domain analysis: `project-service` owns
`/v1/projects` and has a structurally similar folder-tree concept
(`ProjectGroup`), but flagged that `ProjectGroup` and `FolderWorkspace` are
distinct concepts — "project-organizing folders vs. non-git folder
workspaces added to the workspace." `project-service.md` §4 confirms the
distinction from the target-design side: `ProjectGroup` is "id, project_id
(nullable...), parent_group_id (self-referential tree), name" — a
*grouping* entity with no `path` field at all, used to organize `Project`
records into a tree. `folderWorkspace.*`, per its frontend call sites
(`repos.ts`), manages standalone filesystem paths added directly to the
workspace, independent of any `Project` — closer in shape to `project-service`'s
**`Repo`** entity (§4: "id, project_id, host-relative absolute path... Pure
metadata, no working-tree state") than to `ProjectGroup`, except a
`FolderWorkspace` is not git-backed and has no owning `Project` — it's
added standalone, the same way `repos.ts`'s own naming (a sibling store
next to project-scoped repos) suggests.

**This solution proposes a new, standalone `folder_workspaces` table**,
following `Repo`'s column shape (§5's `repos` table) with the `project_id`
FK dropped and a `dev_server_id` pointer added in its place — a folder
workspace needs to know *which host* the path lives on (the same logical
pointer `Project.dev_server_id` provides), since it has no project to
inherit that from. Not a repurposing of `ProjectGroup` — that would
conflate two concepts BUG-010 already found were distinct, and would force
every `ProjectGroup` consumer to handle a nullable `path` it never has
today.

---

## Design — Proto additions (`project.proto`)

```protobuf
service ProjectService {
  // ... existing RPCs unchanged ...

  // ── Folder workspaces — standalone, non-git filesystem paths added
  // directly to the workspace (repos.ts's sibling store to project-scoped
  // Repo entries). No project_id — see design note above. ─────────────
  rpc CreateFolderWorkspace(CreateFolderWorkspaceRequest) returns (FolderWorkspace);
  rpc UpdateFolderWorkspace(UpdateFolderWorkspaceRequest) returns (FolderWorkspace);
  rpc DeleteFolderWorkspace(DeleteFolderWorkspaceRequest) returns (google.protobuf.Empty);
  rpc ListFolderWorkspaces(ListFolderWorkspacesRequest) returns (ListFolderWorkspacesResponse);
  rpc GetFolderWorkspacePathStatus(GetFolderWorkspacePathStatusRequest) returns (GetFolderWorkspacePathStatusResponse);
}

message FolderWorkspace {
  string id = 1;
  string dev_server_id = 2;   // logical FK -> infra-fleet-service.dev_servers
  string path = 3;            // absolute path on the bound dev server
  string name = 4;            // display name (defaults to basename(path) if unset on create)
  string added_by = 5;        // logical FK -> tenant-service.users
  google.protobuf.Timestamp created_at = 6;
}

message CreateFolderWorkspaceRequest {
  string dev_server_id = 1;
  string path = 2;
  string name = 3;
}

message UpdateFolderWorkspaceRequest {
  string id = 1;
  string name = 2;   // the only mutable field — path/dev_server_id are re-add, not edit, same posture as project-service's dev_server_id rebind guard
}

message DeleteFolderWorkspaceRequest { string id = 1; }
message ListFolderWorkspacesRequest {}   // tenant-scoped, no filter params observed at the frontend call site
message ListFolderWorkspacesResponse { repeated FolderWorkspace folder_workspaces = 1; }

message GetFolderWorkspacePathStatusRequest {
  string dev_server_id = 1;
  string path = 2;
}
message GetFolderWorkspacePathStatusResponse {
  // PATH_STATUS_AVAILABLE | PATH_STATUS_ALREADY_FOLDER_WORKSPACE |
  // PATH_STATUS_ALREADY_REPO | PATH_STATUS_INVALID
  string status = 1;
  string existing_folder_workspace_id = 2;  // set when status == ALREADY_FOLDER_WORKSPACE
}
```

### `GetPathStatus` is a DB-conflict check, not a live filesystem probe

BUG-010's dispatch-model finding is explicit: `folderWorkspace.*` is
🟢 Postgres-only in the old backend, with **no** relay to the Dev Server
Agent — unlike `projectGroup.scanNested`/`importNested` (a different
namespace), `getPathStatus` never did a real `stat()` against the target
host even in TS. This solution preserves that: `GetFolderWorkspacePathStatus`
answers purely from `project-service`'s own tables — is this
`(dev_server_id, path)` pair already registered as a folder workspace
(`ALREADY_FOLDER_WORKSPACE`), or does it collide with an existing `Repo`
row for the same host (`ALREADY_REPO`, avoiding a git repo silently
double-added as a plain folder) — plus a cheap format check (`INVALID` for
a non-absolute path), never an actual filesystem existence check. This is
a **known, intentional divergence from what "path status" might suggest**
— flagged here explicitly rather than silently narrowing the contract:
callers should not read `PATH_STATUS_AVAILABLE` as "this directory exists
on disk," only as "nothing in `project-service` already claims it." If a
real fs-existence check is later wanted, it would need to relay through
`infra-fleet-service`/the Dev Server Agent (the same pattern `files.stat`
uses per SOL-009) — a deliberate scope expansion beyond what BUG-010's
dispatch-model finding calls for, not something this solution adds
speculatively.

---

## Design — Data model (Postgres, `project` schema)

Follows `project-service.md` §5's `repos` table shape directly (see
comparison above), with `project_id` dropped and `dev_server_id` added —
same pattern the doc itself uses for `projects.dev_server_id`:

```sql
CREATE TABLE folder_workspaces (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL,        -- logical FK -> infra-fleet-service.dev_servers
  path           TEXT NOT NULL,
  name           TEXT NOT NULL,
  added_by       UUID NOT NULL,        -- logical FK -> tenant-service.users
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, dev_server_id, path)
);
CREATE INDEX idx_folder_workspaces_tenant ON folder_workspaces (tenant_id);
```

RLS enabled (`tenant_id`-scoped), matching every other table in this
service per `05-data-architecture.md` — same posture §5 already documents
for `projects`/`repos`/`worktrees`. The `UNIQUE (tenant_id, dev_server_id,
path)` constraint is what `GetFolderWorkspacePathStatus`'s
`ALREADY_FOLDER_WORKSPACE` check queries against, and what `CreateFolderWorkspace`
relies on at the DB layer as the authoritative conflict check (the RPC-level
`GetPathStatus` call is a UX convenience for pre-flight validation, not the
only enforcement point).

---

## Design — `usecase/` layer

Standard CRUD shape, no relay/dispatch branching (per the dispatch-model
finding above) — the simplest of the four namespaces in this batch. One
port added to `project-service`'s existing `ports.go`:

```go
// internal/usecase/ports.go (extended)
type FolderWorkspaceRepository interface {
    Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error)
    Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error)
    Delete(ctx context.Context, id string) error
    ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error)
    FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error)
    RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error) // cross-check against repos table
}
```

```go
// internal/usecase/create_folder_workspace.go
func (uc *FolderWorkspaceUseCase) Create(ctx context.Context, id Identity, in CreateFolderWorkspaceInput) (domain.FolderWorkspace, error) {
    if !filepath.IsAbs(in.Path) {
        return domain.FolderWorkspace{}, domain.ErrInvalidPath
    }
    fw := domain.FolderWorkspace{
        TenantID:    id.TenantID,
        DevServerID: in.DevServerID,
        Path:        filepath.Clean(in.Path),
        Name:        cmp.Or(in.Name, filepath.Base(in.Path)),
        AddedBy:     id.UserID,
    }
    // DB UNIQUE constraint is the real conflict guard; GetPathStatus is a
    // pre-flight convenience the frontend calls separately (per BUG-010's
    // "called twice" note) — this usecase still surfaces the same
    // ErrPathAlreadyRegistered on a constraint violation, not a generic 500.
    return uc.repo.Create(ctx, fw)
}
```

```go
// internal/usecase/get_folder_workspace_path_status.go
func (uc *FolderWorkspaceUseCase) GetPathStatus(ctx context.Context, id Identity, devServerID, path string) (domain.PathStatus, error) {
    if !filepath.IsAbs(path) {
        return domain.PathStatus{Status: domain.PathStatusInvalid}, nil
    }
    clean := filepath.Clean(path)
    if existing, err := uc.repo.FindByPath(ctx, id.TenantID, devServerID, clean); err != nil {
        return domain.PathStatus{}, err
    } else if existing != nil {
        return domain.PathStatus{Status: domain.PathStatusAlreadyFolderWorkspace, ExistingID: existing.ID}, nil
    }
    if isRepo, err := uc.repo.RepoPathExists(ctx, id.TenantID, devServerID, clean); err != nil {
        return domain.PathStatus{}, err
    } else if isRepo {
        return domain.PathStatus{Status: domain.PathStatusAlreadyRepo}, nil
    }
    return domain.PathStatus{Status: domain.PathStatusAvailable}, nil
}
```

Authorization: per `project-service.md` §9's posture ("`CreateProject`
requires only authentication"), `folder_workspaces` rows have no membership
model of their own (unlike `Project`) — any authenticated tenant member can
create/list/update/delete their own tenant's folder workspaces. `Delete`/
`Update` additionally check `added_by == caller` OR global admin, mirroring
the narrowest applicable rule from §9 rather than requiring a new OPA
policy shape for a single-owner-no-membership entity.

---

## Design — `wscompat` wiring

Straightforward CRUD dispatch, mirrors `registerAnnotationChannels`'s
existing four-method shape exactly (this namespace is almost the same
size — 5 methods vs. annotation's 4):

```go
// ── folderWorkspace.* ────────────────────────────────────────────────

func registerFolderWorkspaceChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("folderWorkspace.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateFolderWorkspace(ctx, &projectv1.CreateFolderWorkspaceRequest{
			DevServerId: in.DevServerID, Path: in.Path, Name: in.Name,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("folderWorkspace.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpdateFolderWorkspace(ctx, &projectv1.UpdateFolderWorkspaceRequest{Id: in.ID, Name: in.Name})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("folderWorkspace.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteFolderWorkspace(ctx, &projectv1.DeleteFolderWorkspaceRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("folderWorkspace.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		resp, err := client.ListFolderWorkspaces(ctx, &projectv1.ListFolderWorkspacesRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetFolderWorkspaces(), nil
	})

	r.Register("folderWorkspace.getPathStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type statusArgs struct {
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
		}
		in, err := decodeArg[statusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetFolderWorkspacePathStatus(ctx, &projectv1.GetFolderWorkspacePathStatusRequest{
			DevServerId: in.DevServerID, Path: in.Path,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

`RegisterRealChannels` gains `registerFolderWorkspaceChannels(r,
projectClient)` — `projectClient` is already constructed in `main.go`
(used for the existing `/v1/projects` REST routes), no new dial needed.
Unlike `devServer.*`/`fleet.*` (SOL for BUG elsewhere), no
`gatewaygrpc.AttachIdentity` call is shown above because `project-service`'s
existing RPCs take `tenant_id` via the inbound ctx the same way
`git.status`/`git.diff` do today (per `channels.go`'s own doc comment on
which services need explicit identity attachment vs. not) — verify this
against `project-service`'s actual interceptor behavior before wiring; if
it turns out to require explicit metadata like `infra-fleet-service` does,
add the same `AttachIdentity` call `registerDevServerChannels` uses.

---

## Test plan

- `project-service/internal/usecase/create_folder_workspace_test.go` —
  create then list round-trips; duplicate `(dev_server_id, path)` for the
  same tenant returns `ErrPathAlreadyRegistered`, not a generic DB error.
- `get_folder_workspace_path_status_test.go` — three cases: available (no
  conflict), `ALREADY_FOLDER_WORKSPACE` after a prior create, `ALREADY_REPO`
  against a fake `RepoPathExists` returning true; non-absolute path →
  `INVALID` without any repository call (assert the fake records zero
  calls, confirming no fs probe is attempted per the "not a live check"
  design note).
- `update_folder_workspace_test.go` / `delete_folder_workspace_test.go` —
  non-owner, non-admin caller rejected.
- `adapter/postgres/folder_workspace_repository_test.go` — `testcontainers-go`
  Postgres, `UNIQUE` constraint violation surfaces as a typed conflict
  error the usecase layer maps to `ErrPathAlreadyRegistered`.
- `wscompat/channels_test.go` — one test per channel, fake
  `ProjectServiceClient`, mirroring `annotation_channels_test.go`'s
  existing shape.

## References

- `specs/backend-go/bugs/missing-v1/BUG-010-folderworkspace-channels-not-implemented.md` — problem statement, the `ProjectGroup`-is-a-distinct-concept finding this solution builds on
- `specs/backend-go/tdd/services/project-service.md:74-119` (§3 API surface, RPC-list convention this extends), `:163-186` (§4 domain model, `Repo`/`ProjectGroup` shape comparison), `:190-261` (§5 data model, `repos` table this solution's `folder_workspaces` table mirrors), `:337-350` (§9 security notes, authorization posture applied above)
- `backend-go/proto/orca/project/v1/project.proto:12-45` — existing RPC list, no folder-workspace RPCs (confirms BUG-010's finding)
- `backend-go/services/project-service/internal/usecase/ports.go:131` — `ProjectGroupRepository`, the closest existing port this solution's `FolderWorkspaceRepository` sits alongside
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:84-175` — `registerAnnotationChannels`'s existing four-method CRUD pattern this solution's `registerFolderWorkspaceChannels` mirrors
- `backend-go/services/api-gateway/cmd/server/main.go:168` — `projectClient` already constructed, reused for `registerFolderWorkspaceChannels`
