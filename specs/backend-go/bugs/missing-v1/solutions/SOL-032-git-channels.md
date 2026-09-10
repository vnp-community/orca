# SOL-032: Wire `git.*`'s 4 ready RPCs, then build the ~28 missing `git-gateway-service` RPCs by extending the existing dispatch pattern

**Resolves:** [BUG-032](../BUG-032-git-channels-partially-implemented.md)
**Service:** `git-gateway-service` (new RPCs + `GitExecutor` port extension) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
- `backend-go/services/git-gateway-service/internal/usecase/*.go` (new — one file per RPC, per this service's own "one usecase per RPC" convention)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (extend `GitExecutor`)
- `backend-go/services/git-gateway-service/internal/domain/*.go` (new value objects: `BranchInfo`, `ConflictEntry`, `CommitRef`, `CompareResult`, `SubmoduleStatus`, `ForkSyncStatus`, `UpstreamStatus`)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go`
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Status:** ✅ Implemented — all 11 task(s) (TASK-206–216) DONE; see each task file's own Status/Verify section for evidence.

---

## Why this is the largest, highest-leverage proposal in the pass

32 of 34 `git.*` methods are unreachable from the frontend today (BUG-032).
`git.*` backs the product's entire code-review/commit/branch workflow —
nothing else in this audit has comparable blast radius. The good news,
confirmed by reading `git-gateway-service.md` and the actual scaffold
(`internal/usecase/ports.go`, `internal/adapter/grpcclient/relay_executor.go`):
**the hard part — the dynamic local-vs-relay dispatch mechanism — is already
built, tested, and used by all 6 existing RPCs.** This proposal is a
mechanical extension of that mechanism to ~29 more RPCs, grouped by family so
the work is reviewable and shippable in slices, not a redesign.

`git-gateway-service.md` §3's own API sketch already anticipates most of this
surface (`ListBranches`/`CreateBranch`/`SwitchBranch`/`DeleteBranch`/
`MergeBranch`/`RebaseBranch`/`GetLog`/`GetCommit`/`Blame`/`GetConflicts`/
`ResolveConflict`/`AbortMerge`/`ContinueMerge`/`GeneratePRDescription`) —
but that sketch predates the frontend's real, narrower call surface
(`rpc-catalog.md:196-234`). Where the two disagree on naming or granularity
(the doc's `SwitchBranch` vs. the frontend's `checkout`; the doc's single
`GetLog`/`GetCommit`/`Blame` vs. the frontend's more granular `history`/
`commitCompare`/`branchCompare`/`commitDiff`/`branchDiff`), this proposal
follows the frontend's real 34-method contract — `git-gateway-service.md` is
a sketch it says to fill in, not a message-by-message contract to reproduce
verbatim. BUG-032's dispatch-model note that **8 of the history/compare
methods already had dedicated Dev Server Agent RPC handlers in the old TS
backend** (`checkIgnored`, `submoduleStatus`, `history`, `branchCompare`,
`commitCompare`, `branchDiff`, `commitDiff`, `forkSync`) is important: the
relay side of those 8 is low-risk — the Dev Server Agent handler already
exists, only the Go RPC/usecase/local-exec side is new work.

---

## ⚠️ §0 — Correction pass (read before implementing anything below): real agent contract vs. this doc's original design

[BUG-036](../BUG-036-git-relay-methods-unreachable-on-agent.md) traced the
full frontend→backend→agent→backend→frontend flow against the REAL
`agent/` source (not just `git-gateway-service.md`'s sketch) and found two
separate classes of problem this section corrects:

1. **Reachability** — 26 of the 34 methods below have no handler on the
   agent process backend-go's transport actually reaches ("Part A",
   `agent-rpc-dispatch.ts`) at all, on any connection mode. 8 do
   (`history`, `branchCompare`, `commitCompare`, `branchDiff`,
   `commitDiff`, `checkIgnored`, `forkSync`, `submoduleStatus` — Part A
   already re-exports these from "Part B"'s ops modules). [TASK-227](../tasks/TASK-227-expose-git-status-diff-commit-on-agent-part-a.md)
   is the agent-side fix for the rest — **read it and its own method table
   before starting any Group below**, since it now covers all 26, not just
   the 3 my first pass caught.
2. **Shape** — even where a method IS reachable (Part A or, once
   TASK-227 lands, Part B via Part A), several of this doc's original
   request/response designs don't match the real agent contract at all —
   not a param-rename, a genuinely different operation shape. These need
   design fixes in the Go proto/usecase layer, not just relay-call
   corrections.

The corrected classification, straight from `specs/agent/api/agent-rpc-catalog-git-fs.md`'s
real tables (cited per row):

| Method | Reachable today? | Real agent params | This doc's original design | Verdict |
|---|:---:|---|---|---|
| `status` | ❌ (needs TASK-227) | `worktreePath, includeIgnored?, limit?` | `{repoPath}` (Part 1, already-shipped `RelayExecutor.GetStatus`) | ⚠️ param rename only (`repoPath`→`worktreePath`) — see [SOL-036](./SOL-036-expose-git-ops-on-agent-part-a.md)'s "Companion fix" note, tracked as [TASK-228](../tasks/TASK-228-fix-existing-relay-param-names-and-diff-shape.md) |
| `diff` | ❌ | `worktreePath, filePath (required), staged, compareAgainstHead?` | `{repoPath, staged}` — **whole-repo**, no `filePath` | ❌ **wrong shape**, not a rename — real `git.diff` is per-file. `GetDiff`'s Go signature needs a `filePath` param added. TASK-228. |
| `commit` | ❌ | `worktreePath, message` — no paths, assumes pre-staged | `{repoPath, message, paths}` | ⚠️ extra `paths` field the agent ignores (harmless) — but means staging must genuinely happen via `stage`/`bulkStage` first, not inside `Commit` itself. TASK-228. |
| `push` | ❌ | `worktreePath, pushTarget?, forceWithLease?, publish` (git-fs catalog, "Remote ops") — no plain `remote`+`branch` | `{repoPath, remote, branch}` | ❌ **needs redesign** — real push resolves a `pushTarget` object (`git-handler-push-target.ts`), not a bare remote+branch pair; this also carries real safety logic (blocks pushing a fork-tracking branch to `origin` unless configured). Flagged, not solved here — see "Open design questions" below. |
| `pull` | ❌ | `worktreePath, pushTarget?` | `{repoPath}` | ❌ same `pushTarget` redesign as `push` |
| `generateCommitMessage` | ✅ (`ai.complete` already on Part A) | n/a — no change | matches | ✅ no fix needed |
| `history` | ✅ | `worktreePath, limit?, baseRef?` — **no pagination cursor** | `{worktree_id, ref, limit, cursor}` | ⚠️ drop `cursor`/pagination — the real agent has none; rename `ref`→`baseRef` |
| `branchCompare` | ✅ | `worktreePath, baseRef` (single ref — always vs. current HEAD) | `{worktree_id, base_branch, head_branch}` (two-sided) | ❌ **wrong shape** — real op is HEAD-vs-`baseRef`, not two arbitrary branches |
| `commitCompare` | ✅ | `worktreePath, commitId` (single commit vs. **its own parent**) | `{worktree_id, base_sha, head_sha}` (two arbitrary commits) | ❌ **wrong shape** — matches `backend-agent-execution-boundary.md`'s own description ("Diffs a commit against its parent"), which this doc's original design contradicted |
| `branchDiff` | ✅ | `worktreePath, baseRef, includePatch?, filePath?, oldPath?` — **per-file capable** | `{worktree_id, base_branch, head_branch}`, no `filePath` | ❌ wrong shape, same two-sided + missing-filePath issues as above |
| `commitDiff` | ✅ | `worktreePath, commitOid, parentOid?, filePath (required), oldPath?` — **per-file** | `{worktree_id, sha}` whole-commit | ❌ **wrong shape** — missing required `filePath`, same class of bug as `diff` |
| `checkIgnored` | ✅ | `worktreePath, paths[]` → returns **only the ignored subset** (`string[]`) | response `{path, ignored}` for every input path | ⚠️ response shape too rich — real agent returns which paths are ignored, not a bool per path |
| `forkSync` | ✅ | `worktreePath, expectedUpstream` (**required**) | `{worktree_id}` — missing `expectedUpstream` entirely | ❌ missing required param |
| `submoduleStatus` | ✅ | `worktreePath, submodulePath` (**one submodule per call**), `area?` | `{worktree_id}` → `repeated SubmoduleInfo` (bulk/list) | ❌ **wrong shape** — real op is per-submodule, not "list all submodules"; needs a `.gitmodules`-enumeration step client-side first, or a redesign |
| `upstreamStatus` | ❌ (needs TASK-227) | `worktreePath, pushTarget?` | `{worktree_id}` | ⚠️ param rename + optional `pushTarget` |
| `checkout` | ❌ | `worktreePath, branch` — **no create-branch semantics** | `{worktree_id, ref, create}` | ❌ real `git.checkout` doesn't support `-b`; `create:true` needs composing as a separate `git checkout -b` via a different mechanism (or dropped from this RPC's scope) |
| `localBranches` | ❌ | `worktreePath` → `{current, branches[]}` — **names only, no ahead/behind/upstream** | `repeated BranchInfo{name,upstream,ahead,behind,is_current,is_remote}` | ❌ real response is far narrower — either narrow the Go type to match, or compose via `git.exec`'s `for-each-ref` instead (loses the dedicated-RPC safety benefit) |
| `fastForward` | ❌ | `worktreePath, pushTarget?` (remote-ops table) | `{worktree_id, branch}` | ❌ same `pushTarget` redesign as push/pull |
| `rebaseFromBase` | ❌ | `worktreePath, baseRef` | `{worktree_id, base_branch}` | ⚠️ param rename only — this one's close |
| `abortRebase` / `abortMerge` | ❌ | `worktreePath` only | `{worktree_id}` | ⚠️ param rename only — closest fixes in the whole set |
| `conflictOperation` | ❌ | `worktreePath` only → returns `'merge'\|'rebase'\|'cherry-pick'\|'unknown'` (**a detector**) | `{worktree_id, path, operation}` → executes ours/theirs/markResolved | ❌ **fundamentally wrong** — real `git.conflictOperation` only detects which kind of conflict is in progress; it does not resolve individual files. This doc's design conflated two different operations. Needs a full redesign, not a shape tweak — see "Open design questions" below. |
| `discard` | ❌ | `worktreePath, filePath` | `{worktree_id, path}` | ⚠️ param rename only |
| `bulkDiscard` | ❌ | `worktreePath, filePaths[]` | `{worktree_id, paths}` | ⚠️ param rename only |
| `stage` / `unstage` | ❌ | `worktreePath, filePath` (**single file only** — no bulk variant) | collapsed into `Stage`/`Unstage` RPCs taking `repeated paths` | ⚠️ works if `RelayExecutor` always calls `git.bulkStage`/`git.bulkUnstage` (which accept any count ≥1) instead of branching on `len(paths)==1` — simpler than maintaining two relay call shapes |
| `bulkStage` / `bulkUnstage` | ❌ | `worktreePath, filePaths[]` | same 2 RPCs as above | ✅ matches once relay always targets the bulk variant per the note above |
| `fetch` | ❌ | `worktreePath, pushTarget?` — **always prunes** per the git command shown (`git fetch --prune [remote]`) | `{worktree_id, remote, prune}` | ⚠️ drop the `prune` bool (always true), same `pushTarget` question as push/pull |
| `remoteCommitUrl` / `remoteFileUrl` | n/a — no agent method on either Part A or Part B | pure local computation | matches (Option (a) in this doc, unchanged) | ✅ no fix needed — correctly designed as local-only from the start |
| `discoverCommitMessageModels` | n/a — calls `ai-provider-service`, not the agent | n/a | matches | ✅ no fix needed |
| `cancel*` (2 methods) | n/a — no async job exists | n/a | matches | ✅ no fix needed |

**Bottom line: 6 of 34 methods (`generateCommitMessage`, `remoteCommitUrl`,
`remoteFileUrl`, `discoverCommitMessageModels`, and the 2 `cancel*`
methods) are correctly designed as-is and need no fix. 15 need at least a
param rename or a dropped/added field (mechanical, low-risk — `status`,
`history`, `checkIgnored`, `upstreamStatus`, `rebaseFromBase`,
`abortRebase`, `abortMerge`, `discard`, `bulkDiscard`, `stage`/`unstage`/
`bulkStage`/`bulkUnstage`, `commit`, `fetch`). 13 need a genuine shape
redesign, not a mechanical fix (`diff`, `push`, `pull`, `branchCompare`,
`commitCompare`, `branchDiff`, `commitDiff`, `submoduleStatus`, `checkout`,
`localBranches`, `conflictOperation`, `fastForward` — 12 listed, plus the
`pushTarget` question touching `push`/`pull`/`fastForward`/`fetch`
together as one design decision, not four separate ones).**

### Open design questions this correction pass does NOT resolve

Flagging honestly rather than guessing a specific answer without a live
agent to verify against:

1. **`push`/`pull`/`fastForward`/`fetch`'s `pushTarget` concept** — the
   real agent resolves a structured `pushTarget` (never a bare URL;
   resolves to a configured remote name first; blocks pushing a
   fork-tracking branch to `origin` unless `branch.<b>.merge` targets
   itself) via `resolveRelayPushTarget`/`assertGitPushTargetShape`. Before
   implementing Groups A/D's push/pull/fetch/fastForward methods, read
   `git-handler-push-target.ts` in full and design `PushTargetInput`
   properly — don't guess a `{remote, branch}` passthrough as the
   original design did, it will either be rejected by the agent's shape
   validator or (worse) silently bypass the fork-branch safety check.
2. **`conflictOperation`'s real per-file resolve** — the real agent has
   no dedicated RPC for "resolve this one conflicted file as ours/theirs."
   Options: (a) compose via `git.exec`'s whitelisted `checkout`
   subcommand (loses the narrower, purpose-built-RPC safety Part B's
   design otherwise provides elsewhere), (b) treat `git.conflictOperation`
   as ONLY the detector (matches real contract) and design a *separate*,
   genuinely new agent-side RPC for per-file resolution (an `agent/`
   change beyond TASK-227's scope, which only re-exposes existing Part B
   logic). Needs a decision before Group A ships its `ConflictOperation`
   RPC.
3. **`submoduleStatus`'s per-submodule vs. list mismatch** — needs either
   a `.gitmodules`-parsing step (client-side, to get the list of
   submodule paths to call `git.submoduleStatus` once each for) or
   accepting a narrower single-submodule RPC and having the frontend call
   it per-submodule instead of expecting one bulk response.
4. **`localBranches`'s missing ahead/behind/upstream data** — decide
   whether to narrow `BranchInfo` to match the real agent's `{current,
   branches[]}` (names only), or compose the richer data via `git.exec`'s
   `for-each-ref` (in Part A's 21-subcommand whitelist, though not in Part
   B's stricter 14-subcommand `git.exec` whitelist — the two `git.exec`
   surfaces have DIFFERENT whitelists per `agent-rpc-catalog-git-fs.md`,
   another thing to verify before choosing this path).

None of these 4 are solved by this correction pass — they're real design
decisions that need the actual agent source open (`git-handler-push-target.ts`,
`git-handler-status-ops.ts`'s `detectConflictOperation`) and ideally a live
agent to test against, not something to confidently fabricate from a
frontend spec. Treat Group A (branch/ref) and the push/pull/fetch/fastForward
half of Group D as **blocked on these decisions**, not ready to implement
as originally written below. Groups B (staging, once relay targets the
bulk variant per the table above), C (history/compare, mostly param
renames + the per-file diff fix), and E (AI-assist) are in better shape —
prioritize those once TASK-227 lands.

---

## Part 1 — Wiring-only quick wins: `commit`/`push`/`pull`/`generateCommitMessage`

**Revised status per §0 above: only `generateCommitMessage` is a true
zero-risk quick win today.** `commit`/`push`/`pull` compile and wire
cleanly (this section's code is still correct Go) but will fail at
runtime for relay-dispatched worktrees until TASK-227 (agent
reachability) and, for `push`/`pull`, the `pushTarget` redesign in §0's
open question #1, both land. Ship `generateCommitMessage`'s channel
immediately; hold `commit`/`push`/`pull`'s channels as "code complete,
runtime-blocked" until their prerequisites close.

Per BUG-032, these 4 already have real usecases, gRPC methods, and REST
routes (`git_routes.go:26-29`). This is a pure `wscompat` wrapper addition,
following `registerGitChannels`'s existing `git.status`/`git.diff` pattern
(`channels.go:221-252`) exactly — same `decodeArg`/`client.X(ctx, ...)`/
return-response shape, no `gatewaygrpc.AttachIdentity` (git-gateway-service's
proto carries no tenant field and this file's existing git/task/automation
channels all call clients with the bare inbound ctx — see `channels.go`'s
`devServer.*` section doc comment for the one place that convention does
NOT apply, which is not here).

```go
// Add to registerGitChannels (channels.go), after the existing git.diff registration.

r.Register("git.commit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type commitArgs struct {
		WorktreeID string   `json:"worktreeId"`
		Message    string   `json:"message"`
		Paths      []string `json:"paths"`
	}
	in, err := decodeArg[commitArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.Commit(ctx, &gitgatewayv1.CommitRequest{
		WorktreeId: in.WorktreeID, Message: in.Message, Paths: in.Paths,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.push", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type pushArgs struct {
		WorktreeID string `json:"worktreeId"`
		Remote     string `json:"remote"`
		Branch     string `json:"branch"`
	}
	in, err := decodeArg[pushArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.Push(ctx, &gitgatewayv1.PushRequest{
		WorktreeId: in.WorktreeID, Remote: in.Remote, Branch: in.Branch,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.pull", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type pullArgs struct {
		WorktreeID string `json:"worktreeId"`
	}
	in, err := decodeArg[pullArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.Pull(ctx, &gitgatewayv1.PullRequest{WorktreeId: in.WorktreeID})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.generateCommitMessage", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type genArgs struct {
		WorktreeID string `json:"worktreeId"`
	}
	in, err := decodeArg[genArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.GenerateCommitMessage(ctx, &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: in.WorktreeID})
	if err != nil {
		return nil, err
	}
	return resp, nil
})
```

Update the file's header comment (`channels.go:17-19`'s channel-count
annotation) from "2" to "6" for `git.*`, matching the existing convention of
keeping that count accurate.

---

## Part 2 — The ~29 methods with no backing RPC, grouped by family

### The dispatch mechanism these all reuse — don't invent a second one

Every existing RPC (`get_status.go`, `get_diff.go`, `commit.go`, `push.go`,
`pull.go`) shares one shape, centralized in `ports.go`:

```go
// ports.go — already exists, unchanged by this proposal
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
}

func dispatchExecutor(ctx context.Context, resolver ConnectionResolver, local, relay GitExecutor, worktreeID string) (GitExecutor, string, error) {
	conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return nil, "", err
	}
	if conn.Connected {
		return relay, conn.RepoPath, nil
	}
	return local, conn.RepoPath, nil
}
```

Two implementations satisfy `GitExecutor` today: `localgit.Executor`
(shells out to the local `git` binary) and `grpcclient.RelayExecutor`
(calls `infra-fleet-service`'s generic `Relay(connectionId, method,
paramsJson)` RPC with method names `"git.status"`/`"git.commit"`/etc. —
`relay_executor.go:12-18`). **The extension for every method below is
additive to this exact shape**: add the method to `GitExecutor`, implement
it in both `localgit.Executor` (a `git` invocation) and
`grpcclient.RelayExecutor` (a `Relay` call with a new `"git.<name>"` method
string), add a `usecase/<name>.go` file that calls `dispatchExecutor` then
the new method — identical to `commit.go`'s existing shape. No new
decision point, no second dispatcher.

One implementation note as the interface grows to ~34 methods: keep
`GitExecutor` as a single interface rather than splitting it, since
`dispatchExecutor`'s single call site is the only consumer and both
`localgit.Executor` and `grpcclient.RelayExecutor` must implement every
method regardless of how the Go interface is sliced — Go's "small
interfaces at the consumer" guidance doesn't gain anything here with only
one consumer. Group the new methods into the proto service by family (below)
for reviewability, not into separate Go interfaces.

### Group A — Branch / ref operations (9 methods)

Representative: **`Checkout`**.

```protobuf
// worktree_id + ref (branch/tag/commit-ish) + create (git checkout -b
// semantics). Mirrors git-gateway-service.md §3's SwitchBranch/CreateBranch
// pair, collapsed into one RPC since the frontend's checkout call already
// takes a create flag (rpc-catalog.md:196-234) rather than being two calls.
rpc Checkout(CheckoutRequest) returns (CheckoutResponse);

message CheckoutRequest {
  string worktree_id = 1;
  string ref = 2;
  bool create = 3;
}
message CheckoutResponse {
  bool success = 1;
  string branch = 2; // resolved current branch after the operation
}
```

```go
// usecase/checkout.go — identical shape to commit.go
type Checkout struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func (uc *Checkout) Execute(ctx context.Context, in CheckoutInput) (domain.CheckoutResult, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	return executor.Checkout(ctx, repoPath, in.Ref, in.Create)
}
```

`localgit.Executor.Checkout`: `git checkout <ref>` or `git checkout -b <ref>`
(both available since Git 2.5, no `GitCapabilityCache` fallback needed, per
`localgit`'s existing baseline-compatibility note). `RelayExecutor.Checkout`
relays method `"git.checkout"` with `{repoPath, ref, create}` params — same
`relay()` helper every existing method already uses.

Rest of the group (same `GitExecutor`-method-per-RPC shape):

| Method | RPC | Request fields | Response | Notes |
|---|---|---|---|---|
| `localBranches` | `ListLocalBranches` | `worktree_id` | `repeated BranchInfo` | `BranchInfo` per §4's domain model already sketched (name, upstream, ahead/behind, is_current, is_remote) |
| `fastForward` | `FastForward` | `worktree_id`, `branch` | `success` | `git merge --ff-only <branch>` locally |
| `rebaseFromBase` | `RebaseFromBase` | `worktree_id`, `base_branch` | `success`, `had_conflicts` | `git rebase <base_branch>`; conflict = nonzero exit + `.git/rebase-merge` present |
| `abortRebase` | `AbortRebase` | `worktree_id` | `success` | `git rebase --abort` |
| `abortMerge` | `AbortMerge` | `worktree_id` | `success` | `git merge --abort` — matches `git-gateway-service.md` §3's own `AbortMerge` name exactly |
| `conflictOperation` | `ConflictOperation` | `worktree_id`, `path`, `operation` (`ours`/`theirs`/`markResolved`) | `success` | Collapses TS's `ResolveConflict`-per-file into one op-typed RPC matching the frontend's actual single-call shape |
| `discard` | `Discard` | `worktree_id`, `path` | `success` | `git checkout -- <path>` (tracked) or `rm` (untracked, mirroring TS `git.discard`'s untracked-file handling) |
| `bulkDiscard` | `BulkDiscard` | `worktree_id`, `paths` (repeated) | `success`, `failed_paths` (repeated) | Batches `Discard`'s per-file operation; reports partial failure rather than all-or-nothing, since a mixed batch (some tracked, some already-clean) is a real case |

### Group B — Staging operations (4 methods → 2 RPCs)

Representative: **`Stage`**. `stage` and `bulkStage` are the same wire
operation at different call-site granularity (single file vs. multi-select)
— propose **collapsing them onto one RPC that already takes a repeated
field**, rather than 4 RPCs for what's functionally 2 operations:

```protobuf
rpc Stage(StageRequest) returns (StageResponse);
rpc Unstage(UnstageRequest) returns (UnstageResponse);

message StageRequest {
  string worktree_id = 1;
  repeated string paths = 2; // single-element for git.stage, full selection for git.bulkStage
}
message StageResponse {
  bool success = 1;
}
// UnstageRequest/UnstageResponse mirror Stage/StageResponse exactly.
```

`wscompat` registers `git.stage`, `git.bulkStage`, `git.unstage`,
`git.bulkUnstage` as 4 separate channels that all call the same 2 RPCs —
the collapse happens at the proto/usecase layer, not the WS channel layer,
so the frontend's 4 distinct call sites need no change. `localgit`:
`git add -- <paths...>` / `git restore --staged -- <paths...>` (Git 2.23+;
`git reset HEAD -- <paths...>` is the pre-2.23 fallback per
`docs/reference/git-compatibility.md`'s Git 2.25 baseline — 2.23 predates
the baseline, so no fallback branch is actually needed, but flag the
`git-compatibility.md` check as a required step before locking this in).

### Group C — History / compare operations (9 methods)

Representative: **`History`**.

```protobuf
rpc History(HistoryRequest) returns (HistoryResponse);

message HistoryRequest {
  string worktree_id = 1;
  string ref = 2;    // empty = current branch
  int32 limit = 3;
  string cursor = 4; // opaque pagination cursor (SHA of the last-seen commit)
}
message HistoryResponse {
  repeated CommitRef commits = 1;
  string next_cursor = 2;
}
```

`CommitRef` reuses the value object `git-gateway-service.md` §4 already
specifies (SHA, author, committer, timestamp, message, parent SHAs) — no new
domain type needed for this one. `localgit`: `git log --format=<...>
[ref] [-<limit>] [--skip after cursor]`. `RelayExecutor`: relays
`"git.history"` — **one of the 8 methods in this group BUG-032 confirms
already had a dedicated Dev Server Agent handler in the old TS backend**,
so this relay leg is low-risk; verify the exact param/result field names
against `specs/agent/api/agent-rpc-catalog-git-fs.md` before finalizing,
same caveat `relay_executor.go`'s doc comment already flags for the 5
existing relay methods.

Rest of the group:

| Method | RPC | Request fields | Response | Old-TS agent handler already exists? |
|---|---|---|---|---|
| `commitCompare` | `CommitCompare` | `worktree_id`, `base_sha`, `head_sha` | `repeated CommitRef`, files-changed summary | Yes (BUG-032) |
| `branchCompare` | `BranchCompare` | `worktree_id`, `base_branch`, `head_branch` | same shape as `CommitCompare` | Yes (BUG-032) |
| `commitDiff` | `CommitDiff` | `worktree_id`, `sha` | `unified_diff` (reuses `GetDiffResponse`'s shape) | Yes (BUG-032) |
| `branchDiff` | `BranchDiff` | `worktree_id`, `base_branch`, `head_branch` | `unified_diff` | Yes (BUG-032) |
| `submoduleStatus` | `SubmoduleStatus` | `worktree_id` | `repeated SubmoduleInfo` (path, current_sha, tracked_sha, dirty) | Yes (BUG-032) |
| `checkIgnored` | `CheckIgnored` | `worktree_id`, `paths` (repeated) | `repeated {path, ignored}` | Yes (BUG-032) |
| `forkSync` | `ForkSync` | `worktree_id` | `ForkSyncStatus` (ahead, behind, diverged) | Yes (BUG-032) |
| `upstreamStatus` | `UpstreamStatus` | `worktree_id` | `UpstreamStatus` (has_upstream, ahead, behind) | **Not listed among BUG-032's 8** — confirm against `agent-rpc-catalog-git-fs.md` before assuming an existing agent handler; may need a new Dev Server Agent-side handler, unlike its 8 siblings in this group |

### Group D — Remote operations (3 methods)

Representative: **`Fetch`** — the one operation in this group that's a real
git network call needing host dispatch like every other group:

```protobuf
rpc Fetch(FetchRequest) returns (FetchResponse);
message FetchRequest {
  string worktree_id = 1;
  string remote = 2; // empty = default remote
  bool prune = 3;
}
message FetchResponse {
  bool success = 1;
}
```

`remoteCommitUrl`/`remoteFileUrl` are a different shape entirely — pure
string construction (`https://github.com/<org>/<repo>/commit/<sha>` etc.)
from the worktree's configured remote URL, not a git *operation*. Two
options, flagged as a scope decision rather than settled here:

- **(a) Keep them in `git-gateway-service`** as cheap RPCs that resolve
  the worktree's `origin` URL (`git remote get-url origin`, dispatched the
  same way as every other method — a worktree's remote can differ between
  local and relay hosts, so this still needs the dispatch, just does no
  git-network work once dispatched) and pattern-match the host
  (github.com/gitlab.com/bitbucket.org) to build the URL.
