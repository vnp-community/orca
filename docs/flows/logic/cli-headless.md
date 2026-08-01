# Luồng Dữ liệu — CLI & Headless

**Domain:** CLI & Headless Mode  
**Nghiệp vụ:** BL-CLI-01 → BL-CLI-03  
**Kiến trúc thực tế:** Orca là Electron Desktop App — CLI giao tiếp với Electron Main Process qua IPC (không phải Daemon Unix Socket độc lập)  
**Cập nhật:** 2026-08-01

> **⚠️ Chú ý quan trọng về kiến trúc thực tế:**  
> HLD mô tả CLI giao tiếp với "Daemon Process" qua Unix Socket / HTTP.  
> Thực tế code (`src/main/automations/service.ts`): `import type { WebContents } from 'electron'` →  
> **AutomationService là Electron module, không phải headless daemon độc lập.**  
> BL-CLI-03 "headless daemon" trong codebase hiện tại = Orca chạy không có GUI window, nhưng vẫn là Electron process.

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Orca CLI (`orca` binary) | Client | Command-line interface |
| Electron Main Process | Backend | Headless orchestration server |
| IPC / WebSocket | Transport | CLI ↔ Electron communication |
| AutomationService | Business Logic | AutomationEngine, Scheduler |
| Store (SQLite via electron-store) | Persistence | All state |
| Git Binary | External | Git operations |
| Dev Server (Relay) | Remote Execution | Agent spawn on remote |

---

## BL-CLI-01 — Tạo Worktree qua CLI

```
DevOps / Alex
    │
    ▼
$ orca worktree create --branch feature/auth --agent claude
    │
    ▼
[CLI Tool — Orca CLI binary]
    ├─ Parse args: { baseRef: 'feature/auth', agentType: 'claude' }
    ├─ Connect to Orca: WebSocket hoặc HTTP :6768 (xem BL-CLI-03)
    └─ Send RPC: { type: 'worktree.create', payload: { baseRef, agentType } }
    │
    ▼
[Electron Main Process — OrcaRuntime RPC handler]
    ├─ WorktreeManager.create() → Git CLI → SQLite
    │   (nếu remote: relay.call('git.worktree.add', ...) → Dev Server)
    └─ Response: { type: 'worktree.created', result: { id, path } }
    │
    ▼
[CLI] stdout:
    Worktree created: /path/to/repo-feature-auth
    Branch: feature/auth
    Agent: starting...

CI/CD variant:
    $ orca worktree create --branch $BRANCH --agent claude --json
    → stdout: {"id":"uuid","path":"/path","status":"ready"}

Luồng:
CLI → WebSocket/HTTP → Electron Main → WorktreeManager → Git CLI + SQLite
                                      (nếu remote) → relay.call → Dev Server
    ← Response → stdout (human or JSON)
```

---

## BL-CLI-02 — Quản lý Agent qua CLI

```
DevOps/Admin
    │
    ▼
$ orca agent start --worktree <id> --prompt "Fix all TypeScript errors"
    │
    ▼
[CLI] WebSocket/IPC: { type: 'agent.start', payload: { worktreeId, initialPrompt } }
    │
    ▼
[Electron Main Process] OrcaRuntime RPC handler
    ├─ Nếu LOCAL worktree: spawn agent PTY locally (node-pty)
    ├─ Nếu REMOTE worktree (dev server):
    │   relay = RelayConnectionPool.getOrConnect(devServerId)
    │   relay.call('agent.spawn', { cmd, env, cwd })    ← Dev Server
    │   [Dev Server: node-pty.spawn(agentBinary)]
    └─ Stream events về CLI:
        { type: 'agent.status', status: 'starting' }
        { type: 'agent.status', status: 'idle' }
    │
    ▼
[CLI] stdout real-time:
    [10:01:23] Agent starting...
    [10:01:25] Agent ready (idle)

$ orca agent stop --worktree <id>
    → IPC: { type: 'agent.stop', payload: { worktreeId } }
    → [Main] SIGINT → PTY  (local) | relay.call('agent.kill') (remote)

Luồng:
CLI → WebSocket/HTTP → Electron Main → OrcaRuntime
    (local)  → node-pty → Agent process
    (remote) → relay → Dev Server → node-pty → Agent process
    ← streaming events ← agent status → stdout real-time
```

