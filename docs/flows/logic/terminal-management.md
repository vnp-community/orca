# Luồng Dữ liệu — Terminal Management

**Domain:** Terminal Management  
**Nghiệp vụ:** BL-TM-01 → BL-TM-04  
**Cập nhật:** 2026-08-01 — Phản ánh đúng kiến trúc 3-tier (Web/Backend/Dev Server)

> **⚠️ Kiến trúc thực tế:**  
> HLD cũ mô tả "Daemon Process + Unix Socket". Thực tế code:  
> - **Desktop (Electron)**: Main Process → `orca-runtime.ts` → local node-pty (Electron IPC)  
> - **Web/Server mode**: Browser → WebSocket → Orca Server → relay.call('pty.*') → **Dev Server** → node-pty  
> Terminal trong **Web/Server mode phải chạy 100% trên Dev Server** thông qua relay.  
> File user đang mở: `remote-runtime-pty-transport.ts` — đây là **Browser side** kết nối qua WebSocket RPC đến Orca Server, sau đó đến Dev Server PTY.

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Browser (xterm.js) | UI | Terminal renderer, split pane layout |
| WebSocket / HTTP | Transport | Browser ↔ Orca Server RPC |
| Orca Server | Business Logic | WorkspaceContextManager, relay routing |
| Dev Server (relay) | **Runtime** | **node-pty, PTY lifecycle, shell process** |
| SQLite (Server DB) | Persistence | orca_terminal_sessions, scrollback_snapshots |
| Shell Process (on Dev Server) | External | $SHELL (bash/zsh/fish) on Dev Server |

> **Tất cả PTY/shell thực thi phải xảy ra trên Dev Server (remote host), không phải Orca Server.**

---

## BL-TM-01 — Tạo PTY Session (Remote)

```
Trigger: mở project workspace, mở terminal mới, khởi động agent
    │
    ▼
[Browser — xterm.js pane]
    │ WebSocket RPC: terminal.create { projectId, worktreeId, cols, rows, env? }
    ▼
[Orca Server — WorkspaceContextManager.createTerminal()]
    ├─ Auth: requireSession(userId)
    ├─ Permission: ProjectGrantService.hasAccess(userId, projectId, 'view')
    ├─ Get relay: ProjectServerRouter.getRelayForProject(projectId, userId)
    │   → RelayConnectionPool.getOrConnect(devServerId, server)
    │
    ├─ relay.call('pty.create', {               ← Dev Server
    │     cols, rows,
    │     cwd: project.repoPath,
    │     env: { ...resolvedProfile.envVars }
    │   })
    │
    ▼
[Dev Server — relay/pty-handler.ts]
    ├─ IF Dev Server chưa cài node-pty (agent cũ / chưa setup): trả lỗi
    │   → Browser hiển thị thông báo rõ ràng "máy này chưa hỗ trợ terminal"
    ├─ node-pty.spawn(shellPath, [], { cwd, cols, rows, env })
    ├─ PTY_REGISTRY.set(ptyId, { pty, cwd, connectedAt })
    └─ Return: { ptyId, sessionId }
    │
    ▼
[Orca Server] nhận ptyId
    ├─ INSERT orca_terminal_sessions { ptyId, userId, projectId, devServerId }  ← DB
    └─ Push: terminal:created { ptyId, sessionId } → Browser (WebSocket)
    │
    ▼
[Browser] xterm.js attach I/O:
    ├─ terminal.onData → WebSocket RPC: terminal.input { ptyId, data }
    └─ onResize → WebSocket RPC: terminal.resize { ptyId, cols, rows }

PTY Data Stream (from Dev Server):
    Dev Server PTY output → relay stream event → Orca Server
    → WebSocket push → Browser (xterm.js render)

Luồng:
Browser → WS RPC → Orca Server → relay.call('pty.create') → Dev Server → node-pty
                                → DB (INSERT session)
                                → WS push → Browser (xterm.js ready)
PTY output: Dev Server → relay stream → Orca Server → WS push → Browser

Input:  { projectId, worktreeId, cols, rows }
Output: { ptyId, sessionId, status: 'ready' }
```

