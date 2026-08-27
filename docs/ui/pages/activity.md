# Activity ("Agents")

**Route / trigger:** `activeView === 'activity'`. Opened via the sidebar "Agents" button (`frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`, `Bell` icon) which calls `openActivityPage()`. The button is feature-flagged: it only renders when `settings.experimentalActivity === true` (`shouldShowAgentsButton`). While active, the titlebar swaps in `ActivityTitlebarControls` (`frontend/src/renderer/src/components/activity/ActivityTitlebarControls.tsx`) showing a Back button, a "agents" label, and an unread-count badge.
**Top-level component:** `ActivityPrototypePage` (`frontend/src/renderer/src/components/activity/ActivityPrototypePage.tsx`, ~2100 lines)

## Purpose
A cross-workspace inbox of AI agent activity: one row per running/finished agent pane ("thread") across every repo/worktree, so a user with many agents running in parallel can see which ones are waiting on them (blocked/waiting/done) without hunting through the worktree sidebar tab by tab.

## Layout
Two-pane layout under a shared `<main>`; no page-level header/toolbar (that lives in the titlebar via `ActivityTitlebarControls`).

```
┌────────────────────────────┬──────────────────────────────────────┐
│ aside (resizable, 320-720, │ section (flex-1)                      │
│ default 480px)             │  - selected thread header (agent icon,│
│  - Filter input + Group-by │    state dot, title, repo badge)      │
│    select + unread toggle  │  - terminal portal (mirrors the real  │
│    + "..." options menu    │    TerminalPane content, or an empty  │
│  - grouped, scrollable     │    state if no live tab / nothing     │
│    thread list             │    selected)                          │
└────────────────────────────┴──────────────────────────────────────┘
```

- Left `<aside>`: search `Input`, a `Select` for `groupBy` (`status | project | worktree | agent`), a `Toggle` (Bell icon) to filter to unread only, and `ActivityThreadOptionsMenu` (`ActivityThreadOptionsMenu.tsx`) for compact-mode + "mark all read". Below that, threads are grouped into `ActivityThreadGroup`s (rendered via `ActivityStatusGroupHeader` + `ThreadRow` per thread). Resizing is handled by `useSidebarResize`.
- Right `<section>`: when a thread is selected, shows its header (agent icon via `AgentIcon` from `@/lib/agent-catalog`, `ThreadAgentStateIndicator`, `EventRepoBadge`, worktree name) and two stacked absolutely-positioned portal-target `<div>`s (`primary`/`secondary` slots) that `TerminalPane` renders its real xterm instance into (see Notable details). If no thread is selected, shows an empty state (`MessageSquareText`/`TerminalSquare` icon + copy).

## Data shown
- **Agent threads**: built client-side from store state via `buildActivityEvents` + `buildAgentPaneThreads` (local functions in this file), fed by a `useShallow` selection of:
  - `agentStatusByPaneKey`, `migrationUnsupportedByPtyId`, `retainedAgentsByPaneKey` — per-pane agent hook status (state: `working | blocked | waiting | done`, `stateHistory`, `stateStartedAt`)
  - `tabsByWorktree`, `worktreeMap` (via `getWorktreeMapFromState`), `repoMap` (via `getRepoMapFromState`)
  - `acknowledgedAgentsByPaneKey` / `acknowledgeAgents` / `unacknowledgeAgents` — read/unread tracking per `paneKey`
  - `agentStatusEpoch` — bump signal that forces stale-status recompute without a wall-clock dependency
  - A thread (`AgentPaneThread`) is keyed by `paneKey` (`${tabId}:${leafId}`, see `parsePaneKey`/`shared/stable-pane-id`), not by workspace — so one entry exists per agent pane even if a worktree has several.
- **Unread badge**: `useActivityUnreadCount` (`frontend/src/renderer/src/components/activity/useActivityUnreadCount.ts`) counts `done|blocked|waiting` state entries (including `stateHistory`) not yet acknowledged; used both by the sidebar badge (`'sidebar-badge'` mode, counts unread worktrees) and the titlebar badge (`'agent-events'` mode, counts unread state-history events).
- **Selected thread's terminal**: not fetched by this page — it's the *same* `TerminalPane` instance that lives in the (hidden) workspace tree, portaled into this page's `primary`/`secondary` target divs via `setActivityTerminalPortals` / `useActivityTerminalPortals` (`activity-terminal-portal.ts`), keyed by `{worktreeId, tabId, paneKey}`.

## Key interactions
- Click a thread row to select it (`selectThread`) — this also calls `activateThreadTerminal`, which reorients the underlying workspace (`setActiveRepo`/`setActiveWorktree`/`setActiveTabType('terminal')`) and focuses the real pane so the portal has something live to attach to.
- "Jump" icon on a row (`jumpToWorkspace`) marks it read and navigates to that worktree in the main workspace view (`activateAndRevealWorktree`), leaving Activity.
- Mark a single thread unread (`markThreadUnread` → `unacknowledgeAgents`) or "mark all read" from the options menu.
- Filter by free-text query (`Filter...` input, matched via `activityThreadMatchesSearchQuery`), toggle unread-only, group by status/project/worktree/agent, toggle compact row mode.
- Resize the thread list column by dragging the right edge (`useSidebarResize`).
- Global shortcut focuses the filter input (`handleActivityFilterFocusShortcut`), even while a terminal has keyboard focus.

## Notable implementation details / known issues
- **Terminal portaling, not a second PTY**: the right pane never creates its own terminal/PTY. It publishes a portal target element; the real `TerminalPane` (mounted in the hidden workspace tree) detects a matching descriptor and physically moves its DOM node here via `findActivityTerminalPortal`. This avoids double-owning a PTY/xterm instance per agent.
- **Two portal slots (primary/secondary) exist to prevent a "wrong terminal" flash** when switching threads: the new terminal mounts into the *inactive* slot underneath the currently-visible one, and only swaps to front (`activePortalSlotId`) once its portal status reports `ready` (see `useActivityTerminalPortalStatus`, `stagedThread`/`visibleThread` logic). This is a fairly intricate piece of state machine — read the extensive "Why" comments around lines 1600-1700 before touching it.
- Portal descriptor publication happens in `useLayoutEffect` (not `useEffect`) specifically so Terminal's subscriber re-renders in the same commit — using `useEffect` was observed to flash the previous terminal for one frame.
- Retained/closed agents (`retainedAgentsByPaneKey`) can still appear as threads after their tab closes; in that case `selectedHasLiveTab` is false and the right pane shows a "Agent terminal closed" placeholder instead of a portal.
- This file is one of the larger single components in the app (~2100 lines) covering event modeling, thread grouping, portal orchestration, and row rendering in one file — worth splitting further if extended.
