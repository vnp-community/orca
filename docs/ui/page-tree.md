# Page Tree

Orca's renderer has no conventional router. Three separate top-level apps share
one component tree; inside the main app, exactly one `activeView` is visible at
a time, switched from the store, not the URL.

```
frontend/ (renderer)
│
├── Desktop entry (main.tsx) ─────────────────────────────┐
├── Web entry (web/main.tsx)                               │
│     ├── /auth/config reachable → multi-user web app       │
│     │     └── not authenticated → Login ──────────────────┼──(after login)──┐
│     └── /auth/config 404 → legacy pair-code web app        │                 │
│                                                             ▼                 ▼
└── Admin console entry (components/admin/AdminApp.tsx,      Main App (App.tsx)
    separate bundle — server operators only)                 │
      │                                                       │  activeView (Zustand `ui` slice) —
      ├── Admin ▸ Users        (UsersPage)                    │  exactly one visible, switched via
      ├── Admin ▸ Sessions     (SessionsPage)                 │  setActiveView() from SidebarNav /
      ├── Admin ▸ Policies     (PoliciesPage)                 │  status bar / keyboard shortcuts
      └── Admin ▸ Audit Log    (AuditPage)                    │
                                                                ├── terminal (default) ── Workspace / Terminal
                                                                │     ├── Landing              (no worktree selected)
                                                                │     ├── WorktreeCreationPanel (mid-creation)
                                                                │     └── main IDE layout:
                                                                │           ├── Left sidebar — repos, worktrees, worktree palette
                                                                │           ├── Center — terminal panes / editor tabs
                                                                │           ├── Right sidebar — file explorer, git, diff comments,
                                                                │           │     browser pane, AI Vault, checks panel
                                                                │           └── Status bar
                                                                │
                                                                ├── workspace ── Project Workspace (Beta)  [F38, multi-user]
                                                                │     ├── ProjectSwitcher (header)
                                                                │     └── WorkspaceLayout
                                                                │           ├── ExplorerPanel (file tree)
                                                                │           ├── GitPanel
                                                                │           │     ├── Changes tab  (StagingArea + CommitForm + DiffViewer)
                                                                │           │     ├── History tab  (GitHistory)
                                                                │           │     ├── Branches tab (BranchManager)
                                                                │           │     └── Pull Requests tab (PullRequestList — not wired to a
                                                                │           │           backend RPC yet, shows "not available")
                                                                │           ├── TaskGraphPanel
                                                                │           ├── AgentPanel
                                                                │           └── WorkflowMonitor / SshStatusSegment
                                                                │
                                                                ├── settings ── Settings (single page, many in-page sections)
                                                                │     └── sections (left nav inside the page, not separate activeViews):
                                                                │           General, Appearance, Terminal, Git (Source Control AI),
                                                                │           Shortcuts, Sessions, MCP, Voice, Orchestration,
                                                                │           Computer Use, Notifications, Experimental, …
                                                                │           (full list generated at runtime by useSettingsNavigationMetadata())
                                                                │
                                                                ├── tasks ── Tasks (TaskPage)
                                                                │     ├── Task board / list (GitHub / GitLab / Linear / Jira sources)
                                                                │     ├── Task detail drawer
                                                                │     └── PullRequestPage (PR detail panel, opened from a task)
                                                                │
                                                                ├── activity ── Activity (ActivityPrototypePage)
                                                                │     └── Agent run feed across worktrees ("Agents" in the sidebar)
                                                                │
                                                                ├── automations ── Automations (AutomationsPage)
                                                                │     └── Scheduled/triggered automation definitions + run history
                                                                │
                                                                ├── space ── Space (WorkspaceSpacePage)
                                                                │     └── Disk-usage analyzer / worktree cleanup tool (treemap +
                                                                │           table); NOT a kanban/task board despite the name.
                                                                │           Reached from the status bar's Resource Usage popover
                                                                │           or a disk-full recovery banner — not the sidebar.
                                                                │
                                                                ├── skills ── Skills (SkillsPage)
                                                                │     └── Installed/available agent skills catalog
                                                                │
                                                                └── mobile ── Mobile (MobilePage)
                                                                      └── QR pairing + Orca Mobile companion app status
```

See [`pages/`](./pages/) for a per-page description of data and layout.
