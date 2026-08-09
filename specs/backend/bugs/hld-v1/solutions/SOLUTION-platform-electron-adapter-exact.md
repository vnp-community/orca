# SOLUTION: BUG-BE-HLD-017 — `ElectronAdapter` không tồn tại; Platform Abstraction Layer bất đối xứng

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:** `backend/src/platform/types.ts`, `backend/src/platform/app-interface.ts`, `backend/src/platform/ipc-interface.ts`, `backend/src/platform/window-interface.ts`, `backend/src/platform/storage-interface.ts`, `backend/src/platform/system-interface.ts`, `backend/src/platform/context.ts`, toàn bộ `backend/src/platform/adapters/node/*.ts`, `desktop/src/main/index.ts`, `backend/src/platform/stubs/electron-node-wrapper.ts`, `desktop/src/platform/*` (bản sao song song), `docs/hld/backend-server-architecture.md`, `docs/hld/v1/C3-components.md`

---

## 1. Tóm tắt bug

`docs/hld/backend-server-architecture.md §3` và `docs/hld/v1/C3-components.md §C3.6` mô tả `IPlatformServices` được implement bởi **hai** adapter cùng cấp — `ElectronAdapter` (desktop) và `NodeAdapter` (server) — để business logic swap platform mà không đổi code. Thực tế chỉ `NodeAdapter` tồn tại; `ElectronAdapter` chưa từng được viết. `desktop/src/main/index.ts` import thẳng package `electron` thật, không đi qua interface trừu tượng nào cả.

Ticket gốc (`BUG-BE-HLD-017-electron-adapter-missing-platform-abstraction-asymmetric.md` §"Đề xuất fix") đề xuất nguyên văn **2 lựa chọn**, trích lại đầy đủ ở đây:

> 1. **Trước tiên: làm rõ phạm vi thật của thiết kế này** — có khả năng ý định ban đầu chỉ áp cho các module v5.0+ mới (Profile/Project/AI Provider/Workflow/Task, vốn có 0 hit `electron` trực tiếp), không phải toàn bộ `desktop/src/main`. Nếu đúng vậy, sửa lại câu chữ tài liệu cho rõ ràng — đây là fix rẻ nhất.
> 2. Nếu ý định là bao phủ toàn bộ, đây là hạng mục lớn (cần audit số lượng file `desktop/src/main` import electron trực tiếp) — cần roadmap riêng, ưu tiên thấp hơn các bug bảo mật khác trong `hld-v1/`.

Ticket liên quan `BUG-FE-HLD-005` (góc nhìn frontend, cùng phát hiện) đã được xử lý theo **Lựa chọn 1** — kết luận đây là câu chữ tài liệu bị phát biểu quá rộng, chỉ áp dụng cho 5 module v5.0 mới, và **không sửa 72 file** `src/main` hiện có (xem `specs/frontend/bugs/hld-v1/solutions/SOLUTION-FE-HLD-005-iplatformservices-scope.md`). Tài liệu này (bug BE) trình bày cả hai lựa chọn: câu hỏi cần hỏi trước khi quyết định (mục 3), và skeleton code đầy đủ nếu tổ chức chọn Lựa chọn 2 — build `ElectronAdapter` thật (mục 4).

---

## 2. Bằng chứng từ code thật

### 2.1 `IPlatformServices` — interface cần implement

**File:** `backend/src/platform/types.ts` — **Lines:** 20–27

```typescript
export interface IPlatformServices {
  readonly mode: PlatformMode
  readonly app: IApp
  readonly ipc: IIpcBridge
  readonly windowManager: IWindowManager
  readonly storage: ISecureStorage
  readonly system: ISystemInfo
}
```

Comment aspirational xác nhận ý định thiết kế ban đầu — **File:** `backend/src/platform/types.ts` — **Line:** 5:

```typescript
 * Implementations: ElectronAdapter (desktop) and NodeAdapter (server).
```

`PlatformMode` (cùng file, dùng làm discriminator) — **Line:** 17: `export type PlatformMode = 'electron' | 'node'`. NodeAdapter set `mode: 'node'` (xem 2.2.6) nhưng không có adapter nào set `mode: 'electron'`.

### 2.2 Từng field — interface đầy đủ + NodeAdapter đối chiếu

#### 2.2.1 `app: IApp`

**File:** `backend/src/platform/app-interface.ts` — **Lines:** 5–40

