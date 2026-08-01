# Terminal Create Flow

> **Scope:** Mô tả luồng `terminal.create` theo đúng thiết kế kiến trúc HLD.
>
> **Nguyên tắc thiết kế (HLD):**
> - **Backend (Control Plane):** Auth, session routing, RBAC, scrollback snapshot — **KHÔNG** tự chạy PTY
> - **Dev Server (Data Plane):** PTY spawn thực sự, shell execution, I/O streaming
> - **Kết nối:** Dev Server Agent chủ động **outbound connect** vào Backend WebSocket (`/agent`)
>
> **Key files:**
> - [`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`](../../src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) — Browser PTY transport
> - [`src/main/session/ws-session-router.ts`](../../src/main/session/ws-session-router.ts) — WS auth + per-user process proxy
> - [`src/main/session/session-manager.ts`](../../src/main/session/session-manager.ts) — User process lifecycle
> - [`src/main/runtime/rpc/methods/terminal.ts`](../../src/main/runtime/rpc/methods/terminal.ts) — `terminal.create` RPC handler
> - [`src/main/runtime/orca-runtime.ts`](../../src/main/runtime/orca-runtime.ts) — `createTerminal()` orchestration
> - [`src/main/dev-server/dev-server-relay-bridge.ts`](../../src/main/dev-server/dev-server-relay-bridge.ts) — Route pty.spawn → Agent
> - [`src/main/dev-server/agent-ws-server.ts`](../../src/main/dev-server/agent-ws-server.ts) — `/agent` WS endpoint nhận agent connect
> - [`src/relay/pty-handler.ts`](../../src/relay/pty-handler.ts) — PTY spawn thực sự trên Dev Server

---

## Phân tầng trách nhiệm (Control Plane vs Data Plane)

Theo HLD, hệ thống chia làm 2 tầng rõ ràng:

| Tầng | Thành phần | Trách nhiệm với Terminal |
|------|-----------|--------------------------|
| **Control Plane** | Orca Backend Server | Auth, session routing, RBAC, scrollback snapshot lưu DB, phân phối RPC |
| **Data Plane** | Dev Server Agent | PTY spawn, shell execution, I/O streaming, resize, kill — **toàn bộ runtime** |

> Backend **KHÔNG** tự chạy PTY. Mọi PTY đều chạy trên Dev Server Agent.

---

## Business Logic áp dụng

| Mã | Tên | Yêu cầu |
|----|-----|---------|
| **BL-TM-01** | Tạo PTY Session | PTY tạo trên Dev Server; detect platform (POSIX/ConPTY); spawn $SHELL |
| **BL-TM-02** | Split Terminal | Mỗi split = PTY riêng độc lập trên Dev Server (BR-TM-05) |
| **BL-TM-03** | Scrollback Persistence | Backend lưu snapshot vào `terminal_scrollback_snapshots` (SQLite, max 50MB) |
| **BL-TM-04** | Shell Integration | OSC 133 sequences parse trên Dev Server, stream về Browser |
| **BR-TM-01** | Cleanup on close | PTY phải cleanup khi tab đóng — không để zombie process trên Dev Server |
| **BR-TM-02** | Resize propagation | Resize phải propagate đến PTY process ngay lập tức |
| **BR-TM-03** | Color support | PTY phải hỗ trợ 256-color và true color (`TERM=xterm-256color`) |
| **BR-TM-04** | Shell path | Resolve từ `$SHELL` env trên Dev Server, không hardcode |

---

## Luồng tổng quan

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         [PRE-CONDITION]                                 │
│  Dev Server Agent đã connect inbound: wss://backend/agent               │
│  AgentWebSocketServer đã assign session = SshChannelMultiplexer         │
│  User đã login: POST /auth/local → cookie orca_session                  │
└───────────────────────────────────────────────────────────────────────┬─┘
                                                                        │
                                                                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  BROWSER (React Renderer — remote-runtime-pty-transport.ts)            │
