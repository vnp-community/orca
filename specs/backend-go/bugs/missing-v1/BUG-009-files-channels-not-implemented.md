# BUG-009: `files.*` channels not implemented in backend-go

**Service:** none — no backend-go service exposes file read/write/stat RPCs; `git-gateway-service` is the closest candidate by worktree scoping but only implements git plumbing (status/diff/commit/push/pull/generate-commit-message)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** High — this is one of the largest namespaces (18 methods) and backs the file viewer, file search, and every file-tree mutation (create/rename/copy/delete) — core to daily app usage.
**Symptom:** Every `files.*` call from `runtime-file-client.ts` and the workspace file viewer/search/context-menu components times out with `channel "files.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ❌ Open

---

## Description

None of the 18 `files.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"files\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

No backend-go service implements file read/write/stat RPCs today. The
closest candidate by domain (worktree-scoped I/O) is `git-gateway-service`,
but its proto only exposes git plumbing:

```
$ grep -n 'rpc ' backend-go/proto/orca/gitgateway/v1/gitgateway.proto
11:  rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
12:  rpc GetDiff(GetDiffRequest) returns (GetDiffResponse);
13:  rpc Commit(CommitRequest) returns (CommitResponse);
14:  rpc Push(PushRequest) returns (PushResponse);
15:  rpc Pull(PullRequest) returns (PullResponse);
16:  rpc GenerateCommitMessage(GenerateCommitMessageRequest) returns (GenerateCommitMessageResponse);
```

(`backend-go/services/git-gateway-service/internal/usecase/` only has
`commit.go`, `generate_commit_message.go`, `get_diff.go`, `get_status.go`,
`pull.go`, `push.go` — no read/write/stat/dir-listing usecases.) A repo-wide
check for `ReadFile`/`WriteFile`-shaped RPCs across every `backend-go/proto/`
package returns nothing. This is a **service-doesn't-have-this-capability
gap**, not just a missing wscompat handler over an existing gRPC method —
whoever picks this up needs to design and build a new file-I/O RPC surface
(either as a new service or as an addition to `git-gateway-service`, given
both are worktree-scoped) before wscompat can wire it.

`registry.go`'s `NewDefaultServiceRegistry()`
(`backend-go/services/api-gateway/internal/domain/registry.go:82-101`) has no
`/v1/files` (or similar) routing rule either, consistent with no service
owning this.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `files.commitUpload` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:628` | Always-local bookkeeping in the old backend (no fs I/O) — see Dispatch model |
| `files.copy` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:458` | Failed for Dev-Server-backed repos even in the old backend (see Dispatch model) |
| `files.createDir` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:410,710` | |
| `files.createDirNoClobber` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:564` | |
| `files.delete` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:485,506,597,639`, `frontend/src/renderer/src/components/workspace/FileContextMenu.tsx:44` | |
| `files.listAll` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:755` | |
| `files.listMarkdownDocuments` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:795` | |
| `files.read` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:172`, `frontend/src/renderer/src/components/workspace/FileViewer.tsx:64` | |
| `files.readChunk` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:282,308` | Unsupported for ANY remote target even in the old backend, by design (see Dispatch model) |
| `files.readDir` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:370` | |
| `files.readPreview` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:183,211,326` | |
| `files.rename` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:430` | Failed for Dev-Server-backed repos even in the old backend (see Dispatch model) |
| `files.search` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:734`, `frontend/src/renderer/src/components/workspace/FileSearchPanel.tsx:35` | |
| `files.stat` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:815,1034` | |
| `files.unwatch` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:1012` | Always-local bookkeeping in the old backend (no fs I/O) |
| `files.write` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:389` | |
| `files.writeBase64` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:659` | |
| `files.writeBase64Chunk` | `frontend/src/renderer/src/runtime/runtime-file-client.ts:669` | |

None of these are registered anywhere in `channels.go`, confirmed by the grep
above — this is a full-namespace gap.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:104` (and the
namespace summary at line 12/43), the old TypeScript backend used the 🔀
**dynamic** pattern — the same shape as `git.*`: per worktree, decided at
call time by whether the repo has a `connectionId`:

- **Dev-Server-backed** (has a `connectionId`) → relayed to the Dev Server
  Agent over WS-RPC via `getRemoteFilesystemProvider` →
  `DevServerFilesystemProvider` (`agent/src/relay/agent-rpc-dispatch.ts`'s
  `fs.*` cases).
- **Purely local** → the backend executes fs I/O directly on its own host.

No Postgres is in the hot path either way — this is pure filesystem I/O,
local or relayed.

⚠️ **Known gaps in the OLD backend, carried here as architectural context
(not backend-go bugs to fix, just constraints to preserve or consciously
improve on)**:

- The Dev Server Agent's fs surface only implements
  `stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep` — it does **not**
  implement `rename`, `copy`, or `realpath`. So `files.rename` and
  `files.copy` always failed (`NOT_SUPPORTED`) for Dev-Server-backed repos,
  even in the old backend.
- `files.readChunk` was unsupported for **any** remote target (SSH or Dev
  Server) by design — not a bug, an intentional scope limit.
- `files.commitUpload` and `files.unwatch` (along with `files.open`,
  `files.openDiff`, and `files.browseServerDir`, which are outside this
  namespace's 18-method list) were always-local in the old backend — no fs
  I/O at all, just renderer-side notification/bookkeeping.

Whoever implements this in backend-go should decide up front whether to
reproduce the dynamic local/relay split (requiring a new relay-aware
usecase layer, likely alongside or inside `git-gateway-service` given the
worktree-scoping overlap) or take a different architecture — but should at
minimum preserve or explicitly document divergence from the three known
old-backend limitations above.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `files.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:82-101` — `NewDefaultServiceRegistry()`, no files routing rule
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:11-16` — closest candidate service's existing (non-file) RPCs
- `backend-go/services/git-gateway-service/internal/usecase/` — no file read/write/stat usecases
- `specs/frontend/api/backend-agent-execution-boundary.md:12,43,104` — `files.*` 🔀 dispatch classification and known gaps
- `specs/frontend/api/rpc-catalog.md:157-178` — `files.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