```typescript
export interface IApp {
  getVersion(): string
  getPath(name: AppPathName): string
  getAppPath(): string
  readonly isPackaged: boolean
  whenReady(): Promise<void>
  quit(): void
  exit(code?: number): void
  relaunch(): void
  setName(name: string): void
  disableHardwareAcceleration(): void
  on(event: AppEvent, listener: (...args: any[]) => void): this
  off(event: AppEvent, listener: (...args: any[]) => void): this
  once(event: AppEvent, listener: (...args: any[]) => void): this
  emit(event: AppEvent | string, ...args: any[]): boolean
}
```

**NodeAdapter đối chiếu — `NodeApp`** — **File:** `backend/src/platform/adapters/node/app.ts` — **Lines:** 18–126. Điểm mẫu: `class NodeApp extends EventEmitter implements IApp` (line 18), `getPath()` dùng `switch` trên `AppPathName` map vào `os.homedir()`/`os.tmpdir()` (lines 38–68), `whenReady()` resolve ngay lập tức vì Node không có lifecycle GUI (lines 83–86), `relaunch()`/`setName()`/`disableHardwareAcceleration()` là no-op có log cảnh báo (lines 98–108).

#### 2.2.2 `ipc: IIpcBridge`

**File:** `backend/src/platform/ipc-interface.ts` — **Lines:** 7–25

```typescript
export interface IIpcBridge {
  handle(channel: string, listener: IpcHandler): void
  removeHandler(channel: string): void
  on(channel: string, listener: IpcListener): this
  off(channel: string, listener: IpcListener): this
  sendToWindow(windowId: number, channel: string, ...args: any[]): void
  sendToAll(channel: string, ...args: any[]): void
}
```

`IpcEvent` (lines 31–36): `{ readonly sender: { readonly id: number; send(channel, ...args): void } }`.

**NodeAdapter đối chiếu — `NodeIpcBridge`** — **File:** `backend/src/platform/adapters/node/ipc.ts` — **Lines:** 18–129. Điểm mẫu: `handle()`/`removeHandler()` dùng `Map<string, IpcHandler>` nội bộ (lines 30–41) vì không có `ipcMain` thật; `invoke()` (thêm ngoài interface, dùng nội bộ bởi `WebIpcBridge`) dựng `IpcEvent` giả với `sender.id = windowId` (lines 70–88); `sendToWindow`/`sendToAll` route qua `IWindowManager.getAllWindows()` rồi gọi `win.send()` (lines 116–128) — vì Node không có `webContents`, message đi qua WebSocket thay vì IPC kernel thật.

#### 2.2.3 `windowManager: IWindowManager` (+ `IWindow`)

**File:** `backend/src/platform/window-interface.ts` — **Lines:** 2–25 (`IWindow`), 45–51 (`IWindowManager`)

```typescript
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
  send(channel: string, ...args: any[]): void
  on(event: WindowEvent, listener: (...args: any[]) => void): this
  once(event: WindowEvent, listener: (...args: any[]) => void): this
  off(event: WindowEvent, listener: (...args: any[]) => void): this
}

export interface IWindowManager {
  createWindow(options: WindowCreationOptions): IWindow
  getAllWindows(): IWindow[]
  getFocusedWindow(): IWindow | null
  getMainWindow(): IWindow | null
  setMainWindow(window: IWindow | null): void
}
```

**NodeAdapter đối chiếu — `NodeWindow`/`NodeWindowManager`** — **File:** `backend/src/platform/adapters/node/window.ts` — **Lines:** 16–143. Điểm mẫu: `NodeWindow` là "virtual window" — không GUI thật, `isVisible()`/`isFocused()` luôn trả `true` cứng (lines 39–44), `show/hide/focus/restore` là no-op (lines 47–50), `send()` route tới `_sendSubscribers` map thay vì `webContents.send()` thật (lines 70–76) — được `WebIpcBridge` subscribe qua `onSend()` (lines 83–91, method thêm ngoài interface, không có trong `IWindow`) để đẩy qua WebSocket.

#### 2.2.4 `storage: ISecureStorage`

**File:** `backend/src/platform/storage-interface.ts` — **Lines:** 7–16

```typescript
export interface ISecureStorage {
  isEncryptionAvailable(): boolean
  encryptString(plaintext: string): Buffer
  decryptString(encrypted: Buffer): string
}
```