│                                                                         │
│  Trigger: User mở Terminal pane                                         │
│  callRuntime('terminal.create', {                                       │
│    worktree: '<projectId>:<worktreeId>',                                │
│    command?: string,        ← tuỳ chọn (default: $SHELL)               │
│    env?: Record<string,string>,                                         │
│    tabId, leafId,           ← để link với renderer pane                │
│    presentation: 'background' | 'focused',                             │
│    focus: boolean                                                       │
│  })                                                                     │
│  └─► window.api.runtimeEnvironments.call({ selector, method, params }) │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ WebSocket binary frames
                                   │ JSON-RPC 2.0 (13-byte header)
                                   │ wss://backend:6768/
                                   ▼
═══════════════════════ CONTROL PLANE (Orca Backend) ═══════════════════════
┌─────────────────────────────────────────────────────────────────────────┐
│  BƯỚC 1 — Auth & Session Routing (WsSessionRouter)                     │
│                                                                         │
│  ws-session-router.ts:                                                  │
│  ├─ [AUTH] resolveUserFromRequest(cookie: orca_session)                 │
│  │    → AuthManager.getSession() → userId                               │
│  │    FAIL? → ws.close(4401, 'Authentication required')                 │
│  │                                                                      │
│  ├─ [SPAWN] SessionManager.getOrSpawn(userId)                          │
│  │    → fork(user-process-entry.js, {                                   │
│  │          ORCA_USER_ID: userId,                                       │
│  │          ORCA_RPC_AUTH_TOKEN: <token>,                               │
│  │          ORCA_SOCKET_PATH: ~/.orca/users/<userId>/orca.sock          │
│  │      })                                                              │
│  │    FAIL (ENOENT)? → ws.close(1011, 'fork error')                    │
│  │    FAIL (30s timeout)? → ws.close(1011, 'spawn timeout')            │
│  │                                                                      │
│  └─ [PROXY] net.createConnection(socketPath)                           │
│         → proxy WS frames ↔ Unix socket bidirectionally                │
│         FAIL (ECONNREFUSED)? → ws.close(1011, 'session unavailable')   │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ Unix domain socket
                                   │ ~/.orca/users/<userId>/orca.sock
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  BƯỚC 2 — RPC Dispatch (User Process)                                  │
│  user-process-entry.ts → OrcaRuntimeRpcServer                          │
│                                                                         │
│  ├─ [AUTH] parseAndAuth(ORCA_RPC_AUTH_TOKEN)                           │
│  │    Token được inject qua IPC 'ready' message từ SessionManager      │
│  │    FAIL? → disconnect với error 'forbidden'                          │
│  │                                                                      │
│  ├─ [RBAC] checkScopedTokenPermission('terminal.create', serverId)     │
│  │    Scoped token phải có allowedServerIds chứa devServerId            │
│  │    FAIL? → RPC error { code: -32003, message: 'forbidden' }         │
│  │                                                                      │
│  └─ dispatcher.dispatch('terminal.create', params)                     │
│         → methods/terminal.ts:1285                                      │
│         → runtime.createTerminal(params.worktree, opts)                 │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ Internal function call
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  BƯỚC 3 — Terminal Orchestration (OrcaRuntimeService)                  │
│  orca-runtime.ts:createTerminal()                                       │
│                                                                         │
│  ├─ resolveTerminalWorkspaceLaunchScope(worktreeSelector)               │
│  │    → workspace = { id, path, connectionId, devServerId }             │
│  │    connectionId là key để route đến đúng relay session               │
│  │                                                                      │
│  ├─ resolveAgentTerminalCreateOptions(workspace, opts)                  │
│  │    → merge profile env (ProfileResolver, cache 60s)                  │
│  │    → inject: ORCA_USER_ID, ORCA_PROJECT_ID, paneKey                 │
│  │    → shell: resolve từ $SHELL (BR-TM-04)                             │
│  │                                                                      │
│  ├─ [BACKGROUND SPAWN] shouldCreateInBackground = true                 │
│  │    (Web Server mode, no renderer window — luôn spawn qua relay)      │
│  │    FAIL? (!ptyController.spawn) → error 'runtime_unavailable'        │
│  │                                                                      │
│  ├─ preAllocatedHandle = createPreAllocatedTerminalHandle()             │
│  │    → mint stable handle/tabId/leafId trước khi spawn                │
│  │    → để paneKey attribution chính xác cho agent hooks               │
│  │                                                                      │
│  ├─ buildTerminalWorkspaceEnv(workspace, baseEnv, paneKey)             │
│  │    Inject environment:                                               │
│  │    ORCA_PANE_KEY, ORCA_WORKTREE_ID, ORCA_USER_ID                    │
│  │    GH_CONFIG_DIR=~/.config/gh/<userId>/ (per-user isolation)        │
│  │    TERM=xterm-256color (BR-TM-03)                                   │
│  │                                                                      │
│  └─ ptyController.spawn({                                               │
│         connectionId,   ← route đến Dev Server relay session            │
│         cwd, command, env,                                              │
│         cols: 120, rows: 40,                                            │
│         preAllocatedHandle, tabId, leafId, worktreeId                   │
│     })                                                                  │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ connectionId → DevServerRelayBridge
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  BƯỚC 4 — Relay Routing (DevServerRelayBridge)                         │
│  dev-server-relay-bridge.ts                                             │
│                                                                         │
│  ├─ lookup session bởi connectionId (devServerId)                      │
│  │    this.session = SshChannelMultiplexer                              │
│  │    FAIL (session = null)? → error 'Not connected'                   │
│  │         → Nguyên nhân: Agent chưa connect hoặc WS dropped           │
│  │                                                                      │
│  ├─ [RECONNECT QUEUE] nếu _reconnecting = true:                        │
│  │    → queue request, chờ tối đa 20s cho session restore              │
│  │    → FAIL (timeout 20s)? → error 'Not connected'                    │
│  │                                                                      │
│  └─ callWithTimeout('pty.spawn', {                                      │
│         cwd, command, env, cols: 120, rows: 40,                         │
│         terminalHandle: preAllocatedHandle,                             │
│         worktreeId, userId, tabId, leafId                               │
│     }, 30_000ms)                                                        │
│     → session.request('pty.spawn', params)                             │
│     FAIL (timeout 30s)? → error 'Relay call timed out'                 │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ JSON-RPC over WebSocket
                                   │ (Agent outbound session)
