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

import { EventEmitter } from 'node:events'
import { getPlatform, isPlatformInitialized } from '../context'

function tryGetPlatform() {
  if (!isPlatformInitialized()) {return null}
  try {
    return getPlatform()
  } catch {
    return null
  }
}

// ─── app ─────────────────────────────────────────────────────────────────────
export const app = {
  getVersion: () => tryGetPlatform()?.app.getVersion() ?? '0.0.0',
  getPath: (name: string) => tryGetPlatform()?.app.getPath(name) ?? '/tmp',
  getAppPath: () => tryGetPlatform()?.app.getAppPath?.() ?? process.cwd(),
  get isPackaged() {
    return tryGetPlatform()?.app.isPackaged ?? true
  },
  whenReady: () => tryGetPlatform()?.app.whenReady() ?? Promise.resolve(),
  quit: () => tryGetPlatform()?.app.quit() ?? process.exit(0),
  exit: (code?: number) => tryGetPlatform()?.app.exit(code) ?? process.exit(code ?? 0),
  relaunch: () => console.warn('[ElectronStub] relaunch() is a no-op in server mode'),
  setName: (_name: string) => {},
  disableHardwareAcceleration: () => {},
  // dock (macOS) — no-op in server mode
  dock: {
    hide: () => {},
    show: () => {},
    setIcon: () => {},
    setBadge: (_txt: string) => {}
  },
  requestSingleInstanceLock: () => true,
  hasSingleInstanceLock: () => true,
  setPath: (_name: string, _p: string) => {},
  getLocale: () => 'en-US',
  commandLine: {
    appendSwitch: (_sw: string, _val?: string) => {},
    getSwitchValue: (_sw: string) => ''
  },
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
  emit: (event: string, ...args: any[]): boolean => {
    return tryGetPlatform()?.app.emit?.(event, ...args) ?? false
  },
  removeListener: (event: string, listener: (...args: any[]) => void) => {
    tryGetPlatform()?.app.off(event as any, listener)
    return app
  },
  removeAllListeners: (_event?: string) => {
    return app
  }
}

