# CR-002 — Node.js Adapter Implementation

**Status:** Proposed  
**Priority:** 🔴 Critical  
**Depends on:** CR-001  
**Blocks:** CR-005, CR-007

---

## Mục tiêu

Implement **NodeAdapter** — hiện thực hóa toàn bộ `IPlatformServices` bằng các primitive Node.js thuần, không có bất kỳ phụ thuộc nào vào `electron` module. Đây là replacement chính thức cho `src/main/mocks/electron.ts`.

---

## Bối cảnh & Vấn đề

`src/main/mocks/electron.ts` hiện tại có các vấn đề:
1. **Phân tán và thiếu tổ chức** — class/object mock lẫn lộn không có hierarchy rõ ràng
2. **Type unsafe** — nhiều `any`, `as any` casts
3. **Không có events thực sự** — EventEmitter được inherit nhưng không gửi events hữu ích
4. **Duplicate members** — do được patch thêm nhiều lần (isMinimized, isVisible, v.v.)
5. **Không testable độc lập** — phụ thuộc vào cách Vite alias `'electron'`

---

## Giải pháp Đề xuất

### 1. Tạo `src/platform/adapters/node/`

```
src/platform/adapters/
├── electron/
│   └── index.ts          # ElectronAdapter (CR tiếp theo, để sau)
└── node/
    ├── index.ts           # NodeAdapter — implements IPlatformServices
    ├── app.ts             # NodeApp implements IApp
    ├── window.ts          # NodeWindow, NodeWindowManager
    ├── ipc.ts             # NodeIpcBridge — WebSocket/in-process bridge
    ├── storage.ts         # NodeSecureStorage — file-based fallback
    ├── system.ts          # NodeSystemInfo
    ├── notification.ts    # NodeNotification — log-only
    └── __tests__/
        ├── app.test.ts
        ├── ipc.test.ts
        └── window.test.ts
```

### 2. NodeApp Implementation

```typescript
// src/platform/adapters/node/app.ts
import { EventEmitter } from 'node:events'
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { homedir, tmpdir } from 'node:os'
import type { IApp, AppPathName, AppEvent } from '../../app-interface'

export class NodeApp extends EventEmitter implements IApp {
  readonly isPackaged: boolean = true
  
  private readonly userDataPath: string
  
  constructor(options: { userDataPath?: string } = {}) {
    super()
    this.userDataPath = options.userDataPath 
      ?? process.env.ORCA_USER_DATA_PATH 
      ?? join(homedir(), '.orca')
    // Ensure userData directory exists
    mkdirSync(this.userDataPath, { recursive: true })
  }
  
  getVersion(): string {
    return process.env.ORCA_VERSION ?? '0.0.0'
  }
  
  getPath(name: AppPathName): string {
    const home = homedir()
    switch (name) {
      case 'userData':   return this.userDataPath
      case 'appData':    return join(home, '.config')
      case 'home':       return home
      case 'temp':       return tmpdir()
      case 'exe':        return process.execPath
      case 'module':     return __dirname
      case 'desktop':    return join(home, 'Desktop')
      case 'documents':  return join(home, 'Documents')
      case 'downloads':  return join(home, 'Downloads')
      case 'music':      return join(home, 'Music')
      case 'pictures':   return join(home, 'Pictures')
      case 'videos':     return join(home, 'Videos')
      default:           return join(this.userDataPath, name)
    }
  }
  
  getAppPath(): string {
    // Resolve to the directory containing package.json
    return process.env.ORCA_APP_PATH ?? __dirname
  }
  
  async whenReady(): Promise<void> {
    // In Node mode, we're always "ready"
    return Promise.resolve()
  }
  
  quit(): void {
    this.emit('before-quit')
    this.emit('will-quit')
    process.exit(0)
  }
  
  exit(code = 0): void {
    process.exit(code)
  }

  // Unused in Node mode, but required by interface
  setName(_name: string): void {}
  disableHardwareAcceleration(): void {}
  relaunch(): void { 
    console.warn('[NodeApp] relaunch() is a no-op in Node mode') 
  }
}
```

### 3. NodeWindowManager & NodeWindow

