# F38 — Project Workspace (Unified Project IDE)

| Trường | Giá trị |
|--------|---------|
| **ID** | F38 |
| **Tên** | Project Workspace — Unified Project IDE |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-011 |
| **HLD References** | C3.12, C4.10 |

---

## Mô tả

Khi user chọn một **Project** từ danh sách, toàn bộ giao diện chuyển sang **Project Workspace** — một môi trường làm việc hợp nhất gồm: File Explorer (duyệt files trên dev server), Terminal, Agent Control, Workflow Panel, Task Graph, và Git UI. Mọi hoạt động đều được thực hiện **trực tiếp trên dev server** chứa code của project qua relay.

---

## Layout tổng thể

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🏠 Orca    [Projects ▼: vnp-blc-backend 🟢]      [@user ▼]  [Settings]    │
├──────────────────────────────────────────────────────────────────────────────┤
│ Sidebar (narrow)          │           Main Content Area                      │
│ ├── 📁 Explorer           │  ┌── Tabs: [Explorer] [Git] [Agent] [Tasks] ──┐  │
│ ├── 🔀 Git                │  │                                              │  │
│ ├── 🤖 Agent              │  │   [Active tab content — see F38/F39]        │  │
│ ├── ⚡ Workflows          │  │                                              │  │
│ ├── 📋 Tasks              │  │                                              │  │
│ └── 💻 Terminal           │  └──────────────────────────────────────────── ┘  │
│                           │                                                    │
│                           │  [Bottom Panel]                                    │
│                           │  Terminal 1 | Terminal 2 | + New                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Project Selection Flow

```
User → Sidebar → Projects → chọn "vnp-blc-backend"
    │
    ├── Load project info: { devServerId, repoPath, defaultBranch }
    ├── Check dev server online (FleetHealthMonitor)
    │   → offline: "Server unavailable" banner, allow offline browse (cached)
    │
    ├── Initialize workspace context:
    │   WorkspaceContext {
    │     project: OrcaProject
    │     devServer: SshHost
    │     connection: DevServerRelayBridge
    │     resolvedProfile: OrcaProfile   // for agent env
    │     currentWorktree: Worktree | null
    │   }
    │
    ├── Load initial data (parallel):
    │   ├── relay.call('fs.readDir', { path: repoPath, depth: 2 })
    │   ├── relay.call('git.status', { cwd: repoPath })
    │   ├── relay.call('git.branch', { cwd: repoPath })
    │   └── getActiveWorkflows(projectId)
    │
    └── Render Project Workspace UI
```

---

## Explorer Tab — Remote File System

```
Explorer
├── /srv/projects/vnp-blc  [📁 root]
│   ├── 📁 src/
│   │   ├── 📁 main/
│   │   │   ├── 📄 index.ts         [M] ← git modified
│   │   │   └── 📄 auth-manager.ts  [A] ← git added
│   │   └── 📁 renderer/
│   ├── 📁 tests/
│   ├── 📄 package.json
│   └── 📄 .gitignore
├── [🔄 Refresh]  [🔍 Search files]
└── [⚡ Open in Editor] → (future: Monaco integration)
```

**File operations via relay:**
- Click file → load content via `relay.call('fs.readFile')` → display in read-only viewer
- Right-click → Copy path, Open in Terminal (cd to dir)
- Search files: `relay.call('fs.search', { pattern, cwd })` → fuzzy match

---

## Agent Tab — Integrated Agent Control

```
Agent Control (for project: vnp-blc-backend)
┌──────────────────────────────────────────────────────────┐
│ AI Provider: Anthropic claude-opus-4-5 [from: Dept]      │
│ Dev Server: dev-alpha.internal [● healthy]               │
│ Worktree: feature/auth-bcrypt ← current                  │
│                                                          │
│ [Switch Worktree ▼]  [New Worktree]                      │
├──────────────────────────────────────────────────────────┤
│ Prompt (supports {{task.*}}, {{project.*}}):             │
│ ┌────────────────────────────────────────────────────┐   │
│ │ Refactor the auth module to use bcrypt 12 rounds.  │   │
│ │ Update tests accordingly.                          │   │
│ └────────────────────────────────────────────────────┘   │
│                                                          │
│ [▶ Run Agent]  [📎 Attach Task]  [📋 Use Template]       │
├──────────────────────────────────────────────────────────┤
│ Agent Output (live stream):                              │
│ 14:23:01  Starting claude --trust standard               │
│ 14:23:02  Reading src/auth/auth-manager.ts...            │
│ 14:23:08  Writing bcrypt implementation...               │
│ 14:23:15  ✅ Done — 3 files modified                      │
│ [⏹ Stop] [💾 Save session] [→ Go to Git]                │
└──────────────────────────────────────────────────────────┘
```

---

## Workflows Tab

```
Workflows (for project: vnp-blc-backend)
┌──────────────────────────────────────────────────────────┐
│ [▶ Quick Run] [+ New Run] [📚 Library]                   │
│                                                          │
│ Recent Executions:                                       │
│  ● Full-Stack Feature Dev   14:18   4/5 steps ● RUNNING │
│    [View Progress]                                       │
│  ✅ Code Review + PR        13:45   5/5 steps  DONE      │
│    [View] [Re-run]                                       │
│                                                          │
│ Quick Templates:                                         │
│  [Standard Feature Dev] [Hotfix] [Code Review]          │
└──────────────────────────────────────────────────────────┘
```

---

## Tiêu chí chấp nhận

- [ ] Project selector (dropdown/list) → switch workspace context
- [ ] Workspace context: load project + server + profile + worktrees
- [ ] Offline state: show cached file tree, disable write operations
- [ ] Explorer tab: remote file tree (lazy-load directories)
- [ ] File viewer: read file content via relay
- [ ] File search: fuzzy search via relay
- [ ] Agent tab: provider display (from resolved profile), worktree selector, prompt editor, live output
- [ ] "Attach Task" → link running agent to a task
- [ ] Workflows tab: quick run + recent executions + templates
- [ ] Tasks tab: project tasks (filtered by project, tree/board view)
- [ ] Bottom terminal: PTY sessions on project's dev server
- [ ] Server status indicator (online/degraded/offline)

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Workspace context | `src/renderer/src/context/WorkspaceContext.tsx` |
| Project selector | `src/renderer/src/components/workspace/ProjectSelector.tsx` |
| Workspace layout | `src/renderer/src/components/workspace/WorkspaceLayout.tsx` |
| Explorer panel | `src/renderer/src/components/workspace/ExplorerPanel.tsx` |
| Remote file viewer | `src/renderer/src/components/workspace/RemoteFileViewer.tsx` |
| Agent panel | `src/renderer/src/components/workspace/AgentPanel.tsx` |
| Workflows panel | `src/renderer/src/components/workspace/WorkflowsPanel.tsx` |
| Tasks panel | `src/renderer/src/components/workspace/TasksPanel.tsx` |
| Server status bar | `src/renderer/src/components/workspace/ServerStatusBar.tsx` |
