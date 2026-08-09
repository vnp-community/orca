# TASK-HLD-027: (CONDITIONAL) Implement ElectronAdapter theo skeleton — chỉ làm nếu TASK-HLD-026 kết luận cần build đầy đủ

**Priority:** 🟢 LOW (điều kiện — có thể không bao giờ thực hiện tuỳ kết luận TASK-HLD-026)
**Effort:** ~3-5 ngày (implement đầy đủ + test conformance suite + integration test thật)
**Status:** ⏸️ KHÔNG THỰC HIỆN (theo điều kiện đã nêu ở trên) — 2026-08-09. TASK-HLD-026 đã phân tích và **đề xuất Lựa chọn 1** (chỉ sửa tài liệu, nhất quán với kết luận đã DONE của `BUG-FE-HLD-005` phía frontend + không có consumer thật nào cho `createElectronAdapter`) — xem chi tiết bằng chứng trong TASK-HLD-026. Vì đề xuất là Lựa chọn 1, task này bị huỷ theo đúng quy tắc đã ghi ở mục "ĐIỀU KIỆN" bên trên; không viết `ElectronAdapter` skeleton. Lưu ý: TASK-HLD-026 vẫn đang chờ chữ ký chính thức từ tech lead/PO — nếu con người sau này đảo ngược thành Lựa chọn 2, cần mở lại task này.
**Bug refs:** BUG-BE-HLD-017
**Solution ref:** [SOLUTION-platform-electron-adapter-exact.md](../solutions/SOLUTION-platform-electron-adapter-exact.md) §4, §5, §6
**Depends on:** TASK-HLD-026 (chỉ tiến hành nếu kết luận là "Lựa chọn 2 — build đầy đủ")

---

## ⚠️ ĐIỀU KIỆN: chỉ thực hiện nếu TASK-HLD-026 kết luận cần build

Task này **KHÔNG được bắt đầu** cho tới khi TASK-HLD-026 có kết luận bằng văn bản chọn "Lựa chọn 2 — build ElectronAdapter đầy đủ". Nếu TASK-HLD-026 kết luận "Lựa chọn 1 — chỉ sửa tài liệu" (giống kết luận đã có ở `BUG-FE-HLD-005` phía frontend), task này bị **huỷ**, không thực hiện.

## Mục tiêu

Implement `ElectronAdapter` — factory tạo `IPlatformServices` bằng Electron API thật, đối xứng với `NodeAdapter` (`backend/src/platform/adapters/node/`). Skeleton code đầy đủ đã có sẵn trong solution file, type-compile được nhưng còn nhiều `TODO` cần hoàn thiện trước khi production-ready (xem mục "Việc cần hoàn thiện thêm" bên dưới).

## File cần sửa/tạo

```
backend/src/platform/adapters/electron/index.ts                    (tạo mới — theo skeleton)
backend/src/platform/adapters/electron/__tests__/app.test.ts        (tạo mới)
backend/src/platform/adapters/electron/__tests__/window.test.ts     (tạo mới)
backend/src/platform/adapters/electron/__tests__/ipc.test.ts        (tạo mới)
backend/src/platform/adapters/electron/__tests__/storage.test.ts    (tạo mới)
backend/src/platform/adapters/electron/__tests__/system.test.ts     (tạo mới)
backend/src/platform/adapters/electron/__tests__/index.test.ts      (tạo mới)
desktop/src/main/index.ts                                            (sửa — wire setPlatform(createElectronAdapter()) TRƯỚC bất kỳ import nào dùng platform)
```

## Thay đổi cụ thể

### `backend/src/platform/adapters/electron/index.ts` — skeleton đầy đủ

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

### Việc cần hoàn thiện thêm trước khi production-ready (đánh dấu `TODO` trong skeleton)

1. **`ElectronApp.getPath()` type mismatch** — `AppPathName` cho phép `string` fallback tuỳ ý, nhưng `electron.app.getPath()` throw nếu tên không nằm trong danh sách literal cố định của Electron. Cast trong skeleton che giấu lỗi runtime tiềm ẩn — cần validate/whitelist trước khi cast, hoặc catch + fallback về `getPath('userData')`.
2. **`ElectronApp.emit()`** — `electron.app` không expose `emit()` public trong typings. Nếu caller thật sự cần re-emit synthetic event, cân nhắc route qua `EventEmitter` nội bộ riêng thay vì ép kiểu `electronApp`.
3. **`ElectronWindowManager.createWindow()`** — map tường minh từng field của `WindowCreationOptions` thay vì cast mù, đặc biệt khi wire vào `desktop/src/main/window/createMainWindow.ts` thật (không được làm mất default Orca-specific: icon, `webPreferences`, `titleBarOverlay`...) — xem câu hỏi #7 ở TASK-HLD-026.
4. **`ElectronIpcBridge.off()`** — Electron's `ipcMain` không hỗ trợ gỡ 1 listener cụ thể dễ dàng khi nhiều `IpcListener` share cùng 1 `electronIpcMain.on(channel, ...)` subscription. Cần quyết định chiến lược cleanup (gọi `electronIpcMain.removeAllListeners(channel)` khi `Set` rỗng?) trước khi dùng trong code có vòng đời window ngắn.
5. **`ElectronSecureStorage.encryptString()` fallback khi `isEncryptionAvailable() === false`** — theo quyết định câu hỏi #6 ở TASK-HLD-026 (throw và để caller fallback về `NodeSecureStorage`, hay compose `NodeSecureStorage` ngay trong `ElectronSecureStorage`).

