# TDD-10: Platform Abstraction Layer

**Document:** TDD-10 (NEW — restructure_v1)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Platform Abstraction — Interfaces, Node Adapters, Electron Stubs  
**Source files:**
- `src/platform/types.ts`
- `src/platform/context.ts`
- `src/platform/index.ts`
- `src/platform/*-interface.ts`
- `src/platform/adapters/node/`
- `src/platform/stubs/`
- `src/platform/__tests__/`

> **Status: ✅ IMPLEMENTED** — 166/166 tests | 0 TS errors | 0 electron imports

---

## 1. Mục tiêu

Platform Abstraction Layer giải quyết vấn đề **Electron coupling**:

```
TRƯỚC (v1.x):
  src/main/index.ts   → import 'electron' trực tiếp
  src/server/index.ts → require('electron') qua Module.prototype hack
  Tests               → không thể chạy không có Electron

SAU (v2.0):
  src/main/index.ts   → vẫn import 'electron' (Electron mode)
  src/server/index.ts → import '../platform' (NodeAdapter)
  electron module     → được redirect qua vite alias → electron-node-wrapper.ts
  Tests               → 100% Node.js vitest, không cần Electron
```

---

## 2. Interface Hierarchy

```
IPlatformServices
├── app: IApp
│     getPath(name): string
│     getVersion(): string
│     isPackaged: boolean
│     whenReady(): Promise<void>
│     quit(): void
│     exit(code): void
│     on/off/emit (EventEmitter)
│     relaunch(): void
│     setName(name): void
│     disableHardwareAcceleration(): void
│
├── windowManager: IWindowManager
│     createWindow(opts?): IWindow
│     getWindowById(id): IWindow | undefined
│     getAllWindows(): IWindow[]
│     getFocusedWindow(): IWindow | null
│     setMainWindow(win | null): void
│     getMainWindow(): IWindow | null
│
├── ipc: IIpcBridge
│     handle(channel, handler): void
│     removeHandler(channel): void
│     on(channel, listener): void
│     off(channel, listener): void
│     emit(channel, event, ...args): void
│     sendToWindow(windowId, channel, ...args): void
│     sendToAll(channel, ...args): void
│     invoke(channel, windowId, ...args): Promise<any>
│
├── storage: ISecureStorage
│     isEncryptionAvailable(): boolean
│     encryptString(plaintext): Buffer
│     decryptString(ciphertext): string
│
└── system: ISystemInfo
      platform: string
      arch: string
      version: string
      locale: string
```

---

## 3. Context Module (`src/platform/context.ts`)

```typescript
// Singleton pattern — init TRƯỚC khi load bất kỳ module nào khác
let _platform: IPlatformServices | null = null

export function setPlatform(services: IPlatformServices): void
// throws nếu gọi 2 lần

export function getPlatform(): IPlatformServices
// throws 'Platform not initialized. Call setPlatform() first.' nếu chưa init

export function isPlatformInitialized(): boolean
// pure predicate, không throw

export function _resetPlatformForTesting(): void
// chỉ dùng trong tests
```

**Thứ tự init bắt buộc trong `src/server/index.ts`:**
```typescript
// 1. Tạo adapter
const adapter = createNodeAdapter({ userDataPath })
// 2. Set platform TRƯỚC KHI import bất kỳ module nào dùng electron
setPlatform(adapter)
// 3. Dynamic import sau
const { initializeOrcaServices } = await import('../main/server-bootstrap')
```

---

## 4. NodeAdapter (`src/platform/adapters/node/`)

### 4.1 NodeApp

```typescript
export class NodeApp extends EventEmitter implements IApp {
  readonly isPackaged = true  // production mode

  constructor(options?: { userDataPath?: string })
  // options.userDataPath → getPath('userData')
  // fallback: ORCA_USER_DATA_PATH env → ~/.orca

  getPath(name: AppPathName): string
  // Supported: 'userData', 'home', 'temp', 'downloads', 'desktop', 'documents'

  getVersion(): string   // từ package.json version
  whenReady(): Promise<void>  // Promise.resolve() immediately
  quit(): void           // emit 'before-quit', 'will-quit', process.exit(0)
  exit(code): void       // process.exit(code)
  relaunch(): void       // log warning only
  setName(): void        // no-op
  disableHardwareAcceleration(): void  // no-op
}
```

### 4.2 NodeWindow + NodeWindowManager

```typescript
export class NodeWindow extends EventEmitter implements IWindow {
  readonly id: number
  isVisible(): boolean   // false (no GUI)
  isFocused(): boolean   // false
  destroy(): void        // idempotent, emit 'closed' once

  send(channel: string, ...args: any[]): void
  // Notify tất cả onSend subscribers

  onSend(channel: string, callback: (args: any[]) => void): () => void
  // Returns unsubscribe function
}

export class NodeWindowManager implements IWindowManager {
  private _nextId = 1    // auto-increment

  createWindow(): NodeWindow
  getWindowById(id): NodeWindow | undefined
  getAllWindows(): NodeWindow[]
  getFocusedWindow(): null  // no GUI
  setMainWindow(win | null): void
  getMainWindow(): NodeWindow | null
}
```