═══════════════════════ DATA PLANE (Dev Server Agent) ═══════════════════════
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  BƯỚC 5 — PTY Spawn (Dev Server Agent — relay/pty-handler.ts)          │
│  Đây là nơi duy nhất PTY thực sự được tạo (BL-TM-01)                  │
│                                                                         │
│  ├─ [CONTEXT VERIFY] ContextVerifier.verify(rpcExecutionContext)        │
│  │    HMAC-SHA256 signed context, TTL 30s                               │
│  │    FAIL? → JSON-RPC error { code: -32001, message: 'context invalid'}│
│  │                                                                      │
│  ├─ [ISOLATION CHECK] PTY ownership check: ptyId bound to userId       │
│  │    SecureFs.validatePath(cwd, projectRoot + allowedRoots)            │
│  │    FAIL (path traversal)? → error 'path not allowed'                 │
│  │                                                                      │
│  ├─ [PLATFORM DETECT] (BL-TM-01 step 2a)                              │
│  │    POSIX (Linux/macOS): node-pty native → openpty()                 │
│  │    Windows: ConPTY via node-pty                                      │
│  │                                                                      │
│  ├─ [SHELL RESOLVE] (BR-TM-04)                                         │
│  │    shell = env.SHELL ?? '/bin/bash'                                  │
│  │    resolveDefaultShell(platform) → verify binary exists             │
│  │    FAIL (not found)? → error { code: -32001, 'shell not found' }    │
│  │                                                                      │
│  ├─ node-pty.spawn(shell, [], {                                         │
│  │      name: 'xterm-256color',    ← BR-TM-03                          │
│  │      cols: 120, rows: 40,                                            │
│  │      cwd: <resolved cwd>,                                            │
│  │      env: { ...process.env, ...injectedEnv }                        │
│  │  })                                                                  │
│  │                                                                      │
│  ├─ [STARTUP COMMAND] gửi startup command vào PTY stdin nếu có        │
│  │    delivery: 'immediate' | 'shell-ready' | 'provider'               │
│  │    shell-ready: chờ OSC 133 A trước khi gửi (BL-TM-04)             │
│  │                                                                      │
│  ├─ [RESIZE HANDLER] pty.onResize → propagate về Browser (BR-TM-02)   │
│  │                                                                      │
│  ├─ [OUTPUT STREAM] pty.onData:                                         │
│  │    → scan OSC 133 sequences (BL-TM-04 shell integration)            │
│  │    → batch output (8ms interval, 16KB chunks)                       │
│  │    → encode relay frame → ws.send                                   │
│  │                                                                      │
│  └─ [CLEANUP HANDLER] pty.onExit: (BR-TM-01)                          │
│         disposeManagedPty()                                              │
│         → emit 'pty.exited' { ptyId, exitCode }                        │
│         → EventBus → Backend session cleanup                           │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ PTY data stream (relay protocol)
                                   │ pty.data frames → WebSocket → Backend
                                   ▼