Comment ngay trong interface đã chỉ rõ ý định (lines 4–5): *"NodeAdapter uses AES-256-GCM with a file-based key. ElectronAdapter delegates to Electron's OS keychain integration."* — đây là bằng chứng rõ nhất cho thấy `ElectronAdapter.storage` được thiết kế để dùng `electron.safeStorage` (OS keychain: Keychain trên macOS, libsecret trên Linux, DPAPI trên Windows) chứ không phải AES-GCM tự chế như NodeAdapter.

**NodeAdapter đối chiếu — `NodeSecureStorage`** — **File:** `backend/src/platform/adapters/node/storage.ts` — **Lines:** 25–85. Điểm mẫu: AES-256-GCM (`node:crypto`), key file `userData/.crypto/storage.key` mode `0o600` (lines 69–84), wire format `[iv(12)][tag(16)][ciphertext]` (lines 44–53).

#### 2.2.5 `system: ISystemInfo`

**File:** `backend/src/platform/system-interface.ts` — **Lines:** 2–17

```typescript
export interface ISystemInfo {
  getPlatform(): NodeJS.Platform
  getTotalMemory(): number
  getFreeMemory(): number
  getCpuCount(): number
  getHostname(): string
}
```

**NodeAdapter đối chiếu — `NodeSystemInfo`** — **File:** `backend/src/platform/adapters/node/system.ts` — **Lines:** 7–27. Dùng thẳng `node:os` (`process.platform`, `totalmem()`, `freemem()`, `cpus().length`, `hostname()`) — không có gì Electron-specific ở field này, Electron main process vẫn có full quyền `node:os`.

#### 2.2.6 Factory — `createNodeAdapter()`

**File:** `backend/src/platform/adapters/node/index.ts` — **Lines:** 31–46

```typescript
export function createNodeAdapter(options: NodeAppOptions = {}): IPlatformServices {
  const app = new NodeApp(options)
  const windowManager = new NodeWindowManager()
  const ipc = new NodeIpcBridge(windowManager)
  const storage = new NodeSecureStorage(app)
  const system = new NodeSystemInfo()

  return { mode: 'node', app, ipc, windowManager, storage, system }
}
```

### 2.3 Tình trạng `ElectronAdapter` — xác nhận KHÔNG tồn tại

```
$ ls backend/src/platform/adapters/
node/
```

Không có `electron/`. `desktop/src/platform/` (bản sao song song của cùng cấu trúc platform layer, `diff` với `backend/src/platform/types.ts` cho kết quả **giống hệt**) cũng chỉ có `adapters/node/` và `adapters/web/` — **không có `adapters/electron/` ở bất kỳ nơi nào trong repo**.

`desktop/src/main/index.ts` — **Line:** 8:

```typescript
import { app, BrowserWindow, dialog, ipcMain, nativeTheme } from 'electron'
```

Grep toàn bộ `desktop/src/main/` xác nhận **không có** lời gọi `setPlatform()`/`getPlatform()` nào trong file entry point này — main process Electron thật hoàn toàn không đi qua `IPlatformServices`.

Hai file `backend/src/platform/stubs/electron-node-wrapper.ts` và `electron-web-stub.ts` làm chiều **ngược lại**: giả lập module `electron` (`export const app = { getVersion: () => tryGetPlatform()?.app.getVersion() ?? '0.0.0', ... }`, xem `electron-node-wrapper.ts` lines 29–41) để code viết theo API Electron chạy được dưới NodeAdapter/web khi build alias `'electron'` trỏ vào các file này (`vite.server.config.ts`) — đây là **stub giả lập Electron cho Node**, không phải "ElectronAdapter" (implement `IPlatformServices` bằng Electron thật) theo đúng nghĩa kiến trúc đã vẽ.

**Kết luận:** Platform Abstraction Layer hiện tại **bất đối xứng đúng như tên bug** — 100% các consumer thật (`desktop/src/main/index.ts` và toàn bộ cây `src/main` nó import) chạy trên Electron API trực tiếp, không một dòng nào đi qua `IPlatformServices`; chỉ nhánh server/Node mới thực sự dùng interface trừu tượng.

---

## 3. (a) CHECKLIST câu hỏi cần hỏi PO/tech lead trước khi build

Trước khi viết `ElectronAdapter`, cần xác nhận ý định thật — build sai phạm vi sẽ tốn effort vô ích hoặc để lại 1 abstraction layer chết (không ai gọi):