---

## BL-CLI-03 — Chạy Orca Headless Mode

```
DevOps
    │
    ▼
$ orca daemon start   (hoặc: orca --headless)
    │
    ▼
[Electron Main Process — headless mode]
    ├─ Không mở BrowserWindow (headless flag)
    ├─ AutomationService.start() — khởi tạo Scheduler
    ├─ HTTP server :6768 — REST API
    ├─ WebSocket server :6768/ws — cho CLI clients
    └─ Listen for connections...
    │
    ▼
$ orca status
    → HTTP GET http://localhost:6768/api/status
    ← { uptime, worktreeCount, activeAgents, automations }
    → stdout

$ orca daemon stop
    → HTTP POST http://localhost:6768/api/shutdown
    → graceful shutdown: stop agents → close DB → exit

HEADLESS AUTOMATION (CI/CD):
$ orca run --automation "nightly-review" --wait
    → HTTP POST http://localhost:6768/api/automations/:id/run
    → [Main] AutomationService.runNow(automationId)
        (nếu remote target) → relay.call('agent.spawn') → Dev Server
    → stdout: result summary
    → exit code: 0 (success) | 1 (partial failure) | 2 (full failure)

Luồng:
$ orca daemon start → Electron Main (headless)
                    → HTTP/WS server :6768
$ orca <commands> → HTTP/WS → Main → execute
                 ← response → stdout
    (agent exec): Main → relay → Dev Server → node-pty → Agent

Dev Server connection:
    Dev Server ──WS connect──► Orca Main  (Dev Server = WS client)
    Orca Main ──JSON-RPC──► Dev Server    (dispatch commands)
    Dev Server ──stream──► Orca Main      (results, output)
```

---

## Sơ đồ tổng quan — CLI & Headless

```
┌────────────────────┐                    ┌─────────────────────────────┐
│  Orca CLI          │  HTTP / WebSocket   │  Orca Electron Main Process │
│  $ orca worktree   │◄───────────────►   │  OrcaRuntime                │
│  $ orca agent      │  :6768             │  AutomationService          │
│  $ orca daemon     │                    │  WorktreeManager            │
│  $ orca run        │                    │  SessionManager (multi-user)│
└────────────────────┘                    └──────────┬──────────────────┘
         │                                           │
         │ JSON output                    ┌──────────▼──────────────────┐
         ▼                                │  SQLite (electron-store /   │
    stdout / stderr                       │  better-sqlite3)            │
    exit code                            └──────────┬──────────────────┘
                                                    │
                                         ┌──────────▼──────────────────┐
                                         │  Dev Server (Remote)        │
                                         │  relay.call('agent.spawn')  │
                                         │  relay.call('git.*')        │
                                         │  relay.call('shell.eval')   │
                                         └─────────────────────────────┘

CI/CD Integration:
GitHub Actions → $ orca run --automation nightly → exit 0/1
                 ← stdout JSON results
```

---

## Khác biệt HLD vs Implementation

| HLD Mô tả | Thực tế |
|-----------|---------|
| `Daemon Process` độc lập | Electron Main Process (headless mode) |
| `Unix Socket: ~/.orca/orca.sock` | HTTP/WS :6768 |
| `CLI → Unix Socket → Daemon` | `CLI → HTTP/WS → Electron Main` |
| `AgentManager.start()` trong Daemon | Chưa implement (BUG-AG-ORCH-005) |
| Automation agent spawn trực tiếp | `webContents.send` → Renderer → back to Main |
