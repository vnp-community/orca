# BUG-021: Nearly every `git.*` wscompat channel decodes `worktreeId` — every real frontend caller sends `worktree` (a selector string) instead

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_git.go` (`registerGitChannels` and friends — ~40 handlers)
**Severity:** 🔴 CRITICAL — every git write/history/branch/diff operation in Project Workspace fails with `GITGATEWAY_MISSING_WORKTREE_ID`, not just push
**Status:** ✅ Root cause CONFIRMED (systemic — same bug class already fixed once for `git.status` alone, never audited further). ✅ Fixed for all ~40 `git.*` handlers + unit-tested (build/vet/test all pass, new regression tests for `git.push`'s `pushTarget` mapping specifically), ✅ **deployed** (`2026.09.15-git-worktree-key-fix`). `files.*` handlers have a related-but-distinct mismatch, tracked separately (see "Related, not fixed here" below).

## Symptom (live, user-reported)

```
Push failed: rpc error: code = InvalidArgument desc = GITGATEWAY_MISSING_WORKTREE_ID: worktree_id is required
```
Reported immediately after BUG-020 (git.status crash fix) let the Git tab actually render for the first time — the user tried to Sync (push) and hit the exact same underlying "worktree id never reaches git-gateway-service" shape as BUG-012/CR-PW-010, but at a completely different layer (this is a wscompat arg-decoding bug, not a resolver/dispatch bug).

## Root cause — confirmed systemic, not isolated to push

`git.status`'s own handler (`channels.go:522-542`) already carries this exact diagnosis in its comment, written when THAT ONE handler was fixed:

> "Every real caller (WorkspaceContext.tsx, useGit.ts, use-code-review.ts, runtime-git-client.ts, web-preload-api.ts's status) sends the selector under `"worktree"` (toRuntimeWorktreeSelector, `"id:"`-prefixed) — never `"worktreeId"`."

**That fix was never extended to any other `git.*` handler.** Auditing `channels_git.go` (grep for `WorktreeID string`) found ~40 handlers still decoding the wrong key: `git.commit`, `git.push`, `git.pull`, `git.generateCommitMessage`, `git.diff`, `git.checkout`, `git.localBranches`, `git.fastForward`, `git.rebaseFromBase`, `git.abortRebase`, `git.abortMerge`, `git.conflictOperation`, `git.resolveConflict`, `git.discard`, `git.bulkDiscard`, `git.stage`/`git.bulkStage`, `git.unstage`/`git.bulkUnstage`, `git.history`, `git.checkIgnored`, `git.forkSync`, `git.upstreamStatus`, `git.commitCompare`, `git.branchCompare`, `git.commitDiff`, `git.branchDiff`, `git.submoduleStatus`, `git.fetch`, `git.remoteCommitUrl`, `git.remoteFileUrl`, `git.generatePullRequestFields`, `git.merge`, `git.stash.push`, `git.stash.pop`, `git.branch.create`, `git.branch.delete`, `git.push.progress`, `git.pull.progress`.

Confirmed live via `frontend/src/renderer/src/runtime/runtime-git-client.ts`: **every single one** of these operations is called as `{worktree: toRuntimeWorktreeSelector(context.worktreeId), ...otherFields}` — never `{worktreeId: ...}`. So every one of these ~40 backend handlers has `in.WorktreeID` always empty, tripping git-gateway-service's `GITGATEWAY_MISSING_WORKTREE_ID` guard on every real call — this whole surface has likely never worked, just never noticed because `git.status` (the very first call any Git tab render makes) itself failed first, for unrelated reasons (BUG-012/CR-PW-010/011), until this session's fixes finally let it through.

## Fix

Mechanical, uniform across all ~40 sites (same shape `git.status`'s existing fix already established):
1. Rename each handler's `WorktreeID string \`json:"worktreeId"\`` field tag to `\`json:"worktree"\``.
2. Wrap every use of the decoded value with `stripWorktreeSelectorPrefix(in.WorktreeID)` (existing helper, `channels_worktree.go:709`, strips the `"id:"` prefix) before passing it into the proto request (or the in-memory generate-message/PR-fields cancellation key, which also uses it).

`git.push` specifically also needed one more mapping (not just the key rename): the real caller (`useGit.ts`'s `push()`) sends `pushTarget: {remoteName, branchName}`, not flat `remote`/`branch` fields — `PushRequest` proto has no `pushTarget` concept, so the handler now reads `pushTarget.remoteName`/`pushTarget.branchName` (falling back to flat `remote`/`branch` if sent directly, for any other caller) — `publish`/`forceWithLease` are still not honored (pre-existing, already-documented `relay_executor.go` limitation — SOL-032 §0 open question #1, unrelated to this fix).

## Related, NOT fixed here — needs its own audit

`internal/adapter/wscompat/channels_git.go`'s `files.*` handlers (readFile, writeFile, statFile, readDir, createFile, deleteFile, renameFile, copyFile, search, etc. — lines ~935-1165) have the **same** `worktreeId`-vs-`worktree` mismatch, confirmed via the same grep — **plus an additional, distinct mismatch**: the real frontend caller (`runtime-file-client.ts`) sends `relativePath`, while these handlers decode `path`. Fixing `files.*` needs its own per-handler audit (param names may differ more than just these two), not a blind copy of this fix — deliberately out of scope for this pass, which is scoped to the user's actually-reported symptom (Git tab operations).

## Testing

- Existing `TestGitStatusChannel_DecodesWorktreeSelector`-style tests exist for a few handlers already (`git.status`, `git.upstreamStatus` per earlier greps) — extend this pattern to at least `git.push` (the reported symptom) and spot-check 2-3 others (`git.commit`, `git.stage`) for the same regression guard shape.
- Full `go test ./services/api-gateway/...` as the broad safety net given the volume of sites touched.

## Related

- [BUG-020](./BUG-020-git-status-wscompat-channel-returns-raw-proto-not-frontend-shape.md) — the sibling bug (git.status's response SHAPE was wrong; this bug is EVERY OTHER git.* call's request KEY being wrong) found in the same investigation chain, both only exposed once SOL-013/014 made `GetStatus` succeed for the first time.
- [CR-PW-010](../../../docs/crs/v3/project-workspace/CR-PW-010-git-status-worktree-id-resolver-broken.md)/[CR-PW-011](../../../docs/crs/v3/project-workspace/CR-PW-011-dispatch-executor-never-relays-to-dev-server.md) — the deeper git-gateway-service-side bugs already fixed; this bug is a shallower, wscompat-layer bug on a completely different code path (api-gateway, not git-gateway-service), coincidentally producing the same `GITGATEWAY_MISSING_WORKTREE_ID` error text.