```typescript
// src/platform/adapters/node/window.ts
import { EventEmitter } from 'node:events'
import type { IWindow, IWindowManager, WindowCreationOptions } from '../../window-interface'

// In-process event bus to simulate window→renderer messaging
export class NodeWindow extends EventEmitter implements IWindow {
  readonly id: number
  private _destroyed = false
  
  // Subscribers: maps channel → Set of listeners
  private readonly sendSubscribers = new Map<string, Set<(args: any[]) => void>>()
  
  constructor(id: number) {
    super()
    this.id = id
  }
  
  // IWindow state queries
  isDestroyed(): boolean { return this._destroyed }
  isMinimized(): boolean { return false }
  isMaximized(): boolean { return false }
  isFullScreen(): boolean { return false }
  isVisible(): boolean { return true }
  isFocused(): boolean { return true }
  
  // IWindow actions
  show(): void {}
  hide(): void {}
  focus(): void {}
  restore(): void {}
  
  close(): void { this.destroy() }
  
  destroy(): void {
    if (this._destroyed) return
    this._destroyed = true
    this.emit('closed')
    this.removeAllListeners()
  }
  
  // Communication — broadcast to WebSocket clients subscribed to this window
  send(channel: string, ...args: any[]): void {
    const subs = this.sendSubscribers.get(channel)
    if (subs) {
      for (const cb of subs) cb(args)
    }
  }
  
  // Allow WebSocket transport to subscribe to window messages
  onSend(channel: string, callback: (args: any[]) => void): () => void {
    let set = this.sendSubscribers.get(channel)
    if (!set) {
      set = new Set()
      this.sendSubscribers.set(channel, set)
    }
    set.add(callback)
    return () => set!.delete(callback)
  }
}

export class NodeWindowManager implements IWindowManager {
  private windows = new Map<number, NodeWindow>()
  private mainWindow: NodeWindow | null = null
  private nextId = 1
  
  createWindow(_options: WindowCreationOptions): IWindow {
    const win = new NodeWindow(this.nextId++)
    this.windows.set(win.id, win)
    win.once('closed', () => this.windows.delete(win.id))
    return win
  }
  
  getAllWindows(): IWindow[] {
    return [...this.windows.values()]
  }
  
  getFocusedWindow(): IWindow | null {
    // In Node mode, return main window as "focused"
    return this.mainWindow
  }
  
  getMainWindow(): IWindow | null {
    return this.mainWindow
  }
  
  setMainWindow(window: IWindow | null): void {
    this.mainWindow = window as NodeWindow | null
  }
}
```

### 4. NodeIpcBridge

```typescript
// src/platform/adapters/node/ipc.ts
import { EventEmitter } from 'node:events'
import type { IIpcBridge, IpcHandler, IpcListener, IpcEvent } from '../../ipc-interface'
import type { IWindowManager } from '../../window-interface'

/**
 * NodeIpcBridge — replaces Electron IPC in Node mode.
 * 
 * Strategy:
 * - ipcMain.handle(channel, handler) → registers handler in memory
 * - When a WebSocket RPC call arrives, it's dispatched here
 * - ipcMain.on(channel, listener) → used for fire-and-forget events
 * - window.webContents.send(channel, ...) → pushes to WS clients
 */
export class NodeIpcBridge extends EventEmitter implements IIpcBridge {
  private readonly handlers = new Map<string, IpcHandler>()
  private readonly listeners = new Map<string, Set<IpcListener>>()
  private readonly windowManager: IWindowManager
  
  constructor(windowManager: IWindowManager) {
    super()
    this.windowManager = windowManager
  }
  
  // === Registration ===
  
  handle(channel: string, listener: IpcHandler): void {
    if (this.handlers.has(channel)) {
      console.warn(`[NodeIpcBridge] Overwriting handler for channel: ${channel}`)
    }
    this.handlers.set(channel, listener)
  }
  
  removeHandler(channel: string): void {
    this.handlers.delete(channel)
  }
  
  on(channel: string, listener: IpcListener): this {
    let set = this.listeners.get(channel)
    if (!set) {
      set = new Set()
      this.listeners.set(channel, set)
    }
    set.add(listener)
    return this
  }
  
  off(channel: string, listener: IpcListener): this {
    this.listeners.get(channel)?.delete(listener)
    return this
  }
  
  // === Invocation (called by WebSocket transport) ===
  
  async invoke(channel: string, senderWindowId: number, ...args: any[]): Promise<any> {
    const handler = this.handlers.get(channel)
    if (!handler) {
      throw new Error(`No IPC handler registered for channel: ${channel}`)
    }
    
    const event: IpcEvent = {
      sender: {
        id: senderWindowId,
        send: (replyChannel, ...replyArgs) => {
          this.sendToWindow(senderWindowId, replyChannel, ...replyArgs)
        }
      }
    }
    
    return handler(event, ...args)
  }
  
  emit(channel: string, event: IpcEvent, ...args: any[]): boolean {
    const set = this.listeners.get(channel)
    if (set) {
      for (const listener of set) {
        listener(event, ...args)
      }
    }
    return super.emit(channel, event, ...args)
  }
  
  // === Push notifications ===
  
  sendToWindow(windowId: number, channel: string, ...args: any[]): void {
    const windows = this.windowManager.getAllWindows()
    const win = windows.find(w => w.id === windowId)
    win?.send(channel, ...args)
  }
  
  sendToAll(channel: string, ...args: any[]): void {
    for (const win of this.windowManager.getAllWindows()) {
      win.send(channel, ...args)
    }
  }
}
```

