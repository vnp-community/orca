# BUG-031: `worktree.*` channels not implemented in backend-go

**Service:** `project-service` (worktree bookkeeping) + `git-gateway-service` (actual git operations — currently missing entirely)
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — worktree lifecycle is core navigation, tightly coupled to `git.*` (already the largest partial gap in this audit: only `git.status`/`git.diff` are wired)
**Symptom:** Every `worktree.*` call (`list`, `set`, `rm`, `detectedList`, `prefetchCreateBase`, `resolveMrBase`, `resolvePrBase`, `forceDeleteBranch`) falls through to `notImplementedHandler` and times out client-side
**Status:** ✅ Resolved — see TASK-192–196 (5 task(s), all DONE) for implementation evidence.

---

## Description

`specs/frontend/api/rpc-catalog.md` lists 8 `worktree.*` methods assigned to
this report:

```
grep -n '"worktree\.' services/api-gateway/internal/adapter/wscompat/channels.go
```

returns **zero matches** — no `worktree.*` channel is registered in
`RegisterRealChannels` (`channels.go:79-89`). Every call reaches
`registry.go`'s `notImplementedHandler` (`registry.go:59`).

**Correction to the "likely owning service" assumption**: worktree ownership
in backend-go is split, not concentrated in `git-gateway-service`:

- **`project-service`** owns worktree *bookkeeping metadata* —
  `proto/orca/project/v1/project.proto:31-38` has
  `RecordWorktreeCreated`/`RecordWorktreeRemoved`/`ListWorktrees`/
  `SetWorktreeActivation`/`RenameWorktree`, all real, with a Postgres-backed
  `WorktreeRepository`
  (`services/project-service/internal/adapter/postgres/worktree_repository.go`)
  and usecases under `services/project-service/internal/usecase/`. The proto
  comment at `project.proto:31-33` is explicit: this surface is
  "metadata only, never authoritative for on-disk" state.
  `RecordWorktreeCreated`'s own doc comment
  (`internal/usecase/record_worktree_created.go:22-24`) spells out the
  intended flow: "called by git-gateway-service AFTER the real
  `git worktree add` succeeded on the Dev Server Agent — this usecase only
  writes the bookkeeping row, it never triggers the git operation itself."

- **`git-gateway-service`** is the intended owner of the *actual* git
  worktree operations (`git worktree add`/`remove`) per that doc comment,
  but **its proto has no worktree RPCs at all**:
  `proto/orca/gitgateway/v1/gitgateway.proto:10-16` only has `GetStatus`,
  `GetDiff`, `Commit`, `Push`, `Pull`, `GenerateCommitMessage` — matching the
  existing `registerGitChannels` pattern in `channels.go:221-247`, which
  wires exactly `git.status`/`git.diff` for real (the doc comment there
  already flags `commit`/`push`/`pull` as stubs). No worktree create/remove
  RPC exists anywhere in `git-gateway-service`'s usecase directory either
  (`internal/usecase/` has only `get_diff.go`, `get_status.go`, `push.go`,
  `commit.go`, `pull.go`, `generate_commit_message.go`).