- **(b) Push this to the frontend** if `project-service` already exposes
  the remote URL to the client directly, making it a pure client-side
  string template with no backend round-trip at all.

**Recommendation: (a)** — `git-gateway-service` is already the single
source of truth for "what does this worktree's remote look like" (every
other method in this doc resolves through it), and a client-side
implementation would need to duplicate the GitHub/GitLab/Bitbucket URL
templates the backend would otherwise own once.

```protobuf
rpc RemoteCommitUrl(RemoteCommitUrlRequest) returns (RemoteUrlResponse);
rpc RemoteFileUrl(RemoteFileUrlRequest) returns (RemoteUrlResponse);

message RemoteCommitUrlRequest { string worktree_id = 1; string sha = 2; }
message RemoteFileUrlRequest   { string worktree_id = 1; string path = 2; string ref = 3; }
message RemoteUrlResponse      { string url = 1; }
```

### Group E — AI-assist operations (4 methods)

Representative: **`GeneratePullRequestFields`** — follows
`GenerateCommitMessage`'s already-established pattern exactly (§3.1:
gather diff/status context via the same dispatch path, then relay the
completion call to the Dev Server Agent's `ai.complete`; this service never
holds an LLM client of its own):

```protobuf
rpc GeneratePullRequestFields(GeneratePullRequestFieldsRequest) returns (GeneratePullRequestFieldsResponse);

message GeneratePullRequestFieldsRequest {
  string worktree_id = 1;
  string base_branch = 2;
}
message GeneratePullRequestFieldsResponse {
  string title = 1;
  string description = 2;
}
```