// ─── ipcMain ─────────────────────────────────────────────────────────────────
export const ipcMain = {
  handle: (channel: string, listener: any) => {
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
  emit: (_channel: string, ..._args: any[]) => false,
  removeListener: (channel: string, listener: any) => {
    tryGetPlatform()?.ipc.off(channel, listener)
    return ipcMain
  },
  removeAllListeners: (_channel?: string) => {
    return ipcMain
  }
}

// ─── BrowserWindow ───────────────────────────────────────────────────────────
export class BrowserWindow extends EventEmitter {
  private _win: any = null
  id: number = -1

  constructor(options?: any) {
    super()
    const platform = tryGetPlatform()
    if (platform) {
      this._win = platform.windowManager.createWindow(options ?? {})
      this.id = this._win.id
      // Forward window events to BrowserWindow EventEmitter
      this._win.on('closed', () => this.emit('closed'))
    } else {
      console.warn('[ElectronStub] BrowserWindow created before platform init')
      this.id = Math.floor(Math.random() * 10000)
    }
  }

  get webContents(): any {
    return {
      id: this.id,
      send: (channel: string, ...args: any[]) => {
        this._win?.send?.(channel, ...args)
      },
      getURL: () => '',
      getTitle: () => '',
      isDestroyed: () => this._win?.isDestroyed() ?? true,
      openDevTools: () => {},
      closeDevTools: () => {},
      isDevToolsOpened: () => false,
      loadURL: async () => {},
      loadFile: async () => {},
      invalidate: () => {},
      setBackgroundThrottling: () => {},
      setZoomLevel: () => {},
      setWindowOpenHandler: () => {},
      reloadIgnoringCache: () => {},
      isCrashed: () => false,
      session: {
        webRequest: { onBeforeSendHeaders: () => {} },
        getUserAgent: () => 'OrcaServer/1.0',
        setUserAgent: () => {},
        clearStorageData: async () => {},
        clearCache: async () => {},
        setPermissionRequestHandler: () => {},
        setPermissionCheckHandler: () => {},
        setDisplayMediaRequestHandler: () => {},
        setDevicePermissionHandler: () => {},
        addWordToSpellCheckerDictionary: () => {},
        removeListener: () => {},
        on: () => {},
        fromPartition: (_partition: string) => ({
          getUserAgent: () => 'OrcaServer/1.0',
          cookies: {
            get: async () => [],
            set: async () => {},
            remove: async () => {}
          }
        })
      }
    }
  }

  isDestroyed(): boolean {
    return this._win?.isDestroyed() ?? true
  }
  isMinimized(): boolean {
    return this._win?.isMinimized() ?? false
  }
  isMaximized(): boolean {
    return this._win?.isMaximized() ?? false
  }
  isFullScreen(): boolean {
    return this._win?.isFullScreen() ?? false
  }
  isVisible(): boolean {
    return this._win?.isVisible() ?? true
  }
  isFocused(): boolean {
    return this._win?.isFocused() ?? true
  }

  show(): void {
    this._win?.show?.()
  }
  hide(): void {
    this._win?.hide?.()
  }
  focus(): void {
    this._win?.focus?.()
  }
  restore(): void {
    this._win?.restore?.()
  }
  close(): void {
    this._win?.close?.()
  }
  destroy(): void {
    this._win?.destroy?.()
  }
  blur(): void {}
  maximize(): void {}
  unmaximize(): void {}
  minimize(): void {}
  setFullScreen(_flag: boolean): void {}
  setAlwaysOnTop(_flag: boolean): void {}
  setOpacity(_opacity: number): void {}
  center(): void {}
  flashFrame(_flag: boolean): void {}
  setProgressBar(_progress: number): void {}
  setThumbarButtons(_buttons: any[]): void {}
  setTitle(_title: string): void {}
  setMenu(_menu: any): void {}
  setMenuBarVisibility(_visible: boolean): void {}
  setAutoHideMenuBar(_hide: boolean): void {}

  send(channel: string, ...args: any[]): void {
    this._win?.send?.(channel, ...args)
  }

  loadURL(_url: string): Promise<void> {
    return Promise.resolve()
  }

  loadFile(_filePath: string): Promise<void> {
    return Promise.resolve()
  }

  setBounds(_bounds: any): void {}
  getBounds() {
    return { x: 0, y: 0, width: 1280, height: 800 }
  }
  getSize(): [number, number] {
    return [1280, 800]
  }
  getContentSize(): [number, number] {
    return [1280, 800]
  }
  setSize(_w: number, _h: number): void {}
  setMinimumSize(_w: number, _h: number): void {}
  setMaximumSize(_w: number, _h: number): void {}
  setResizable(_val: boolean): void {}

  static getAllWindows(): BrowserWindow[] {
    const platform = tryGetPlatform()
    if (!platform) {return []}
    return platform.windowManager.getAllWindows().map((w) => {
      const bw = new BrowserWindow()
      bw._win = w
      bw.id = w.id
      return bw
    })
  }

  static getFocusedWindow(): BrowserWindow | null {
    const win = tryGetPlatform()?.windowManager.getFocusedWindow()
    if (!win) {return null}
    const bw = new BrowserWindow()
    bw._win = win
    bw.id = win.id
    return bw
  }

  static fromWebContents(_wc: any): BrowserWindow | null {
    return null
  }

  static fromId(_id: number): BrowserWindow | null {
    return null
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
  getCursorScreenPoint: () => ({ x: 0, y: 0 }),
  on: (_event: string, _listener: any) => {},
  removeListener: (_event: string, _listener: any) => {}
}

// ─── nativeTheme ─────────────────────────────────────────────────────────────
export const nativeTheme = new EventEmitter() as EventEmitter & {
  shouldUseDarkColors: boolean
  themeSource: string
}
;(nativeTheme as any).shouldUseDarkColors = false
;(nativeTheme as any).themeSource = 'system'

// ─── shell ───────────────────────────────────────────────────────────────────
export const shell = {
  openExternal: (url: string): Promise<void> => {
    console.warn('[ElectronStub] shell.openExternal() is a no-op in server mode:', url)
    return Promise.resolve()
  },
  openPath: (path: string): Promise<string> => {
    console.warn('[ElectronStub] shell.openPath() is a no-op in server mode:', path)
    return Promise.resolve('')
  },
  showItemInFolder: (_path: string): void => {},
  beep: (): void => {}
}

// ─── dialog ──────────────────────────────────────────────────────────────────
export const dialog = {
  showOpenDialog: async (_win: any, _opts?: any) => ({ canceled: true, filePaths: [] }),
  showSaveDialog: async (_win: any, _opts?: any) => ({ canceled: true, filePath: '' }),
  showMessageBox: async (_win: any, _opts?: any) => ({ response: 0, checkboxChecked: false }),
  showErrorBox: (_title: string, content: string) => console.error('[Dialog]', content)
}

// ─── safeStorage ─────────────────────────────────────────────────────────────
export const safeStorage = {
  isEncryptionAvailable: () => tryGetPlatform()?.storage.isEncryptionAvailable() ?? false,
  encryptString: (text: string): Buffer =>
    tryGetPlatform()?.storage.encryptString(text) ?? Buffer.from(text, 'utf-8'),
  decryptString: (encrypted: Buffer): string =>
    tryGetPlatform()?.storage.decryptString(encrypted) ?? encrypted.toString('utf-8')
}

// ─── systemPreferences ───────────────────────────────────────────────────────
export const systemPreferences = {
  getUserDefault: (_key: string, _type?: string) => null,
  subscribeNotification: (_event: string, _callback: any) => {},
  unsubscribeNotification: (_id: number) => {},
  getMediaAccessStatus: (_type: string) => 'granted' as const,
  askForMediaAccess: async (_type: string) => true,
  getColor: (_color: string) => '#000000'
}

// ─── net ─────────────────────────────────────────────────────────────────────
export const net = {
  isOnline: () => true,
  request: (_options: any) => {
    console.warn('[ElectronStub] net.request() not available in server mode')
    return null
  }
}

// ─── session ─────────────────────────────────────────────────────────────────
const mockSession = {
  webRequest: { onBeforeSendHeaders: () => {} },
  getUserAgent: () => 'OrcaServer/1.0',
  setUserAgent: () => {},
  clearStorageData: async () => {},
  clearCache: async () => {},
  setPermissionRequestHandler: () => {},
  setPermissionCheckHandler: () => {},
  setDisplayMediaRequestHandler: () => {},
  setDevicePermissionHandler: () => {},
  addWordToSpellCheckerDictionary: () => {},
  cookies: {
    get: async () => [],
    set: async () => {},
    remove: async () => {}
  },
  removeListener: () => {},
  on: () => {}
}

export const session = {
  defaultSession: mockSession,
  fromPartition: (_partition: string) => mockSession
}

// ─── powerMonitor ────────────────────────────────────────────────────────────
export const powerMonitor = new EventEmitter()

// ─── powerSaveBlocker ────────────────────────────────────────────────────────
export const powerSaveBlocker = {
  start: (_type: string) => 1,
  stop: (_id: number) => {},
  isStarted: (_id: number) => false
}

// ─── webContents ─────────────────────────────────────────────────────────────
export const webContents = {
  getAllWebContents: () => [],
  getFocusedWebContents: () => null,
  fromId: (_id: number) => null
}

// ─── nativeImage ─────────────────────────────────────────────────────────────
const mockImage = {
  isEmpty: () => false,
  resize: () => mockImage,
  toPNG: () => Buffer.from([]),
  toJPEG: () => Buffer.from([]),
  toDataURL: () => '',
  getBitmap: () => Buffer.from([]),
  getNativeHandle: () => Buffer.from([])
}

export const nativeImage = {
  createEmpty: () => mockImage,
  createFromPath: (_path: string) => mockImage,
  createFromBuffer: (_buffer: Buffer, _opts?: any) => mockImage,
  createFromDataURL: (_dataURL: string) => mockImage,
  createFromBitmap: (_bitmap: any, _opts: any) => mockImage
}

// ─── Tray ────────────────────────────────────────────────────────────────────
export class Tray extends EventEmitter {
  constructor(_icon: any) {
    super()
  }
  setContextMenu(_menu: any): void {}
  setToolTip(_tip: string): void {}
  setTitle(_title: string): void {}
  destroy(): void {}
}

// ─── Menu / MenuItem ──────────────────────────────────────────────────────────
export class Menu {
  static buildFromTemplate(_template: any[]): Menu {
    return new Menu()
  }
  static setApplicationMenu(_menu: Menu | null): void {}
  static getApplicationMenu(): Menu {
    return new Menu()
  }
  popup(_opts?: any): void {}
  append(_item: any): void {}
}

export class MenuItem {
  constructor(_options: any) {}
}

// ─── Notification ─────────────────────────────────────────────────────────────
export class Notification extends EventEmitter {
  constructor(_options?: any) {
    super()
  }
  show(): void {
    console.info('[ElectronStub] Notification.show() — server mode')
  }
  close(): void {}
  static isSupported(): boolean {
    return false
  }
}

// ─── BaseWindow / WebContentsView ─────────────────────────────────────────────
export class BaseWindow extends EventEmitter {
  constructor() {
    super()
  }
}

export class WebContentsView {
  webContents: any = {}
  constructor() {}
}

// ─── autoUpdater ─────────────────────────────────────────────────────────────
export const autoUpdater = new EventEmitter()

// ─── clipboard ────────────────────────────────────────────────────────────────
export const clipboard = {
  readText: () => '',
  writeText: (_text: string) => {},
  readImage: () => null,
  writeImage: (_image: any) => {},
  clear: () => {}
}

// ─── globalShortcut ──────────────────────────────────────────────────────────
export const globalShortcut = {
  register: (_accel: string, _callback: () => void) => false,
  unregister: (_accel: string) => {},
  unregisterAll: () => {},
  isRegistered: (_accel: string) => false
}

// ─── crashReporter ───────────────────────────────────────────────────────────
export const crashReporter = {
  start: (_opts?: any) => {}
}

// Default export — for `import electron from 'electron'` usage
const electronStub = {
  app,
  ipcMain,
  BrowserWindow,
  screen,
  nativeTheme,
  shell,
  dialog,
  safeStorage,
  systemPreferences,
  powerMonitor,
  powerSaveBlocker,
  net,
  session,
  webContents,
  nativeImage,
  Tray,
  Menu,
  MenuItem,
  Notification,
  BaseWindow,
  WebContentsView,
  autoUpdater,
  clipboard,
  globalShortcut,
  crashReporter
}

export default new Proxy(electronStub, {
  get(target, prop) {
    if (prop in target) {
      return target[prop as keyof typeof target]
    }
    console.warn(`[ElectronStub] Accessed unimplemented property: ${String(prop)}`)
    return {}
  }
})
