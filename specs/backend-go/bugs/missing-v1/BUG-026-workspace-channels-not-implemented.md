# BUG-026: `workspace.*` channels not implemented in backend-go

**Service:** `api-gateway` (dispatch) — likely owner is whichever service ends up owning `files.*`/`git.*` (currently `git-gateway-service` for the latter; no `files.*`-owning service confirmed wired yet)
**File:** `internal/adapter/wscompat/channels.go`
**Severity:** Medium — called on every `WorkspaceContext` mount/refresh, so frequently hit despite being "legacy".
**Symptom:** `workspace.refreshFileTree` `callRuntimeRpc` calls fall through to `registry.go`'s `notImplementedHandler` and return `channel "workspace.refreshFileTree" is not yet implemented in backend-go`.
**Status:** ❌ Open

---

## Description

`grep -n '"workspace\.' internal/adapter/wscompat/channels.go` returns **zero
matches** — the single `workspace.*` method (`workspace.refreshFileTree`) is not
registered.

Per `specs/frontend/api/rpc-catalog.md:481-485`, `workspace.*` covers "Legacy
workspace init/teardown/file-tree/git-status (mostly superseded by
files.*/git.*)"; `workspace.refreshFileTree` is called from
`renderer/src/context/WorkspaceContext.tsx`.

`git.*` is partially wired in `wscompat` (`git.status`/`git.diff` only, via
`registerGitChannels`, `channels.go:221-256`, backed by `git-gateway-service`'s
`GitGatewayService`), but no file-tree-shaped RPC exists on
`gitgateway.proto` (`GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`,
`GenerateCommitMessage` only — `gitgateway.proto:10-17`) or on any other wired
proto checked for this audit (`project.proto`, `infrafleet.proto`). `files.*` — the
namespace this method is described as "superseded by" — has no `register*Channels`
function in `channels.go` either (confirmed by the same absence in the file's own
13-channel inventory comment, `channels.go:16-18`), and as of this report there is no
`BUG-0XX-files-channels-not-implemented.md` yet in
`specs/backend-go/bugs/missing-v1/` (directory listing at time of writing: only
`BUG-001`, `BUG-002`, `BUG-003`) — `files.*` is a closely related sibling gap, not
yet independently reported.

So `workspace.refreshFileTree` has no plausible backing RPC in backend-go today
under either its own name or its stated successor namespaces.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `workspace.refreshFileTree` | `renderer/src/context/WorkspaceContext.tsx` | No backing RPC on `GitGatewayService` (`gitgateway.proto:10-17`) or any other checked proto. Sibling namespace `files.*` (its stated successor) is also entirely unwired in `channels.go` and has no bug report yet in this directory. |

---

## Dispatch model

`specs/frontend/api/backend-agent-execution-boundary.md` groups `files.*` and
`git.*` under the same "Worktree/host-scoped — can relay to the Dev Server Agent per
call" section (`backend-agent-execution-boundary.md:99-118`), both marked 🔀 dynamic:

> `git.*` (34 methods) | 🔀 | ... (`backend-agent-execution-boundary.md:103`)
> `files.*` (~20 methods) | 🔀 | Same pattern via `getRemoteFilesystemProvider`. ...
> (`backend-agent-execution-boundary.md:104`)

`workspace.refreshFileTree` is not itself broken out as a row in that doc — this is
inferred, not quoted. Given rpc-catalog.md's own framing ("mostly superseded by
files.*/git.*"), it plausibly follows the same 🔀 dynamic-dispatch family: relay to
the Dev Server Agent per-worktree when the worktree's `connectionId` is set, execute
locally (read the on-disk tree directly) otherwise.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:16-18` — 13-channel inventory comment (no `files.*`/`workspace.*` entries)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-256` — `registerGitChannels` (`git.status`/`git.diff` only)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-17` — `GitGatewayService` (no file-tree RPC)
- `specs/frontend/api/rpc-catalog.md:481-485` — `workspace.*` catalog entry and method table
- `specs/frontend/api/backend-agent-execution-boundary.md:103-104` — `git.*`/`files.*` 🔀 dispatch rows (closest documented analog; `workspace.refreshFileTree` not independently broken out)
- `specs/backend-go/bugs/missing-v1/` — directory listing confirms no `files.*` bug report exists yet (sibling gap)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report this follows the shape of
