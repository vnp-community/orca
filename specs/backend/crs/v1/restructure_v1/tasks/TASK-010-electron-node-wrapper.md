# TASK-010: Tạo `electron-node-wrapper.ts` stub

**Source:** SOL-BE-003, SOL-BE-004  
**Phase:** 2 | **Effort:** M (1.5–2 giờ)  
**Depends on:** TASK-001, TASK-002, TASK-008

---

## Objective

Tạo `src/platform/stubs/electron-node-wrapper.ts` — module này được dùng như **alias cho `'electron'`** trong build Node.js server. Thay vì import Electron, `src/main/` sẽ import module này và mọi call đều được delegate tới `getPlatform()`.

**Nguyên tắc:** File này phải export tất cả các symbols mà `src/main/` sử dụng từ `'electron'`.

---

## Context cần đọc trước

Đọc file sau để biết những gì `src/main/` import từ electron:

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
grep -r "from 'electron'" src/main/ | grep -v "mocks" | sort | head -30
grep -r "require('electron')" src/main/ | grep -v "mocks" | sort | head -10
```

---

## File to create

### `src/platform/stubs/electron-node-wrapper.ts`

```typescript
/**
 * electron-node-wrapper.ts — Electron API Stub for Node.js Server Mode
 *
 * This module is aliased as 'electron' in vite.server.config.ts.
 * When src/main/ code does `import { app, ipcMain } from 'electron'`,
 * it imports from HERE instead, which delegates to the NodeAdapter
 * via getPlatform().
 *
 * Rules:
 * 1. Never import from 'electron'
 * 2. Delegate everything to getPlatform() or safe defaults
 * 3. Log a warning for any feature that cannot be emulated in server mode
 * 4. Never crash — always provide safe fallbacks
 */

import { getPlatform, isPlatformInitialized } from '../context'
import type { IpcEvent } from '../ipc-interface'

function tryGetPlatform() {
  if (!isPlatformInitialized()) return null
  try { return getPlatform() } catch { return null }
}

// ─── app ─────────────────────────────────────────────────────────────────────
export const app = {
  getVersion: () => tryGetPlatform()?.app.getVersion() ?? '0.0.0',
  getPath: (name: string) => tryGetPlatform()?.app.getPath(name) ?? '/tmp',
  getAppPath: () => tryGetPlatform()?.app.getAppPath?.() ?? process.cwd(),
  isPackaged: tryGetPlatform()?.app.isPackaged ?? true,
  whenReady: () => tryGetPlatform()?.app.whenReady() ?? Promise.resolve(),
  quit: () => tryGetPlatform()?.app.quit() ?? process.exit(0),
  exit: (code?: number) => tryGetPlatform()?.app.exit(code) ?? process.exit(code ?? 0),
  relaunch: () => console.warn('[ElectronStub] relaunch() is a no-op in server mode'),
  setName: (_name: string) => {},
  disableHardwareAcceleration: () => {},
  on: (event: string, listener: (...args: any[]) => void) => {
    tryGetPlatform()?.app.on(event as any, listener)
    return app
  },
  off: (event: string, listener: (...args: any[]) => void) => {
    tryGetPlatform()?.app.off(event as any, listener)
    return app
  },
  once: (event: string, listener: (...args: any[]) => void) => {
    tryGetPlatform()?.app.once?.(event as any, listener)
    return app
  },
}

// ─── ipcMain ─────────────────────────────────────────────────────────────────
export const ipcMain = {
  handle: (channel: string, listener: IpcEvent | any) => {
    tryGetPlatform()?.ipc.handle(channel, listener)
  },
  removeHandler: (channel: string) => {
    tryGetPlatform()?.ipc.removeHandler(channel)
  },
  on: (channel: string, listener: any) => {
    tryGetPlatform()?.ipc.on(channel, listener)
    return ipcMain
  },
  off: (channel: string, listener: any) => {
    tryGetPlatform()?.ipc.off(channel, listener)
    return ipcMain
  },
}

// ─── BrowserWindow ───────────────────────────────────────────────────────────
export class BrowserWindow {
  private _win: ReturnType<typeof tryGetPlatform> extends null ? never : any

  constructor(_options?: any) {
    const platform = tryGetPlatform()
    if (platform) {
      this._win = platform.windowManager.createWindow(_options ?? {})
    } else {
      console.warn('[ElectronStub] BrowserWindow created before platform init')
      this._win = null
    }
  }

  get id(): number { return this._win?.id ?? -1 }

  get webContents(): any {
    return {
      id: this._win?.id ?? -1,
      send: (channel: string, ...args: any[]) => {
        this._win?.send(channel, ...args)
      },
      getURL: () => '',
      getTitle: () => '',
      isDestroyed: () => this._win?.isDestroyed() ?? true,
      session: {
        getUserAgent: () => 'Orca-Server/1.0',
        fromPartition: (_partition: string) => ({
          getUserAgent: () => 'Orca-Server/1.0',
          cookies: { get: async () => [], set: async () => {}, remove: async () => {} },
          setProxy: async () => {},
          clearCache: async () => {},
        }),
        defaultSession: {
          getUserAgent: () => 'Orca-Server/1.0',
        }
      }
    }
  }