```go
// usecase/generate_pull_request_fields.go — same shape as
// generate_commit_message.go: gather context via dispatchExecutor
// (GetDiff/History against base_branch), then call AICompleter.Complete.
func (uc *GeneratePullRequestFields) Execute(ctx context.Context, in GeneratePullRequestFieldsInput) (domain.PRFields, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.PRFields{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	diff, err := executor.GetDiff(ctx, repoPath, false)
	if err != nil {
		return domain.PRFields{}, err
	}
	prompt := buildPRFieldsPrompt(diff, in.BaseBranch)
	content, err := uc.ai.Complete(ctx, /* connectionID */ repoPath, prompt)
	if err != nil {
		return domain.PRFields{}, err
	}
	return parsePRFields(content), nil
}
```

The remaining 3 methods in this group are **not** dispatch operations at
all — each needs a distinct design note, not a mechanical extension:

- **`discoverCommitMessageModels`** — lists which AI models/providers are
  available for commit-message/PR-field generation. This is
  `ai-provider-service`'s data (provider/account context), not a git
  operation — `git-gateway-service` should relay this to
  `ai-provider-service`'s account-listing RPC rather than fabricating a
  worktree-dispatch flow for it. Proposed as a thin passthrough RPC:
  `rpc DiscoverCommitMessageModels(DiscoverCommitMessageModelsRequest) returns (DiscoverCommitMessageModelsResponse)`
  with `{repeated ModelInfo models}`, implemented by calling
  `ai-provider-service` directly (no `ConnectionResolver`/`GitExecutor`
  involved).
