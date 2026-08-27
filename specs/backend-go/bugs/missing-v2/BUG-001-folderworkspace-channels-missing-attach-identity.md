# BUG-001: `folderWorkspace.*` channels never attach caller identity — every call fails with `PROJECT_NO_TENANT`

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_emulator_folderworkspace_host.go`
**Severity:** High — the entire `folderWorkspace.*` namespace (5/5 methods) is unusable end-to-end despite being "wired"
**Symptom:** `folderWorkspace.list` (and every other `folderWorkspace.*` channel) returns `{"ok":false,"error":{"message":"rpc error: code = Unauthenticated desc = PROJECT_NO_TENANT: no tenant in request context"}}` for an authenticated, cookie-valid caller.
**Status:** 🔴 Open — found live 2026-08-27 via `tests/client/rpc-catalog.spec.ts`, root-caused by direct code comparison.

---

## Description

`project-service`'s usecases all guard on `tenant.RequireTenantID(ctx)` — every
usecase in `internal/usecase/*.go` calls it first and returns
`PROJECT_NO_TENANT` if the tenant isn't present in the gRPC request's
context. For this to succeed, the calling side (`wscompat`'s channel
handler) must attach the caller's resolved `Identity` (tenant + user, taken
from the validated session cookie) onto the outgoing gRPC context before
invoking the client method — every OTHER `wscompat` file that calls into
`project-service` does this via `gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})`.

`channels_emulator_folderworkspace_host.go`'s five `folderWorkspace.*`
handlers never do this — they pass the bare, unmodified incoming `ctx`
straight to the gRPC client call:

```go
// channels_emulator_folderworkspace_host.go:340-346
r.Register("folderWorkspace.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
	resp, err := client.ListFolderWorkspaces(ctx, &projectv1.ListFolderWorkspacesRequest{})
	// ...
})
```

vs. the sibling pattern this file omits (verified present in every other
`project-service`-calling file):

```go
// channels_tenant_project.go:216-234 (project.list, for contrast)
r.Register("project.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	// ...
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := client.ListProjects(rpcCtx, &projectv1.ListProjectsRequest{
		TenantId: id.TenantID, PageToken: in.PageToken, PageSize: in.PageSize,
	})
	// ...
})
```

All five handlers in the affected file have the same gap:

| Channel | Line | Missing `AttachIdentity` |
|---|---|---|
| `folderWorkspace.create` | `channels_emulator_folderworkspace_host.go:291` | ✅ missing |
| `folderWorkspace.update` | `channels_emulator_folderworkspace_host.go:310` | ✅ missing |
| `folderWorkspace.delete` | `channels_emulator_folderworkspace_host.go:326` | ✅ missing |
| `folderWorkspace.list` | `channels_emulator_folderworkspace_host.go:340` | ✅ missing |
| `folderWorkspace.getPathStatus` | `channels_emulator_folderworkspace_host.go:348` | ✅ missing |

## Confirmed

- `backend-go/services/project-service/internal/usecase/folder_workspace.go:58` —
  `Create`'s first line is `tenant.RequireTenantID(ctx)`, erroring
  `PROJECT_NO_TENANT` on failure. Same guard at lines 144 (`List`) and 161
  (`GetPathStatus`).
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_emulator_folderworkspace_host.go:291-370` —
  none of the 5 `folderWorkspace.*` registrations call `gatewaygrpc.AttachIdentity`.
- Contrast confirmed against 3 sibling files that DO call it for every
  `project-service`-bound handler: `channels_tenant_project.go:227`
  (`project.list`), `channels_repo_ssh_status_workspace.go:98` (`repo.list`),
  `channels_worktree.go:167` (`worktree.list`).
- Live-verified 2026-08-27 against `172.20.2.39:6769`: `folderWorkspace.list`
  called with a valid cookie-authenticated session returns
  `PROJECT_NO_TENANT` while, in the same session, `project.list` (which DOES
  attach identity) reaches a different, later failure (`PROJECT_LIST_FAILED`
  — see BUG-003) instead of `PROJECT_NO_TENANT`. Same caller, same session,
  different channel, different failure mode — isolates the bug to the
  channel handler, not the caller's auth state.

## Suggested Fix

Add the same `ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})`
line (and ideally the same `rpcTimeout` context deadline every sibling
handler applies) to all 5 `folderWorkspace.*` registrations before their
gRPC client call.

## Regression Test Gap

`channels_repo_ssh_status_workspace_test.go`'s pattern of asserting
`gotReq` shape via a fake client wouldn't have caught this — the request
message itself is correct; only the *context* is wrong. A test needs to
assert on the context `AttachIdentity` populates (e.g. a fake
`project-service` client that reads identity back out of `ctx` and errors
if absent), not just the request struct.