  isDestroyed(): boolean { return this._win?.isDestroyed() ?? true }
  isMinimized(): boolean { return this._win?.isMinimized() ?? false }
  isMaximized(): boolean { return this._win?.isMaximized() ?? false }
  isFullScreen(): boolean { return this._win?.isFullScreen() ?? false }
  isVisible(): boolean { return this._win?.isVisible() ?? true }
  isFocused(): boolean { return this._win?.isFocused() ?? true }

  show(): void { this._win?.show?.() }
  hide(): void { this._win?.hide?.() }
  focus(): void { this._win?.focus?.() }
  restore(): void { this._win?.restore?.() }
  close(): void { this._win?.close?.() }
  destroy(): void { this._win?.destroy?.() }

  send(channel: string, ...args: any[]): void {
    this._win?.send(channel, ...args)
  }

  loadURL(_url: string): Promise<void> {
    return Promise.resolve()
  }

  loadFile(_filePath: string): Promise<void> {
    return Promise.resolve()
  }

  setTitle(_title: string): void {}
  setMenu(_menu: any): void {}
  setMenuBarVisibility(_visible: boolean): void {}
  setAutoHideMenuBar(_hide: boolean): void {}
  setThumbarButtons(_buttons: any[]): void {}
  setProgressBar(_progress: number): void {}
  flashFrame(_flag: boolean): void {}
  center(): void {}
  setBounds(_bounds: any): void {}
  getBounds() { return { x: 0, y: 0, width: 1280, height: 800 } }
  getSize(): [number, number] { return [1280, 800] }
  setSize(_w: number, _h: number): void {}
  setMinimumSize(_w: number, _h: number): void {}
  setMaximumSize(_w: number, _h: number): void {}
  setResizable(_val: boolean): void {}
  maximize(): void {}
  minimize(): void {}
  unmaximize(): void {}
  setFullScreen(_flag: boolean): void {}
  on(event: string, listener: (...args: any[]) => void) {
    this._win?.on(event as any, listener)
    return this
  }
  once(event: string, listener: (...args: any[]) => void) {
    this._win?.once(event as any, listener)
    return this
  }
  off(event: string, listener: (...args: any[]) => void) {
    this._win?.off(event as any, listener)
    return this
  }
  removeListener(event: string, listener: (...args: any[]) => void) {
    this._win?.off(event as any, listener)
    return this
  }

  static getAllWindows(): BrowserWindow[] {
    const platform = tryGetPlatform()
    if (!platform) return []
    return platform.windowManager.getAllWindows().map(w => {
      const bw = new BrowserWindow()
      bw._win = w
      return bw
    })
  }

  static getFocusedWindow(): BrowserWindow | null {
    const win = tryGetPlatform()?.windowManager.getFocusedWindow()
    if (!win) return null
    const bw = new BrowserWindow()
    bw._win = win
    return bw
  }
}

// ─── screen ──────────────────────────────────────────────────────────────────
export const screen = {
  getPrimaryDisplay: () => ({
    workAreaSize: { width: 1920, height: 1080 },
    workArea: { x: 0, y: 0, width: 1920, height: 1080 },
    bounds: { x: 0, y: 0, width: 1920, height: 1080 },
    scaleFactor: 1
  }),
  getAllDisplays: () => [],
  getCursorScreenPoint: () => ({ x: 0, y: 0 })
}

// ─── nativeTheme ─────────────────────────────────────────────────────────────
export const nativeTheme = {
  shouldUseDarkColors: false,
  themeSource: 'system' as 'system' | 'light' | 'dark',
  on: (_event: string, _listener: any) => {},
  off: (_event: string, _listener: any) => {},
}

// ─── shell ───────────────────────────────────────────────────────────────────
export const shell = {
  openExternal: (url: string) => {
    console.warn('[ElectronStub] shell.openExternal() is a no-op in server mode:', url)
    return Promise.resolve()
  },
  openPath: (path: string) => {
    console.warn('[ElectronStub] shell.openPath() is a no-op in server mode:', path)
    return Promise.resolve('')
  },
  showItemInFolder: () => {},
  beep: () => {},
}

// ─── dialog ──────────────────────────────────────────────────────────────────
export const dialog = {
  showOpenDialog: async (_win: any, _opts: any) => ({ canceled: true, filePaths: [] }),
  showSaveDialog: async (_win: any, _opts: any) => ({ canceled: true, filePath: '' }),
  showMessageBox: async (_win: any, _opts: any) => ({ response: 0, checkboxChecked: false }),
  showErrorBox: (_title: string, content: string) => console.error('[Dialog]', content),
}