### 4.3 NodeIpcBridge

```typescript
export class NodeIpcBridge extends EventEmitter implements IIpcBridge {
  handle(channel, handler): void
  // log warning nếu overwrite handler

  removeHandler(channel): void

  invoke(channel, windowId, ...args): Promise<any>
  // Tạo IpcEvent với sender.id = windowId
  // throws Error (not string) nếu channel không có handler

  on/off(channel, listener): void
  emit(channel, event, ...args): void
  // không throw kể cả khi listener throw

  sendToWindow(windowId, channel, ...args): void
  // Route qua windowManager.getWindowById(windowId).send()

  sendToAll(channel, ...args): void
  // Iterate getAllWindows(), gọi send() trên từng window
  // Không throw nếu không có window nào
}
```

### 4.4 NodeSecureStorage

```typescript
export class NodeSecureStorage implements ISecureStorage {
  // Algorithm: AES-256-GCM (authenticated encryption)
  // Key file: userData/.crypto/storage.key (mode 0o600)
  // IV: 12 bytes random per encrypt → ciphertext khác nhau cho cùng plaintext

  isEncryptionAvailable(): boolean  // false nếu crypto init thất bại
  encryptString(plaintext: string): Buffer
  // format: [iv:12 bytes][authTag:16 bytes][ciphertext]
  decryptString(ciphertext: Buffer | string): string
  // graceful fallback: trả về '' nếu decode thất bại
}
```

### 4.5 NodeSystemInfo

```typescript
export class NodeSystemInfo implements ISystemInfo {
  readonly platform = process.platform  // 'linux', 'darwin', 'win32'
  readonly arch = process.arch          // 'x64', 'arm64'
  readonly version = process.version    // Node.js version
  readonly locale = Intl.DateTimeFormat().resolvedOptions().locale
}
```

### 4.6 Factory Function

```typescript
// src/platform/adapters/node/index.ts
export interface NodeAppOptions {
  userDataPath?: string
}

export function createNodeAdapter(options?: NodeAppOptions): IPlatformServices {
  const app = new NodeApp(options)
  const windowManager = new NodeWindowManager()
  const ipc = new NodeIpcBridge(windowManager)  // ipc biết về windowManager để sendToWindow
  const storage = new NodeSecureStorage(app.getPath('userData'))
  const system = new NodeSystemInfo()
  return { app, windowManager, ipc, storage, system }
}
```

---

## 5. WebIpcBridge (`src/platform/adapters/node/web-ipc-bridge.ts`)

Server-side component xử lý IPC qua WebSocket connection:

```typescript
// Message protocol (WebSocket → Server):
// Client → Server (invoke):  { id, type: 'invoke', channel, args }
// Client → Server (send):    { type: 'send', channel, args }
//
// Server → Client (result):  { id, type: 'result', result }
// Server → Client (error):   { id, type: 'error', message }
// Server → Client (push):    { type: 'push', channel, args }

type ReplyFn = (msg: string) => void

export class WebIpcBridge {
  constructor(ipc: IIpcBridge)

  async handleWebSocketMessage(raw: string, windowId: number, reply: ReplyFn): Promise<void>
  // Parse JSON → dispatch to ipc.invoke() or ipc.emit()
  // invoke: reply with result/error
  // send: fire-and-forget, no reply
  // Unknown type: silently ignore
  // Malformed JSON: reply with error

  pushToClients(channel: string, args: any[], broadcast: ReplyFn): void
  // Sends: { type: 'push', channel, args }
}
```

**Phân biệt với OrcaRuntimeRpcServer protocol:**
```typescript
// OrcaRuntimeRpcServer: { method: 'worktree.create', ... }  ← JSON-RPC style
// WebIpcBridge:         { type: 'invoke', channel, args }  ← IPC style

// Phân biệt trong ws-transport.ts:
if (msg.type === 'invoke' || msg.type === 'send') {
  await webIpcBridge.handleWebSocketMessage(raw, windowId, reply)
} else {
  rpcDispatcher.dispatch(msg)
}
```

---

## 6. Electron Stubs

### 6.1 `electron-node-wrapper.ts` (Server/Node mode)

```
src/platform/stubs/electron-node-wrapper.ts
```

Được dùng khi build với `vite.server.config.ts`:
```typescript
// vite.server.config.ts
resolve: {
  alias: {
    'electron': resolve(__dirname, 'src/platform/stubs/electron-node-wrapper.ts')
  }
}
```