### 5. NodeAdapter Factory

```typescript
// src/platform/adapters/node/index.ts
import { NodeApp } from './app'
import { NodeWindowManager } from './window'
import { NodeIpcBridge } from './ipc'
import { NodeSecureStorage } from './storage'
import { NodeSystemInfo } from './system'
import type { IPlatformServices } from '../../types'

export interface NodeAdapterOptions {
  userDataPath?: string
}

export function createNodeAdapter(options: NodeAdapterOptions = {}): IPlatformServices {
  const app = new NodeApp({ userDataPath: options.userDataPath })
  const windowManager = new NodeWindowManager()
  const ipc = new NodeIpcBridge(windowManager)
  const storage = new NodeSecureStorage(app)
  const system = new NodeSystemInfo()
  
  return {
    mode: 'node',
    app,
    ipc,
    windowManager,
    storage,
    system
  }
}
```

### 6. NodeSecureStorage

```typescript
// src/platform/adapters/node/storage.ts
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'
import type { ISecureStorage } from '../../storage-interface'
import type { IApp } from '../../app-interface'

/**
 * File-based encrypted storage using AES-256-GCM.
 * Falls back to base64 encoding if crypto setup fails.
 * 
 * NOTE: This is NOT as secure as Electron's safeStorage (OS keychain).
 * For production, consider integrating with OS keychain via `keytar`.
 * The interface allows swapping implementations without code changes.
 */
export class NodeSecureStorage implements ISecureStorage {
  private key: Buffer | null = null
  private readonly keyPath: string
  
  constructor(app: IApp) {
    const dataDir = join(app.getPath('userData'), '.crypto')
    mkdirSync(dataDir, { recursive: true, mode: 0o700 })
    this.keyPath = join(dataDir, 'storage.key')
    this.initKey()
  }
  
  isEncryptionAvailable(): boolean {
    return this.key !== null
  }
  
  encryptString(plaintext: string): Buffer {
    if (!this.key) {
      return Buffer.from(plaintext, 'utf-8')
    }
    const iv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', this.key, iv)
    const encrypted = Buffer.concat([cipher.update(plaintext, 'utf-8'), cipher.final()])
    const tag = cipher.getAuthTag()
    return Buffer.concat([iv, tag, encrypted])
  }
  
  decryptString(data: Buffer): string {
    if (!this.key) {
      return data.toString('utf-8')
    }
    try {
      const iv = data.subarray(0, 16)
      const tag = data.subarray(16, 32)
      const encrypted = data.subarray(32)
      const decipher = createDecipheriv('aes-256-gcm', this.key, iv)
      decipher.setAuthTag(tag)
      return decipher.update(encrypted).toString('utf-8') + decipher.final('utf-8')
    } catch {
      // Fallback: treat as plain text (backward compat with unencrypted data)
      return data.toString('utf-8')
    }
  }
  
  private initKey(): void {
    try {
      let rawKey: Buffer
      try {
        rawKey = readFileSync(this.keyPath)
      } catch {
        rawKey = randomBytes(64)
        writeFileSync(this.keyPath, rawKey, { mode: 0o600 })
      }
      this.key = scryptSync(rawKey, 'orca-storage-v1', 32)
    } catch (err) {
      console.warn('[NodeSecureStorage] Could not initialize encryption key:', err)
      this.key = null
    }
  }
}
```

