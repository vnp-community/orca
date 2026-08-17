# Agent → provides → Backend: `git.*` / `fs.*`

Every RPC method the **agent** process exposes for the **backend** to call,
scoped to the `git.*` and `fs.*` namespaces (plus their dedicated streaming
protocols). See [`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md)
for every other namespace (`pty.*`, `ai.*`, `github.*`/`gitlab.*`,
`externalAutomations.*`, `preflight.*`, `ports.*`, `workspace.*`, `shell.*`, …),
and [`connection-modes.md`](./connection-modes.md) for the transport these
calls ride on.

## Two independent implementations

Per [`connection-modes.md`](./connection-modes.md) §0, the agent runs **two
parallel RPC surfaces**, and `git.*`/`fs.*` exist — differently — on both:

- **Part A — WS-connected Dev Server Agent** (`agent/src/relay/agent-rpc-dispatch.ts`
  + friends): a local process the backend connects to directly over WebSocket
  (`direct-websocket`/`relay-websocket` modes). A flat `switch(rpc.method)`
  router; each case dynamically imports its handler module.
- **Part B — SSH Relay Daemon** (`agent/src/relay/relay.ts` + `dispatcher.ts`
  `RelayDispatcher`, `relay-ssh` mode): handler classes (`GitHandler`,
  `FsHandler`, …) call `dispatcher.onRequest(method, handler)` in their
  constructors, wired in `relay.ts:470-492`.

Part A's `git.history`/`branchCompare`/`commitCompare`/`branchDiff`/`commitDiff`/
`checkIgnored`/`forkSync`/`submoduleStatus` (in `agent-git-handler-extended.ts`)
explicitly **reuse** Part B's ops modules (`git-handler-ops.ts`,
`git-handler-commit-diff-ops.ts`, `git-handler-check-ignore.ts`,
`git-handler-status-ops.ts`, `git-handler-submodule-ops.ts`) — no logic is
duplicated for those specific methods, only re-exposed under Part A's router.

**Backend actually talks to Part B for most git/fs traffic**: the `git.*`/`fs.*`
methods confirmed as real backend call sites (`DevServerGitProvider`,
`DevServerFilesystemProvider`, in `backend/src/main/providers/`) use the fuller,
more-validated Part B method set (`git.status`, `git.diff`, `git.stage`, `fs.grep`,
`fs.glob`, `fs.watch`, etc. — all Part B only). Part A's `git.exec`/`fs.readDir`
family exists for the small number of features that still talk to the agent over
a direct WebSocket rather than SSH relay.

---

## `git.*` — Part A (WS Dev Server Agent)

| Method | Registration | Handler | Params | Returns | Description | Security |
|---|---|---|---|---|---|---|
| `git.exec` | `agent-rpc-dispatch.ts:310` | `handleGitExec` — `agent-git-handler.ts:121` | `args: string[]`, `cwd?`, `timeout?` (≤60000ms) | `{stdout,stderr,exitCode}` | Spawns `git <args>` (`shell:false`) in `cwd`, captures output | `ALLOWED_GIT_SUBCOMMANDS` (21 subcommands, below) + blanket `SHELL_METACHARACTERS` regex `/[&\|;$\`<>\\!]/` on every arg; blocks `config --global/--system user.name/user.email` (must use `preflight.setGitIdentity` instead) |
| `git.execStream` | `agent-rpc-dispatch.ts:321` | `handleGitExecStream` — `agent-git-handler.ts:204` | same as `git.exec` | streamed (see below) | Same validation; streams stdout/stderr line-by-line via WS frames instead of buffering | same whitelist |
| `git.history` | `agent-rpc-dispatch.ts:334` | `handleGitHistory` — `agent-git-handler-extended.ts:57` | `worktreePath`, `limit?`, `baseRef?: string\|null` | `{items, ...}` | Commit log for a worktree | fixed-shape `git log`, not free-form exec |
| `git.branchCompare` | `:345` | `handleGitBranchCompare` — `:78` | `worktreePath`, `baseRef` | `{summary, entries[]}` | Diffs merge-base..HEAD against `baseRef` | rejects `baseRef` starting with `-` |
| `git.commitCompare` | `:356` | `handleGitCommitCompare` — `:106` | `worktreePath`, `commitId` | `{summary, entries[]}` | Diffs a commit against its parent | requires full-length object IDs |
| `git.branchDiff` | `:367` | `handleGitBranchDiff` — `:117` | `worktreePath`, `baseRef`, `includePatch?`, `filePath?`, `oldPath?` | diff entries array | Per-file diff entries vs a base ref | rejects `baseRef` starting with `-` |
| `git.commitDiff` | `:378` | `handleGitCommitDiff` — `:139` | `worktreePath`, `commitOid`, `parentOid?`, `filePath`, `oldPath?` | single diff-result object | Single-file diff for one commit | `assertFullGitObjectId` enforced |
| `git.checkIgnored` | `:389` | `handleGitCheckIgnored` — `:154` | `worktreePath`, `paths: string[]` | `string[]` (ignored subset) | Batched `git check-ignore` | paths sent over stdin, not argv |
| `git.forkSync` | `:400` | `handleGitForkSync` — `:163` | `worktreePath`, `expectedUpstream` (required) | fork-sync result | Syncs fork's default branch from upstream | `validateGitForkSyncExpectedUpstream({required:true})`; 60s timeout, non-interactive git |
| `git.submoduleStatus` | `:411` | `handleGitSubmoduleStatus` — `:184` | `worktreePath`, `submodulePath`, `area?: 'staged'\|'untracked'\|'unstaged'` | status + `entries[]` | Status for one submodule, incl. commit-range diff if pointer moved | `resolveSubmoduleWorktreePath` traversal guard |
| `git.pr.create` | `:510` | `handleGitHubPrCreate` — `external-api-connector.ts:169` | `title, body?, base?, cwd?, userId?, draft?` | `{url,number,title,state}` (+`alreadyExisted:true` on dedup) | **Alias** of `github.pr.create` — same idempotent impl (dedup by branch), so either method name is safe to call | `SHELL_METACHARACTERS` on `title`/`base`; rate-limit circuit breaker |
| `git.worktree.list` | `:525` | `handleGitWorktreeList` — `agent-git-handler.ts:287` | `cwd?` | `{worktrees: WorktreeInfo[]}` | `git worktree list --porcelain`, parsed via `parseWorktreePorcelain` | none extra |
| `git.worktree.add` | `:536` | `handleGitWorktreeAdd` — `:311` | `path, branch, createBranch?, cwd?` | delegates to `handleGitExec` result | `git worktree add [-b] <branch> <path>` | requires non-empty `path`/`branch`; `SHELL_METACHARACTERS` check; `validateWorktreePath` — path-traversal guard |
| `git.worktree.remove` | `:547` | `handleGitWorktreeRemove` — `:365` | `path, force?, cwd?` | delegates to `handleGitExec` result | `git worktree remove [--force] <path>` | requires non-empty `path`; `SHELL_METACHARACTERS` check |
| `git.clone` | `agent-rpc-dispatch.ts` (after `git.worktree.remove`) | `handleGitClone` — `agent-git-clone-handler.ts` (new, 2026-08-15 pass 5) | `url: string, targetPath: string` | `{path: targetPath}` | `git clone --progress -- <url> <targetPath>`, streams `git.clone.output` via `makeNotifier` | rejects a leading `-` or embedded NUL in `url`/`targetPath` (argv-injection guard); `buildRelayGitEnv()`. Was missing entirely on Part A until this fix — see [`gaps-and-findings.md`](./gaps-and-findings.md) #11. Only the `{url,targetPath}` shape is supported (Part A has no `progressId`-based args-shape caller) |

**`ALLOWED_GIT_SUBCOMMANDS`** for Part A's `git.exec`/`git.execStream`
(`agent-git-handler.ts:41-64`) — **21 subcommands**:
`status, diff, add, restore, commit, push, pull, fetch, branch, checkout, merge,
rebase, stash, log, worktree, remote, tag, show, rev-parse, config, describe,
shortlog`. Still materially **broader** than Part B's whitelist below (no
per-subcommand *shape* restriction — `push`/`pull`/`fetch`/`merge`/`rebase`/
`stash`/arbitrary `commit -m` messages are allowed with real arguments,
since Part A has no dedicated per-operation RPCs to delegate to instead —
see [`gaps-and-findings.md`](./gaps-and-findings.md) #4), but as of the
2026-08-15 pass-3 fix every call also goes through
`agent-git-exec-validator.ts`'s `assertNoGitInjectionFlags`: rejects any
flag before the subcommand (blocks `-c core.sshCommand=...` and other
global-option injection), `--upload-pack=`/`--receive-pack=`/`--exec=`/
`-o`/`--output` anywhere, and `config` without an explicit read-only flag
or with any write flag present (`--file` path traversal, `--global`/
`--system`, etc.) — on top of the pre-existing blanket
shell-metacharacter regex.

### `git.execStream` streaming shape (Part A, inline WS frames)

`{type:'stream.chunk', line, source?}` per line, terminated by
`{type:'stream.end', exitCode}` — sent as multiple frames replying to the
original request id. Fire-and-forget, no ack/credit window.

---

## `fs.*` — Part A (WS Dev Server Agent, `fs-agent-extensions.ts`)

| Method | Handler | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `fs.readDir` | `handleFsReadDir` @35 | `path`, `depth?` (clamped ≤5, default 1) | `{entries: FileTreeNode[], path}` | Recursive directory listing up to depth | **None** — `isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)`; an absolute path is honored unchanged |
| `fs.readFile` | `handleFsReadFile` @97 | `path` | `{content, encoding:'base64'\|'utf-8', isBinary, path}` | Whole-file read w/ binary detection | **None** — same unconfined path resolution |
| `fs.grep` | `handleFsGrep` @149 | `root?` (default workDir), `pattern`, `maxResults?` (≤200, default 50) | `{matches: GrepMatch[], total, truncated}` | `rg --json` (fallback plain `grep -r`) content search | **None** — absolute `root` honored as-is |
| `fs.stat` | `handleFsStat` @322 | `path` | `{path,size,mtime,isDir,isFile,isLink,mode}` | File/dir stat | **None** |
| `fs.glob` | `handleFsGlob` @365 | `pattern`, `cwd?` (default workDir), `ignore?` (default `['node_modules','.git','dist','out']`) | `{paths, cwd, total}` (capped 200) | `find <cwd> -maxdepth 10 ... -name <pattern>` | **None, weakest of all** — `cwd` used directly, no `isAbsolute` check at all |
| `fs.writeFile` | `handleFsWriteFile` @427 | `path`, `content?`, `encoding?` (default utf-8) | `{ok:true, path, bytes}` | Creates parent dirs + writes (10MB cap) | **The only handler with real confinement**: `resolvedPath.startsWith(resolvedWork + '/') \|\| resolvedPath === resolvedWork`, else `InvalidParams "Path outside project root"` |
| `fs.mkdir` | `handleFsMkdir` @524 | `path` | `{ok:true, path}` | `mkdir -p` | **None** |
| `fs.rmdir` | `handleFsRmdir` @554 | `path`, `recursive?` | `{ok:true, path}` | Removes dir (recursive via `rm -rf` if flagged) | **Weak, not real confinement** — only refuses if `config.workDir.startsWith(absPath) \|\| absPath === '/'` (blocks deleting workDir/its ancestors, or `/`); an unrelated absolute path outside workDir passes through |
| `fs.watch` | `handleFsWatch` @683 | `path`; injected `notify` callback | `{ok:true, path}` | Refcounted watch registry (`AGENT_WATCH_MAP`); manual per-subdir `fs.watch()` on Linux (native `recursive` ignored there), single recursive watch on macOS/Windows | **None** on path; capped at `MAX_LINUX_WATCH_DIRS=4000` |
| `fs.unwatch` | `handleFsUnwatch` @733 | `path` | `{ok:true}` | Decrements refcount, closes watcher(s) at 0 | n/a |
| `fs.listDirectory` | `handleFsListDirectory` — `fs-agent-directory-browse.ts` (new, 2026-08-15 pass 5) | `path`, `includeGitStatus?` | `{entries:[{name,path,isDirectory:true,isGitRepo}],platform}` | Subdirectory-only listing for a folder picker — near-verbatim port of Part B's `FsDirectoryBrowserHandler.listDirectory` | **none** — raw `params.path` used directly, matching Part B's equally-unconfined behavior. Was missing entirely on Part A until this fix — see [`gaps-and-findings.md`](./gaps-and-findings.md) #11 |

There is **no shared path-confinement helper** across these handlers — each
resolves `isAbsolute(raw) ? raw : join(config.workDir, raw)` ad hoc; only
`fs.writeFile` enforces containment. See
[`gaps-and-findings.md`](./gaps-and-findings.md).

### `fs.watch` push-notification (Part A)

Push method `fs.changed` → `{path, eventType, filename}` — **one event per
notification**, singular shape (contrast with Part B's batched shape below).

---

## `git.*` — Part B (SSH Relay Daemon, `GitHandler` + ops modules)

### Staging / working-tree ops

| Method | Handler | Delegates to | Params | Git commands | Returns | Security |
|---|---|---|---|---|---|---|
| `git.status` | `getStatus` @445 | `getStatusOp` (`git-handler-status-ops.ts:59`) | `worktreePath`, `includeIgnored?`, `limit?`, `bypassEffectiveUpstreamNegativeCache?` | `git -c core.quotePath=false status --porcelain=v2 --branch --untracked-files=all [--ignored=matching]`, `disableOptionalLocks`; per-area `diff --numstat -M` | `{entries[],conflictOperation,head?,branch?,upstreamStatus?,ignoredPaths?,didHitLimit?,statusLength?}` | `limit` coerced to finite ≥0 int |
| `git.submoduleStatus` | `getSubmoduleStatus` @454 | `getStatusOp` + `git-handler-submodule-ops.ts` | `worktreePath`,`submodulePath`,`area?` | status inside resolved submodule dir + range diff if pointer moved | merged status result | `resolveSubmoduleWorktreePath` rejects empty/NUL/absolute/escaping paths |
| `git.checkIgnored` | `checkIgnored` @496 | `checkIgnoredPathsOp` (`git-handler-check-ignore.ts:10`) | `worktreePath`,`paths[]` | `git check-ignore` via stdin, chunked | `string[]` | paths via stdin, not argv; exit 1 treated as success |
| `git.diff` | `getDiff` @508 | `computeDiff` (`git-handler-ops.ts:77`) | `worktreePath`,`filePath`,`staged`,`compareAgainstHead?`,`__streamResponse?` | `git show <oid>:<path>` (readBlobAtOid/Index) + working-file read | `buildDiffResult(...)` diff object (or streamed sentinel) | explicit `path.resolve`/`path.relative` traversal guard |
| `git.stage` | `stage` @590 | — | `worktreePath`,`filePath` | `git add -- :(literal)<filePath>` | void | literal pathspec |
| `git.unstage` | `unstage` @622 | — | `worktreePath`,`filePath` | `git restore --staged -- :(literal)<filePath>` | void | literal pathspec |
| `git.bulkStage` | `bulkStage` @633 | — | `worktreePath`,`filePaths[]` | chunked (100) `git add --` | void | literal pathspecs |
| `git.bulkUnstage` | `bulkUnstage` @650 | — | `worktreePath`,`filePaths[]` | chunked `git restore --staged --` | void | literal pathspecs |
| `git.abortMerge` | `abortMerge` @667 | — | `worktreePath` | `git merge --abort` | void | none |
| `git.abortRebase` | `abortRebase` @677 | — | `worktreePath` | `git rebase --abort` | void | none |
| `git.checkout` | `checkout` @687 | — | `worktreePath`,`branch` | `git checkout <branch> --` | `{ok:true,branch}` | rejects `branch` starting with `-` |
| `git.localBranches` | `localBranches` @706 | — | `worktreePath` | `git for-each-ref --format=%(HEAD)%09%(refname:short) refs/heads/` | `{current,branches[]}` | none |
| `git.discard` | `discard` @760 | `removeSafeUntrackedDiscardTarget` | `worktreePath`,`filePath` | `git restore --worktree --source=HEAD` or `git clean -ffdx` | void | `assertInWorktree` (rejects `.`/`..`/absolute/root-resolving paths) |
| `git.bulkDiscard` | `bulkDiscard` @794 | shared helper | `worktreePath`,`filePaths[]` | tracked/untracked split, chunked restore/clean | void | `assertInWorktree` per path first |
| `git.conflictOperation` | `conflictOperation` @871 | `detectConflictOperation` (`git-handler-status-ops.ts:38`) | `worktreePath` | fs checks on `.git`/`MERGE_HEAD`/`rebase-merge`/`CHERRY_PICK_HEAD` | `'merge'\|'rebase'\|'cherry-pick'\|'unknown'` | `resolveGitDir` follows worktree `gitdir:` pointer |
| `git.commit` | `commit` @601 | `commitChangesRelay` (`git-handler-worktree-ops.ts:153`) | `worktreePath`,`message` | `git commit -m <message>` w/ per-connection identity env | `{success,error?}` | rejects empty/whitespace message; identity from `getClientGitIdentity(clientId)` — never global config |

### Branch/commit compare & diff

| Method | Handler | Delegates to | Params | Git commands | Returns | Security |
|---|---|---|---|---|---|---|
| `git.branchCompare` | `branchCompare` @876 | `branchCompareOp` (`git-handler-ops.ts:124`) | `worktreePath`,`baseRef` | `rev-parse HEAD/baseRef`, `merge-base`, `diff --name-status -M -C`+numstat, `rev-list --count` | `{summary{...},entries[]}` | rejects `baseRef` starting `-` |
| `git.commitCompare` | `commitCompare` @900 | `commitCompareOp` (`git-handler-commit-diff-ops.ts:15`) | `worktreePath`,`commitId` | `rev-parse --verify <id>^{commit}`, `rev-list --parents`, `diff`/`diff-tree --root` | `{summary{...},entries[]}` | `assertFullGitObjectId` — full 40/64-hex SHA only |
| `git.branchDiff` | `branchDiff` @1198 | `branchDiffEntries` (`git-handler-ops.ts:218`) | `worktreePath`,`baseRef`,`includePatch?`,`filePath?`,`oldPath?`,`__streamResponse?` | merge-base diff + per-file blob reads if `includePatch` | diff-result array (or streamed) | rejects `baseRef` starting `-`; in-flight dedupe |
| `git.commitDiff` | `commitDiff` @1230 | `commitDiffEntry` (`git-handler-commit-diff-ops.ts:124`) | `worktreePath`,`commitOid`,`parentOid?`,`filePath`,`oldPath?`,`__streamResponse?` | `git show <oid>:<path>` for both sides | single diff-result object | `assertFullGitObjectId` on both OIDs |
| `git.upstreamStatus` | `upstreamStatus` @906 | shared `git-publish-target-status`/`git-effective-upstream` | `worktreePath`,`pushTarget?` | `check-ref-format`, `log --cherry-mark` (patch-equiv check) | upstream status object | `assertGitPushTargetShape` |
| `git.history` | `history` @500 | shared `loadGitHistoryFromExecutor` | `worktreePath`,`limit?`,`baseRef?` | `git log --format=... -z --topo-order --decorate=full -n<limit+1>` | `{items,currentRef?,remoteRef?,baseRef?,hasIncomingChanges,hasOutgoingChanges,hasMore,limit}` | `limit` clamped |
| `git.isGitRepo` | `isGitRepo` @1389 | — | `dirPath` | `git rev-parse --show-toplevel` | `{isRepo,rootPath}` | failure → not-a-repo, not error |

### Remote ops (fetch/push/pull/rebase/fork-sync)

| Method | Handler | Params | Git commands | Returns | Security |
|---|---|---|---|---|---|
| `git.fetch` | `fetch` @954 | `worktreePath`,`pushTarget?` | `git fetch --prune [remote]` | void | `assertGitPushTargetShape` + `check-ref-format`; errors normalized (no path leakage) |
| `git.fetchRemoteTrackingRef` | `fetchRemoteTrackingRef` @1010 | `worktreePath`,`remote`,`branch`,`ref`,`skipAutoMaintenance?` | `git fetch --no-tags <remote> +refs/heads/<branch>:<ref>` | void | remote must be in `git remote` output; `ref` must equal exactly `refs/remotes/<remote>/<branch>` — narrower than `git.exec`, which blocks `fetch` entirely |
| `git.fetchGitLabMergeRequestHead` | `fetchGitLabMergeRequestHead` @1063 | `worktreePath`,`remote`,`mrIid: number` | `git fetch --no-tags <remote> refs/merge-requests/<mrIid>/head` | void | `mrIid` positive safe integer; `remote` validated against configured remotes |
| `git.push` | `push` @1103 | `worktreePath`,`pushTarget?`,`forceWithLease?`,`publish` | `git push [--force-with-lease] --set-upstream <remote> <refspec>` | void | `resolveRelayPushTarget` (`git-handler-push-target.ts:159`) — never pushes to a bare URL (resolves to a configured remote name first); blocks pushing a fork-tracking branch to `origin` unless `branch.<b>.merge` targets itself |
| `git.pull` | `pull`→`pullWithArgs` @1133/1169 | `worktreePath`,`pushTarget?` | `git pull [<remote> <branch>]` | void | same target validation as push/fetch |
| `git.fastForward` | `fastForward`→`pullWithArgs(['--ff-only'])` @1175 | same | `git pull --ff-only ...` | void | same |
| `git.rebaseFromBase` | `rebaseFromBase` @1179 | `worktreePath`,`baseRef` | `git pull --rebase <remote> <branch>` | void | source resolved via `resolveGitRemoteRebaseSource` |
| `git.forkSync` | `forkSync` @978 | `worktreePath`,`expectedUpstream` | delegated to `syncForkDefaultBranch` | delegated result | `validateGitForkSyncExpectedUpstream({required:true})`; non-interactive, 60s abortable timeout |

### Worktree ops

| Method | Handler | Delegates to | Params | Git commands | Returns | Security |
|---|---|---|---|---|---|---|
| `git.listWorktrees` | `listWorktrees` @1461 | `parseWorktreeList` (`git-handler-utils.ts:138`) | `repoPath` | `git worktree list --porcelain -z` (fallback for git&lt;2.36) | array of worktree info | fails closed to `[]` on error |
| `git.addWorktree` | `addWorktree` @1492 | `addWorktreeOp` (`git-handler-worktree-ops.ts:31`) | `repoPath`,`branchName`,`targetDir`,`base?`,`checkoutExistingBranch?`,`noCheckout?` | `git worktree add [--no-track][-b <branch>][--no-checkout] <dir> [<base>]`; sets `branch.<n>.base`, `push.autoSetupRemote` | void | rejects `branchName`/`base` starting `-`; `validateWorktreePath` confines target to `workDir`/parent/`/tmp`/`/var/tmp`, rejects NUL |
| `git.removeWorktree` | `removeWorktree` @1496 | `removeWorktreeOp` (`git-handler-worktree-remove.ts:124`) | `worktreePath`,`force?`,`deleteBranch?`(default true),`forceBranchDelete?` | `git worktree remove [--force]`; `branch -d/-D` w/ prune-retry | `{}` or `{preservedBranch:{branchName,head?}}` | `assertWorktreeUnlockedForRemoval` blocks removing a locked worktree; safe `-d` unless forced |
| `git.worktreeIsClean` | `worktreeIsClean` @1502 | `worktreeIsCleanOp` (`git-handler-worktree-ops.ts:139`) | `worktreePath`,`includeUntracked?`(default true) | `git status --porcelain [--untracked-files=all\|no]` | `{clean,stdout?}` | none |
| `git.refreshLocalBaseRefForWorktreeCreate` | same-named @1506 | `refreshLocalBaseRefForWorktreeCreateOp` | `repoPath`,`fullRef`,`remoteTrackingRef`,`ownerWorktreePath?`,`checkOnly?` | `check-ref-format`, `merge-base --is-ancestor`, `reset --hard` or `update-ref` CAS | void | requires `refs/heads/`/`refs/remotes/` prefixes; re-verifies fast-forward + clean tree server-side (closes TOCTOU) |
| `git.renameCurrentBranch` | `renameCurrentBranch` @1346 | — | `worktreePath`,`newBranch` | `check-ref-format --branch`, `git branch -m` | void | rejects `-`-prefixed name; exists because `git.exec` blocks destructive branch flags |
| `git.forceDeletePreservedBranch` | `forceDeletePreservedBranch` @1367 | `forceDeletePreservedRelayBranch` (`git-handler-branch-cleanup.ts:33`) | `repoPath`,`branchName`,`expectedHead` | checked-out guard via `worktree list`; `git update-ref -d refs/heads/<b> <expectedHead>` (CAS) | void | rejects empty/NUL/`-`-prefixed names; compare-and-swap prevents deleting a moved ref |

### Exec / clone (whitelisted passthrough)

| Method | Handler | Params | Returns | Security |
|---|---|---|---|---|
| `git.exec` | `exec` @1252 | `args: string[]`,`cwd`,`__streamResponse?` | `{stdout,stderr}` (or streamed) | **Full per-subcommand whitelist** — see below |
| `git.clone` | `handleClone` (dispatch) → `clone`/`spawnClone` (args-based) or `cloneSimple`/`spawnCloneSimple` (url-based) | **Either** `{args: string[], cwd, progressId}` **or** `{url: string, targetPath: string}` — the handler detects which shape it received | `{stdout,stderr}` (args shape) or `{path: targetPath}` (url shape) | **Both shapes validated.** Args shape: `validateGitExecArgs` + `validateCloneArgs` (`clone [--progress] -- <url> <dir>`, dir must be one safe path segment). Url shape: rejects a leading `-` or embedded NUL in either `url` or `targetPath` (argv-injection guard), uses `buildRelayGitEnv()`. Streams `git.clone.output` `{data, clientId}` for the url shape (stdout+stderr), `git.cloneProgress` `{progressId, phase, percent}` for the args shape. |

**Fixed (was a handler-override bug):** `git.clone` used to have two
independently-registered handlers with incompatible param shapes —
`RelayDispatcher.onRequest` is last-registration-wins with no duplicate-key
guard, so `GitCloneHandler`'s later, unvalidated registration silently
shadowed `GitHandler`'s validated one, and every real `git.clone` call was
served by the unvalidated path. Both shapes turned out to have genuine
live callers (`repo-remote-ipc.ts` uses the url shape,
`ssh-git-provider.ts` uses the args shape), so the fix merges both behind
one dispatch (`GitHandler.handleClone`) rather than picking one — see
[`gaps-and-findings.md`](./gaps-and-findings.md) #3 for the full before/after.
`RelayDispatcher.onRequest`/`onNotification` now also warn on any duplicate
registration, so this bug class is caught at wiring time going forward.

### `git-exec-validator.ts` — `git.exec`/`git.clone` command whitelist (Part B)

Global preconditions: no `-`-prefixed args before the subcommand (blocks
`-c key=value` injection); `GLOBAL_DENIED_FLAGS` on every subcommand:
`--output, -o, --exec-path, --work-tree, --git-dir` (also catches
`--flag=value` form).

`ALLOWED_GIT_SUBCOMMANDS` — **14 subcommands only**, each with its own extra rule:

- `rev-parse`, `log`, `show-ref`, `ls-remote`, `merge-base`, `ls-files`,
  `for-each-ref`, `check-ref-format` — no extra restriction.
- `branch` — denies `BRANCH_DESTRUCTIVE_FLAGS`: `-d -D --delete -m -M --move
  -c -C --copy`.
- `remote` — denies write subcommands `REMOTE_WRITE_SUBCOMMANDS`: `add remove
  rm rename set-head set-branches set-url prune update`.
- `symbolic-ref` — denies `-d --delete -m`; rejects ≥2 positional args.
- `diff` — must include `--cached`/`--staged`; every other arg must be in
  `DIFF_ALLOWED_FLAGS`: `--cached --staged --name-status --patch --minimal
  --no-color --no-ext-diff`.
- `clone` — must be exactly `clone [--progress] -- <url> <dir>`; `<dir>` a
  single safe segment (no `/`,`\`,`\0`, not `.`/`..`).
- `init` — must be exactly `['init']`.
- `commit` — must be exactly `['commit','--allow-empty','-m',<message>]`.
- `config` — must include a `CONFIG_READ_ONLY_FLAGS`
  (`--get --get-all --list --get-regexp -l`); denies `CONFIG_WRITE_FLAGS`
  (`--add --unset --unset-all --replace-all --rename-section --remove-section
  --edit -e --file -f --global --system`) — `--file`/`-f` specifically denied
  because it can redirect config reads to leak an arbitrary file's contents.

**Not allowed at all** (rejected outright, unlike Part A's `git.exec`):
`status, add, restore, push, pull, fetch, checkout, merge, rebase, stash,
worktree, tag, show, describe, shortlog, clean, gc, bisect`.

### `git-response-stream.ts` — streaming protocol for `git.diff`/`branchDiff`/`commitDiff`/`exec`

Triggered when the request sets `params.__streamResponse === true` and the
serialized result exceeds `GIT_RESPONSE_STREAM_THRESHOLD` (256KB). Sentinel RPC
result: `{__orcaGitResponseStream: {streamId, totalBytes, chunkCount}}`.
Payload base64-chunked at `GIT_RESPONSE_CHUNK_SIZE` (128KB) so multi-byte UTF-8
never splits mid-chunk.

- `git.responseChunk` (bulk-lane notify) → `{streamId, seq, data: base64}`
- `git.responseEnd` (bulk-lane notify) → `{streamId}` (normal completion only)
- `git.responseError` (bulk-lane notify) → `{streamId, message}`
- `git.responseAck` (client→relay notification) → `{streamId, seq}` — cumulative
  ack, gated to `entry.ownerClientId === clientId`
- `git.cancelResponseStream` (client→relay notification) → `{streamId}` —
  aborts, gated to owning client

Credit window: at most `STREAM_ACK_WINDOW_CHUNKS` = 4 unacked chunks in flight;
`waitForAck` has a 1000ms (`STREAM_ACK_STALL_RECHECK_MS`) safety re-check. On
client detach, `dispatcher.onClientDetached` wakes all parked pumps. `git.clone`
does **not** use this mechanism (has its own `git.cloneProgress`/
`git.clone.output` channel and truncates buffered output to 4096 chars).

---

## `fs.*` — Part B (SSH Relay Daemon, `FsHandler` / `FsDirectoryBrowserHandler`)

| Method | Handler | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `fs.readDir` | `readDir` @130 | `dirPath` | `Array<{name,isDirectory,isSymlink}>` | Immediate-children listing, resolves symlinked dirs | none beyond `expandTilde` |
| `fs.readFile` | `readFile` @148 → `readRelayFileContent` (`fs-handler-file-read.ts:15`) | `filePath` | `{content,isBinary,isImage?,mimeType?}` | Whole-file read | size cap `MAX_TEXT_FILE_SIZE`/`MAX_PREVIEWABLE_BINARY_SIZE`; binary sniff via NUL-byte probe |
| `fs.readFileStream` | `readFileStream` @160 → `readRelayFileStreamMetadata` (`file-read.ts:73`) | `filePath`,`flowControl?:'ack'` | `StreamMetadata` (see below) | Chunked large-file read | same size cap; registry caps `MAX_CONCURRENT_STREAMS=16` |
| `fs.readTerminalArtifact` | `readVerifiedTerminalArtifact` (`fs-handler-terminal-artifact.ts:33`) | `filePath`,`expectedRealPath`,`expectedStatIdentity?`,`maxBytes?` | `{content,isBinary,isImage?,mimeType?}` | Reads a terminal-produced artifact w/ identity verification | realpath must equal `expectedRealPath`; `O_NOFOLLOW` (no symlink follow); rejects hard-linked files (`nlink>1`); dev/ino/nlink/size/mtime match |
| `fs.tempDir` | `tempDir` @172 | none | `string` | OS tmpdir path | none |
| `fs.writeFile` | `writeFile` @191 | `filePath`,`content` | void | UTF-8 write, rejects if target is a dir | only dir-vs-file check, **no path confinement** |
| `fs.writeTerminalArtifact` | `writeVerifiedTerminalArtifact` (`terminal-artifact.ts:57`) | `filePath,content,expectedRealPath,expectedStatIdentity?,maxBytes?` | `{ok:true,stat}` | Atomic temp+rename overwrite | same identity checks, re-verified before AND after write |
| `fs.stat` | `stat` @214 | `filePath` | `{size,type,mtime,mtimeMs,dev?,ino?,nlink?}` | Follows symlinks to target | none |
| `fs.lstat` | `lstat` @238 | `filePath` | same shape, no-follow | Doesn't follow symlinks | none |
| `fs.deletePath` | `deletePath` @243 | `targetPath`,`recursive?` | void | Delete file/dir | only recursive-flag guard, **no path confinement** |
| `fs.createFile` | `createFile` @253 | `filePath` | void | `mkdir -p` parent + exclusive create (`wx`) | none |
| `fs.createDir` | `createDir` @260 | `dirPath` | void | Recursive mkdir | none |
| `fs.createDirNoClobber` | `createDirNoClobber` @265 | `dirPath` | void | Non-recursive, fails if exists | none |
| `fs.rename` | `rename` @270 | `oldPath,newPath` | void | Move, overwrites dest | none |
| `fs.renameNoClobber` | `renameNoClobber` @276 | `oldPath,newPath` | void | Move, no overwrite | `assertNoClobberRenameDestinationAvailable` (checked remotely on the actual remote FS) |
| `fs.copy` | `copy` @285 | `source,destination` | void | Recursive copy tree, fails if dest exists | `errorOnExist:true` |
| `fs.realpath` | `realpath` @299 | `filePath` | `string` | Canonicalize path | n/a (this is the primitive) |
| `fs.search` | `search` @304 → `searchWithRg`/`searchWithGitGrep` | `query,rootPath,caseSensitive?,wholeWord?,useRegex?,includePattern?,excludePattern?,maxResults?` | `SearchResult` | Content search, rg → git-grep fallback | `maxResults` clamped to `DEFAULT_MAX_RESULTS`(2000); subprocess `cwd` scoped to `rootPath`, no explicit traversal check beyond that |
| `fs.listFiles` | `listFiles`/`runListFilesScan` @339 → rg / git ls-files / readdir walk | `rootPath`,`excludePaths?` | `string[]` (root-relative) | Full workspace enumeration for Quick Open | `ListFilesScanCoordinator` caps one full scan per client, coalesces duplicates |
| `fs.workspaceSpaceScan` | `workspaceSpaceScan` @411 → `scanWorkspaceSpaceDirectory` | `rootPath` | delegated | Scans a workspace "Space" dir for cleanup UI | not fully audited |
| `fs.watch` (request) | `RelayFilesystemWatchRegistry.watch` @61 | `rootPath` | resolves once initial subscribe completes | Subscribes client to change events under root, refcounted | `MAX_RELAY_WATCH_ROOTS=20` |
| `fs.unwatch` (notification) | `RelayFilesystemWatchRegistry.unwatch` @90 | `rootPath` | none | Releases client's watch interest | none |
| `fs.cancelStream` (notification) | `cancelStream` @176 → `RelayStreamRegistry.abort` | `streamId` | none | Aborts in-progress `fs.readFileStream` pump | none |
| `fs.streamAck` (notification) | `streamAck` @183 → `RelayStreamRegistry.recordAck` | `streamId,seq` | none | Advances credit window for stream pacing | none |
| `fs.listDirectory` | `FsDirectoryBrowserHandler.listDirectory` (`fs-handler-directory-browse.ts:26`) | `path`,`includeGitStatus?` | `{entries:[{name,path,isDirectory:true,isGitRepo}],platform}` | Subdirectory-only listing for a folder picker | **none** — no `expandTilde` even, raw `params.path` used directly |

**No path confinement anywhere in Part B's `fs.*` surface** — deliberate and
documented: `context.ts:20-29` states the FS-side allowlist was intentionally
removed because "the relay runs as the SSH user and trusts the renderer
process... a compromised renderer can already weaponize `pty.spawn`/`git.exec`
to reach any path the SSH user can reach" (citing a since-deleted
`docs/relay-fs-allowlist-removal.md` — broken doc reference, see
[`gaps-and-findings.md`](./gaps-and-findings.md)). The only two methods with
any per-path integrity check are `fs.readTerminalArtifact`/
`fs.writeTerminalArtifact`, and those check TOCTOU identity of a
previously-granted path, not workspace confinement. `RelayContext`
(`context.ts`) is an inert stub — `registerRoot()` is a documented no-op kept
only for wire back-compat.

### `fs.watch` push-notification protocol (Part B, batched — contrast with Part A)

- Push method: `fs.changed` via `dispatcher.notify`.
- Normal: `{events: [{kind: <WatcherProcessEvent.type>, absolutePath,
  isDirectory?}, ...]}`, batched (`MAX_BATCHED_WATCHER_EVENTS`).
- Overflow/error: `{events: [{kind:'overflow', absolutePath: rootPath}]}` —
  synthetic "re-scan this root" signal on watcher error/interrupt/buffer-overflow.
- Recovery: `recoverWatch()` emits overflow then re-subscribes with a monotonic
  `generation` fence; unrecoverable → watch closed, client must re-`fs.watch`.
- No event-level idempotency/dedup — subscription refcounting only.

### `fs.readFileStream` streaming protocol (Part B, `fs-stream-registry.ts`)

Constants: `STREAM_CHUNK_SIZE=256KB`, `MAX_CONCURRENT_STREAMS=16`,
`STREAM_ACK_WINDOW_CHUNKS=4`, `STREAM_ACK_STALL_RECHECK_MS=1000ms`.

- Setup response (`StreamMetadata`): `{streamId?, totalSize, isBinary, isImage?,
  mimeType?, chunkEncoding?:'base64', resultEncoding?, empty?}`.
- Chunk push (bulk lane): `fs.streamChunk` → `{streamId, seq, data: base64}`,
  each exactly `STREAM_CHUNK_SIZE` except possibly the last (short-read guard
  → `ESTREAMTRUNCATED`).
- Ack (client→relay): `fs.streamAck` → `{streamId, seq}` (cumulative, monotonic).
- Cancel (client→relay): `fs.cancelStream` → `{streamId}`.
- Termination: success → `fs.streamEnd {streamId}`; failure →
  `fs.streamError {streamId, code, message}`; cancel/stale → silent exit, no
  terminal frame.
- Cleanup: every path funnels through `registry.release(streamId)`.

---

## Streaming/push protocol summary (git/fs only)

| Protocol | Namespace | Part | Trigger | Push method(s) | Ack/cancel |
|---|---|---|---|---|---|
| `git.execStream` | `git.*` | A | `git.execStream` call | `{type:'stream.chunk',line,source?}`, `{type:'stream.end',exitCode}` | none |
| `fs.watch` | `fs.*` | A | `fs.watch` call | `fs.changed` → `{path,eventType,filename}` | `fs.unwatch` |
| git response streaming | `git.diff`/`branchDiff`/`commitDiff`/`exec` | B | `__streamResponse:true` + result &gt;256KB | `git.responseChunk`/`git.responseEnd`/`git.responseError` | `git.responseAck`/`git.cancelResponseStream` |
| `git.clone` progress | `git.clone` | B | any clone call | `git.clone.output` (url-shape callers) or `git.cloneProgress` (args-shape callers) — both live now that both shapes route through `handleClone`, see fix in #3 | none |
| `fs.watch` | `fs.*` | B | `fs.watch` call | `fs.changed` → `{events:[{kind,absolutePath,isDirectory?}]}` (batched) or overflow | `fs.unwatch` |
| `fs.readFileStream` | `fs.*` | B | `fs.readFileStream` call | `fs.streamChunk`/`fs.streamEnd`/`fs.streamError` | `fs.streamAck`/`fs.cancelStream` |

---

## Sources

`agent/src/relay/agent-rpc-dispatch.ts`, `agent-git-handler.ts`,
`agent-git-handler-extended.ts`, `agent-git-exec-validator.ts`, `fs-agent-extensions.ts`,
`external-api-connector.ts`, `relay.ts`, `dispatcher.ts`, `context.ts`,
`git-handler.ts`, `git-handler-ops.ts`, `-status-ops.ts`, `-commit-diff-ops.ts`,
`-worktree-ops.ts`, `-worktree-list.ts`, `-worktree-remove.ts`,
`-branch-cleanup.ts`, `-push-target.ts`, `-submodule-ops.ts`,
`-check-ignore.ts`, `-clone.ts`, `-utils.ts`, `-local-base-ref-refresh.ts`,
`git-exec-validator.ts`, `git-response-stream.ts`, `fs-handler.ts`,
`fs-handler-file-read.ts`, `-list-files.ts`, `-directory-browse.ts`,
`-terminal-artifact.ts`, `-git-fallback.ts`, `-utils.ts`, `-install-rg.ts`,
`-readdir-fallback.ts`, `relay-filesystem-watch-registry.ts`,
`fs-stream-registry.ts`.