Tất cả Electron exports được delegate đến `getPlatform()`:
```typescript
export const app = {
  getPath: (name) => getPlatform().app.getPath(name),
  getVersion: () => getPlatform().app.getVersion(),
  // ...
}

export const ipcMain = {
  handle: (ch, fn) => getPlatform().ipc.handle(ch, fn),
  // ...
}

export const safeStorage = {
  encryptString: (s) => getPlatform().storage.encryptString(s),
  decryptString: (b) => getPlatform().storage.decryptString(b),
  // ...
}
```

Graceful fallback khi platform chưa init (`tryGetPlatform()` trả về null → safe defaults).

### 6.2 `electron-web-stub.ts` (Browser/SPA mode)

```
src/platform/stubs/electron-web-stub.ts
```

Được dùng khi build với `vite.web-spa.config.ts`:
```typescript
// vite.web-spa.config.ts
resolve: {
  alias: {
    'electron': resolve(__dirname, 'src/platform/stubs/electron-web-stub.ts')
  }
}
```

Pure no-ops — cho phép browser code import electron mà không crash:
```typescript
export const app = {
  getPath: () => '',
  getVersion: () => '0.0.0',
  // ...
}
export const ipcMain = { handle: () => {}, on: () => {}, /* ... */ }
export const safeStorage = {
  isEncryptionAvailable: () => false,
  encryptString: () => Buffer.alloc(0),
  decryptString: () => '',
}
```

---

## 7. Test Strategy

```
Tests cần cover:
├── src/platform/__tests__/context.test.ts          → 6 tests
├── src/platform/__tests__/interface-conformance.ts → conformance helpers
├── src/platform/adapters/node/__tests__/
│   ├── app.test.ts            → 30 tests (IApp conformance)
│   ├── window.test.ts         → 29 tests (IWindow/IWindowManager)
│   ├── ipc.test.ts            → 18 tests (IIpcBridge)
│   ├── storage.test.ts        → 10 tests (AES-256-GCM, key persistence)
│   ├── system.test.ts         → 5 tests
│   ├── index.test.ts          → 17 tests (factory, wiring)
│   └── web-ipc-bridge.test.ts → 16 tests (invoke/send/push/error/malformed)
└── src/platform/adapters/web/__tests__/
    └── rpc-client.test.ts     → 15 tests

Total: 10 test files | 166 tests | 100% pass
```

**Nguyên tắc:**
- Không import `'electron'` trong bất kỳ test nào
- Dùng conformance helpers (`runIAppConformanceTests`) cho interface compliance
- Test storage với temp directory, cleanup sau mỗi test
- Test IpcBridge với mock windowManager

---

## 8. Thứ tự khởi tạo bắt buộc

```
Web Server Mode (src/server/index.ts):
  1. createNodeAdapter({ userDataPath })
  2. setPlatform(adapter)          ← TRƯỚC KHI import electron modules
  3. dynamic import server-bootstrap
  4. initializeOrcaServices()      ← init Store, PTY daemon, OrcaRuntimeService
  5. startHttpServer()             ← serve out/web/
  6. Signal handlers (SIGINT/SIGTERM → graceful shutdown)

Electron Mode (src/main/index.ts):
  1. app.whenReady()
  2. (ElectronAdapter implicitly available via native Electron)
  3. Store.init(), OrcaRuntimeService, RpcServer...
  4. createMainWindow()
```

---

## 9. Key Files Reference

| File | Role |
|------|------|
| `src/platform/types.ts` | `IPlatformServices`, `PlatformMode` |
| `src/platform/context.ts` | `setPlatform()`, `getPlatform()`, `isPlatformInitialized()` |
| `src/platform/index.ts` | Re-exports all platform types |
| `src/platform/app-interface.ts` | `IApp`, `AppPathName` |
| `src/platform/window-interface.ts` | `IWindow`, `IWindowManager`, `WindowCreationOptions` |
| `src/platform/ipc-interface.ts` | `IIpcBridge`, `IpcEvent`, `IpcHandler`, `IpcListener` |
| `src/platform/storage-interface.ts` | `ISecureStorage` |
| `src/platform/system-interface.ts` | `ISystemInfo` |
| `src/platform/adapters/node/app.ts` | `NodeApp extends EventEmitter implements IApp` |
| `src/platform/adapters/node/window.ts` | `NodeWindow`, `NodeWindowManager` |
| `src/platform/adapters/node/ipc.ts` | `NodeIpcBridge extends EventEmitter implements IIpcBridge` |
| `src/platform/adapters/node/storage.ts` | `NodeSecureStorage` (AES-256-GCM) |
| `src/platform/adapters/node/system.ts` | `NodeSystemInfo` |
| `src/platform/adapters/node/index.ts` | `createNodeAdapter()` factory |
| `src/platform/adapters/node/web-ipc-bridge.ts` | `WebIpcBridge` |
| `src/platform/adapters/web/rpc-client.ts` | Browser-side RPC |
| `src/platform/stubs/electron-node-wrapper.ts` | Electron API → NodeAdapter delegate |
| `src/platform/stubs/electron-web-stub.ts` | Electron API → no-op (browser) |
