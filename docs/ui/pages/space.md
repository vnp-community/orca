# Space (Workspace Storage Manager)

**Route / trigger:** `activeView === 'space'`. Opened via `openSpacePage()` — reachable from the status bar's Resource Usage popover (`frontend/src/renderer/src/components/status-bar/ResourceUsageStatusSegment.tsx`, "Space" affordance) and from `TerminalPane.tsx`'s `openDiskSpaceAnalyzer`, which is offered as a recovery action on a terminal session-state-save failure banner (i.e. when the disk is likely full). Not reachable from the primary sidebar nav.
**Top-level component:** `WorkspaceSpacePage` (`frontend/src/renderer/src/components/workspace-space/WorkspaceSpacePage.tsx`) — a thin shell wrapping `WorkspaceSpaceManagerPanel` (`frontend/src/renderer/src/components/status-bar/WorkspaceSpaceManagerPanel.tsx`, ~2100 lines)

## Purpose
Disk-usage analyzer and cleanup tool for worktrees: scans every repo's worktrees, visualizes what's consuming space (treemap + per-file breakdown), and lets the user bulk-delete worktrees to reclaim space. Labeled "Space" with a "Beta" badge in the header.

Note: despite the name, this page is **not** a kanban/task board — that is a separate sidebar feature (`WorkspaceKanbanDrawer` and friends in `frontend/src/renderer/src/components/sidebar/`), unrelated to `activeView === 'space'`.

## Layout
`WorkspaceSpacePage` supplies just the page chrome (Back button, `HardDrive` icon, "Space"/"Beta" title, subtitle, Escape-to-close handling) and hosts `WorkspaceSpaceManagerPanel` in a scrollable `max-w-7xl` container.

```
┌ Back  [HardDrive] Space [Beta]  "Workspace disk usage and reclaimable worktree storage." ┐
├────────────────────────────────────────────────────────────────────────────────────────┤
│ Metrics strip: Scanned | Reclaimable | Workspaces (n/total) | Last updated                │
│ Scan/Cancel button + status line                                                          │
│ [WorkspaceTreemap  (1.4fr)]           [BreakdownList (0.6fr)]                             │
│ Selection bar: "n selected · x reclaimable"  [Clear] [Delete selected]                    │
│ Filter input | Sort select (size/name/repo/activity) | [Deletable/All] | [Select/Clear]    │
│ Table: one WorkspaceRow per worktree (checkbox, name, repo, branch, size, decision icons)  │
└────────────────────────────────────────────────────────────────────────────────────────┘
```
Sub-pieces inside `WorkspaceSpaceManagerPanel.tsx`: `Metric`/`UpdatedMetric` (top strip), `WorkspaceTreemap` (visual size map, click to inspect/zoom), `BreakdownList`/`BreakdownRow` (top-level files/dirs of the inspected worktree, via `SizeBar`), `WorkspaceRow` (table row with `WorkspaceDecisionHoverCard`, `StatusBadge`, `DecisionLine` explaining why a worktree can/can't be deleted), `CheckButton`/`SortIndicator`.

## Data shown
- **Scan results**: `state.workspaceSpaceAnalysis: WorkspaceSpaceAnalysis | null` (`frontend/src/shared/workspace-space-types.ts`) — `{ scannedAt, totalSizeBytes, reclaimableBytes, worktreeCount, scannedWorktreeCount, unavailableWorktreeCount, repos: WorkspaceSpaceRepoSummary[], worktrees: WorkspaceSpaceWorktree[] }`. Each `WorkspaceSpaceWorktree` carries `sizeBytes, reclaimableBytes, status ('ok'|'missing'|'permission-denied'|'unavailable'|'error'), topLevelItems: WorkspaceSpaceItem[] (name/path/kind/sizeBytes), isMainWorktree, isRemote, isSparse, canDelete, lastActivityAt`.
- **Scan lifecycle**: `state.workspaceSpaceScanning`, `state.workspaceSpaceScanProgress` (`WorkspaceSpaceScanProgress`: `scanId, state ('running'|'cancelling'), scannedRepoCount/totalRepoCount, scannedWorktreeCount/totalWorktreeCount, currentRepoDisplayName, currentWorktreeDisplayName`), `state.workspaceSpaceScanError` — all from `frontend/src/renderer/src/store/slices/workspace-space.ts`, driven by `refreshWorkspaceSpace()` / `cancelWorkspaceSpaceScan()`.
- **Decision context per row**: cross-referenced against live app state to explain deletability/warn before delete — `tabsByWorktree`, `ptyIdsByTabId`, `agentStatusByPaneKey`, `migrationUnsupportedByPtyId`, `retainedAgentsByPaneKey`, `openFiles`/`editorDrafts` (dirty buffers), `browserTabsByWorktree`, `gitStatusByWorktree`/`remoteStatusesByWorktree` (uncommitted/unpushed changes), `activeWorktreeId`.
- **Removal**: deletion goes through `runWorktreeBatchDelete` (`frontend/src/renderer/src/components/sidebar/delete-worktree-flow.ts`) and updates local analysis optimistically via `removeWorkspaceSpaceWorktrees`.

## Key interactions
- **Scan / Cancel**: button toggles between "Scan" (no analysis yet), "Refresh" (re-scan), and "Cancel" while `isScanning`; progress and errors surface inline. Scanning is resumable in the background — "You can leave this page; the last result stays visible."
- **Inspect a worktree**: click a treemap tile or table row to populate `BreakdownList` with its top-level files/dirs, sized with `SizeBar`.
- **Zoom** the treemap into one worktree (`ZoomIn`/`ZoomOut` controls, `zoomedWorktree` state).
- **Filter/sort** the table by name, size, repo, or last activity; toggle "Deletable only" vs "All".
- **Select** worktrees (checkbox per row, "Select visible deletable" bulk action) and **Delete selected** — shows a sticky selection bar with count + reclaimable bytes, routes through the same force-delete/uncommitted-changes safety checks as the sidebar delete flow (`WorktreeForceDeleteReason`).
- **Escape** or the Back button returns to the previous view (`closeSpacePage`), deferring to any open dialog/menu first.

## Notable implementation details / known issues
- `WorkspaceSpaceManagerPanel.tsx` carries an explicit `/* eslint-disable max-lines */` with a "Why" comment: the treemap, selection, breakdown, and table pieces intentionally share one scan-state resource-manager surface rather than being split into separate files. Per `AGENTS.md`, any further growth here should go through the `max-lines-baseline.txt` exception process, not a new inline disable.
- Git-status refresh for visible rows is throttled to a concurrency of 6 (`GIT_STATUS_REFRESH_CONCURRENCY`) via `getWorkspaceSpaceGitStatusRefreshCandidates`/`refreshGitStatusForWorktree`, to avoid hammering git across many worktrees at once when the table becomes visible.
- Despite the page-level "Beta" badge and disk-usage framing, the component/prop names ("Space Analyzer") differ from the file's own name (`WorkspaceSpaceManagerPanel`) — worth keeping in mind when searching for this feature by name.
