# Workspace / Terminal

**Route / trigger:** Default view on launch. `activeView === 'terminal'` in the Zustand app store (`frontend/src/renderer/src/store/`), set via `setActiveView('terminal')`. Reached automatically on startup/hydration, by clicking a worktree in the left sidebar (`components/sidebar/WorktreeList.tsx`), or by navigating away from any other top-level view (Settings, Tasks, Automations, Project Workspace, etc.) back to the workspace list.
**Top-level component:** `App` — `frontend/src/renderer/src/App.tsx` (the whole app shell; the terminal view is one of several `activeView` branches rendered inside it, around line 2237 onward). The center-column workhorse is `Terminal` — `frontend/src/renderer/src/components/Terminal.tsx`.

## Purpose
This is Orca's main, default screen — the terminal-centric "workspace" experience where a developer works inside a git worktree: spawning/splitting terminal panes (shells, AI agents like Claude/Codex), browsing/editing files, viewing git status, and managing multiple worktrees/repos side by side. Every user lands here first; Settings, Tasks, Project Workspace (Beta), etc. are secondary views reached from the same shell.

## Layout
Three-column shell, similar to a code editor:

```
┌─────────────┬──────────────────────────────────────┬───────────────┐
│  Sidebar     │  Terminal / editor / browser panes    │  RightSidebar │
│ (worktrees)  │  (TabGroupSplitLayout, tab strip)      │ (explorer/git/│
│  ~220-500px  │  flexible, center column               │  checks/ports)│
│  resizable   │                                        │  resizable    │
└─────────────┴──────────────────────────────────────┴───────────────┘
                        StatusBar (bottom, ~24px, optional)
```