---

## BL-TM-02 — Split Terminal

```
Người dùng (Alex/Carlos)
    │ Ctrl+\ hoặc click split button
    ▼
[Browser] SplitPaneManager.split({ direction: 'horizontal' | 'vertical' })
    │ WebSocket RPC: terminal.create { projectId, cols, rows }   ← BL-TM-01
    ▼
[Orca Server] → relay.call('pty.create', ...) → Dev Server → node-pty.spawn (mới)
    ├─ Mỗi split panel = 1 PTY trên Dev Server (ptyId riêng biệt)
    └─ INSERT orca_terminal_sessions (session thứ 2)  ← DB
    │
    ▼
[Browser] render new xterm.js panel trong split layout
    └─ Subscribe to stream cho ptyId mới

Isolation:
- Mỗi pane = 1 node-pty process độc lập trên Dev Server
- Resize 1 pane → terminal.resize {ptyId} → relay.call('pty.resize') → chỉ ảnh hưởng pane đó

Luồng:
User → Browser (split layout) → WS RPC terminal.create → Orca Server → relay.call('pty.create') → Dev Server
```

---

## BL-TM-03 — Lưu và Khôi phục Scrollback

```
SAVE FLOW (khi workspace deactivate hoặc terminal đóng):
    │
    ▼
[Orca Server — WorkspaceContextManager.saveTerminalScrollback({ ptyId })]
    │ relay.call('pty.scrollback', { ptyId })     ← Dev Server
    ▼
[Dev Server] đọc PTY scrollback buffer → gzip compress → return compressed
    │
    ▼
[Orca Server]
    ├─ INSERT orca_terminal_scrollback_snapshots
    │   { ptyId, projectId, userId, buffer: gzip, savedAt }   ← Server DB
    └─ relay.call('pty.destroy', { ptyId })  ← Dev Server: terminate PTY

RESTORE FLOW (khi workspace reactivate):
    │
    ▼
[Orca Server] SELECT buffer FROM orca_terminal_scrollback_snapshots
              WHERE projectId=? AND userId=? ORDER BY savedAt DESC  ← DB
    │ relay.call('pty.create', { restoreScrollback: true, ... })  ← Dev Server
    ▼
[Dev Server] spawn mới PTY → inject gzip buffer → decompress → write to PTY output
    │
    ▼
[Browser] xterm.js hiển thị restored content

Luồng Save:
Orca Server → relay.call('pty.scrollback') → Dev Server (read buffer)
            → DB (INSERT snapshot) → relay.call('pty.destroy') → Dev Server

Luồng Restore:
Orca Server → DB (SELECT snapshot) → relay.call('pty.create', scrollback)
            → Dev Server → xterm inject → Browser (display restored)
```

---

## BL-TM-04 — Shell Integration (OSC 133) + Agent Hook

```
[Shell Process on Dev Server] output OSC 133 escape sequences:
    ESC ] 133 ; A ST  → prompt start
    ESC ] 133 ; B ST  → prompt end / command start
    ESC ] 133 ; C ST  → command output start
    ESC ] 133 ; D ; <exit_code> ST → command end
    │
    ▼
[Dev Server relay] stream PTY output → relay stream → Orca Server
    │
    ▼
[Orca Server — OSC133Parser]
    ├─ Parse sequences → extract: { promptText, command, outputStart, exitCode }
    ├─ Emit structured events:
    │   shell:promptDetected { ptyId, cwd }
    │   shell:commandStarted { ptyId, command }
    │   shell:commandFinished { ptyId, exitCode, duration }
    │
    ▼
[Browser — xterm.js]
    ├─ Highlight command zones (click-to-select output)
    ├─ Show exit code indicator
    └─ Enable "jump to command" navigation

AGENT HOOK (OSC custom sequences từ agent trên Dev Server):
    Dev Server PTY → relay stream → Orca Server:
    ├─ Parse agent-specific OSC → agent state machine
    └─ Trigger: agent:toolCallStarted / agent:toolCallFinished
       → Browser (agent activity overlay)

Luồng:
Shell (Dev Server) → PTY output → relay stream → Orca Server (OSC parse)
                  → Browser (visual decorations + navigation)
                  → Orca Server (agent hook dispatch)
```