### `desktop/src/main/index.ts` — wire adapter (chỉ nếu phạm vi đã xác nhận cần desktop consumer thật)

```typescript
import { createElectronAdapter } from '../../../backend/src/platform/adapters/electron'
import { setPlatform } from '../../../backend/src/platform/context'
setPlatform(createElectronAdapter())
```

**Phải gọi TRƯỚC bất kỳ import nào dùng `getPlatform()`** — cùng yêu cầu thứ tự đã áp dụng cho `NodeAdapter` (`specs/backend/tdd/v5/10-platform-layer.md §3, §8).

## Lưu ý rủi ro quan trọng

- **Electron main vs renderer:** skeleton chỉ hợp lệ trong **main process**. `BrowserWindow`, `ipcMain`, `safeStorage`, `app` không tồn tại trong renderer/sandbox context — nếu `getPlatform()` vô tình bị bundle vào renderer, sẽ crash ngay khi load module. Cần thêm lint rule chặn import `platform/adapters/electron` ngoài `desktop/src/main/`.
- **`safeStorage` không khả dụng trên Linux headless/SSH** (theo `AGENTS.md §SSH Use Case`, Orca không giả định local-only) — rủi ro bảo mật/crash cần xử lý tường minh, không để mặc định throw không catch.
- **Cross-platform** (`AGENTS.md §Cross-Platform Support`): `BrowserWindowConstructorOptions`, `titleBarStyle`, tray/menu accelerator khác nhau đáng kể giữa macOS/Linux/Windows — skeleton không tự xử lý, cần audit lại toàn bộ platform-specific logic đang nằm trong `createMainWindow.ts` để không mất hành vi nếu `ElectronWindowManager.createWindow()` thay thế nó.
- **Chi phí thật lớn hơn skeleton này:** skeleton chỉ là điểm khởi đầu cho interface — di chuyển các cơ chế lifecycle đặc thù (single-instance lock, GPU fallback, WSL barrier, tray, crash breadcrumb...) của `desktop/src/main/index.ts` (2355 dòng) qua interface là phần tốn effort lớn nhất, KHÔNG nằm trong skeleton.
- **Không phá vỡ NodeAdapter:** `IPlatformServices.mode` được cả 2 adapter set độc lập (`'node'`/`'electron'`) — bất kỳ code rẽ nhánh theo `mode` trong tương lai cần test cả 2 giá trị.

## Verification

```bash
cd /opt/repos/orca
pnpm --filter backend tsc --noEmit
pnpm --filter backend test platform/adapters/electron

# Xác nhận không có import electron lọt vào renderer bundle
grep -rn "platform/adapters/electron" desktop/src/renderer/ 2>/dev/null
# Expected: 0 kết quả
```

Test cần bổ sung (đối xứng `TDD v5 §7`, 166 test cho NodeAdapter):

1. `app.test.ts` — mock `electron.app` (`vi.mock('electron')`), verify `getVersion/getPath/getAppPath/isPackaged/whenReady/quit/exit/relaunch/setName/disableHardwareAcceleration` delegate đúng tham số tới mock.
2. `window.test.ts` — mock `BrowserWindow`, verify `createWindow()` map `WindowCreationOptions` → `BrowserWindowConstructorOptions` không mất field; verify `getFocusedWindow()`/`getAllWindows()` đồng bộ registry khi window `destroy()`.
3. `ipc.test.ts` — mock `ipcMain`, verify `handle()` bọc đúng `IpcEvent.sender.id`/`sender.send`; verify `sendToWindow`/`sendToAll` không throw khi `windowId` không tồn tại.
4. `storage.test.ts` — mock `safeStorage`, test riêng nhánh `isEncryptionAvailable() === false` (dễ bị bỏ sót nhất — môi trường CI Linux có thể luôn `true` hoặc luôn `false` tuỳ keyring cài sẵn).
5. `system.test.ts` — có thể tái dùng gần như nguyên bản `adapters/node/__tests__/system.test.ts` vì implementation giống hệt `NodeSystemInfo`.
6. `index.test.ts` — verify `createElectronAdapter()` trả `mode: 'electron'` và tất cả 5 field non-null.
7. **Conformance suite dùng chung:** nếu có sẵn `runIAppConformanceTests`/tương đương cho `NodeAdapter` (`specs/backend/tdd/v5/10-platform-layer.md §7`), chạy lại đúng bộ đó cho `ElectronAdapter` để đảm bảo 2 adapter tương thích hành vi ở mức interface, không chỉ mức type.
8. **Integration test thật (không mock):** ít nhất 1 smoke test chạy trong Electron process thật (không `vi.mock('electron')`) verify `createElectronAdapter()` không throw khi gọi trong `app.whenReady()` — mock-only test không bắt được lỗi API Electron thật đổi signature giữa các version.
