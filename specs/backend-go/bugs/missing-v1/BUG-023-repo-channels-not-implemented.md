# BUG-023: `repo.*` channels not implemented in backend-go

**Service:** `api-gateway` (dispatch) — likely split between `project-service` and `git-gateway-service` (see below)
**File:** `internal/adapter/wscompat/channels.go`
**Severity:** High — `repo.list`/`repo.add`/`repo.clone` are the core onboarding flow (adding your first repo); every method in this namespace currently fails.
**Symptom:** Every `repo.*` `callRuntimeRpc` from the renderer falls through to `registry.go`'s `notImplementedHandler` and returns `channel "repo.X" is not yet implemented in backend-go`.
**Status:** ✅ Resolved — see TASK-151–161 (11 task(s), all DONE) for implementation evidence.

---

## Description

`grep -n '"repo\.' internal/adapter/wscompat/channels.go` returns **zero matches** —
none of the 13 `repo.*` methods frontend calls are registered. `RegisterRealChannels`
(channels.go:64-82) wires 8 `register*Channels` functions; none of them touch `repo.*`.

Unlike `git.*` (2/34 methods wired) or `devServer.*` (2/3 wired), `repo.*` has **no
partial coverage at all** — this is a fully unimplemented namespace, not a gap in an
otherwise-wired one.

The namespace does not split cleanly onto one owning service. Checking both plausible
owners' protos:

- `project.proto`'s `ProjectService` (`proto/orca/project/v1/project.proto:26-29`) has a
  real "Repo catalog" surface: `AddRepo`, `ListRepos`, `ReorderRepos`, `RemoveRepo`
  (`project.proto:144-173`), each taking/returning a `Repo{id, project_id, url,
  display_name, position}` — this is catalog CRUD against Postgres, matching
  `repo.add`/`repo.list`/`repo.reorder`/`repo.rm` closely in shape.
- `gitgateway.proto`'s `GitGatewayService` (`proto/orca/gitgateway/v1/gitgateway.proto:10-17`)
  exposes only `GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`, `GenerateCommitMessage` —
  **no** `Clone`, `BaseRefDefault`, or `SearchRefs` RPC exists anywhere in this proto.

So of the 13 methods:

- **4 have a plausible backing RPC today** (`repo.add`→`AddRepo`, `repo.list`→`ListRepos`,
  `repo.reorder`→`ReorderRepos`, `repo.rm`→`RemoveRepo`), all on `project-service` — but
  even these are unregistered in `wscompat`, *and* `project-service`'s gRPC client
  (`projectClient`, dialed at `cmd/server/main.go:168`) is never passed into
  `wscompat.RegisterRealChannels` (`main.go:241` passes only `annotationClient,
  taskClient, gitClient, automationClient, infraFleetClient, rateLimiter`) — so wiring
  these 4 channels requires a `RegisterRealChannels` signature change, not just a new
  `register*Channels` function.
- **`repo.update` has no backing RPC** — `project.proto`'s Repo surface has no
  `UpdateRepo`; only whole-record `AddRepo`/`RemoveRepo`/`ReorderRepos` exist.
