# Luồng Dữ liệu — Project Workspace

**Domain:** Project Workspace  
**Nghiệp vụ:** BL-PW-01 → BL-PW-04  
**Kiến trúc tham chiếu:** HLD v1 — Profile/Project Service (C3.12), ADR-011/012, F38/F39

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Developer/Lead Browser | UI | Explorer panel, Git panel, Agent panel, Workflow panel |
| Orca Web Server | Backend | WorkspaceContext manager |
| ProjectService | Business Logic | Project + dev server lookup |
| RelayConnectionPool | Business Logic | Persistent SSH relay connections |
| FleetHealthMonitor | Business Logic | Server health cache |
| ProfileResolver | Business Logic | User profile for agent env |
| Dev Server (relay) | Remote | git, fs, agent ops |
| Server Database | Persistence | orca_projects, orca_worktrees, etc. |

---

## BL-PW-01 — Project Workspace Context

```
Developer/Lead
    │
    ▼
[Browser] Sidebar → chọn project "vnp-blc-backend"
    │ WebSocket RPC: workspace.switch({ projectId })
    ▼
[Orca Web Server — WorkspaceContextManager.switch(userId, projectId)]
    │
    ├─ Permission check:
    │   ProjectGrantService.hasAccess(userId, projectId, 'view')  ← DB
    │
    ├─ Teardown previous workspace (nếu có):
    │   - Close terminal sessions với warning (nếu có session đang chạy)
    │   - Stop git status poll
    │   - Keep relay nếu same devServer
    │
    ├─ Load project + server:
    │   project = ProjectService.get(projectId)     ← Server DB
    │   server  = SshHostService.get(project.devServerId)  ← Server DB
    │
    ├─ Establish/reuse relay:
    │   relay = RelayConnectionPool.get(project.devServerId)
    │   IF !relay OR status !== 'connected':
    │     relay = DevServerRelayBridge.connect(server)
    │     → SSH connect → Relay WS handshake
    │
    ├─ Check server health:
    │   healthStatus = FleetHealthMonitor.getCached(server.id)
    │   IF unreachable: offlineMode=true (read-only, cached data)
    │
    ├─ Load workspace data (parallel Promise.all):
    │   [gitStatus, worktrees, fileTreeRoot, activeWorkflows] = await Promise.all([
    │     relay.call('git.status', { cwd: project.repoPath }),
    │     relay.call('git.worktree.list', { repoPath: project.repoPath }),
    │     relay.call('fs.readDir', { path: project.repoPath, depth: 2 }),
    │     WorkflowService.getActiveExecutions(projectId)   ← DB
    │   ])
    │
    ├─ Resolve profile:
    │   resolvedProfile = ProfileResolver.resolve(userId)   ← Cache/DB
    │
    ├─ Build WorkspaceContext → push to Browser (SSE/WebSocket)
    │
    └─ Start background polls:
        git status: mỗi 5s (khi Git tab active hoặc agent running)
        server health: mỗi 30s

Luồng:
User → WebSocket RPC → WorkspaceContextManager
     → Server DB (permission + project + server)
     → RelayConnectionPool (get or create relay)
     → Promise.all:
         relay.call('git.status')       → Dev Server
         relay.call('git.worktree.list') → Dev Server
         relay.call('fs.readDir')        → Dev Server
         WorkflowService (DB)
     → ProfileResolver (cache/DB)
     → WorkspaceContext → Browser (render UI)
```

---

## BL-PW-02 — Remote File Explorer

```
Developer/Lead
    │
    ▼
[Browser] Explorer panel — mở project workspace
    └─ File tree đã có từ BL-PW-01 (depth 2)
    │
    ▼
EXPAND DIRECTORY:
[Browser] click folder icon
    │ WebSocket RPC: fs.readDir({ path: '/path/to/dir', depth: 1 })
    ▼
[Orca Web Server] relay.call('fs.readDir', { path, depth })
    │ SSH Relay → Dev Server
    ▼
[Dev Server — fs-handler.ts]
    ├─ fs.readdir(path, { withFileTypes: true })
    ├─ Apply .gitignore filter (optional)
    └─ Return: [{ name, type: 'file'|'dir', size, modifiedAt }]
    │
    ▼
[Browser] expand folder in tree

OPEN FILE:
[Browser] click file
    │ WebSocket RPC: fs.readFile({ path })
    ▼
[relay.call('fs.readFile', { path })]
    → Dev Server: fs.readFile(path) → base64 / text content
    │
    ▼
[Browser] open file in editor (Monaco / read-only view)

FILE SEARCH (ripgrep):
[Browser] search bar → Ctrl+Shift+F
    │ WebSocket RPC: fs.search({ pattern, path: repoPath, glob: '*.ts' })
    ▼
[relay.call('fs.search', { pattern, glob })]
    → Dev Server: ripgrep binary: rg --json <pattern> --glob <glob>
    → stream results
    │
    ▼
[Browser] search results panel with file/line/context

Luồng:
User → Explorer click → WebSocket RPC → relay.call → Dev Server (fs ops)
                                                     → Renderer (tree update)
```

---

## BL-PW-03 — Remote Git UI Operations

