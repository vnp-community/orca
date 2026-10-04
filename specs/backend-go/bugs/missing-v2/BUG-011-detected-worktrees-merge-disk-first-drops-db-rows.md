# BUG-011: `mergeDetectedWorktrees` is disk-first — a DB-tracked worktree silently disappears from the sidebar whenever live on-disk detection returns it late or empty

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_worktree.go` (`mergeDetectedWorktrees`, backs `worktree.detectedList`)
**Severity:** High — makes a just-created, fully valid worktree invisible in "Project Workspace (Beta)"'s sidebar, with no error shown anywhere
**Symptom:** User creates a worktree successfully (confirmed in DB: `project.worktrees` row exists, `status='active'`, `active=true`, correct `project_id`/`repo_id`/`path`). Opening the project in "Project Workspace (Beta)" shows only "Loading…" then "No worktree selected — Select a worktree from the sidebar to continue" — **the sidebar has nothing to select**, no error toast, no console error.
**Status:** 🟡 **Correction (2026-09-15, same day)** — the disk-first merge itself is **intentional, documented design**, not an oversight; see "Correction" section below. Real question is now narrower and still open.

---

## ⚠️ Correction — the original diagnosis below was incomplete

Re-reading `mergeDetectedWorktrees`'s own doc comment (present in the source all along, missed in the first pass):

> "A bookkept worktree missing from onDisk is deliberately NOT included — the frontend's own reconciliation (`getRemovedWorktreeIdsAfterAuthoritativeScan` in `store/slices/worktrees.ts`) already purges a bookkept id that isn't in this result's ids... duplicating that purge decision here would be a second, harder-to-keep-in-sync source of truth."

This is a real, deliberate contract: `worktree.detectedList`'s result is meant to be **authoritative** (the handler literally sets `"authoritative": true` unconditionally) — the frontend uses "not in this list" as its **signal to purge a deleted worktree** from local state. Naively adding a union pass (DB rows not on disk still shown) — my original "fix direction" below — would **fight this existing mechanism**: a worktree deleted for real would risk reappearing, or the two purge decisions (frontend's vs. this union) could disagree and thrash. **Do not implement the union fix as originally written** without redesigning the frontend purge logic to match, which is out of scope for a quick fix.

Also confirmed: `worktree.detectedList`'s handler (`channels_worktree.go:504-528`) uses `errgroup.Wait()` — if `DetectWorktrees` itself errors, the **whole handler returns an error**, it never reaches `mergeDetectedWorktrees` with a silently-incomplete `onDisk`. So `mergeDetectedWorktrees` only ever runs after `DetectWorktrees` **succeeded** — meaning the real open question is narrower than originally framed: **why did a successful `DetectWorktrees` call return an on-disk list missing a worktree that `git worktree list --porcelain` (run directly via SSH, verified in this same investigation) correctly reported?** That points at `git-gateway-service`'s `DetectWorktrees` usecase or its relay/agent chain (`git.worktree.list`), not at the merge function itself. Not yet root-caused — needs live log correlation on the next occurrence (same methodology as BUG-009), not a code fix guessed from this session's evidence alone.

## Original diagnosis (superseded in part — root cause narrower than stated here, kept for the record)


## Root Cause — CONFIRMED

```go
// channels_worktree.go:759
func mergeDetectedWorktrees(repoID, projectID string, onDisk []*gitgatewayv1.DetectedWorktreeGitInfo, known []*projectv1.Worktree) []detectedWorktreeView {
	knownByPath := make(map[string]*projectv1.Worktree, len(known))
	for _, w := range known {
		knownByPath[w.GetPath()] = w
	}

	out := make([]detectedWorktreeView, 0, len(onDisk))
	for i, info := range onDisk {   // ← iterates onDisk, NOT known
		...
	}
	return out
}
```

The result list is built by iterating `onDisk` (git-gateway-service's live `DetectWorktrees` relay call — `git worktree list --porcelain` run on the dev server via the agent). `known` (the DB-tracked `project.worktrees` rows, fetched via `ListWorktrees`) is used **only to enrich** an entry that's already in `onDisk` (real `id`, `Ownership: "orca-managed"`, etc.) — it is **never used to add an entry that `onDisk` is missing**. If `onDisk` is empty or incomplete for any reason (a slow/flaky relay hop, the dev server agent momentarily unhealthy, a timing race right after `CreateWorktree` returns, or any other transient failure of the live scan), every DB-tracked worktree not present in that particular scan **silently vanishes** from the result — with zero error surfaced, since this isn't a failure path, it's the function's designed happy-path behavior.

## Confirmed NOT a filesystem/data problem

- `project.worktrees` row for the affected worktree (`01579d39-64c0-482f-a0e6-eeb577e404f7`, repo `aiops-v3`) is fully correct: `project_id='325d2acc-...'`, `repo_id='45f573e8-...'`, `path='/opt/repos/aiops-v3-golang-production-ready'`, `active=true`, `status='active'`.
- Ran `git worktree list --porcelain` directly on the dev server (`test-01`, from the repo root `/opt/repos/aiops-v3`) — **it correctly lists both worktrees**, including the affected one:
  ```
  worktree /opt/repos/aiops-v3
  HEAD 0a6216df214546e3de6d19d66c55a69395465dfa
  branch refs/heads/main

  worktree /opt/repos/aiops-v3-golang-production-ready
  HEAD 0a6216df214546e3de6d19d66c55a69395465dfa
  branch refs/heads/golang-production-ready
  ```
  So the on-disk data genuinely exists and is discoverable by the exact command `DetectWorktrees` should be running — confirming the gap is in the merge/relay path, not the filesystem.

## Why this matters beyond the one report

Even after the correction above, the underlying user-visible risk is real: "Project Workspace (Beta)"'s sidebar's list of worktrees is only ever as reliable as the LATEST successful `DetectWorktrees` scan. If that scan itself (not the merge afterward) ever returns an incomplete-but-technically-successful result — the still-open question this bug now tracks — a valid worktree disappears from the UI with no error, and (per the design comment) the frontend may actively purge it from local state too.

## Fix direction — REVISED after the correction above

**Do NOT implement a union/merge-logic fix in `mergeDetectedWorktrees`** — it would fight the documented, intentional "authoritative scan drives purge" contract. Instead:

1. **First, get live evidence.** Next occurrence: check `git-gateway-service`'s logs for the exact `DetectWorktrees` call in question — was its `git.worktree.list` relay to the agent itself incomplete, slow, or racing something? This needs the SAME log-cause methodology that worked for BUG-009/BUG-010, applied to `DetectWorktrees` specifically (it doesn't go through `apperrors` since it didn't error).
2. **Only after that evidence exists**, decide whether the fix belongs in `git-gateway-service` (e.g., a retry-once-before-trusting-empty policy inside `DetectWorktrees` itself, which does NOT conflict with the authoritative-scan contract since it still produces one single authoritative answer, just a more reliable one) or elsewhere.
3. If a UI-facing improvement is still wanted independent of root cause, consider: `worktree.detectedList`'s response could carry an explicit "this scan looked thin/suspicious" signal (e.g., on-disk count dropped from the previous known scan) so the frontend can show a "still verifying…" state instead of immediately treating it as authoritative — this preserves the purge contract's correctness while giving the user a chance to catch a bad scan before it purges anything, without redesigning the contract itself.

### Concrete diagnostic procedure (2026-09-15) — no code change needed to start diagnosing

Checked the agent-side handler (`agent/src/relay/agent-git-worktree-handler.ts`'s `handleGitWorktreeList`) — it **already logs** exactly what's needed for this:
```ts
log.info(`git.worktree.list: cwd=${cwd} count=${worktrees.length}`)
```
This line goes to the **dev server agent's own log** (systemd journal), not `docker logs` on any `orca-go-*` container — check it via:
```bash
ssh ubuntu@<dev-server-ip> "journalctl -u orca-agent-<host-name> --since '10 min ago' | grep 'git.worktree.list'"
```
Next occurrence: compare the logged `count` (and `cwd`) at the exact timestamp against how many worktrees the repo actually had — if `count` is already wrong at the agent, the bug is in `git worktree list --porcelain` itself or its `cwd` resolution (upstream of `parseWorktreePorcelain`, which was independently re-checked and correctly handles the exact porcelain format confirmed live on `test-01` in this same investigation). If `count` is correct at the agent but `git-gateway-service`/`api-gateway` end up with fewer, the bug is in the relay/response path instead. Either way, this ONE log line resolves which half of the chain is actually at fault — no new logging code needed before the next attempt.

## Related

- Discovered while diagnosing the same end-to-end task as [BUG-010](./BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md) (getting a worktree created and visible for the "aiops-v3" repo family) — that bug's fix (SOL-011) is what got worktree creation itself working; this bug is the next link in the same chain (creation succeeds, but the result isn't reliably visible).
- Same "shared boundary function silently drops data instead of erroring" shape as [BUG-005](./BUG-005-wscompat-empty-lists-serialize-as-null.md) (empty lists) and the `solutions/README.md`'s "Cross-cutting design theme" — worth keeping in mind if a `SOL-011-b`-style fix is designed later: prefer failing loud (or falling back safely) over silently returning less data than the DB actually has.