- **`cancelGenerateCommitMessage`** / **`cancelGeneratePullRequestFields`**
  — both imply an in-flight, cancellable async job, but
  `GenerateCommitMessage`/`GeneratePullRequestFields` are synchronous unary
  RPCs today: the client blocks for the whole call, so there is no
  server-side job to cancel. Proposed short-term fix: **no new RPC** — wire
  the WS envelope's own cancellation signal (if `wscompat`'s `Registry`
  exposes one; if not, this is the actual gap to close) to cancel the
  handler's `context.Context`, which then cancels the outbound gRPC call
  and, transitively, the `Relay` call to the Dev Server Agent. Only if AI
  generation later moves to a genuinely async model (job ID + poll/stream)
  would a real `Cancel` RPC be needed — scope that as a separate follow-up,
  not part of this proposal, since building a cancel RPC against an
  operation that isn't actually async yet would be solving a problem that
  doesn't exist server-side.

---

## `wscompat` wiring — deeper-gap methods

Once the RPCs above exist, wiring follows the exact Part 1 pattern —
`decodeArg` → `client.<RPC>(ctx, &...Request{...})` → return response.
Sketch for one representative per group (the rest are the same shape):

```go
r.Register("git.checkout", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type checkoutArgs struct {
		WorktreeID string `json:"worktreeId"`
		Ref        string `json:"ref"`
		Create     bool   `json:"create"`
	}
	in, err := decodeArg[checkoutArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.Checkout(ctx, &gitgatewayv1.CheckoutRequest{
		WorktreeId: in.WorktreeID, Ref: in.Ref, Create: in.Create,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.stage", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type stageArgs struct {
		WorktreeID string   `json:"worktreeId"`
		Paths      []string `json:"paths"`
	}
	in, err := decodeArg[stageArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.Stage(ctx, &gitgatewayv1.StageRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
	if err != nil {
		return nil, err
	}
	return resp, nil
})
r.Register("git.bulkStage", /* identical body to git.stage — see Group B */ nil) // placeholder for the identical handler
```

