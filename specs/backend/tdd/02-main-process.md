# TDD-02: Main Process

**Document:** TDD-02  
**Domain:** Electron Main Process  
**Source files:** `src/main/index.ts`, `src/main/startup/`, `src/main/window/`, `src/main/ipc/register-core-handlers.ts`

---

## 1. Trách nhiệm

Main Process là **orchestrator trung tâm** của Orca:
- Quản lý app lifecycle (start, quit, update)
- Tạo và quản lý BrowserWindow(s)
- Fork PTY daemon process
- Khởi động RPC server (WebSocket + Unix socket)
- Đăng ký IPC handlers cho renderer process
- Quản lý system tray
- Xử lý single-instance lock
- Telemetry + crash reporting

---

## 2. Startup Phases

### Phase 0: Pre-ready (synchronous)

```typescript
// src/main/startup/configure-process.ts
configureElectronNetworkCompatibility()   // proxy, cert
configureDevUserDataPath()                // dev mode data isolation
enableRendererHeapHeadroom()              // 4GB heap limit
enableMainProcessGpuFeatures()            // WebGL, canvas

// src/main/startup/ensure-virtual-display.ts
ensureVirtualDisplayForHeadlessServe()    // Linux: start Xvfb :99

// Single instance lock
acquireSingleInstanceLock()               // exit nếu instance khác đang chạy
```

### Phase 1: app.whenReady()

```typescript
// Persistence
initDataPath(app.getPath('userData'))
Store = new Store()         // SQLite init + migrations

// Services
initObservability()         // log files setup
initTelemetry()             // PostHog, Sentry
initDaemonPtyProvider()     // fork daemon-entry.js

// Runtime
const runtime = new OrcaRuntimeService(store, ...)
const rpcServer = new OrcaRuntimeRpcServer({ runtime, ... })
await rpcServer.start()     // WebSocket :6768 + Unix socket
```

### Phase 2: Window / Serve

```typescript
if (isServeMode) {
  // Headless: print pairing URL, stay alive
  printPairingInfo()
} else {
  // Desktop: create window, tray, menu
  const win = createMainWindow()
  createSystemTray(win)
  registerAppMenu(win)
  startFirstWindowStartupServices(win, store)
}
```

---

## 3. IPC Registration

```typescript
// src/main/ipc/register-core-handlers.ts
export function registerCoreHandlers(store: Store, runtime: OrcaRuntimeService): void {
  // Đăng ký tất cả ipcMain handlers
  registerFilesystemHandlers(store)     // filesystem.ts
  registerPtyHandlers()                 // pty.ts
  registerSshHandlers(store)            // ssh.ts
  registerWorktreeHandlers(store)       // worktrees.ts
  registerRepoHandlers(store)           // repos.ts
  registerSettingsHandlers(store)       // settings.ts
  registerNotificationHandlers()        // notifications.ts
  registerSpeechHandlers()              // speech.ts
  // ... 30+ handler groups
}
```

**Pattern:** Mỗi handler group trong `src/main/ipc/` export một function `register*Handlers()`.

---

## 4. Cấu trúc IPC Handlers

### Filesystem handlers (`ipc/filesystem.ts`)

```typescript
// ~78K code — lớn nhất trong ipc/
// Handles:
ipcMain.handle('filesystem:readFile', ...)
ipcMain.handle('filesystem:writeFile', ...)
ipcMain.handle('filesystem:listDir', ...)
ipcMain.handle('filesystem:search', ...)
ipcMain.handle('filesystem:move', ...)
ipcMain.handle('filesystem:delete', ...)
// SSH path variants: filesystem:readFile:ssh, etc.
```

### PTY handlers (`ipc/pty.ts`)

```typescript
// Proxy tới daemon via DaemonPtyAdapter/DaemonPtyRouter
ipcMain.handle('pty:create', ...)      // tạo terminal session
ipcMain.handle('pty:write', ...)       // gửi input vào terminal
ipcMain.handle('pty:resize', ...)      // thay đổi kích thước
ipcMain.handle('pty:kill', ...)        // kết thúc process
ipcMain.on('pty:data', ...)            // subscribe terminal output
```

### SSH handlers (`ipc/ssh.ts`)

```typescript
ipcMain.handle('ssh:listTargets', ...)
ipcMain.handle('ssh:addTarget', ...)
ipcMain.handle('ssh:removeTarget', ...)
ipcMain.handle('ssh:connect', ...)
ipcMain.handle('ssh:disconnect', ...)
ipcMain.handle('ssh:importFromConfig', ...)
ipcMain.handle('ssh:listConnections', ...)
```

---

## 5. Window Management

```typescript
// src/main/window/createMainWindow.ts
export function createMainWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      sandbox: false,   // cần cho node integration qua preload
      contextIsolation: true
    }
  })
  // Load renderer hoặc dev server
  if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
    win.loadURL(process.env['ELECTRON_RENDERER_URL'])
  } else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }
  return win
}
```

