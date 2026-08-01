# CR-001 — Platform Interface & Adapter Layer

**Status:** ✅ Implemented  
**Priority:** 🔴 Critical  
**Depends on:** —  
**Blocks:** CR-002, CR-003

---

## Mục tiêu

Định nghĩa một **PlatformInterface** — lớp trừu tượng mỏng giữa business logic của Orca và Electron runtime. Đây là nền tảng để toàn bộ các CR khác có thể thực thi mà **không sửa code Electron gốc**.

---

## Bối cảnh & Vấn đề

Hiện tại, `src/main/index.ts` và các service trong `src/main/` import trực tiếp từ `electron`:
```ts
import { app, BrowserWindow, ipcMain, nativeTheme } from 'electron'
```

Điều này tạo ra **hard coupling**: không có cách nào chạy backend services mà không có Electron runtime (hoặc mock toàn bộ API surface).

Giải pháp hiện tại (`src/main/mocks/electron.ts`) là một **emergency workaround** — nó mock toàn bộ Electron module, nhưng:
- Phải liên tục cập nhật khi Electron API mới được dùng
- Không type-safe (dùng `any` nhiều chỗ)
- Không phân biệt được API nào thực sự cần vs không cần trong Node mode

---

## Giải pháp Đề xuất

### 1. Tạo `src/platform/` directory

```
src/platform/
├── index.ts              # Re-export tất cả interfaces
├── types.ts              # Core platform types
├── app-interface.ts      # IApp interface
├── window-interface.ts   # IWindow interface
├── ipc-interface.ts      # IIpcBridge interface
├── storage-interface.ts  # ISecureStorage interface
├── system-interface.ts   # ISystemInfo interface
└── notification-interface.ts  # INotification interface
```

### 2. Định nghĩa Core Interfaces

#### `src/platform/app-interface.ts`
```typescript
export interface IApp {
  // Lifecycle
  getVersion(): string
  getPath(name: AppPathName): string
  getAppPath(): string
  isPackaged: boolean
  whenReady(): Promise<void>
  quit(): void
  exit(code?: number): void
  
  // Event subscription (subset cần thiết)
  on(event: AppEvent, listener: (...args: any[]) => void): this
  off(event: AppEvent, listener: (...args: any[]) => void): this
  once(event: AppEvent, listener: (...args: any[]) => void): this
}

export type AppPathName = 
  | 'userData' | 'appData' | 'home' | 'temp' 
  | 'exe' | 'module' | 'desktop' | 'documents' 
  | 'downloads' | 'music' | 'pictures' | 'videos'

export type AppEvent = 
  | 'ready' | 'quit' | 'before-quit' | 'will-quit'
  | 'activate' | 'open-url' | 'second-instance'
```

#### `src/platform/window-interface.ts`
```typescript
export interface IWindow {
  readonly id: number
  
  // State queries
  isDestroyed(): boolean
  isMinimized(): boolean
  isMaximized(): boolean
  isFullScreen(): boolean
  isVisible(): boolean
  isFocused(): boolean
  
  // Actions
  show(): void
  hide(): void
  focus(): void
  restore(): void
  close(): void
  destroy(): void
  
  // Communication
  send(channel: string, ...args: any[]): void
  
  // Events
  on(event: WindowEvent, listener: (...args: any[]) => void): this
  once(event: WindowEvent, listener: (...args: any[]) => void): this
  off(event: WindowEvent, listener: (...args: any[]) => void): this
}

export type WindowEvent = 
  | 'closed' | 'close' | 'ready-to-show' | 'focus' 
  | 'blur' | 'minimize' | 'maximize' | 'restore'

export interface IWindowManager {
  // Factory
  createWindow(options: WindowCreationOptions): IWindow
  // Queries
  getAllWindows(): IWindow[]
  getFocusedWindow(): IWindow | null
  // Main window lifecycle
  getMainWindow(): IWindow | null
  setMainWindow(window: IWindow | null): void
}
```