`git.bulkStage`/`git.bulkUnstage` register the same handler function as
`git.stage`/`git.unstage` (a shared closure, not duplicated bodies) — one
more small economy this collapse buys.

---

## Test plan

- **`usecase/*_test.go`** — one table-driven test per new usecase, following
  `dispatch_test.go`'s existing convention: fake `ConnectionResolver` (both
  connected/not-connected cases) + fake `GitExecutor`, asserting the branch
  selection (local vs. relay) and that the resolved `repoPath` reaches the
  executor call. No real Postgres/gRPC — matches
  `03-clean-architecture-guidelines.md`'s usecase-test policy.
- **`adapter/localgit/executor_test.go`** — extend with real-git-binary
  fixtures (a temp repo per test, following `executor_test.go`'s existing
  pattern) for `Checkout`/`Stage`/`Unstage`/`Fetch`/`ListLocalBranches`/
  `FastForward`/`AbortMerge`/`AbortRebase`/`Discard`/`History`/
  `CommitDiff`/`BranchDiff`/`CheckIgnored`. Conflict/rebase-abort cases need
  a fixture that actually produces a conflict (two branches editing the same
  line) to test against real `git` state, not a mocked one.
- **`adapter/grpcclient/relay_executor_test.go`** — extend
  `fakeInfraFleetServiceClient` assertions (`grpcclient_test.go`'s existing
  pattern) to verify each new method sends the right `Relay.Method` string
  and JSON param shape, and correctly unmarshals a fake result — one test
  per new relay method, no live Dev Server Agent needed.