---

## 6. GPU Crash Recovery

```typescript
// src/main/crash-reporting/gpu-crash-fallback-decision.ts
class GpuCrashFallbackTracker {
  // Theo dõi GPU crashes trong sliding window
  // Nếu >= threshold crashes → write fallback marker
  // Lần sau khởi động đọc marker → disable GPU acceleration
}

// src/main/startup/gpu-fallback-marker.ts
readActiveGpuFallbackMarker()    // đọc marker
writeGpuFallbackMarker(env)      // ghi marker
// GpuFallbackEnvironment: 'software-rasterizer' | 'no-gpu'
```

---

## 7. Update Lifecycle

```typescript
// src/main/updater.ts (~55K code)
// electron-updater với custom logic:
// - Kiểm tra GitHub releases
// - Differential updates (delta)
// - Pre-release feed support
// - macOS: DMG install flow
// - Windows: NSIS installer
// - Linux: không có auto-update (AppImage/deb thủ công)

// Hooks:
updater.on('update-available', ...)
updater.on('update-downloaded', ...)
updater.on('before-quit-for-update', ...)
```

---

## 8. Telemetry & Observability

```typescript
// src/main/telemetry/client.ts
initTelemetry()    // PostHog + Sentry init
track('event', properties)
trackAppOpenedOnce()

// Consent model:
// - Không gửi telemetry nếu user opt-out
// - Sentry crash reports luôn bật (safety)
// - Usage events chỉ khi opted-in

// src/main/observability/
// - Log files: ~/.config/orca/logs/
// - Log rotation, level control
```

---

## 9. Headless Serve Mode

```typescript
// src/main/index.ts — serve mode branch
const serveArgs = {
  port: getServePort(),              // default: 6768
  pairingAddress: getPairingAddress(), // wss://domain
  noPairing: getNoPairing(),
  mobilePairing: getMobilePairing()
}

// Output JSON để CLI parse:
// { "type": "serve-ready", "pairingUrl": "orca://pair?code=..." }
// { "type": "web-ui", "url": "https://domain/web-index.html?pair=..." }

// Quá trình:
// 1. Xvfb start (Linux)
// 2. app.whenReady()
// 3. Store init
// 4. Runtime init
// 5. RpcServer.start(port)
// 6. Generate pairing offer
// 7. Print JSON → stdout
// 8. Keep alive (no window, no tray)
```

---

## 10. Addendum v2.0: Web Server Mode (restructure_v1) — IMPLEMENTED ✅

> **Date:** 2026-07-23

### Server Mode vs Electron Mode

| Khía cạnh | Electron Mode (`src/main/index.ts`) | Server Mode (`src/server/index.ts`) |
|-----------|-------------------------------------|-------------------------------------|
| Platform | Electron native APIs | `NodeAdapter` (platform abstraction) |
| Window | `BrowserWindow` | `NodeWindow` (no GUI, virtual) |
| IPC | `ipcMain` (Electron) | `NodeIpcBridge` + `WebIpcBridge` |
| Storage | `safeStorage` (Electron) | `NodeSecureStorage` (AES-256-GCM) |
| Startup | Electron `app.whenReady()` | Direct async bootstrap |
| UI | Electron renderer | Web SPA (`out/web/`) via HTTP |

### Server Mode Startup Sequence

```
src/server/index.ts:
  1. Parse env (ORCA_PORT, ORCA_HTTP_PORT, ORCA_USER_DATA_PATH)
  2. createNodeAdapter({ userDataPath })
  3. setPlatform(adapter)                    ← PHẢI TRƯỚC IMPORT electron modules
  4. import('../main/server-bootstrap')
  5. initializeOrcaServices({ platform, port })
     ├── initDataPath()
     ├── new Store()
     ├── initStatsPath() + new StatsCollector()
     ├── initOrcaProfilePaths() [non-fatal]
     ├── initDaemonPtyProvider() [non-fatal]
     ├── new OrcaRuntimeService(store, stats)
     └── new OrcaRuntimeRpcServer({ runtime, userDataPath, enableWebSocket, wsPort })
         .start()
  6. startHttpServer(httpPort, out/web/) [if web bundle exists]
  7. SIGINT/SIGTERM → graceful shutdown
```

### electron-node-wrapper.ts — Key mechanism

```typescript
// vite.server.config.ts resolves:
//   import { app } from 'electron'
//   → src/platform/stubs/electron-node-wrapper.ts

// electron-node-wrapper.ts:
export const app = {
  getPath: (name) => getPlatform().app.getPath(name),
  // ...
}
// Toàn bộ src/main/ code dùng 'electron' → thực ra dùng NodeAdapter
```

### Tham khảo

- [TDD-10: Platform Layer](./10-platform-layer.md)
- [TDD-11: Web Server Mode](./11-web-server-mode.md)
- `src/server/index.ts`
- `src/main/server-bootstrap.ts`