1. **Phạm vi áp dụng:** `IPlatformServices` có ý định bao phủ *toàn bộ* `desktop/src/main` (hiện ~190 file import trực tiếp `electron` theo cấu trúc import ở `desktop/src/main/index.ts`), hay chỉ 5 module v5.0 mới (`profile`, `project`, `ai-providers`, `workflow`, `task` — vốn đã có 0 hit `electron` trực tiếp)? Bug `BUG-FE-HLD-005` đã kết luận Lựa chọn 1 (chỉ 5 module mới) cho nhánh frontend — quyết định ở backend có nhất quán với kết luận đó không, hay backend có lý do riêng để mở rộng phạm vi?
2. **Ai là consumer thật:** Có call site nào trong roadmap gần (không phải lý thuyết) sẽ `import { createElectronAdapter } from '../platform/adapters/electron'` và `setPlatform(...)` trong `desktop/src/main/index.ts` chưa? Nếu chưa có consumer cụ thể, đây là gap lý thuyết — build trước khi có nhu cầu là speculative work, vi phạm nguyên tắc "Additive only" đã nêu trong `docs/crs/v1/restructure_v1/README.md`.
3. **Lợi ích thực tế:** `desktop/src/main/index.ts` (2355 dòng, đã đọc) tự quản lý lifecycle Electron rất chi tiết — single-instance lock, GPU fallback marker, WSL reconciliation, tray, crash breadcrumbs, dev-parent watchdog... Những cơ chế này có thực sự cần trừu tượng hóa qua `IPlatformServices.app`, hay chúng gắn chặt với Electron-specific API (`app.commandLine.appendSwitch`, `dialog.showMessageBox`, single-instance lock) đến mức việc bọc qua interface chỉ thêm một lớp indirection không giá trị?
4. **Field nào không có ý nghĩa cho Electron:** Interface hiện có `ipc.sendToWindow(windowId, ...)`/`sendToAll` được NodeAdapter dùng để giả lập `webContents.send()` qua WebSocket — trên Electron thật, đây chỉ là passthrough 1-1 tới `BrowserWindow.webContents.send()`. Có field/method nào trong `IPlatformServices` chỉ có ý nghĩa cho server mode (ví dụ liên quan `WebIpcBridge`/WebSocket protocol) mà Electron không cần implement thật, chỉ cần no-op?
5. **Ưu tiên so với các bug khác:** `hld-v1/` còn các bug bảo mật mức độ cao hơn (ticket này tự xếp 🟡 MEDIUM). Có nên defer việc build `ElectronAdapter` đầy đủ, và trong lúc chờ chỉ sửa câu chữ tài liệu (Lựa chọn 1 của ticket) để không hứa hẹn sai kiến trúc hiện có?
6. **`safeStorage` availability trên Linux:** `electron.safeStorage.isEncryptionAvailable()` có thể trả `false` trên Linux nếu không có backend keyring (libsecret/gnome-keyring/kwallet) khả dụng — đặc biệt phổ biến trên SSH/headless/container theo use case Orca hỗ trợ (`AGENTS.md §SSH Use Case`). `ElectronAdapter.storage` khi `isEncryptionAvailable() === false` nên fallback về plaintext (mất bảo mật) hay tái sử dụng `NodeSecureStorage` (AES-256-GCM file-based) làm fallback? Cần quyết định trước khi viết `encryptString`/`decryptString`.
9. **Tương thích test:** `TDD v5 §7` liệt kê 166 test 100% chạy trên Node.js vitest, không import `'electron'`. Nếu build `ElectronAdapter`, test cho nó có bắt buộc phải mock `electron` module (`vi.mock('electron')`) hay chạy trong môi trường Electron thật (Playwright/Spectron-style)? Ai maintain test đó và infra CI có hỗ trợ chạy Electron headless trên Linux CI runner không?
7. **`windowManager.createWindow()` config:** `desktop/src/main/window/createMainWindow.ts` (chưa đọc chi tiết trong scope bug này) hẳn có `webPreferences`, kích thước, icon... rất cụ thể cho Orca. `ElectronAdapter.windowManager.createWindow(options: WindowCreationOptions)` có nên tái sử dụng logic đó, hay `WindowCreationOptions` hiện tại (chỉ có `width/height/minWidth/minHeight/show/frame/transparent/titleBarStyle` + passthrough `[key: string]: any`) đã đủ generic để không cần đổi `createMainWindow.ts`?
8. **Múi giờ triển khai:** Nếu chọn Lựa chọn 2 (build đầy đủ), đây có phải là 1 CR/roadmap item riêng biệt (như `docs/crs/v1/restructure_v1/CR-002-node-adapter.md` đã làm cho NodeAdapter) hay được gộp trực tiếp vào fix bug này?

---

