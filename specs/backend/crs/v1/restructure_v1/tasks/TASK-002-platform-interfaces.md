# TASK-002: Tạo 5 interface files trong `src/platform/`

**Source:** SOL-BE-001  
**Phase:** 1 | **Effort:** S (30–60 min)  
**Depends on:** TASK-001

---

## Objective

Tạo 5 interface files định nghĩa toàn bộ contract của platform layer:
- `app-interface.ts` — IApp
- `window-interface.ts` — IWindow, IWindowManager
- `ipc-interface.ts` — IIpcBridge
- `storage-interface.ts` — ISecureStorage
- `system-interface.ts` — ISystemInfo

Và file re-export `index.ts`.

---

## Files to create

### 1. `src/platform/app-interface.ts`

```typescript
/**
 * IApp — abstraction over Electron's `app` module.
 * NodeAdapter implements this without any Electron dependency.
 */
export interface IApp {
  /** App version from package.json or ORCA_VERSION env */
  getVersion(): string

  /** Get well-known paths (userData, home, temp, etc.) */
  getPath(name: AppPathName): string

  /** Full path to the app installation directory */
  getAppPath(): string

  /** True when running as packaged app (not dev mode) */
  readonly isPackaged: boolean

  /** Resolves immediately in Node mode */
  whenReady(): Promise<void>

  /** Graceful shutdown */
  quit(): void

  /** Hard exit with code */
  exit(code?: number): void

  /** Restart app (no-op in Node mode) */
  relaunch(): void

  /** Set app display name (no-op in Node mode) */
  setName(name: string): void

  /** Disable GPU acceleration (no-op in Node mode) */
  disableHardwareAcceleration(): void

  on(event: AppEvent, listener: (...args: any[]) => void): this
  off(event: AppEvent, listener: (...args: any[]) => void): this
  once(event: AppEvent, listener: (...args: any[]) => void): this
  emit(event: AppEvent, ...args: any[]): boolean
}

export type AppPathName =
  | 'userData' | 'appData' | 'home' | 'temp'
  | 'exe' | 'module' | 'desktop' | 'documents'
  | 'downloads' | 'music' | 'pictures' | 'videos'
  | string  // fallback

export type AppEvent =
  | 'ready' | 'quit' | 'before-quit' | 'will-quit'
  | 'activate' | 'open-url' | 'second-instance'
  | string
```

### 2. `src/platform/window-interface.ts`

```typescript
/** IWindow — abstraction over Electron's BrowserWindow */
export interface IWindow {
  readonly id: number

  isDestroyed(): boolean
  isMinimized(): boolean
  isMaximized(): boolean
  isFullScreen(): boolean
  isVisible(): boolean
  isFocused(): boolean

  show(): void
  hide(): void
  focus(): void
  restore(): void
  close(): void
  destroy(): void

  /** Send a message to the window's renderer/WebSocket clients */
  send(channel: string, ...args: any[]): void

  on(event: WindowEvent, listener: (...args: any[]) => void): this
  once(event: WindowEvent, listener: (...args: any[]) => void): this
  off(event: WindowEvent, listener: (...args: any[]) => void): this
}

export type WindowEvent =
  | 'closed' | 'close' | 'ready-to-show' | 'focus'
  | 'blur' | 'minimize' | 'maximize' | 'restore'
  | string

export interface WindowCreationOptions {
  width?: number
  height?: number
  minWidth?: number
  minHeight?: number
  show?: boolean
  frame?: boolean
  transparent?: boolean
  titleBarStyle?: string
  [key: string]: any  // passthrough for Electron-specific options
}

/** IWindowManager — factory and registry for windows */
export interface IWindowManager {
  createWindow(options: WindowCreationOptions): IWindow
  getAllWindows(): IWindow[]
  getFocusedWindow(): IWindow | null
  getMainWindow(): IWindow | null
  setMainWindow(window: IWindow | null): void
}
```

### 3. `src/platform/ipc-interface.ts`

```typescript
/**
 * IIpcBridge — abstraction over Electron's ipcMain.
 *
 * NodeAdapter provides an in-process implementation that
 * dispatches via WebSocket (via WebIpcBridge in SOL-BE-003).
 */
export interface IIpcBridge {
  /** Register an async handler for a channel (like ipcMain.handle) */
  handle(channel: string, listener: IpcHandler): void

  /** Remove a registered handler */
  removeHandler(channel: string): void

  /** Subscribe to fire-and-forget events (like ipcMain.on) */
  on(channel: string, listener: IpcListener): this

  /** Unsubscribe */
  off(channel: string, listener: IpcListener): this

  /** Push a message to a specific window/client */
  sendToWindow(windowId: number, channel: string, ...args: any[]): void

  /** Broadcast to all connected windows/clients */
  sendToAll(channel: string, ...args: any[]): void
}

export type IpcHandler = (event: IpcEvent, ...args: any[]) => Promise<any> | any

export type IpcListener = (event: IpcEvent, ...args: any[]) => void

export interface IpcEvent {
  readonly sender: {
    readonly id: number
    send(channel: string, ...args: any[]): void
  }
}
```

### 4. `src/platform/storage-interface.ts`

```typescript
/**
 * ISecureStorage — abstraction over Electron's safeStorage.
 *
 * NodeAdapter uses AES-256-GCM with a file-based key.
 * ElectronAdapter delegates to Electron's OS keychain integration.
 */
export interface ISecureStorage {
  /** True if encryption is available (false = will store as plain bytes) */
  isEncryptionAvailable(): boolean

  /** Encrypt a plaintext string → Buffer */
  encryptString(plaintext: string): Buffer

  /** Decrypt a Buffer → plaintext string */
  decryptString(encrypted: Buffer): string
}
```

### 5. `src/platform/system-interface.ts`

```typescript
/** ISystemInfo — platform/OS information queries */
export interface ISystemInfo {
  /** Host OS platform */
  getPlatform(): NodeJS.Platform

  /** Total system memory in bytes */
  getTotalMemory(): number

  /** Free system memory in bytes */
  getFreeMemory(): number

  /** Number of CPU cores */
  getCpuCount(): number

  /** OS hostname */
  getHostname(): string
}
```

### 6. `src/platform/index.ts`

```typescript
/**
 * Platform Abstraction Layer — Public API
 *
 * Import platform types and context from this entry point.
 * Do NOT import directly from sub-files in application code.
 */

export type { PlatformMode, IPlatformServices } from './types'
export type { IApp, AppPathName, AppEvent } from './app-interface'
export type { IWindow, IWindowManager, WindowCreationOptions, WindowEvent } from './window-interface'
export type { IIpcBridge, IpcHandler, IpcListener, IpcEvent } from './ipc-interface'
export type { ISecureStorage } from './storage-interface'
export type { ISystemInfo } from './system-interface'
export { setPlatform, getPlatform, isPlatformInitialized } from './context'
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile — no errors in platform/
npx tsc --noEmit 2>&1 | grep "platform/" | head -20

# Verify no electron imports
grep -r "from 'electron'" src/platform/
# Expected: empty output

# Verify no src/main imports
grep -r "from.*src/main" src/platform/
# Expected: empty output
```

---

## Done criteria

- [x] 6 files tạo thành công: `index.ts`, `types.ts`, `context.ts`, `app-interface.ts`, `window-interface.ts`, `ipc-interface.ts`, `storage-interface.ts`, `system-interface.ts`
- [x] Không có `import` từ `'electron'` trong bất kỳ file nào
- [x] `import { setPlatform, IApp } from 'src/platform'` hoạt động
- [x] TypeScript strict mode: không có `any` không cần thiết