- **`adapter/grpc/server_test.go`** — contract test per new RPC: request →
  usecase call → response translation, per `03-clean-architecture-guidelines.md`'s
  `adapter/grpc` contract-test policy.
- **`wscompat/channels_test.go`** — one test per new channel following
  `TestDevServerListChannel_Success`'s existing shape (fake gRPC client,
  `argsJSON` helper, assert the channel calls through and returns the
  translated response); a `TestGitBulkStageChannel_SharesStageHandler`-style
  test to lock in the shared-closure economy from Group B so a future edit
  can't silently diverge `stage`/`bulkStage` behavior.
- **CI**: `buf breaking` must pass (all additive RPCs/messages, per
  `08-inter-service-communication.md`'s gRPC conventions) — no existing RPC
  signature changes in this proposal.
- **Phased rollout recommendation, revised per §0**: `generateCommitMessage`
  first (true zero-risk quick win). Then [TASK-227](../tasks/TASK-227-expose-git-status-diff-commit-on-agent-part-a.md)
  (agent reachability — blocks everything else). Then Group C
  (history/compare) — now the *cheapest* real win, since 8 of its 9
  methods are already reachable and most of its fixes are mechanical
  param renames, not redesigns (only the per-file `commitDiff`/`branchDiff`
  and single-ref `branchCompare`/`commitCompare` shape fixes need care).
  Then Group B (staging) once relay calls always target the bulk variant.
  Then `status`/`diff`/`commit` (TASK-228). Hold Group A (branch/ref) and
  the `pushTarget`-dependent half of Group D (`push`/`pull`/`fastForward`/`fetch`)
  until §0's open design questions are resolved — implementing them as
  originally written would ship code that's either rejected by the
  agent's shape validator or silently bypasses real safety checks
  (fork-branch push protection).

## References

- [BUG-036](../BUG-036-git-relay-methods-unreachable-on-agent.md),
  [SOL-036](./SOL-036-expose-git-ops-on-agent-part-a.md),
  [TASK-227](../tasks/TASK-227-expose-git-status-diff-commit-on-agent-part-a.md),
  [TASK-228](../tasks/TASK-228-fix-existing-relay-param-names-and-diff-shape.md) —
  §0's source: the real agent contract this whole proposal must be
  reconciled against before implementation, found by tracing the actual
  data flow rather than trusting this doc's original design
- `specs/backend-go/tdd/services/git-gateway-service.md` — full service
  design; §2 (resolve→dispatch→translate), §3 (API surface sketch), §3.1
  (AI-assist relay pattern), §4 (domain value objects), §7 (dependency
  sequence diagram), §10 (migration notes on the ~35 RPCs)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` —
  "Talking to the Dev Server Agent" (Option A relay protocol), gRPC
  conventions (`buf breaking`, mandatory deadlines)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` —
  usecase/port layering, one-usecase-per-RPC convention
- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` —
  full 34-method table, dispatch-model notes on the 8 methods with existing
  Dev Server Agent handlers
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — current 6-RPC
  surface this proposal extends
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:39-96`
  — `GitExecutor`, `ConnectionResolver`, `AICompleter` ports and
  `dispatchExecutor` — the mechanism this proposal extends, not replaces
- `backend-go/services/git-gateway-service/internal/usecase/commit.go`,
  `get_status.go`, `get_diff.go` — the existing per-RPC usecase shape every
  new usecase file follows
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` —
  local `git`-binary executor pattern; baseline Git 2.5 command set, no
  `GitCapabilityCache` needed for anything proposed here (confirm each new
  command against `docs/reference/git-compatibility.md` before merging)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:1-158` —
  `RelayExecutor`'s `relay()` helper and existing method-name convention
  (`"git.status"`, `"git.commit"`, …) every new relay method follows; its
  doc comment's caveat about unverified param/result field names against
  the real Dev Server Agent applies equally to every new method here
- `backend-go/services/infra-fleet-service/internal/usecase/relay.go` — the
  generic `Relay(connectionId, method, params)` RPC every relay leg (old and
  new) dispatches through; no new infra-fleet-service work needed
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-252`
  — `registerGitChannels`, the wiring pattern Part 1 and the deeper-gap
  wiring both follow
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go` —
  channel-test convention (`fake*Client`, `argsJSON` helper) new tests follow
- `specs/frontend/api/rpc-catalog.md:196-234` — full `git.*` frontend
  call-site table (34 methods) this proposal's method list is grounded in
- `specs/agent/api/agent-rpc-catalog-git-fs.md` — Dev Server Agent's actual
  git handler contract; must be reconciled against every new relay method's
  param/result names before implementation, per `relay_executor.go`'s
  existing caveat