## 4. (b) NẾU quyết định build `ElectronAdapter` đầy đủ — skeleton code

**File:** `backend/src/platform/adapters/electron/index.ts` (mới)

Skeleton dưới đây implement từng field của `IPlatformServices` bằng Electron API thật, đối xứng với `NodeAdapter` theo đúng interface đã trích ở mục 2. Comment `// TODO` đánh dấu phần cần hoàn thiện thêm (không 100% production-ready) nhưng type-compile được, không bỏ sót field nào.

```typescript
/**
 * Electron Platform Adapter
 *
 * Factory cho IPlatformServices trong môi trường Electron desktop thật.
 * Đối xứng với NodeAdapter (backend/src/platform/adapters/node/) — implement
 * cùng IPlatformServices bằng Electron API thay vì Node.js primitives.
 *
 * Usage in desktop/src/main/index.ts (TRƯỚC bất kỳ import nào dùng platform):
 *   import { createElectronAdapter } from '../../../backend/src/platform/adapters/electron'
 *   import { setPlatform } from '../../../backend/src/platform/context'
 *   setPlatform(createElectronAdapter())
 */
import { EventEmitter } from 'node:events'
import { cpus, totalmem, freemem, hostname } from 'node:os'
import {
  app as electronApp,
  BrowserWindow,
  ipcMain as electronIpcMain,
  safeStorage,
  type BrowserWindowConstructorOptions
} from 'electron'

import type { IApp, AppPathName, AppEvent } from '../../app-interface'
import type {
  IWindow,
  IWindowManager,
  WindowCreationOptions,
  WindowEvent
} from '../../window-interface'
import type { IIpcBridge, IpcHandler, IpcListener, IpcEvent } from '../../ipc-interface'
import type { ISecureStorage } from '../../storage-interface'
import type { ISystemInfo } from '../../system-interface'
import type { IPlatformServices } from '../../types'

// ─── app: IApp ────────────────────────────────────────────────────────────
// Why: electron.app is itself an EventEmitter — delegate lifecycle straight
// to it instead of re-implementing, so native events (e.g. 'before-quit'
// fired by the OS) still reach listeners registered through IApp.
export class ElectronApp implements IApp {
  get isPackaged(): boolean {
    return electronApp.isPackaged
  }

  getVersion(): string {
    return electronApp.getVersion()
  }

  getPath(name: AppPathName): string {
    // TODO: AppPathName has a `string` fallback member; electron.app.getPath()
    // only accepts its own literal union. Validate/narrow before casting, or
    // catch and fall back to getPath('userData') for unknown names.
    return electronApp.getPath(name as Parameters<typeof electronApp.getPath>[0])
  }

  getAppPath(): string {
    return electronApp.getAppPath()
  }

  async whenReady(): Promise<void> {
    return electronApp.whenReady()
  }

  quit(): void {
    electronApp.quit()
  }

  exit(code = 0): void {
    electronApp.exit(code)
  }

  relaunch(): void {
    electronApp.relaunch()
  }

  setName(name: string): void {
    electronApp.setName(name)
  }

  disableHardwareAcceleration(): void {
    electronApp.disableHardwareAcceleration()
  }

  on(event: AppEvent, listener: (...args: any[]) => void): this {
    electronApp.on(event as any, listener)
    return this
  }

  off(event: AppEvent, listener: (...args: any[]) => void): this {
    electronApp.off(event as any, listener)
    return this
  }

  once(event: AppEvent, listener: (...args: any[]) => void): this {
    electronApp.once(event as any, listener)
    return this
  }

  emit(event: AppEvent | string, ...args: any[]): boolean {
    // TODO: electron.app does not expose a public emit() in its typings
    // (Electron fires events internally). Re-emitting synthetic events is
    // rarely needed — if a caller needs this, route through a local
    // EventEmitter instead of the real electronApp instance.
    return (electronApp as unknown as EventEmitter).emit(event, ...args)
  }
}

// ─── windowManager: IWindowManager ───────────────────────────────────────
export class ElectronWindow implements IWindow {
  constructor(private readonly _win: BrowserWindow) {}

  get id(): number {
    return this._win.id
  }

  isDestroyed(): boolean {
    return this._win.isDestroyed()
  }
  isMinimized(): boolean {
    return this._win.isMinimized()
  }
  isMaximized(): boolean {
    return this._win.isMaximized()
  }
  isFullScreen(): boolean {
    return this._win.isFullScreen()
  }
  isVisible(): boolean {
    return this._win.isVisible()
  }
  isFocused(): boolean {
    return this._win.isFocused()
  }

  show(): void {
    this._win.show()
  }
  hide(): void {
    this._win.hide()
  }
  focus(): void {
    this._win.focus()
  }
  restore(): void {
    this._win.restore()
  }
  close(): void {
    this._win.close()
  }
  destroy(): void {
    this._win.destroy()
  }

  send(channel: string, ...args: any[]): void {
    if (this._win.isDestroyed()) return
    this._win.webContents.send(channel, ...args)
  }

  on(event: WindowEvent, listener: (...args: any[]) => void): this {
    this._win.on(event as any, listener)
    return this
  }
  once(event: WindowEvent, listener: (...args: any[]) => void): this {
    this._win.once(event as any, listener)
    return this
  }
  off(event: WindowEvent, listener: (...args: any[]) => void): this {
    this._win.off(event as any, listener)
    return this
  }

  /** @internal exposed for ElectronWindowManager bookkeeping only */
  get raw(): BrowserWindow {
    return this._win
  }
}

export class ElectronWindowManager implements IWindowManager {
  private readonly _windows = new Map<number, ElectronWindow>()
  private _mainWindow: ElectronWindow | null = null

  createWindow(options: WindowCreationOptions): ElectronWindow {
    // TODO: WindowCreationOptions is a generic passthrough shape; map its
    // known fields explicitly instead of a blind cast once the real call
    // site (desktop/src/main/window/createMainWindow.ts) is wired through
    // this adapter, so Orca-specific defaults (icon, webPreferences,
    // titleBarOverlay, etc.) are not silently dropped.
    const win = new BrowserWindow(options as BrowserWindowConstructorOptions)
    const wrapped = new ElectronWindow(win)
    this._windows.set(win.id, wrapped)
    win.once('closed', () => this._windows.delete(win.id))
    return wrapped
  }

  getAllWindows(): IWindow[] {
    return [...this._windows.values()]
  }

  getFocusedWindow(): IWindow | null {
    const focused = BrowserWindow.getFocusedWindow()
    if (!focused) return null
    return this._windows.get(focused.id) ?? null
  }

  getMainWindow(): IWindow | null {
    return this._mainWindow
  }

  setMainWindow(window: IWindow | null): void {
    this._mainWindow = window as ElectronWindow | null
  }
}

// ─── ipc: IIpcBridge ──────────────────────────────────────────────────────
export class ElectronIpcBridge implements IIpcBridge {
  private readonly _listeners = new Map<string, Set<IpcListener>>()

  constructor(private readonly _windowManager: ElectronWindowManager) {}

  handle(channel: string, listener: IpcHandler): void {
    electronIpcMain.handle(channel, (event, ...args) => {
      const ipcEvent: IpcEvent = {
        sender: {
          // TODO: electron's IpcMainInvokeEvent.sender is a WebContents, not
          // a BrowserWindow — id here is the webContents id, which matches
          // NodeIpcBridge's windowId semantics (see ipc.ts:78-85) as long as
          // ElectronWindow.id is defined consistently. Verify against
          // BrowserWindow.fromWebContents(event.sender)?.id if callers need
          // the window id specifically rather than the webContents id.
          id: event.sender.id,
          send: (replyChannel: string, ...replyArgs: any[]) =>
            event.sender.send(replyChannel, ...replyArgs)
        }
      }
      return listener(ipcEvent, ...args)
    })
  }

  removeHandler(channel: string): void {
    electronIpcMain.removeHandler(channel)
  }

  on(channel: string, listener: IpcListener): this {
    let set = this._listeners.get(channel)
    if (!set) {
      set = new Set()
      this._listeners.set(channel, set)
      // Why: register the electron-side subscription once per channel; fan
      // out to all IpcListeners registered through this bridge, matching
      // NodeIpcBridge's Map<string, Set<IpcListener>> fan-out (ipc.ts:20,45-53).
      electronIpcMain.on(channel, (event, ...args) => {
        const ipcEvent: IpcEvent = {
          sender: {
            id: event.sender.id,
            send: (replyChannel: string, ...replyArgs: any[]) =>
              event.sender.send(replyChannel, ...replyArgs)
          }
        }
        for (const l of this._listeners.get(channel) ?? []) {
          l(ipcEvent, ...args)
        }
      })
    }
    set.add(listener)
    return this
  }

  off(channel: string, listener: IpcListener): this {
    this._listeners.get(channel)?.delete(listener)
    // TODO: electronIpcMain.on() subscription above is not removed when the
    // Set empties — decide whether to call electronIpcMain.removeAllListeners(channel)
    // once _listeners.get(channel)?.size === 0, mirroring handle()/removeHandler() symmetry.
    return this
  }

  sendToWindow(windowId: number, channel: string, ...args: any[]): void {
    const win = this._windowManager.getAllWindows().find((w) => w.id === windowId)
    win?.send(channel, ...args)
  }

  sendToAll(channel: string, ...args: any[]): void {
    for (const win of this._windowManager.getAllWindows()) {
      win.send(channel, ...args)
    }
  }
}

// ─── storage: ISecureStorage ──────────────────────────────────────────────
// Why: delegates to Electron's OS keychain integration (Keychain/libsecret/
// DPAPI) per the design intent documented in storage-interface.ts:5, unlike
// NodeSecureStorage's file-based AES-256-GCM (storage.ts:11-24).
export class ElectronSecureStorage implements ISecureStorage {
  isEncryptionAvailable(): boolean {
    return safeStorage.isEncryptionAvailable()
  }

  encryptString(plaintext: string): Buffer {
    // TODO: decide the fallback when isEncryptionAvailable() === false
    // (common on headless Linux/SSH hosts without a keyring — see AGENTS.md
    // §SSH Use Case). safeStorage.encryptString() throws in that case.
    // Options: throw and let callers fall back to NodeSecureStorage, or
    // wrap NodeSecureStorage as a composed fallback here.
    return safeStorage.encryptString(plaintext)
  }

  decryptString(encrypted: Buffer): string {
    return safeStorage.decryptString(encrypted)
  }
}

// ─── system: ISystemInfo ──────────────────────────────────────────────────
// Why: the Electron main process is still a full Node.js process, so this
// is identical to NodeSystemInfo (system.ts:7-27) — no Electron-specific
// API needed for OS-level info.
export class ElectronSystemInfo implements ISystemInfo {
  getPlatform(): NodeJS.Platform {
    return process.platform
  }
  getTotalMemory(): number {
    return totalmem()
  }
  getFreeMemory(): number {
    return freemem()
  }
  getCpuCount(): number {
    return cpus().length
  }
  getHostname(): string {
    return hostname()
  }
}

// ─── Factory ────────────────────────────────────────────────────────────
/**
 * Create a complete IPlatformServices for Electron desktop mode.
 *
 * MUST be called — and setPlatform() invoked — before any module under
 * desktop/src/main that will be migrated to use getPlatform() is imported,
 * mirroring the ordering requirement documented for NodeAdapter
 * (specs/backend/tdd/v5/10-platform-layer.md §3, §8).
 */
export function createElectronAdapter(): IPlatformServices {
  const app = new ElectronApp()
  const windowManager = new ElectronWindowManager()
  const ipc = new ElectronIpcBridge(windowManager)
  const storage = new ElectronSecureStorage()
  const system = new ElectronSystemInfo()

  return { mode: 'electron', app, ipc, windowManager, storage, system }
}
```

