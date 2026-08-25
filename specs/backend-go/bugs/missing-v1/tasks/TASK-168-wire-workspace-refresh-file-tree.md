# TASK-168: Wire `workspace.refreshFileTree` as a thin wrapper over `files.*`'s directory-listing RPC

**From Solution:** SOL-026
**Priority:** P3 — blocked, do not start until its dependency lands
**Service:** `api-gateway` only — wraps whichever service ends up owning `files.*`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** SOL-009's `files.*` directory-listing RPC (informational — SOL-009 is a different bug/solution outside this task range; that RPC does not exist yet and this task cannot be started until it does. Do not treat this as a TASK-1xx number — no such number is known from here.)
**Status:** `[blocked]` Confirmed still blocked in this worktree/pass — grepped the full `backend-go/proto/` tree and every service's `internal/` for any `files.*`-owning RPC, `FilesServiceClient`, `ReadFile`/`WriteFile`/`ListFiles`-shaped RPC, or `/v1/files` route; none exist. Not implemented, per task instructions — implementing against a placeholder RPC would invent a throwaway parallel directory-listing path that duplicates whatever SOL-009's own solution lands. No code changes made for this task.

---

## Context

`workspace.refreshFileTree` is `rpc-catalog.md`'s own words "mostly
superseded by `files.*`/`git.*`". This task does not redesign either of
those namespaces — SOL-009 covers `files.*` with its own solution and
task breakdown. It only specifies what `workspace.refreshFileTree` should
call **once** a `files.*`-backing RPC exists.

**Why this is blocked, not just deferred:** there is currently no
`files.*`-owning service or RPC anywhere in `backend-go` (confirmed by
BUG-009: no `ReadFile`/`WriteFile` RPC in any `backend-go/proto/` package,
and no `/v1/files` routing rule in the service registry). Wiring
`workspace.refreshFileTree` before that lands would mean inventing a
second, throwaway directory-listing RPC just for this legacy name —
duplicating whatever `files.*`'s own solution designs, and leaving two
divergent file-tree implementations to keep in sync. **Do not implement a
parallel path here** — land `files.*`'s own solution first, then this
wrapper as a small follow-up.

## What the frontend actually needs

`WorkspaceContext.tsx:128` calls it as:

```ts
callRuntimeRpc<BackendFileTreeNode[]>(target, 'workspace.refreshFileTree', { projectId, path: '.' })
```

expecting a **flat array** of one directory level's entries,
`{ name, path, isDir, children? }` (`WorkspaceContext.tsx:26-31`) — the
frontend's own adapter comment confirms this shape mismatch is already
massaged client-side, so the backend RPC does not need to match
`FileNode`'s shape (`type`-discriminated), only `BackendFileTreeNode`'s
(`isDir`-shaped).

## Changes to make (once `files.*`'s backing RPC exists)

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

```go
// ── workspace.refreshFileTree ───────────────────────────────────────────
//
// Legacy method name, superseded by files.* per rpc-catalog.md:481-485 —
// this handler is a pure rename/reshape wrapper over files.list (or
// whichever RPC SOL-009's own task breakdown lands under that name), not
// an independent implementation. Do not add worktree-resolution or
// dispatch logic here; that lives in whichever usecase backs the files.*
// RPC this wraps.
func registerWorkspaceChannels(r *Registry, filesClient filesv1.FilesServiceClient) {
	r.Register("workspace.refreshFileTree", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type refreshArgs struct {
			ProjectID string `json:"projectId"`
			Path      string `json:"path"`
		}
		in, err := decodeArg[refreshArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := filesClient.List(rpcCtx, &filesv1.ListRequest{
			ProjectId: in.ProjectID,
			Path:      in.Path,
		})
		if err != nil {
			return nil, err
		}
		return toBackendFileTreeNodes(resp.GetEntries()), nil // {name, path, isDir, children?} per WorkspaceContext.tsx:26-31
	})
}
```

`filesv1.FilesServiceClient`/`filesv1.ListRequest`/`.List(...)` are
placeholder names above — **replace them with whatever SOL-009's actual
implementation lands as** (client package import path, request message
name, RPC method name, and field names for `projectId`/`path` may all
differ from this sketch). Do not implement this task's code block
verbatim without first checking the real generated stubs SOL-009
produces.

`toBackendFileTreeNodes` is the one piece of real logic here — mapping
whatever the real RPC's response message shape turns out to be onto
`{name, path, isDir, children?}` exactly. Keep it in this file (not
shared) since it exists only to serve this one legacy channel's contract.

Register from `RegisterRealChannels`, next to whichever other channel
group is topically closest once wired — no fixed position required since
this is a single, independent handler.

If `files.*`'s solution ends up owned by `git-gateway-service` instead of
a new `files-service` (both are plausible per BUG-009's "worktree-scoped
I/O" framing), this wrapper changes only its client type and RPC name —
the shape-mapping logic is unaffected.

## Test plan (once `files.*`'s backing RPC exists)

- `channels_test.go` — a single test asserting `workspace.refreshFileTree`
  calls the real `files.*` RPC with the decoded `projectId`/`path` and
  reshapes the response into `{name, path, isDir, children}`, fake
  `FilesServiceClient`.
- No independent usecase/proto test needed — this task adds zero new
  business logic; all real behavior (dispatch, tenant scoping, host
  resolution) belongs to `files.*`'s own solution and is tested there.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Do not attempt to build or merge this task until the `files.*` RPC it
wraps actually exists — the code above will not compile against today's
`backend-go` codebase.