---

## Thay thế `src/main/mocks/electron.ts`

Sau khi CR-002 hoàn thành, `src/server/index.ts` sẽ sử dụng:

```typescript
// src/server/index.ts (updated)
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'

const adapter = createNodeAdapter({
  userDataPath: process.env.ORCA_USER_DATA_PATH
})
setPlatform(adapter)

// Thay vì: import electronMock from '../main/mocks/electron'
// Module resolution vẫn dùng Vite alias nhưng target là adapter thực
```

Vite config sẽ alias `'electron'` → wrapper mỏng sử dụng `getPlatform()` thay vì mock object.

---

## Phạm vi thay đổi

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] src/platform/adapters/node/index.ts` | Factory function |
| `[NEW] src/platform/adapters/node/app.ts` | NodeApp |
| `[NEW] src/platform/adapters/node/window.ts` | NodeWindow, NodeWindowManager |
| `[NEW] src/platform/adapters/node/ipc.ts` | NodeIpcBridge |
| `[NEW] src/platform/adapters/node/storage.ts` | NodeSecureStorage |
| `[NEW] src/platform/adapters/node/system.ts` | NodeSystemInfo |
| `[NEW] src/platform/adapters/node/__tests__/` | Unit tests |

### Files sửa đổi
| File | Thay đổi |
|------|---------|
| `[MODIFY] src/server/index.ts` | Dùng `createNodeAdapter()` thay vì mock |
| `[MODIFY] vite.server.config.ts` | Cập nhật alias `electron` → adapter wrapper |

### Files không thay đổi
- `src/main/mocks/electron.ts` — Giữ lại (có thể deprecate ở CR-007)
- `src/main/**` — **KHÔNG sửa**

---

## Unit Tests yêu cầu

```typescript
// src/platform/adapters/node/__tests__/app.test.ts
describe('NodeApp', () => {
  it('should return userData path from env var', ...)
  it('should create userData directory if not exists', ...)
  it('should emit events correctly', ...)
  it('should call process.exit on quit()', ...)
})

// src/platform/adapters/node/__tests__/ipc.test.ts
describe('NodeIpcBridge', () => {
  it('should invoke registered handler', ...)
  it('should throw for unregistered channel', ...)
  it('should call removeHandler correctly', ...)
  it('should relay sendToWindow to correct window', ...)
})
```

---

## Rủi ro & Biện pháp

| Rủi ro | Biện pháp |
|--------|-----------|
| NodeSecureStorage kém bảo mật hơn OS keychain | Document rõ, cung cấp hook để swap với keytar |
| NodeWindow không có real DOM events | Dùng EventEmitter, đủ cho backend event wiring |
| ipc.handle duplicate handlers | Log warning, cho phép overwrite (same behavior as Electron) |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

Node.js adapters implemented in `src/platform/adapters/node/`:

| File | Status |
|------|--------|
| `src/platform/adapters/node/app.ts` | ✅ Done — `NodeApp` |
| `src/platform/adapters/node/window.ts` | ✅ Done — `NodeWindowManager`, `NodeWindow` |
| `src/platform/adapters/node/ipc.ts` | ✅ Done — `NodeIpcBridge` |
| `src/platform/adapters/node/storage.ts` | ✅ Done — `NodeSecureStorage` |
| `src/platform/adapters/node/system.ts` | ✅ Done — `NodeSystemInfo` |
| `src/platform/adapters/node/web-ipc-bridge.ts` | ✅ Done — WebIpcBridge protocol |
| `src/platform/adapters/node/index.ts` | ✅ Done — factory |