// ─── safeStorage ─────────────────────────────────────────────────────────────
export const safeStorage = {
  isEncryptionAvailable: () => tryGetPlatform()?.storage.isEncryptionAvailable() ?? false,
  encryptString: (text: string): Buffer =>
    tryGetPlatform()?.storage.encryptString(text) ?? Buffer.from(text),
  decryptString: (encrypted: Buffer): string =>
    tryGetPlatform()?.storage.decryptString(encrypted) ?? encrypted.toString('utf-8'),
}

// ─── systemPreferences ───────────────────────────────────────────────────────
export const systemPreferences = {
  getMediaAccessStatus: (_type: string) => 'granted' as const,
  askForMediaAccess: async (_type: string) => true,
  getColor: (_color: string) => '#000000',
}

// ─── powerMonitor ────────────────────────────────────────────────────────────
export const powerMonitor = {
  on: () => {},
  off: () => {},
  getSystemIdleTime: () => 0,
}

// ─── Tray ────────────────────────────────────────────────────────────────────
export class Tray {
  constructor(_icon: any) {}
  setContextMenu(_menu: any): void {}
  setToolTip(_tip: string): void {}
  setTitle(_title: string): void {}
  destroy(): void {}
  on(_event: string, _listener: any) { return this }
}

// ─── Menu / MenuItem ──────────────────────────────────────────────────────────
export class Menu {
  static buildFromTemplate(_template: any[]): Menu { return new Menu() }
  static setApplicationMenu(_menu: Menu | null): void {}
  popup(_opts?: any): void {}
  append(_item: MenuItem): void {}
}

export class MenuItem {
  constructor(_options: any) {}
}

// ─── Notification ─────────────────────────────────────────────────────────────
export class Notification {
  constructor(_options?: any) {}
  show(): void { console.info('[ElectronStub] Notification.show() — server mode') }
  on(_event: string, _listener: any) { return this }
  static isSupported(): boolean { return false }
}

// ─── clipboard ────────────────────────────────────────────────────────────────
export const clipboard = {
  readText: () => '',
  writeText: (_text: string) => {},
  readImage: () => null,
  writeImage: (_image: any) => {},
  clear: () => {},
}

// ─── globalShortcut ──────────────────────────────────────────────────────────
export const globalShortcut = {
  register: (_accel: string, _callback: () => void) => false,
  unregister: (_accel: string) => {},
  unregisterAll: () => {},
  isRegistered: (_accel: string) => false,
}

// ─── crashReporter ───────────────────────────────────────────────────────────
export const crashReporter = {
  start: (_opts?: any) => {},
}

// ─── session ─────────────────────────────────────────────────────────────────
export const session = {
  defaultSession: {
    getUserAgent: () => 'Orca-Server/1.0',
    fromPartition: (_partition: string) => ({
      getUserAgent: () => 'Orca-Server/1.0',
    })
  },
  fromPartition: (_partition: string) => ({
    getUserAgent: () => 'Orca-Server/1.0',
    cookies: { get: async () => [], set: async () => {}, remove: async () => {} },
  })
}

// Default export (some code uses `import electron from 'electron'`)
export default {
  app, ipcMain, BrowserWindow, screen, nativeTheme, shell, dialog,
  safeStorage, systemPreferences, powerMonitor, Tray, Menu, MenuItem,
  Notification, clipboard, globalShortcut, crashReporter, session
}
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "electron-node-wrapper" | head -10

# Verify no electron import
grep "from 'electron'" src/platform/stubs/electron-node-wrapper.ts
# Expected: empty

# Verify all commonly used symbols are exported
node -e "
const stub = require('./src/platform/stubs/electron-node-wrapper')
const required = ['app','ipcMain','BrowserWindow','shell','dialog','safeStorage',
  'screen','nativeTheme','clipboard','Menu','Tray','Notification']
for (const name of required) {
  if (!stub[name]) console.error('MISSING:', name)
  else console.log('OK:', name)
}"
```

---

## Done criteria

- [x] `src/platform/stubs/electron-node-wrapper.ts` tạo thành công
- [x] Không có `import 'electron'` trong file này
- [x] Export: `app`, `ipcMain`, `BrowserWindow`, `shell`, `dialog`, `safeStorage`, `screen`, `nativeTheme`, `clipboard`, `Menu`, `MenuItem`, `Tray`, `Notification`, `clipboard`, `globalShortcut`, `crashReporter`, `session`
- [x] `app` delegate đúng đến `getPlatform().app`
- [x] `ipcMain.handle` delegate đến `getPlatform().ipc.handle`
- [x] `safeStorage` delegate đến `getPlatform().storage`
- [x] Graceful fallback khi platform chưa init (`tryGetPlatform()` trả về null → safe defaults)
