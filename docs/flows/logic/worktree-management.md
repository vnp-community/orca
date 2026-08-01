# Luồng Dữ liệu — Worktree Management

**Domain:** Worktree Management  
**Nghiệp vụ:** BL-WT-01 → BL-WT-05  
**Cập nhật:** 2026-08-01 — Phản ánh đúng kiến trúc 3-tier, runtime trên Dev Server

> **⚠️ Kiến trúc thực tế:**  
> HLD cũ mô tả `Main Process → Git CLI local → Daemon`. Thực tế trong **Web/Server mode**:  
> - Git operations chạy trên **Dev Server** qua `relay.call('git.exec')`, `relay.call('git.worktree.add')`, v.v.  
> - PTY sessions chạy trên Dev Server qua `relay.call('pty.create')` (xem terminal-management.md)  
> - Orca Server là **orchestrator**, không chạy git CLI local (trừ local dev scenario)

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Browser (React UI) | UI | Sidebar, dialogs, worktree cards |
| WebSocket RPC | Transport | Browser ↔ Orca Server |
| Orca Server | Business Logic | WorktreeManager, ProjectServerRouter |
| RelayConnectionPool | Infrastructure | Persistent relay connections per Dev Server |
| Dev Server (relay) | **Runtime** | **git CLI, node-pty, PTY lifecycle** |
| Server DB (SQLite) | Persistence | orca_worktrees, orca_terminal_sessions |

> **Git CLI và PTY đều chạy trên Dev Server. Orca Server chỉ điều phối qua relay.call().**

---

## BL-WT-01 — Tạo Worktree

```
Người dùng (Alex/Maya/Carlos)
    │
    ▼
[Browser] click "New Worktree"
    │ WebSocket RPC: worktree.create { projectId, baseRef, name, path?, agentType? }
    ▼
[Orca Server — WorktreeManager.create()]
    ├─ Auth + Permission: ProjectGrantService.hasAccess(userId, projectId, 'edit')
    ├─ Get relay: ProjectServerRouter.getRelayForProject(projectId, userId)
    │
    ├─ Check disk space (via relay):
    │   relay.call('fs.stat', { path: project.repoPath })        ← Dev Server
    │   IF available_bytes < 100MB: return error
    │
    ├─ Create worktree on Dev Server:
    │   relay.call('git.worktree.add', {                         ← Dev Server
    │     repoPath: project.repoPath,
    │     branch: baseRef,
    │     worktreePath: targetPath
    │   })
    │   [Dev Server: git worktree add <path> <baseRef>]
    │
    ├─ INSERT orca_worktrees {                                    ← Server DB
    │     id, projectId, devServerId,
    │     branch: baseRef, path: targetPath,
    │     status: 'ready', createdAt
    │   }
    │
    ├─ Create PTY session on Dev Server (BL-TM-01):
    │   relay.call('pty.create', { cwd: targetPath, ... })       ← Dev Server
    │   [Dev Server: node-pty.spawn(shell, { cwd: targetPath })]
    │
    └─ emit: worktree:created { worktreeId, path, branch }
    │
    ▼
[Browser] nhận event worktree:created
    └─ Thêm worktree card vào sidebar, terminal panel active

Luồng:
Browser → WS RPC → Orca Server
       → relay.call('fs.stat')         → Dev Server (disk check)
       → relay.call('git.worktree.add') → Dev Server (git)
       → Server DB INSERT orca_worktrees
       → relay.call('pty.create')       → Dev Server (PTY)
       → WS push event → Browser (UI update)

Input:  { projectId, baseRef, name?, path?, agentType? }
Output: { id: UUID, path, branch, status: 'ready' }
```

---

## BL-WT-02 — Fan-out Prompt tới Nhiều Worktree

```
Người dùng (Alex)
    │
    ▼
[Browser] Fan-out dialog: prompt, N (1–10), base branch, agent type
    │ WebSocket RPC: worktree.fanOut { projectId, prompt, n, baseRef, agentType }
    ▼
[Orca Server — WorktreeManager.fanOut()]
    │
    ├── FOR i = 1..N (song song — Promise.allSettled)
    │       │
    │       ├─ BL-WT-01: relay.call('git.worktree.add')  ← Dev Server
    │       │   INSERT orca_worktrees[i]                  ← Server DB
    │       │   relay.call('pty.create', { cwd: path_i }) ← Dev Server
    │       │
    │       └─ BL-AG-01: relay.call('agent.spawn', {     ← Dev Server
    │               ptyId, prompt,
    │               env: { ...resolvedProfile.envVars }
    │             })
    │
    ▼
[Browser] N events: worktree:created + agent:started
    └─ N worktree cards với real-time status

Luồng (parallel):
Browser → WS RPC → Orca Server → Promise.allSettled([
    relay.call('git.worktree.add') × N,  ← Dev Server
    relay.call('pty.create') × N,         ← Dev Server
    relay.call('agent.spawn') × N         ← Dev Server
])
→ Server DB INSERT × N
→ WS push × N events → Browser
```

---

## BL-WT-03 — Xóa Worktree An Toàn