---

## Sơ đồ tổng quan — Terminal Management (3-tier)

```
┌──────────────────────────────────┐
│  Browser — xterm.js              │
│  Split pane 1 (ptyId-A)          │
│  Split pane 2 (ptyId-B)          │
│  OSC decorations                 │
└──────────┬───────────────────────┘
           │ WebSocket RPC (bidirectional)
           │ terminal.create/resize/input/close
           │ stream: terminal:output, terminal:exit
           ▼
┌──────────────────────────────────┐
│  Orca Server                     │
│  WorkspaceContextManager         │
│  OSC133Parser                    │
│  ProjectServerRouter             │
│  RelayConnectionPool             │
└──────────┬───────────────────────┘
           │ relay.call (JSON-RPC)
           │ pty.create / pty.resize / pty.destroy
           │ pty.scrollback / pty.write
           │ stream: pty:output, pty:exit
           ▼
┌──────────────────────────────────┐
│  Dev Server (Remote Host)        │
│  node-pty (PTY_REGISTRY)         │
│  PTY session A: ptyId-A          │
│  PTY session B: ptyId-B          │
│  Shell process ($SHELL)          │
│  Agent process (nếu chạy agent)  │
└──────────────────────────────────┘

DB (Orca Server):
  orca_terminal_sessions { ptyId, userId, projectId, devServerId }
  orca_terminal_scrollback_snapshots { ptyId, buffer, savedAt }

Chiều kết nối (quan trọng):
  Dev Server ──WS connect──► Orca Server  (Dev Server = WS client)
  Orca Server ──relay.call──► Dev Server  (JSON-RPC request)
  Dev Server ──stream──► Orca Server       (PTY output events)
  Orca Server ──WS push──► Browser         (xterm.js data)
```

---

## Giới hạn của Terminal khi Dev Server là agent phone-home (WebSocket)

> Terminal/PTY qua Dev Server mới được hỗ trợ. Traffic của terminal đi qua ĐÚNG kết
> nối WebSocket mà Dev Server Agent đã mở sẵn cho git/file — không mở kết nối riêng.

- **Cần node-pty cài trên đúng máy đó**: Dev Server Agent phải cài `node-pty`
  (build tools + `npm install` + restart agent) thì mới mở được terminal. Đây là
  bước setup riêng của người vận hành Dev Server — tách biệt với việc chỉ kết nối
  Dev Server. Agent chưa cài node-pty vẫn kết nối/dùng git/file bình thường, chỉ
  không mở được terminal (xem lỗi ở BL-TM-01).
- **Không hỗ trợ reattach**: đóng tab (hoặc mất kết nối), hay restart Orca → phiên
  PTY trên Dev Server kết thúc, phải `pty.create` một phiên mới (BL-TM-01). BL-TM-03
  chỉ khôi phục lại **nội dung scrollback** vào một PTY hoàn toàn mới, không resume
  đúng process đang chạy. Khác với terminal trên SSH Host, vốn có thể sống qua
  grace period và reconnect vào đúng phiên cũ.
- **Không có "danh sách terminal đang chạy" hay shell profile đã lưu**: mỗi
  terminal mở trên Dev Server đều dùng default shell của máy đó.

---

## Khác biệt HLD cũ vs. Thực tế

| HLD cũ | Thực tế |
|--------|---------|
| `Main Process → Unix Socket → Daemon → node-pty` | `Orca Server → relay.call('pty.create') → Dev Server → node-pty` |
| `Daemon Process` là local | node-pty chạy trên **remote Dev Server** |
| `IPC (contextBridge)` | WebSocket RPC (Browser ↔ Orca Server) |
| `SQLite local` | Server DB (orca_terminal_sessions) |
| `OSC133Parser` ở Main Process | OSC133Parser ở Orca Server (parse relay stream) |
