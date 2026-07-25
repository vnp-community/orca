# TDD-01: Kiến trúc tổng thể

**Document:** TDD-01  
**Domain:** Architecture Overview  
**Source files:** `src/main/index.ts`, `src/main/startup/`, `src/main/runtime/`  

---

## 1. Process Model

Orca có kiến trúc **multi-process** với 4 process chính:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         ELECTRON MAIN PROCESS                           │
│                         src/main/index.ts                               │
│                                                                         │
│  Khởi động theo thứ tự:                                                 │
│  1. configureProcess (env, signals, GPU flags)                          │
│  2. app.whenReady() → SQLite store + services                           │
│  3. createMainWindow() (hoặc headless serve mode)                       │
│  4. initDaemonPtyProvider() — fork daemon                               │
│  5. OrcaRuntimeRpcServer.start() — WebSocket + Unix socket              │
│                                                                         │
│  Threads: Main JS thread + Electron helper processes                    │
└────────────┬──────────────────────────────────────────┬────────────────┘
             │ fork(daemon-entry.js)                    │ WebSocket/Unix
             ▼                                          ▼
┌─────────────────────────┐              ┌──────────────────────────────┐
│    DAEMON PROCESS        │              │    EXTERNAL CLIENTS          │
│   src/main/daemon/       │              │                              │
│                          │              │  - Orca Desktop App (IPC)    │
│  node-pty instances      │              │  - Browser (wss://)          │
│  Terminal emulation      │              │  - Orca Mobile (WSS + E2EE)  │
│  History manager         │              │  - CLI (Unix socket)         │
│  Session snapshots       │              │                              │
│                          │              │  Auth: token + E2EE          │
│  Protocol: NDJSON over   │              └──────────────────────────────┘
│  Unix socket             │
└────────────┬────────────┘
             │
             │ SSH (system ssh / internal lib)
             ▼
┌─────────────────────────┐
│   REMOTE SSH RELAY       │
│   src/relay/             │
│                          │
│  node-pty (remote)       │
│  Git operations          │
│  SFTP file transfer      │
│  Port scanning           │
│                          │
│  Transport: Multiplexed  │
│  SSH channels            │
└─────────────────────────┘
```

---

## 2. Startup Sequence

```
app.commandLine setup (GPU, Electron flags)
  ↓
ensureVirtualDisplayForHeadlessServe()   [Linux only, nếu headless]
  ↓
hydrateShellPath()                       [merge /usr/local/bin vào PATH]
  ↓
acquireSingleInstanceLock()              [prevent double-launch]
  ↓
app.whenReady()
  ├─ initDataPath()                      [userData dir]
  ├─ initOrcaProfilePaths()              [profile store]
  ├─ Store.init() → SQLite               [persistence]
  ├─ initObservability()                 [logging]
  ├─ initTelemetry()                     [PostHog/Sentry]
  ├─ initDaemonPtyProvider()             [fork PTY daemon]
  ├─ OrcaRuntimeService.init()           [core runtime]
  ├─ OrcaRuntimeRpcServer.start()        [WebSocket port 6768]
  ├─ registerCoreHandlers()              [ipcMain handlers]
  ├─ createMainWindow() or headless      [window or serve mode]
  └─ runManagedHookInstallers()          [agent hook setup]
  ↓
app.on('before-quit'):
  ├─ shutdownDaemon()
  ├─ shutdownObservability()
  └─ shutdownTelemetry()
```

---

## 3. Serve Mode (Headless)

Khi chạy `orca serve`, Electron khởi động **không có cửa sổ**:

```typescript
// src/main/index.ts
const isServe = process.argv.includes('--serve')
if (isServe) {
  // Không gọi createMainWindow()
  // Không gọi createSystemTray()
  // Chỉ start RPC server → WebSocket
  await rpcServer.start()
  // In pairing URL ra stdout
  console.log(JSON.stringify({ type: 'pairing-url', url: pairingUrl }))
}
```

**Virtual Display cho Linux headless:**

```typescript
// src/main/startup/ensure-virtual-display.ts
// Bắt đầu Xvfb :99 nếu DISPLAY chưa có
// Electron cần display để render BrowserWindow (kể cả offscreen)
export function ensureVirtualDisplayForHeadlessServe(): void
```

---

## 4. Execution Host Model

Orca có khái niệm **ExecutionHost** — nơi code chạy:

```typescript
// src/shared/execution-host.ts
type ExecutionHostId =
  | 'local'                    // máy local
  | `ssh-${string}`            // remote server qua SSH
  | `ephemeral-vm-${string}`   // on-demand cloud VM
```

Mỗi **Project** có một `executionHostId`. Khi tạo worktree, Orca route tất cả operations (git, pty, file watch) qua execution host đó.

---

## 5. Data Flow: Client Request → Response

```
Client (Browser/App/CLI)
  │
  │ WebSocket frame (JSON)
  ▼
WebSocketTransport.receive()
  │ Decrypt (E2EE nếu mobile/web)
  ▼
OrcaRuntimeRpcServer.handleMessage()
  │ Auth token check
  │ Long-poll cap check
  ▼
RpcDispatcher.dispatch(request)
  │ Zod schema validation
  ▼
Method handler (e.g., worktree.create)
  │
  ├─ OrcaRuntimeService methods (business logic)
  │   ├─ SQLite persistence
  │   ├─ Git operations
  │   └─ PTY/Daemon calls
  │
  └─ Response → encrypt → WebSocket frame → Client
```

---

## 6. Key Modules Map

| Layer | Module | File |
|-------|---------|------|
| Entry | Main process bootstrap | `src/main/index.ts` |
| Startup | Process config | `src/main/startup/configure-process.ts` |
| Startup | Virtual display | `src/main/startup/ensure-virtual-display.ts` |
| Startup | Daemon init | `src/main/daemon/daemon-init.ts` |
| Core | Runtime service | `src/main/runtime/orca-runtime.ts` |
| Core | RPC server | `src/main/runtime/runtime-rpc.ts` |
| Core | RPC dispatcher | `src/main/runtime/rpc/dispatcher.ts` |
| Core | RPC methods | `src/main/runtime/rpc/methods/` |
| Transport | WebSocket | `src/main/runtime/rpc/ws-transport.ts` |
| Transport | Unix socket | `src/main/runtime/rpc/unix-socket-transport.ts` |
| Security | E2EE | `src/main/runtime/rpc/e2ee-channel.ts` |
| Security | Pairing | `src/shared/pairing.ts` |
| Persistence | SQLite store | `src/main/persistence.ts` |
| SSH | Connection | `src/main/ssh/ssh-connection.ts` |
| SSH | Relay deploy | `src/main/ssh/ssh-relay-deploy.ts` |
| SSH | Relay session | `src/main/ssh/ssh-relay-session.ts` |
| Daemon | PTY adapter | `src/main/daemon/daemon-pty-adapter.ts` |
| Daemon | PTY server | `src/main/daemon/daemon-server.ts` |
| IPC | Core handlers | `src/main/ipc/register-core-handlers.ts` |
| IPC | Filesystem | `src/main/ipc/filesystem.ts` |
| IPC | PTY | `src/main/ipc/pty.ts` |
| IPC | SSH | `src/main/ipc/ssh.ts` |
| Git | Runner | `src/main/git/runner.ts` |
| Relay | Entry | `src/relay/` |

---

## 7. Addendum v2.0: restructure_v1 Updates — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **Status:** Complete

### Kiến trúc mới — Web Server Mode

```
┌────────────────────────────────────────────────────────────────────────┐
│                  SRC/PLATFORM/ (MỚI — restructure_v1)                  │
│                                                                        │
│  IPlatformServices { app, windowManager, ipc, storage, system }        │
│  setPlatform() / getPlatform()  ← singleton                           │
│                                                                        │
│  NodeAdapter (web/server mode)    │  electron-node-wrapper (server)   │
│  createNodeAdapter()              │  → delegate to getPlatform()       │
│  NodeApp, NodeWindow, NodeIpcBridge, NodeSecureStorage                │
└────────────────────────────────────────────────────────────────────────┘

src/server/index.ts (WEB SERVER ENTRY — MỚI):
  1. createNodeAdapter({ userDataPath })
  2. setPlatform(adapter)              ← PHẢI TRƯỚC KHI IMPORT electron modules
  3. initializeOrcaServices()         ← server-bootstrap.ts
  4. startHttpServer(6769, out/web/)  ← http-server.ts
  → RPC WS: :6768  |  HTTP UI: :6769
```

### Key Modules Map — bổ sung v2.0

| Layer | Module | File |
|-------|---------|------|
| **Platform** | **Platform types** | **`src/platform/types.ts`** |
| **Platform** | **Platform context** | **`src/platform/context.ts`** |
| **Platform** | **Node Adapter** | **`src/platform/adapters/node/`** |
| **Platform** | **Web IPC Bridge** | **`src/platform/adapters/node/web-ipc-bridge.ts`** |
| **Platform** | **Electron Stub (server)** | **`src/platform/stubs/electron-node-wrapper.ts`** |
| **Platform** | **Electron Stub (browser)** | **`src/platform/stubs/electron-web-stub.ts`** |
| **Server** | **HTTP static server** | **`src/server/http-server.ts`** |
| **Server** | **Server entry** | **`src/server/index.ts`** |
| **Server** | **Server bootstrap** | **`src/main/server-bootstrap.ts`** |

### Nguyên tắc bổ sung

6. **Platform abstraction**: Không có `import 'electron'` ngoài stubs và Electron adapter
7. **Additive only**: `src/main/` không bị modify — chỉ thêm mới
8. **Dual build targets**: `vite.server.config.ts` (Node) + `vite.web-spa.config.ts` (Browser)
9. **Test isolation**: 166 tests chạy hoàn toàn trong Node.js vitest

### Tham khảo

- [TDD-10: Platform Layer](./10-platform-layer.md)
- [TDD-11: Web Server Mode](./11-web-server-mode.md)