- **Left — `Sidebar`** (`components/sidebar/index.tsx`, default export composed of `SidebarHeader`, `SidebarNav`, `WorktreeList`, `SidebarToolbar`, plus lazy dialogs like `WorktreeMetaDialog`/`RemoveFolderDialog`). Resizable 220–500px (`sidebarWidth` in the store), collapsible via `toggleSidebar`. `SidebarNav` holds the top-level navigation buttons (Settings, Tasks, Automations, Orca Mobile, and the "Project Workspace (Beta)" entry that calls `setActiveView('workspace')`). `WorktreeList` (`components/sidebar/WorktreeList.tsx`, ~5000 lines) is a virtualized, sortable/groupable list of every worktree/folder-workspace across repos, with drag-and-drop reordering, per-card status/PR/agent badges, and inline rename.
- **Center — terminal workbench.** Mounted whenever a worktree is active (`shouldMountTerminalWorkbench`) and kept alive across worktree switches (hidden via CSS `hidden` class rather than unmounted, to preserve PTY/xterm state). Structure: `Terminal` (`components/Terminal.tsx`) → per-worktree `WorktreeSplitSurface` → `TabGroupSplitLayout` (`components/tab-group/TabGroupSplitLayout.tsx`), which recursively renders `SplitNode`s (horizontal/vertical resizable splits) down to leaf `TabGroupPanel`s. Each leaf panel owns a tab strip (terminal, editor, browser, agent tabs) and a `TerminalPane`/editor/browser view for the active tab. When no worktree is active, `Landing` renders instead; during worktree creation, `WorktreeCreationPanel` renders a faux tab strip.
- **Right — `RightSidebar`** (`components/right-sidebar/index.tsx`, default export `RightSidebarInner` wrapped in `React.memo`). A vertical activity bar (`explorer`, `vault` (agent session history), `workspaces` (folder-only), `pr-checks` (folder-only), `source-control` (git-only), `checks` (git-only), `ports` (SSH-only)) plus the active panel's content, routed through `RightSidebarPanelContent` (`components/right-sidebar/right-sidebar-panel-content.tsx`): `FileExplorer`, `SourceControl`, `ChecksPanel`, `PortsPanel`, `AiVaultPanel`, `FolderWorkspaceWorktreesPanel`. Resizable width, kept mounted (not unmounted) while closed for layout stability; unmounted only on the Tasks view.
- **Bottom — `StatusBar`** (`components/status-bar/StatusBar.tsx`), shown when `statusBarVisible`; hosts SSH status, agent status-bar items (claude/codex/gemini/…), resource usage, ports, and the floating-terminal toggle.
- **Titlebar** sits above the sidebar/center columns (or as a floating header over the collapsed sidebar) and hosts window traffic-light space, back/forward worktree history, sidebar toggles, and (in workspace/terminal view) the tab strip's drag region.
- A `FloatingTerminalToggleButton` / `FloatingTerminalPanel` overlay can float above everything if `floatingTerminalEnabled` (a separate always-on-top mini terminal, independent of the main worktree's panes).

## Data shown
- **Worktree list (Sidebar):** `useAllWorktrees()` / `s.worktreesByRepo`, `s.detectedWorktreesByRepo`, `s.worktreeLineageById`, `s.workspaceLineageByChildKey`, `s.activeWorktreeId`, `s.activeWorkspaceKey` (Zustand, `store/slices/worktrees.ts` — `WorktreeSlice`). Each `Worktree` (`frontend/src/shared/types.ts:460`) carries `id` (`${repoId}::${path}`), `repoId`, `projectId`, `hostId`, `displayName`, `comment`, `linkedIssue`/`linkedPR`/`linkedLinearIssue`/`linkedGitLabMR`/etc., `isArchived`, `isUnread`, `isPinned`, `sortOrder`, `lastActivityAt`. Repos come from `s.repos` (`store/slices/repos.ts` — `RepoSlice`). Sort order (`sortBy`: name/smart/recent/repo/manual) is computed client-side from `agentStatusByPaneKey`, `tabsByWorktree`, `ptyIdsByTabId`, `runtimePaneTitlesByTabId`.
- **Terminal panes:** `s.tabsByWorktree`, `s.activeTabId`/`activeTabIdByWorktree`, `s.terminalLayoutsByTabId` (split-tree layout per tab), `s.ptyIdsByTabId` (live PTY handles), `s.pendingStartupByTabId`. PTY I/O flows over IPC/RPC to the backend daemon, not `callRuntimeRpc` REST-style calls used elsewhere.
- **Git status badges on worktree cards:** background polling (`useGitStatusPolling`, gated on `workspaceSessionReady`) feeds per-worktree ahead/behind/dirty state shown as card decorations, independent of the Project Workspace's `git.status` calls.
- **Right sidebar Explorer:** file tree fetched per active worktree (separate from `WorkspaceContext`'s file tree — this uses its own file-explorer hook wired to the main worktree/RPC model, not `workspace.refreshFileTree`).
- **Status bar:** `s.settings.statusBarItems` (which provider chips to show: claude/codex/gemini/antigravity/opencode-go/kimi/minimax/grok/ssh/resource-usage/ports), SSH connection state (`s.sshConnectionStates`), agent status maps.

## Key interactions
- Click a worktree card in `WorktreeList` → `setActiveWorktree` / navigates via worktree history → swaps the visible terminal workbench (previous worktree's panes stay mounted-but-hidden).
- Drag-and-drop worktree cards → reorder (`sortBy: 'manual'`) or drop onto a workspace-status column (`workspaceStatuses`) to relabel status.
- Split/close/detach terminal or editor panes inside a tab group → `TabGroupSplitLayout`'s `useTabDragSplit` + `SplitNode` resize handles (`setTabGroupSplitRatio`).
- Toggle left/right sidebars → `actions.toggleSidebar` / `actions.toggleRightSidebar` (also keyboard shortcuts, `sidebar.left.toggle` / `sidebar.right.toggle`).
- Switch right-sidebar tab (Explorer/Source Control/Checks/Ports/Agents/Attached worktrees/PR Checks) → `setRightSidebarTab` / `showRightSidebarFiles`.
- Open the floating terminal (button or hotkey) → `setFloatingTerminalOpenWithFocus`, independent PTY panel that stays mounted while it owns tabs.
- Navigate to "Project Workspace (Beta)" → `SidebarNav`'s dedicated nav button calls `setActiveView('workspace')` (see `docs/ui/pages/project-workspace.md`).

## Notable implementation details / known issues
- The terminal workbench is deliberately kept mounted across worktree switches and even across `activeWorktreeId` briefly going `null` (sleep/shutdown transitions) via `hasMountedTerminalWorkbenchRef` — unmounting would kill PTYs and xterm buffers. Hidden worktrees use a CSS `hidden` class, not React unmount.
- The sidebar's "smart" sort order intentionally debounces re-sorts (`SORT_SETTLE_MS`) so ambient PTY/agent activity doesn't reshuffle cards mid-interaction, except for structural changes (worktree add/remove) or manual drag, which apply immediately.
- Right-sidebar panels no longer remount on worktree switch (previously `key={activeWorktreeId}` caused an IPC storm re-issuing `watchWorktree`/`readDir`/`git:branchCompare` on every switch and froze the app on Windows); panels now react to `activeWorktreeId` changes via store subscriptions instead.
- This page and the separate "Project Workspace (Beta)" page (`activeView === 'workspace'`) are two independent, non-overlapping implementations of worktree/git/file UI — the Beta page is additive-only and does not replace this flow (see `docs/guides/project-workspace/project-workspace-f38-doc-vs-code.md`).
