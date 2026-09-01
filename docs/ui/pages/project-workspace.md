# Project Workspace (Beta)

**Route / trigger:** `activeView === 'workspace'` in the Zustand app store. Reached via the "Project Workspace (Beta)" button in the left sidebar's nav section (`components/sidebar/SidebarNav.tsx`, `onClick={() => setActiveView('workspace')}`, `FolderKanban` icon). Comment in code: "Giai đoạn 2c (F38) — additive-only 'Project Workspace (Beta)' entry point. Does not replace the Project/Repo sidebar flow" (`App.tsx` around line 312, `docs/guides/project-workspace/project-workspace-f38-doc-vs-code.md` §4 step 8).
**Top-level component:** `WorkspaceLayout` — `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx`, lazy-loaded from `App.tsx`. Rendered together with `ProjectSwitcher` (`components/project/ProjectSwitcher.tsx`) inside a small wrapper in `App.tsx` (`activeView === 'workspace'` branch, ~line 2460): a header bar holding `ProjectSwitcher`, then `WorkspaceLayout` filling the rest.

## Purpose
An experimental, VS Code-style alternative workspace shell (file explorer + git + tasks + agent, all scoped to a single `OrcaProject`), built as a parallel/beta surface alongside the main terminal-centric workflow. It targets a different data model (`OrcaProject` / `project.*` RPCs) than the main app's Repo/Worktree sidebar.

## Layout
Top-to-bottom: tab bar, then a 3-panel resizable row, then a collapsible terminal strip, then a status bar.

```
┌───────────────────────────────────────────────────────────┐
│ ProjectSwitcher (combobox, in App.tsx wrapper above)         │
├───────────────────────────────────────────────────────────┤
│ WorkspaceTabBar: Git | Tasks | Workflows | Agent              │
├───────────┬───────────────────────────────┬─────────────────┤
│ Explorer  │  Center: active tab's panel     │ Right: "details" │
│ 20% (15-  │  50-80% (grows if right panel   │ 30% (20%+),      │
│ 35%)      │  hidden)                         │ collapsible       │
├───────────┴───────────────────────────────┴─────────────────┤
│ (collapsible) Terminal panel — h-48, WorkspaceTerminalPanel    │
├───────────────────────────────────────────────────────────┤
│ status bar: Show/Hide Terminal · SshStatusSegment · Show/Hide  │
│             Panel                                              │
└───────────────────────────────────────────────────────────┘
```

All panels use `ResizablePanelGroup`/`ResizablePanel`/`ResizableHandle` (`components/ui/resizable`). Explorer is always visible; the right "details" panel and the bottom terminal strip are locally-toggled booleans (`rightPanelVisible`, `terminalVisible` — component `useState`, not persisted to the store).

- **`NoProjectSelected`** renders if `project` is null; **`WorkspaceSkeletonLoader`** renders while `isInitializing`.
- **`OfflineBanner`** renders above the tab bar if `isOffline` (dev-server unreachable — read-only mode), with a Retry button that re-calls `switchProject(project.id)`.
- **Left — `ExplorerPanel`** (`components/workspace/ExplorerPanel.tsx`, lazy): file tree driven by `useFileExplorer()` (`hooks/useFileExplorer.ts`), which wraps `WorkspaceContext`'s `fileTree`/`refreshFileTree`. Renders `FileTreeNode` (`components/workspace/FileTreeNode.tsx`) recursively.
- **Center — one of four tabs** (`WorkspaceTabBar`, `components/workspace/WorkspaceTabBar.tsx`):
  - `git` → `GitPanel` (`components/workspace/git/GitPanel.tsx`, lazy): header with branch name + ahead/behind counts + Sync button; sub-tabs Changes/History/Branches/Pull Requests. Changes = `StagingArea` + `CommitForm` + optional `DiffViewer`. History = `GitHistory`. Branches = `BranchManager`. Pull Requests = `PullRequestList`.
  - `tasks` → `TaskGraphPanel` (`components/task/TaskGraphPanel.tsx`, lazy), a thin wrapper around the shared `TaskGraph` component (`task.list` RPC, keyed by `projectId`).
  - `workflows` → `WorkflowMonitor` (`components/workflow/WorkflowMonitor.tsx`, lazy) — not otherwise explored in this pass.
  - `agent` → `AgentPanel` (`components/workspace/AgentPanel.tsx`, lazy) if `currentWorktree` is set, else `NoWorktreeSelected` (a local empty state defined inside `WorkspaceLayout.tsx`).