═══════════════════════════ RESPONSE PATH ═══════════════════════════════════
┌─────────────────────────────────────────────────────────────────────────┐
│  Backend nhận response từ pty.spawn:                                    │
│  { ptyId, handle, cols, rows, cwd }                                     │
│                                                                         │
│  OrcaRuntimeService:                                                    │
│  ├─ registerPty(ptyId, worktreeId, connectionId)                        │
│  ├─ publishPtyBackedMobileSessionTerminal()  ← mobile companion sync   │
│  └─ notifier.revealTerminalSession() nếu presentation = 'focused'      │
│                                                                         │
│  Response về Browser:                                                   │
│  { terminal: { handle, tabId, paneKey, ptyId, worktreeId, surface } }  │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │ RPC response → WS → Browser
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Browser (remote-runtime-pty-transport.ts — sau terminal.create)        │
│                                                                         │
│  handle = created.terminal.handle                                       │
│  remotePtyId = toRemoteRuntimePtyId(handle, runtimeEnvironmentId)      │
│                                                                         │
│  → callRuntime('terminal.subscribe', { terminal: handle, ... })         │
│    → nhận binary TerminalStreamFrame (output, resize, osc events)       │
│    → xterm.js renderer render output                                    │
│                                                                         │
│  → Scrollback persistence (BL-TM-03):                                  │
│    Khi tab đóng → callRuntime('terminal.snapshot.save', { handle })     │
│    Backend lưu vào terminal_scrollback_snapshots (SQLite, max 50MB)    │
│    Khi mở lại → callRuntime('terminal.snapshot.restore', { handle })   │
│    Browser restore output + cursor position + attributes               │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Mô hình kết nối Dev Server Agent → Backend

Theo HLD `direct-websocket` mode (v6 default):

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ORCA BACKEND (b15.openledger.vn)         Control Plane                 │
│                                                                          │
│  AgentWebSocketServer                                                    │
│  ├── Lắng nghe: GET /agent (WS upgrade, port 6768)                      │
│  ├── registerSlot(agentToken, onConnected, onExpired)                   │
│  └── Khi agent connect:                                                  │
│        runOrcaInitiatorHandshake(ws)                                     │
│        → { type: 'handshake-ok', sessionId }                            │
│        → SshChannelMultiplexer(WsTransport)                             │
│        → bridge.session = mux  ← relay session ACTIVE                  │
│                                           ▲                             │
└───────────────────────────────────────────┼──────────────────────────────┘
                                            │ Outbound WS connect
                                            │ wss://backend:6768/agent
                                            │ Header: agentToken
                  [AGENT STARTUP SEQUENCE]  │
                  1. Load config            │
                  2. Init SQLite           │
                  3. Start Health Reporter  │
                  4. ReconnectManager ──────┘ (exp backoff 5s→60s)
                  5. Handshake: { agentToken, name, version, capabilities }
                  6. Ready to receive RPC
┌──────────────────────────────────────────────────────────────────────────┐
│  DEV SERVER AGENT (remote host của project)  Data Plane                 │
│  orca-agent binary (Node.js)                                             │
│                                                                          │
│  RpcServer ← ContextVerifier (HMAC-SHA256, TTL 30s)                    │
│  ├── PtyManager     — PTY create/resize/write/kill (BL-TM-01/02)        │
│  ├── HealthReporter — CPU/RAM/disk → Backend mỗi 60s                   │
│  ├── EventBus       — fan-out PTY events → Gateway stream               │
│  └── ReconnectManager — auto reconnect (5s→60s backoff)                │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Mô hình Scrollback Persistence (BL-TM-03)