```
Người dùng (Alex/Maya/Carlos)
    │
    ▼
[Browser] click "Delete"
    │ WebSocket RPC: worktree.checkSafety { projectId, worktreeId }
    ▼
[Orca Server — WorktreeManager.safetyCheck()]
    ├─ relay.call('git.exec', { args: ['status', '--porcelain'] })  ← Dev Server
    │   → check uncommitted changes
    ├─ SELECT status FROM orca_terminal_sessions WHERE worktreeId=?  ← Server DB
    │   → check running PTY sessions
    └─ relay.call('agent.status', { ptyId })                         ← Dev Server
        → check if agent running
    │
    ▼ safety result → Browser (confirmation dialog)
    │ WebSocket RPC: worktree.delete { worktreeId, force }
    ▼
[Orca Server — WorktreeManager.delete()]
    ├─ relay.call('agent.kill', { ptyId })            ← Dev Server (kill agent)
    ├─ relay.call('pty.destroy', { ptyId })           ← Dev Server (close terminal)
    ├─ relay.call('git.worktree.remove', {            ← Dev Server
    │     repoPath, worktreePath, force: true
    │   })
    │   [Dev Server: git worktree remove --force <path>]
    ├─ DELETE orca_worktrees WHERE id=?               ← Server DB
    ├─ DELETE orca_terminal_sessions WHERE worktreeId=? ← Server DB
    └─ emit: worktree:deleted

Luồng:
Browser → WS RPC → Orca Server:
    Safety: relay.call('git.exec' + 'agent.status') → Dev Server
    Delete: relay.call('agent.kill' + 'pty.destroy' + 'git.worktree.remove') → Dev Server
    → Server DB DELETE
    → WS push event → Browser
```

---

## BL-WT-04 — So sánh Kết quả Giữa Worktrees

```
Người dùng (Alex)
    │
    ▼
[Browser] mở "Compare" view
    │ WebSocket RPC: worktree.compare { projectId, worktreeIds[], baseRef }
    ▼
[Orca Server — WorktreeManager.compare()]
    ├── Promise.all([
    │   relay.call('git.exec', { args: ['diff', baseRef+'...'+branch, '--stat'] }) × N  ← Dev Server
    │   SELECT summary FROM orca_task_sessions × N  ← Server DB
    │   ])
    └─ Merge results → comparison data
    │
    ▼
[Browser] side-by-side comparison panel
    └─ User chọn winner → trigger BL-WT-05

Luồng:
Browser → WS RPC → Orca Server → relay.call('git.exec', [diff]) × N → Dev Server
                              → Server DB (SELECT session summaries)
                              → WS push comparison data → Browser
```

---

## BL-WT-05 — Merge Worktree Thắng

```
Người dùng (Alex/Maya)
    │
    ▼
[Browser] click "Merge", chọn strategy (merge | squash | rebase)
    │ WebSocket RPC: worktree.merge { projectId, worktreeId, strategy, cleanup }
    ▼
[Orca Server — WorktreeManager.merge()]
    ├─ relay.call('git.exec', { args: ['merge-base', '--is-ancestor', branch, 'main'] }) ← Dev Server
    │   → conflict check
    │
    ├─ Execute strategy on Dev Server:
    │   strategy='merge':
    │     relay.call('git.exec', { args: ['merge', branch] })    ← Dev Server
    │   strategy='squash':
    │     relay.call('git.exec', { args: ['merge', '--squash', branch] }) ← Dev Server
    │   strategy='rebase':
    │     relay.call('git.exec', { args: ['rebase', branch] })   ← Dev Server
    │
    ├─ IF cleanup: BL-WT-03 × (N-1) on Dev Server
    ├─ UPDATE orca_worktrees SET status='merged'   ← Server DB
    └─ emit: worktree:merged

Luồng:
Browser → WS RPC → Orca Server → relay.call('git.exec') × (conflict check + merge) → Dev Server
                              → BL-WT-03 × N-1 (cleanup)
                              → Server DB UPDATE/DELETE
                              → WS push event → Browser
```

---

## Sơ đồ tổng quan — Worktree Management (3-tier)

```
┌──────────────────────────────────┐
│  Browser — React UI              │
│  Sidebar worktree list           │
│  Dialogs (create/delete/compare) │
│  Worktree cards + status         │
└──────────┬───────────────────────┘
           │ WebSocket RPC
           ▼
┌──────────────────────────────────┐
│  Orca Server                     │
│  WorktreeManager                 │
│  ProjectServerRouter             │
│  RelayConnectionPool             │
└──────────┬───────────────────────┘
     ┌─────┼────────────────────────┐
     │     │                        │
     ▼     ▼                        ▼
┌────────┐ ┌────────────────────────────┐
│ Server │ │  Dev Server (Remote Host)  │
│  DB    │ │  git CLI (all git ops)     │
│worktree│ │  node-pty (PTY sessions)   │
│sessions│ │  Agent process (if any)    │
└────────┘ └────────────────────────────┘

relay.call methods used:
  git.worktree.add / git.worktree.remove / git.worktree.list
  git.exec (diff, merge, rebase, status, merge-base)
  pty.create / pty.destroy / pty.write
  agent.spawn / agent.kill / agent.status
  fs.stat (disk check)
```