```
Developer/Lead trong Project Workspace
    │
    ▼
GIT STATUS (auto-poll 5s khi active):
    relay.call('git.status', { cwd: project.repoPath })
    → Dev Server: git status --porcelain -u
    → parse → GitStatus object
    → WebSocket push → Browser (Git panel badges + file decorations)

STAGE / UNSTAGE:
[Browser] Git panel → click file → "Stage" / "Unstage"
    │ WebSocket RPC: git.stage({ files: ['src/app.ts'] })
    ▼
[relay.call('git.add', { files })]
    → Dev Server: git add src/app.ts
    → refresh git.status → push update

COMMIT:
[Browser] commit message input → "Commit"
    │ WebSocket RPC: git.commit({ message })
    ▼
[relay.call('git.commit', { message })]
    → Dev Server: git commit -m "<message>"
    → refresh git.status

AI COMMIT MESSAGE:
[Browser] "AI: Generate" button
    │ WebSocket RPC: git.generateCommitMessage({ worktreeId })
    ▼
[relay.call('git.diff', { staged: true })]
    → Dev Server: git diff --staged
    → Orca Server: inject diff into agent PTY
    → parse agent response → message
    → WebSocket push message → Browser (pre-fill input)

PUSH:
[Browser] "Push" button
    │ WebSocket RPC: git.push({ branch })
    ▼
[relay.call('git.push', { branch })]
    → Dev Server: git push origin <branch>
    → stream push progress
    → WebSocket push events → Browser (progress bar)

CREATE PR:
    relay.call('github.pr.create', { title, body, head, base })
    → Dev Server: gh pr create ...
    → return PR URL

Luồng (typical: stage → commit → push → PR):
User → WebSocket RPC × 4 → relay.call × 4 → Dev Server (git + gh CLI)
     ← status updates → Git panel refresh
```

---

## BL-PW-04 — Workspace Integration (Agent+Git+Tasks+Workflows)

```
Developer/Lead — khi Task được gán cho mình
    │
    ▼
[Browser] Task panel → task card → "Open Workspace"
    │ WebSocket RPC: workspace.openTaskContext({ taskId })
    ▼
[Orca Server — TaskWorkspaceIntegrator.open()]
    ├─ Load task context: SELECT * FROM orca_tasks WHERE id=?  ← DB
    ├─ BL-PW-01: switch workspace to task.projectId
    ├─ IF task.worktreeId:
    │   relay.call('git.worktree.switch', { worktreeId: task.worktreeId })
    │   → Dev Server: cd worktreePath
    │   → Update WorkspaceContext.currentWorktree
    │
    ├─ Task detail panel: render { title, description, dependencies, comments }
    │
    ├─ "Run Agent" button → BL-TG-04 flow
    │
    ├─ Agent running → agent events → WorkspaceContext.activeAgentSessionId
    │       → Git panel: auto-refresh ON (poll 5s)
    │       → Explorer: show decorations (modified files)
    │
    └─ Agent completes → WorkspaceEvent: 'agent.complete'
        → Git panel: refresh (git status shows new changes)
        → Explorer: update file decorations
        → Task status: → 'review'
        → [Stage All] → AI commit message → Commit → Push → Create PR

END-TO-END FLOW (Task → Agent → Git → PR):
    Task selected → workspace switch → worktree switch
    → "Run Agent" → agent preamble build → agent spawn (relay)
    → agent works → agent complete
    → git auto-refresh → stage → AI commit msg → commit → push → PR create
    → PR URL → task.prUrl → task status → 'review'

Luồng:
User → workspace.openTaskContext → DB (load task) + BL-PW-01
     → relay (worktree switch) → agent spawn (BL-TG-04)
     → agent events → Git panel refresh
     → git operations (BL-PW-03) → PR creation
     → DB (UPDATE task prUrl + status)
```

---

## Sơ đồ tổng quan — Project Workspace

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Browser — Project Workspace UI                                          │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐           │
│  │  Explorer  │ │  Git Panel │ │  Agent     │ │ Tasks/Wflow│           │
│  │  file tree │ │  status    │ │  Panel     │ │  Panel     │           │
│  └──────┬─────┘ └─────┬──────┘ └─────┬──────┘ └─────┬──────┘           │
└─────────┼─────────────┼──────────────┼──────────────┼───────────────────┘
          │ WebSocket RPC / SSE (all panels share same WS connection)
          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Orca Web Server                                                         │
│  WorkspaceContextManager ─── ProfileResolver ─── ProjectService         │
│  RelayConnectionPool ─────── FleetHealthMonitor                         │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ Server DB + SSH Relay
             ┌─────────────────────┼──────────────────────────┐
             │                     │                           │
    ┌────────▼──────┐    ┌─────────▼───────┐     ┌────────────▼──────┐
    │ Server DB      │    │  Dev Server     │     │  AI Agent (PTY)   │
    │ orca_projects  │    │  git CLI        │     │  via relay spawn  │
    │ orca_tasks     │    │  fs ops         │     │  (BL-TG-04)       │
    │ orca_workflows │    │  ripgrep        │     │                   │
    └───────────────┘    │  gh CLI         │     └───────────────────┘
                         └─────────────────┘
```