- **8 methods have no backing RPC anywhere in backend-go's current protos**:
  `repo.baseRefDefault`, `repo.clone`, `repo.create`, `repo.hooksCheck`,
  `repo.issueCommandRead`, `repo.issueCommandWrite`, `repo.searchRefs`,
  `repo.setupScriptImports`. These are git-working-tree-shaped (clone, ref
  resolution, git hooks, `.orca`/setup-script import scanning) and would most
  naturally extend `gitgateway.proto`, but no such RPCs exist yet on
  `GitGatewayService` (`gitgateway.proto:10-17`).

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `repo.add` | `renderer/src/store/slices/repos.ts` | Plausible backing: `ProjectService.AddRepo` (`project.proto:144-152`). Not wired in wscompat; `projectClient` not passed to `RegisterRealChannels`. |
| `repo.baseRefDefault` | `renderer/src/runtime/runtime-repo-client.ts` | No backing RPC on `GitGatewayService` or `ProjectService`. |
| `repo.clone` | `renderer/src/components/onboarding/use-onboarding-flow.ts`, `renderer/src/components/sidebar/useAddRepoCloneFlow.ts` | No `Clone` RPC on `gitgateway.proto` (`gitgateway.proto:10-17`). Core onboarding path. |
| `repo.create` | `renderer/src/components/sidebar/useCreateRepo.ts` | No backing RPC — `AddRepo` registers an existing repo URL, not "create new repo". |
| `repo.hooksCheck` | `renderer/src/runtime/runtime-hooks-client.ts` | No backing RPC anywhere. |
| `repo.issueCommandRead` | `renderer/src/runtime/runtime-hooks-client.ts` | No backing RPC anywhere. |
| `repo.issueCommandWrite` | `renderer/src/runtime/runtime-hooks-client.ts` | No backing RPC anywhere. |
| `repo.list` | `renderer/src/store/slices/repos.ts` | Plausible backing: `ProjectService.ListRepos` (`project.proto:154-160`). Not wired. |
| `repo.reorder` | `renderer/src/store/slices/repos.ts` | Plausible backing: `ProjectService.ReorderRepos` (`project.proto:162-167`). Not wired. |
| `repo.rm` | `renderer/src/store/slices/repos.ts` | Plausible backing: `ProjectService.RemoveRepo` (`project.proto:169-173`). Not wired. |
| `repo.searchRefs` | `renderer/src/runtime/runtime-repo-client.ts` | No backing RPC on `GitGatewayService`. |
| `repo.setupScriptImports` | `renderer/src/runtime/runtime-hooks-client.ts` | No backing RPC anywhere. |
| `repo.update` | `renderer/src/store/slices/github.ts`, `renderer/src/store/slices/repos.ts` | No backing RPC — `project.proto`'s Repo surface has no `UpdateRepo`. |

---

## Dispatch model

`specs/frontend/api/rpc-catalog.md:59` describes `repo` as: "Repo catalog
(list/add/clone/update), the legacy single-user desktop model." The namespace is
**not** broken out as its own row in
`specs/frontend/api/backend-agent-execution-boundary.md`'s dispatch tables (confirmed:
no `repo.` match in that file outside of prose at line 31).

Based on the closely related `git.*`/`worktree.*` rows in that doc (own inference, not
a direct quote):

- `repo.clone` and any future clone/ref-resolution methods would plausibly follow the
  same 🔀 dynamic-dispatch pattern documented for `git.*`
  (`backend-agent-execution-boundary.md:103`) and `worktree.*`
  (`backend-agent-execution-boundary.md:105`) — relay to the Dev Server Agent
  per-worktree when `connectionId` is set on the owning connection, execute locally
  otherwise.
- Catalog CRUD (`repo.add`/`list`/`reorder`/`rm`/`update`) looks 🟢 always
  backend+Postgres, matching the `devServer.list`/`add`/`remove` row
  (`backend-agent-execution-boundary.md:109`, "CRUD against the blob") and consistent
  with `project.proto`'s `Repo` message being a plain Postgres-backed record with no
  host/connection field.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:64-82` — `RegisterRealChannels` (no `repo.*` registration, no `projectClient` param)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/cmd/server/main.go:168` — `projectClient := projectv1.NewProjectServiceClient(projectConn)` (dialed but never passed to wscompat)
- `backend-go/services/api-gateway/cmd/server/main.go:241` — `wscompat.RegisterRealChannels(...)` call site, missing `projectClient`/`gitClient`-for-repo args
- `backend-go/proto/orca/project/v1/project.proto:26-29,144-173` — `ProjectService`'s Repo surface (`AddRepo`/`ListRepos`/`ReorderRepos`/`RemoveRepo`)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-17` — `GitGatewayService` (no clone/baseRefDefault/searchRefs RPC)
- `specs/frontend/api/rpc-catalog.md:59,402-418` — `repo.*` catalog entry and full method table
- `specs/frontend/api/backend-agent-execution-boundary.md:103,105,109` — `git.*`/`worktree.*`/`devServer.list` dispatch rows used as inference basis
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report this follows the shape of