Backend đóng vai trò **lưu trữ** scrollback, Dev Server đóng vai trò **cung cấp** snapshot:

```
┌────────────────────────────────────────────────────────────────────────┐
│  SERIALIZE (Lưu khi tab đóng):                                         │
│                                                                        │
│  Browser: xterm.js serialize() → gzip                                 │
│    → callRuntime('terminal.snapshot.save', { handle, data })          │
│    → Backend: INSERT INTO terminal_scrollback_snapshots               │
│         { worktree_id, handle, serialized_gz, cursor_pos, timestamp } │
│    Limits: max 50MB per worktree (BR-TM-10), expire 30 ngày (BR-TM-12)│
│                                                                        │
│  RESTORE (Khi mở lại worktree):                                        │
│                                                                        │
│  Backend: SELECT FROM terminal_scrollback_snapshots                   │
│    → callRuntime('terminal.snapshot.restore', { handle }) response    │
│    → Browser: decompress + xterm.js restore                           │
│    → Restore: output + cursor position + text attributes (BR-TM-11)   │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Split Terminal (BL-TM-02)

```
User nhấn Cmd+D (horizontal) hoặc Cmd+Shift+D (vertical)
  │
  ▼
Browser: callRuntime('terminal.split', {
    terminal: existingHandle,
    direction: 'horizontal' | 'vertical',
    command?: string
})
  │
  ▼
Backend: runtime.splitTerminal()
  → copy cwd từ PTY hiện tại
  → gọi lại terminal.create flow đầy đủ cho panel mới
  │
  ▼
Dev Server: node-pty.spawn() mới — PTY độc lập (BR-TM-05)
  │
Browser: 2 panels hiển thị độc lập
  - Mỗi panel có ptyId riêng (BR-TM-05)
  - Resize một panel không ảnh hưởng panel kia (BR-TM-07)
  - Đóng một panel không kill panel kia (BR-TM-08)
  - Minimum size: 80 cols × 10 rows (BR-TM-06)
```

---

## Wire Protocol

Theo HLD dev-server-architecture.md §5:

```
┌─────────────────────────────────────────────────────────────┐
│ TYPE[1B] │ SEQ[4B BE] │ ACK[4B BE] │ LEN[4B BE] │ PAYLOAD  │
│          = 13 bytes header                                   │
│ PAYLOAD  = UTF-8 JSON-RPC 2.0                               │
│ TYPE     = 0x01 Regular │ 0x09 KeepAlive                    │
└─────────────────────────────────────────────────────────────┘

PTY output stream (ngược lại về Browser):
  pty.data → encodeDataFrame
  → { type: 'pty.output', ptyId, data: base64 }
  → WebSocket binary → Browser xterm.js render
```

---

## Trace Points & Error Handling

### Trace spans được emit

| Tracer | Span | Tầng | Khi nào |
|--------|------|------|---------|
| `wsSession:route` | `FAIL 'auth required'` | Backend | Cookie invalid / không có |
| `session:spawn` | `FAIL { phase: 'fork' }` | Backend | ENOENT / EACCES |
| `session:spawn` | `FAIL 'spawn timeout 30s'` | Backend | user-process-entry không start |
| `wsSession:route` | `FAIL 'no socket path'` | Backend | child chưa gửi IPC 'ready' |
| `wsSession:route` | `FAIL { phase: 'upstream' }` | Backend | Unix socket ECONNREFUSED |
| `relay:agentCall` | `FAIL 'Not connected'` | Backend→Agent | Agent chưa connect hoặc WS dropped |
| `relay:agentCall` | `FAIL 'timed out 30000ms'` | Agent | PTY spawn hang |
| *(agent side)* | `error 'context invalid'` | Agent | HMAC verify fail |
| *(agent side)* | `error 'shell not found'` | Agent | Binary shell không tồn tại trên remote |
| *(agent side)* | `error 'path not allowed'` | Agent | SecureFs path traversal |

### Chuỗi lỗi thường gặp

```
1. [Backend] Cookie hết hạn
   → ws.close(4401) → Terminal không mở được