---

## 5. Test cần bổ sung nếu triển khai `ElectronAdapter`

Đối xứng với `TDD v5 §7` (166 test cho NodeAdapter, 100% Node vitest, 0 import `'electron'` thật):

- `backend/src/platform/adapters/electron/__tests__/app.test.ts` — mock `electron.app` (`vi.mock('electron')`), verify `getVersion/getPath/getAppPath/isPackaged/whenReady/quit/exit/relaunch/setName/disableHardwareAcceleration` đều delegate đúng tham số tới mock.
- `.../window.test.ts` — mock `BrowserWindow`, verify `createWindow()` map `WindowCreationOptions` → `BrowserWindowConstructorOptions` không mất field; verify `getFocusedWindow()`/`getAllWindows()` đồng bộ với registry nội bộ khi window bị `destroy()`.
- `.../ipc.test.ts` — mock `ipcMain`, verify `handle()` bọc đúng `IpcEvent.sender.id`/`sender.send`; verify `sendToWindow`/`sendToAll` không throw khi `windowId` không tồn tại (đối xứng NodeIpcBridge, `ipc.ts:116-122`).
- `.../storage.test.ts` — mock `safeStorage`, test riêng nhánh `isEncryptionAvailable() === false` (theo quyết định ở mục 3 câu hỏi 6) — đây là nhánh dễ bị bỏ sót nhất vì môi trường CI Linux có thể luôn `true` hoặc luôn `false` tùy keyring cài sẵn.
- `.../system.test.ts` — có thể tái dùng gần như nguyên bản `adapters/node/__tests__/system.test.ts` vì implementation giống hệt NodeSystemInfo.
- `.../index.test.ts` — verify `createElectronAdapter()` trả `mode: 'electron'` và tất cả 5 field non-null, đối xứng `adapters/node/__tests__/index.test.ts`.
- **Conformance suite dùng chung:** nếu `interface-conformance.ts` (`specs/backend/tdd/v5/10-platform-layer.md §7`) có sẵn `runIAppConformanceTests`/tương đương cho NodeAdapter, chạy lại đúng bộ đó cho `ElectronAdapter` để đảm bảo hai adapter thực sự tương thích hành vi ở mức interface, không chỉ mức type.
- **Integration test thật (không mock):** ít nhất 1 smoke test chạy trong Electron process thật (không `vi.mock('electron')`) verify `createElectronAdapter()` không throw khi gọi trong `app.whenReady()` — mock-only test không bắt được lỗi API Electron thật đổi signature giữa các version.