- **Right — inline placeholder.** Not a real component: a `<div>` in `WorkspaceLayout.tsx` showing the literal text "Git details" or "Task detail" depending on the active tab. No dedicated component file.
- **Bottom — `WorkspaceTerminalPanel`** (`components/workspace/WorkspaceTerminalPanel.tsx`), shown only if `terminalVisible` and `currentWorktree` is set; reuses the main app's terminal-pane/PTY infrastructure rather than a separate PTY stack.
- **`FileViewer`** (`components/workspace/FileViewer.tsx`) and **`FileContextMenu`**/**`FileSearchPanel`** (`components/workspace/FileContextMenu.tsx`, `FileSearchPanel.tsx`) exist as standalone components but are not imported/rendered anywhere in this tree today — see Notable implementation details.

## Data shown
All state comes from `WorkspaceContext` (`frontend/src/renderer/src/context/WorkspaceContext.tsx`, `useWorkspace()` hook) plus a few local hooks:
- `project: OrcaProject | null` — fetched via `project.get` RPC on `switchProject(projectId)`; project list for `ProjectSwitcher` comes from `project.list` RPC (`{ id, name, devServerId }[]`).
- `gitStatus: GitStatus | null` — `git.status` RPC with `{ worktree: toRuntimeWorktreeSelector(currentWorktree.id) }`, re-fetched whenever `currentWorktree` changes (worktree selection is synced in from the main sidebar's `WorktreeList`, which calls `setCurrentWorktree` on `activeWorktreeId` change — this Beta page has no worktree picker of its own).
- `fileTree: FileNode | null` — `workspace.refreshFileTree` RPC with `{ projectId, path }`, adapted from a flat `{name, path, isDir, children}[]` response into a rooted `FileNode` tree (`toFileTree`/`mapBackendFileTreeNode`).
- `resolvedProfile: ResolvedProfile | null` — `profile.getResolved` RPC.
- `currentWorktree: Worktree | null` — set externally by `WorktreeList`, not by this page.
- Git sub-panels each own their own RPC calls via `callRuntimeRpc`/`useGit()` (`hooks/useGit.ts`), all scoped with `{ worktree: toRuntimeWorktreeSelector(currentWorktree.id) }`:
  - `StagingArea`/`CommitForm` → `git.status` (via `useGit().refreshFiles`), `git.stage`, `git.unstage`, `git.bulkStage`, `git.bulkUnstage`, `git.commit`, `git.generateCommitMessage` (AI commit message button).
  - `DiffViewer` → `git.diff` with `{ worktree, filePath, staged }`, rendered in a Monaco `DiffEditor`.
  - `GitHistory` → `git.history` with `{ worktree, limit: 50 }`.
  - `BranchManager` → `git.localBranches` (list) and `git.checkout` (switch); stored in `s.branches` (`store/slices/git-panel.ts`).
  - Push (`GitPanel`'s Sync button, and `CommitForm`'s "commit and push") → `useGit().push()`, which calls `pushRuntimeGit()` (not `callRuntimeRpc` directly) with `{ pushTarget: { remoteName: 'origin', branchName } }`.
  - `FileViewer` → `files.read` with `{ worktree, relativePath }` (unused/unmounted currently).
  - `FileSearchPanel` → `files.search` with `{ worktree, query, maxResults }` (unused/unmounted currently).
  - `FileContextMenu`'s delete action → `files.delete` with `{ worktree, relativePath }` (unused/unmounted currently).
- `AgentPanel` does **not** go through `WorkspaceContext`/`callRuntimeRpc` at all — it uses a separate `window.api.agentOrchestration.{start,stop,resume}` IPC surface and `s.remoteAgentSessions[worktreeId]` (a `RemoteAgentSession` store slice), distinct from the main app's terminal-based agent panes.

## Key interactions
- Switch project → `ProjectSwitcher` combobox → `switchProject(projectId)` (`WorkspaceContext`) → fetches `project.get`, `workspace.refreshFileTree`, `profile.getResolved` in parallel; resets `gitStatus`.
- Create a new project → `ProjectSwitcher`'s "Create New Project" item → `CreateProjectDialog`.
- Switch tab (Git/Tasks/Workflows/Agent) → local `activeTab` state in `WorkspaceLayout`, no store/RPC involved.
- Stage/unstage a file, view its diff, write/generate a commit message, commit, commit-and-push → `StagingArea`/`CommitForm` via `useGit()`.
- Checkout an existing branch → `BranchManager`'s per-row "Checkout" button → `git.checkout`.
- View commit history → `GitHistory` tab, read-only list.
- Start/stop/resume a remote agent session for the current worktree → `AgentPanel`'s Start/Stop/Resume/New Session buttons → `window.api.agentOrchestration.*`.
- Toggle the bottom terminal strip or the right "details" panel → status bar's "Show/Hide Terminal" and "Show/Hide Panel" buttons (local component state only).
- Retry after going offline → `OfflineBanner`'s Retry button → re-runs `switchProject`.

## Notable implementation details / known issues
- **Pull Requests tab shows a permanent "not available yet" state.** `PullRequestList.tsx` never calls an RPC — there is no `git.pr.list`/`git.pr.create` method on the backend. The real hosted-PR equivalent is `hostedReview.forBranch`/`hostedReview.create`, which need a `repo` selector; the `OrcaProject` model this Beta page is built on has no such selector yet, so this is flagged in-code as "known gap, not a bug," pending separate backend/design work. `PullRequestForm.tsx` exists and is used elsewhere (`code-review/pr-create-dialog.tsx`) but is not wired into this panel.
- **`BranchManager` has no "create branch" UI by design.** Orca's git model is worktree-per-branch — a new branch is created by creating a new *worktree*, not by branching in place — and the backend only exposes `git.checkout` (switch to an existing branch), not an in-worktree branch-creation RPC. A "Create" button was deliberately left out rather than wired to a nonexistent method.
- **Several RPC method/param-shape bugs were recently fixed here** (comments throughout `GitPanel.tsx`, `GitHistory.tsx`, `DiffViewer.tsx`, `FileViewer.tsx`, `FileSearchPanel.tsx`, `FileContextMenu.tsx` reference "BUG-FE-HLD-002" and "same crash class as GitPanel.tsx's push"): calls previously used nonexistent methods (`git.getLog`, `git.getDiff`, `fs.readFile`, `fs.grep`, `workspace.deleteFile`, `workspace.readFile`) and/or a `{projectId}`-shaped payload instead of the real `{worktree: <selector>}` contract, and `push` used to stream through a nonexistent `/api/rpc/stream` endpoint with an unset bearer token. All now route through `callRuntimeRpc`/`useGit()` with `toRuntimeWorktreeSelector(currentWorktree.id)`.
- **Dead/unwired components:** `FileViewer.tsx`, `FileSearchPanel.tsx`, and `FileContextMenu.tsx` are fully implemented but not imported anywhere in the render tree — `ExplorerPanel`'s Search button has no `onClick`, and `useFileExplorer`'s `viewingFile` state is tracked but never consumed to render `FileViewer`. Clicking a file in the Explorer currently has no visible effect.
- **The right-hand "details" panel is a placeholder** — literal text ("Git details" / "Task detail") in a `<div>` inside `WorkspaceLayout.tsx`, not a real component.
- **Worktree selection is entirely borrowed from the main sidebar.** This page has no worktree picker; `WorktreeList.tsx` (main terminal view) explicitly calls `setCurrentWorktree` on `WorkspaceContext` whenever `activeWorktreeId` changes, so the user must select a worktree from the main sidebar before Git/Agent tabs here become usable (`NoWorktreeSelected` empty state otherwise).
- **`AgentPanel` is a separate agent-orchestration stack** (`window.api.agentOrchestration`, `RemoteAgentSession`) from the terminal view's PTY-based agent panes — two independent implementations of "run an AI agent," not a shared code path.
- There is also a second, apparently-superseded `WorkspaceContextV6.tsx` (`workspace.init`/`workspace.teardown`/`workspace.refreshGitStatus`/`workspace.refreshFileTree` RPCs) that this page does **not** use — the live provider is `WorkspaceContext.tsx`, not `WorkspaceContextV6`.
