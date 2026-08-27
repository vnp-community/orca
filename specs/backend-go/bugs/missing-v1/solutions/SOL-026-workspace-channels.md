# SOL-026: `workspace.refreshFileTree` as a thin wrapper over `files.*`'s directory-listing RPC — blocked on that namespace's own solution landing first

**Resolves:** [BUG-026](../BUG-026-workspace-channels-not-implemented.md)
**Service:** `api-gateway` only — wraps whichever service ends up owning `files.*`
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Status:** 📋 Proposed — blocked on `files.*` (BUG-009) getting a backing RPC; not yet implemented

---

## Scope: one method, kept short per this bug's own framing

`workspace.refreshFileTree` is `rpc-catalog.md`'s own words "mostly
superseded by `files.*`/`git.*`" — this solution does not redesign either
of those namespaces (BUG-009/BUG-032 cover them, with their own solutions
to follow independently). It only specifies what `workspace.refreshFileTree`
should call once one of them exists.

## What the frontend actually needs

`WorkspaceContext.tsx:128` calls it as:

```ts
callRuntimeRpc<BackendFileTreeNode[]>(target, 'workspace.refreshFileTree', { projectId, path: '.' })
```

expecting a **flat array** of one directory level's entries,
`{ name, path, isDir, children? }` (`WorkspaceContext.tsx:26-31`) — the
frontend's own adapter comment confirms this is a shape mismatch it
already massages client-side ("returns a flat array of the requested
dir's entries (`isDir`-shaped), not a single rooted `FileNode`
(`type`-shaped)", `WorkspaceContext.tsx:22-24`), so the backend RPC does
not need to match `FileNode`'s shape — only `BackendFileTreeNode`'s.

This is a directory listing scoped to `{ projectId, path }`, i.e. exactly
the shape a `files.list`/`files.readDir`-equivalent RPC would return once
BUG-009 gives `files.*` a backing service. Per BUG-026's own dispatch
inference (`backend-agent-execution-boundary.md:103-104`'s 🔀 rows for
`git.*`/`files.*`), that RPC would already resolve local-vs-relay per
worktree — `workspace.refreshFileTree` inherits that dispatch for free by
delegating to it, rather than re-implementing resolve-and-dispatch a
second time for one legacy method name.

## Design — thin wrapper, once `files.*` (or its equivalent) exists

```go
// ── workspace.refreshFileTree ───────────────────────────────────────────
//
// Legacy method name, superseded by files.* per rpc-catalog.md:481-485 —
// this handler is a pure rename/reshape wrapper over files.list (BUG-009's
// eventual RPC), not an independent implementation. Do not add
// worktree-resolution or dispatch logic here; that lives in whichever
// usecase backs files.list.
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

`toBackendFileTreeNodes` is the one piece of real logic here — mapping
whatever `files.list`'s proto message shape turns out to be onto
`{name, path, isDir, children?}` exactly. Keep it in this file (not
shared) since it exists only to serve this one legacy channel's contract.

## Why this is blocked, not just deferred

There is currently **no** `files.*`-owning service or RPC anywhere in
backend-go (confirmed by BUG-009: `grep -rn 'ReadFile\|WriteFile'` across
every `backend-go/proto/` package returns nothing, and
`NewDefaultServiceRegistry()` has no `/v1/files` routing rule). Wiring
`workspace.refreshFileTree` before that lands would mean inventing a
second, throwaway directory-listing RPC just for this legacy name —
duplicating whatever `files.*`'s solution designs properly, and leaving
two divergent file-tree implementations to keep in sync. This solution's
recommendation is: implement `files.*`'s own solution first (its own
bug/solution pair), then land this wrapper as a small follow-up PR against
that RPC — not to build a parallel path here.

If `files.*`'s solution ends up owned by `git-gateway-service` instead of
a new `files-service` (both are plausible per BUG-009's "worktree-scoped
I/O" framing), the wrapper above changes only its client type and RPC
name — the shape-mapping logic and dispatch delegation reasoning are
unaffected.

## Test plan

- Once `files.list` (or equivalent) exists: `channels_test.go` — a single
  test asserting `workspace.refreshFileTree` calls `files.list` with the
  decoded `projectId`/`path` and reshapes the response into
  `{name, path, isDir, children}`, fake `FilesServiceClient`.
- No independent usecase/proto test needed — this solution adds zero new
  business logic; all real behavior (dispatch, tenant scoping, host
  resolution) is `files.*`'s own solution's responsibility and tested
  there.

## References

- `frontend/src/renderer/src/context/WorkspaceContext.tsx:19-37,122-132` — `BackendFileTreeNode` shape, actual call site and args, frontend-side reshape comment
- `specs/frontend/api/rpc-catalog.md:481-485` — `workspace.*` catalog entry, "mostly superseded by files.*/git.*"
- `specs/frontend/api/backend-agent-execution-boundary.md:103-104` — 🔀 dispatch rows for `git.*`/`files.*`, inherited by delegation rather than reimplemented
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — confirms no `files.*`-owning service/RPC exists yet; this solution's blocking dependency
- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — sibling namespace's own partial-implementation state, referenced only for context on how `git.*`'s wiring gaps get closed incrementally (same "wrapper-only once the RPC exists" pattern this solution follows)
- `specs/backend-go/bugs/missing-v1/BUG-026-workspace-channels-not-implemented.md` — this bug's own analysis, confirming no backing RPC exists under `workspace.*`'s own name or either stated successor namespace