---

## 6. Rủi ro / lưu ý

- **Electron main vs renderer:** Skeleton ở mục 4 chỉ hợp lệ trong **main process**. `BrowserWindow`, `ipcMain`, `safeStorage`, `app` đều không tồn tại trong renderer/sandbox context — nếu code dùng `getPlatform()` vô tình bị bundle vào renderer (qua import chain), sẽ crash ngay khi load module. Cần lint rule chặn import `platform/adapters/electron` ngoài `desktop/src/main/`.
- **`app.getPath()` type mismatch:** `AppPathName` (backend) có nhiều literal Electron **không** hỗ trợ y hệt (`AppPathName` cho phép `string` fallback tùy ý — Electron's `app.getPath()` throw nếu tên không nằm trong danh sách cố định của nó). Cast trong skeleton (`TODO` ở `ElectronApp.getPath`) che giấu lỗi runtime tiềm ẩn — cần validate/whitelist trước khi cast.
- **`safeStorage` không khả dụng trên Linux headless/SSH:** Theo `AGENTS.md §SSH Use Case`, Orca không được giả định chạy local-only. `safeStorage.isEncryptionAvailable()` có thể trả `false` khi không có keyring backend (phổ biến trên server Linux không có desktop session, hoặc container) — `encryptString()` sẽ throw thay vì trả buffer. Bug hiện tại (mục 3 câu 6) chưa quyết định fallback; nếu build thật, đây là rủi ro bảo mật/crash cần xử lý tường minh, không được để mặc định throw không catch.
- **Cross-platform (`AGENTS.md §Cross-Platform Support`):** `BrowserWindowConstructorOptions`, `titleBarStyle`, tray/menu accelerator đều khác nhau đáng kể giữa macOS/Linux/Windows. Skeleton không tự xử lý — phần này vốn đã được `desktop/src/main/window/createMainWindow.ts` xử lý riêng; nếu `ElectronWindowManager.createWindow()` thay thế nó, phải audit lại toàn bộ platform-specific logic đang nằm trong file đó để không mất hành vi.
- **`ipcMain.on()` không có `off` đối xứng theo channel:** Electron's `ipcMain` không hỗ trợ gỡ 1 listener cụ thể dễ dàng khi nhiều `IpcListener` share cùng 1 `electronIpcMain.on(channel, ...)` subscription (xem `TODO` ở `ElectronIpcBridge.off`). Cần quyết định chiến lược cleanup trước khi dùng trong code có vòng đời window ngắn (window đóng/mở lại nhiều lần).
- **Chi phí thật của Lựa chọn 2:** Ticket đã ước tính đây là "hạng mục lớn" cần audit số file `desktop/src/main` import `electron` trực tiếp. Chỉ riêng `desktop/src/main/index.ts` (2355 dòng) đã có hơn 100 import nội bộ liên quan lifecycle Electron rất đặc thù (single-instance lock, GPU fallback, WSL barrier, tray, crash breadcrumb...) — skeleton ở mục 4 chỉ là **điểm khởi đầu cho interface**, không bao gồm việc di chuyển các cơ chế đó qua interface, vốn là phần tốn effort lớn nhất.
- **Không phá vỡ NodeAdapter:** `IPlatformServices.mode` hiện được cả hai adapter set độc lập (`'node'` / `'electron'`) — bất kỳ code nào rẽ nhánh theo `mode` (nếu có trong tương lai) cần test cả 2 giá trị để tránh regression một chiều.