Net effect: `project-service` can answer "what worktrees exist" and toggle
activation/rename bookkeeping, but **nothing in backend-go can actually
create or remove an on-disk git worktree yet** — the piece
`RecordWorktreeCreated`'s own doc comment assumes exists
(git-gateway-service performing the real `git worktree add`) hasn't been
built.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `worktree.list` | `worktrees.ts:900` | Partial backing: `ListWorktrees` RPC exists (`project.proto:36`), real Postgres-backed bookkeeping (`project-service`). A wscompat wrapper is possible for the metadata list, but note it only reflects what `RecordWorktreeCreated`/`RecordWorktreeRemoved` have been told about — nothing populates those yet (see below). |
| `worktree.set` | `worktrees.ts:1053,1256`, `diffComments.ts:113` | Likely maps to `SetWorktreeActivation` RPC (`project.proto:37`, real). Per the old backend's model this method is 🏠 always-local bookkeeping (which worktree is "active") — `SetWorktreeActivation` matches that shape. Just needs a wscompat wrapper. |
| `worktree.rm` | `worktrees.ts:3309` | **No backing RPC for the actual removal.** `RecordWorktreeRemoved` (`project.proto:35`) only records that a removal happened — it does not perform `git worktree remove`, and no service has a `git worktree remove` RPC (see Description). |
| `worktree.forceDeleteBranch` | `worktrees.ts:3743` | **No backing RPC anywhere.** No git-gateway-service RPC does branch deletion of any kind, forced or not. |
| `worktree.detectedList` | `worktrees.ts:887` | **No backing RPC.** This is a filesystem-scan operation (detecting worktrees that exist on disk but aren't in the bookkeeping table) — no such RPC exists in `project-service` or `git-gateway-service`. |
| `worktree.prefetchCreateBase` | `worktrees.ts:2921` | **No backing RPC.** No git-gateway-service RPC does branch/base prefetching. |
| `worktree.resolveMrBase` | `worktrees.ts:1354`, `useComposerState.ts:3063` | **No backing RPC.** MR (GitLab merge request) base-branch resolution — no such RPC in `git-gateway-service` or anywhere else in backend-go's proto surface. |
| `worktree.resolvePrBase` | `worktrees.ts:1321`, `github-pr-start-point.ts:35` | **No backing RPC.** GitHub PR base-branch resolution — same gap as `resolveMrBase`. |

Only `list` and `set` have a plausible backing RPC (`project-service`'s
`ListWorktrees`/`SetWorktreeActivation`) and are "just needs a wscompat
wrapper" candidates. The other 6 — including the two mutating operations
(`rm`, `forceDeleteBranch`) and all PR/MR base-resolution methods — need new
`git-gateway-service` RPCs before any wscompat wrapper is possible.

---

## Dispatch model

🔀 (mostly) — same `getRemoteGitProvider` registry as `git.*` in the old TS
backend: worktree creation/removal/listing are git operations under the
hood, dispatched through whichever provider (local/SSH/Dev-Server-Agent)
owns the target host. `worktree.set` is 🏠 always-local (never relays — pure
bookkeeping of which worktree is "active"), matching `SetWorktreeActivation`
being pure Postgres in backend-go too. All mutating methods wrote worktree
metadata/lineage to the Postgres blob in the old backend — backend-go's
`project-service.worktrees` table (migration `0004_worktrees`) is the direct
structural equivalent.

**Context-only note (not a backend-go finding, background only):** the old
backend had a known crash bug where `worktree.forceDeleteBranch` called an
optional provider method, `forceDeletePreservedBranch`
(`backend/src/main/providers/types.ts:389`, declared optional with `?`),
implemented by `SshGitProvider`
(`backend/src/main/providers/ssh-git-provider.ts:735-750`, with an explicit
"older SSH relays predate `git.forceDeletePreservedBranch`" fallback
comment) but not guaranteed to exist on every provider variant. Worth
flagging as a design constraint for whichever service implements
`worktree.forceDeleteBranch` in backend-go: implement it uniformly across
both SSH-target and Dev-Server-Agent-backed repos in `git-gateway-service`,
rather than reproducing the old backend's partial-implementation gap where
one provider variant could silently lack the method.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go:79-89,221-247` — `RegisterRealChannels` (no `worktree.*` registration); existing `registerGitChannels` pattern to follow
- `services/api-gateway/internal/adapter/wscompat/registry.go:59` — `notImplementedHandler`
- `proto/orca/project/v1/project.proto:31-38,175-224` — `ProjectService`'s worktree bookkeeping RPCs and messages
- `proto/orca/gitgateway/v1/gitgateway.proto:10-16` — `GitGatewayService` RPC surface (no worktree RPCs)
- `services/project-service/internal/usecase/record_worktree_created.go:22-24` — doc comment describing the intended (unbuilt) git-gateway-service → project-service flow
- `services/project-service/internal/adapter/postgres/worktree_repository.go` — real Postgres persistence for worktree metadata
- `services/git-gateway-service/internal/usecase/` — `get_diff.go`, `get_status.go`, `push.go`, `commit.go`, `pull.go`, `generate_commit_message.go` (no worktree usecases)
- `backend/src/main/providers/types.ts:389`, `ssh-git-provider.ts:735-750` — old backend's optional `forceDeletePreservedBranch` gap (context only)
- `frontend/src/renderer/src/store/slices/worktrees.ts:887,900,1053,1256,1321,1354,2921,3309,3743` — call sites for this report's 8 methods
- `frontend/src/renderer/src/hooks/useComposerState.ts:3063`, `renderer/src/lib/github-pr-start-point.ts:35`, `renderer/src/store/slices/diffComments.ts:113` — additional call sites
- `specs/frontend/api/rpc-catalog.md:494-505` — `worktree.*` catalog entries
