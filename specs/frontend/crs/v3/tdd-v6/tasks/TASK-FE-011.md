# TASK-FE-011: Upgrade GitPanel — Add Pull Requests Tab

**Task ID:** TASK-FE-011
**Phase:** 2 — New Components
**Priority:** P1
**Solution Ref:** SOL-FE-V6-006 (Section 4)
**Estimated effort:** 30 minutes
**Dependencies:** TASK-FE-010 (DiffViewer must be working)
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Upgrade `GitPanel.tsx` to:
1. Add a 4th tab "Pull Requests" (currently only has changes/history/branches)
2. Add streaming push UI with progress display
3. Add branch + sync header above tabs

---

## Execution Results

### 1. Placeholder Creation
- Created `PullRequestList.tsx` placeholder since it did not exist yet (as part of TASK-FE-012).

### 2. Upgrading GitPanel.tsx
- Overwrote `GitPanel.tsx` with the new design.
- The 4th tab `pullrequests` has been added successfully.
- Implemented `handleSync` calling `git.push` RPC to remote `origin`.
- Corrected the UI state to use `gitStatus.aheadBy` and `gitStatus.behindBy` instead of `ahead`/`behind` to match the `GitStatus` type safely.
- Loading/Progress indicator for Pushing is correctly rendering.

### 3. Verification
- `npx tsc` ran on `GitPanel` and yielded 0 errors.

---

## Acceptance Criteria

- [x] `GitPanel` has 4 tabs: `changes`, `history`, `branches`, `pullrequests`
- [x] Header shows branch name and up/down arrows with counts
- [x] "Sync" button calls `git.push` RPC
- [x] `isPushing` shows loading state on Sync button
- [x] `data-testid="git-panel"` on root
- [x] `data-testid="git-tab-pullrequests"` on the PR tab button
- [x] `data-testid="sync-button"` on sync button
- [x] Pull Requests tab renders `PullRequestList`
- [x] No TypeScript errors

---

## Output

Report:
```
GitPanel upgraded: YES
4th tab "pullrequests": ADDED
Branch header: ADDED
Sync button: ADDED
PullRequestList: PLACEHOLDER CREATED
TypeScript errors: 0
```