#### `src/platform/ipc-interface.ts`
```typescript
export interface IIpcBridge {
  // Server-side registration
  handle(channel: string, listener: IpcHandler): void
  removeHandler(channel: string): void
  on(channel: string, listener: IpcListener): void
  off(channel: string, listener: IpcListener): void
  
  // Push notifications to frontend
  sendToWindow(windowId: number, channel: string, ...args: any[]): void
  sendToAll(channel: string, ...args: any[]): void
}

export type IpcHandler = (event: IpcEvent, ...args: any[]) => Promise<any> | any
export type IpcListener = (event: IpcEvent, ...args: any[]) => void

export interface IpcEvent {
  sender: {
    id: number
    send(channel: string, ...args: any[]): void
  }
}
```

#### `src/platform/storage-interface.ts`
```typescript
export interface ISecureStorage {
  isEncryptionAvailable(): boolean
  encryptString(plaintext: string): Buffer
  decryptString(encrypted: Buffer): string
}
```

#### `src/platform/types.ts`
```typescript
// Platform type discriminator
export type PlatformMode = 'electron' | 'node'

// Container holding all platform services
export interface IPlatformServices {
  mode: PlatformMode
  app: IApp
  ipc: IIpcBridge
  windowManager: IWindowManager
  storage: ISecureStorage
  system: ISystemInfo
}
```

### 3. Platform Context (Singleton)

```typescript
// src/platform/context.ts
import type { IPlatformServices } from './types'

let _platform: IPlatformServices | null = null

export function setPlatform(services: IPlatformServices): void {
  if (_platform) {
    throw new Error('Platform already initialized')
  }
  _platform = services
}

export function getPlatform(): IPlatformServices {
  if (!_platform) {
    throw new Error('Platform not initialized. Call setPlatform() first.')
  }
  return _platform
}

export function isPlatformInitialized(): boolean {
  return _platform !== null
}
```

---

## Phạm vi thay đổi

### Files mới (tất cả trong `src/platform/`)
| File | Mô tả |
|------|-------|
| `[NEW] src/platform/index.ts` | Re-export tất cả |
| `[NEW] src/platform/types.ts` | Core types |
| `[NEW] src/platform/app-interface.ts` | IApp interface |
| `[NEW] src/platform/window-interface.ts` | IWindow, IWindowManager interfaces |
| `[NEW] src/platform/ipc-interface.ts` | IIpcBridge interface |
| `[NEW] src/platform/storage-interface.ts` | ISecureStorage interface |
| `[NEW] src/platform/system-interface.ts` | ISystemInfo interface |
| `[NEW] src/platform/notification-interface.ts` | INotification interface |
| `[NEW] src/platform/context.ts` | Singleton context |

### Files không thay đổi
- `src/main/` — **KHÔNG sửa bất kỳ file nào**
- `src/renderer/` — **KHÔNG sửa**
- `src/shared/` — **KHÔNG sửa**

---

## Verification

```bash
# TypeScript compilation phải pass
pnpm tsc --noEmit -p tsconfig.json

# Không có Electron import trong src/platform/
grep -r "from 'electron'" src/platform/ # kết quả phải rỗng
```

---

## Rủi ro & Biện pháp giảm thiểu

| Rủi ro | Biện pháp |
|--------|-----------|
| Interface thiếu method → bị phát hiện muộn | Viết adapter (CR-002) ngay sau, test sẽ catch |
| Interface quá rộng → khó implement NodeAdapter | Bắt đầu với minimum viable subset, mở rộng dần |
| Type conflict với Electron types | Dùng distinct type names, không re-use Electron types |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

All platform interfaces have been defined in `src/platform/`:

| File | Status |
|------|--------|
| `src/platform/index.ts` | ✅ Done |
| `src/platform/types.ts` | ✅ Done |
| `src/platform/app-interface.ts` | ✅ Done |
| `src/platform/ipc-interface.ts` | ✅ Done |
| `src/platform/storage-interface.ts` | ✅ Done |
| `src/platform/system-interface.ts` | ✅ Done |
| `src/platform/context.ts` | ✅ Done — `getPlatform()`/`setPlatform()` singleton |
| `src/platform/rpc-client-interface.ts` | ✅ Done — `IRpcClient` |