2. [Backend] user-process-entry.js ENOENT
   → ws.close(1011) → Terminal không mở được

3. [Backend→Agent] session = null (agent chưa connect)
   → RPC error 'Not connected' → Terminal lỗi ngay

4. [Agent] node-pty không có / cwd không tồn tại
   → timeout 30s → Terminal spinner dài → lỗi timeout

5. [Agent] shell binary không tồn tại trên remote
   → JSON-RPC error -32001 → Terminal lỗi rõ ràng

6. [Backend] Scoped token thiếu allowedServerIds
   → RPC error 'forbidden' → Terminal bị RBAC chặn
```

---

## Cách bật trace để debug

```bash
# Server-side (bật ORCA_TRACE trước khi start)
ORCA_TRACE=1 node out/server/index.js

# Subscribe SSE trace stream real-time
curl -N -H "X-Orca-Trace-Client: 1" https://b15.openledger.vn/api/trace-stream

# Kiểm tra agent connection
curl -s https://b15.openledger.vn/health
# → { status: 'healthy', agents: { connected: N } }
```

---

## Files liên quan

| File | Tầng | Vai trò |
|------|------|---------|
| [`remote-runtime-pty-transport.ts`](../../src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) | Browser | Gọi terminal.create, subscribe stream, scrollback |
| [`ws-session-router.ts`](../../src/main/session/ws-session-router.ts) | Backend | Auth guard WS, proxy → Unix socket |
| [`session-manager.ts`](../../src/main/session/session-manager.ts) | Backend | Fork per-user child process |
| [`user-process-entry.ts`](../../src/main/session/user-process-entry.ts) | Backend | Entry point user process |
| [`methods/terminal.ts`](../../src/main/runtime/rpc/methods/terminal.ts) | Backend | terminal.create/split/subscribe RPC handlers |
| [`orca-runtime.ts`](../../src/main/runtime/orca-runtime.ts) | Backend | createTerminal() orchestration, workspace resolve |
| [`dev-server-relay-bridge.ts`](../../src/main/dev-server/dev-server-relay-bridge.ts) | Backend | Route pty.spawn → Agent, reconnect queue |
| [`agent-ws-server.ts`](../../src/main/dev-server/agent-ws-server.ts) | Backend | /agent WS endpoint, nhận agent inbound connect |
| [`ws-handshake.ts`](../../src/main/dev-server/ws-handshake.ts) | Backend | Handshake protocol khi agent kết nối |
| [`relay/pty-handler.ts`](../../src/relay/pty-handler.ts) | Dev Server | **PTY spawn thực sự** + OSC 133 parse |
| [`relay/pty-shell-launch.ts`](../../src/relay/pty-shell-launch.ts) | Dev Server | Shell resolution theo platform |
| [`relay/pty-shell-utils.ts`](../../src/relay/pty-shell-utils.ts) | Dev Server | $SHELL resolve, cwd detect, process utils |
| [`shared/trace/index.ts`](../../src/shared/trace/index.ts) | Shared | Trace infrastructure |
| [`server/trace-sse-routes.ts`](../../src/server/trace-sse-routes.ts) | Backend | /api/trace-stream SSE endpoint |

---

## Tham khảo

- [HLD: Dev Server Architecture](../hld/dev-server-architecture.md) — §2 Data Plane roles, §3 Dev Server Agent v6, §4 Connection modes
- [HLD: Backend Server Architecture](../hld/backend-server-architecture.md) — §2 Control Plane roles, §7 Per-user session isolation
- [BL-TM-01](../logic/terminal-management/BL-TM-01-tao-pty-session.md) — Tạo PTY Session
- [BL-TM-02](../logic/terminal-management/BL-TM-02-split-terminal.md) — Split Terminal
- [BL-TM-03](../logic/terminal-management/BL-TM-03-scrollback-persistence.md) — Scrollback Persistence
- [BL-TM-04](../logic/terminal-management/BL-TM-04-shell-integration.md) — Shell Integration OSC 133
- [multi-user-session.md](./multi-user-session.md) — Per-user session fork model
- [agent-connection-modes.md](./agent-connection-modes.md) — Agent direct-websocket mode
- [dev-server-connection-types.md](./dev-server-connection-types.md) — 3 connection modes
