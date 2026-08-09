/**
 * @deprecated Use src/platform/stubs/electron-node-wrapper.ts for server mode.
 * This file is kept for legacy Electron-mode testing and compatibility only.
 * New code should import from the platform abstraction layer.
 */
// src/main/mocks/electron.ts
// Mock for Electron APIs used by Orca backend when running as a pure Node.js daemon.

import { EventEmitter } from 'events'
import path from 'path'
import os from 'os'

// ── App ────────────────────────────────────────────────────────
class App extends EventEmitter {
  isPackaged = true
  
  getPath(name: string): string {
    const home = os.homedir()
    switch (name) {
      case 'userData': return path.join(home, '.orca')
      case 'appData': return path.join(home, '.config')
      case 'home': return home
      case 'temp': return os.tmpdir()
      case 'exe': return process.execPath
      case 'module': return __dirname
      case 'desktop': return path.join(home, 'Desktop')
      case 'documents': return path.join(home, 'Documents')
      case 'downloads': return path.join(home, 'Downloads')
      case 'music': return path.join(home, 'Music')
      case 'pictures': return path.join(home, 'Pictures')
      case 'videos': return path.join(home, 'Videos')
      default: return path.join(home, '.orca', name)
    }
  }

  getVersion(): string {
    return process.env.ORCA_VERSION || '1.0.0'
  }

  getLocale(): string {
    return 'en-US'
  }

  getAppPath(): string {
    return __dirname
  }

  setPath(_name: string, _p: string): void {
    // noop
  }
  
  whenReady(): Promise<void> {
    return Promise.resolve()
  }

  quit(): void {
    process.exit(0)
  }

  exit(code?: number): void {
    process.exit(code || 0)
  }
  
  dock = {
    hide: () => {},
    show: () => {},
    setIcon: () => {},
    setBadge: () => {}
  }
  
  requestSingleInstanceLock(): boolean {
    return true
  }
  
  hasSingleInstanceLock(): boolean {
    return true
  }

  setName(_name: string): void {}
  disableHardwareAcceleration(): void {}
  relaunch(): void {}
  
  commandLine = {
    appendSwitch: () => {},
    getSwitchValue: () => ''
  }
}

export const app = new App()

// ── safeStorage ────────────────────────────────────────────────
export const safeStorage = {
  isEncryptionAvailable: () => false,
  encryptString: (plaintext: string) => Buffer.from(plaintext, 'utf-8'),
  decryptString: (encrypted: Buffer) => encrypted.toString('utf-8')
}

// ── ipcMain ────────────────────────────────────────────────────
class IpcMain extends EventEmitter {
  handle(channel: string, listener: (event: any, ...args: any[]) => any): void {
    this.on(channel, async (event, ...args) => {
      try {
        const result = await listener(event, ...args)
        event.sender.send(`${channel}-reply`, { result })
      } catch (error) {
        event.sender.send(`${channel}-reply`, { error: String(error) })
      }
    })
  }

  removeHandler(channel: string): void {
    this.removeAllListeners(channel)
  }
}

export const ipcMain = new IpcMain()

// ── BrowserWindow ──────────────────────────────────────────────
export class BrowserWindow extends EventEmitter {
  static getAllWindows() { return [] }
  static fromWebContents() { return null }
  static fromId() { return null }
  static getFocusedWindow() { return null }

  webContents: any
  id: number

  constructor(_options?: any) {
    super()
    this.id = Math.floor(Math.random() * 10000)
    this.webContents = new EventEmitter()
    this.webContents.send = (channel: string, ..._args: any[]) => {
      console.log(`[MockBrowserWindow] send: ${channel}`)
    }
    this.webContents.openDevTools = () => {}
    this.webContents.loadURL = async () => {}
    this.webContents.loadFile = async () => {}
    this.webContents.invalidate = () => {}
    this.webContents.setBackgroundThrottling = () => {}
    this.webContents.isDestroyed = () => false
    this.webContents.setZoomLevel = () => {}
    this.webContents.setWindowOpenHandler = () => {}
    this.webContents.reloadIgnoringCache = () => {}
    this.webContents.isDevToolsOpened = () => false
    this.webContents.closeDevTools = () => {}
    this.webContents.isCrashed = () => false
    this.webContents.session = mockSessionObject
  }

  loadURL(_url: string): void {}
  loadFile(_file: string): void {}
  show(): void {}
  hide(): void {}
  close(): void {}
  destroy(): void {}
  isDestroyed(): boolean { return false }
  focus(): void {}
  blur(): void {}
  restore(): void {}
  setAlwaysOnTop(_flag: boolean): void {}
  isVisible(): boolean { return true }
  isMinimized(): boolean { return false }
  isMaximized(): boolean { return false }
  isFocused(): boolean { return true }
  maximize(): void {}
  unmaximize(): void {}
  minimize(): void {}
  setFullScreen(_flag: boolean): void {}
  isFullScreen(): boolean { return false }
  setOpacity(_opacity: number): void {}
  setBounds(_bounds: any): void {}
  getBounds() { return { x: 0, y: 0, width: 800, height: 600 } }
  getSize(): [number, number] { return [800, 600] }
  setSize(_w: number, _h: number): void {}
  setMinimumSize(_w: number, _h: number): void {}
  center(): void {}
  setTitle(_title: string): void {}
  setMenu(_menu: any): void {}
  setProgressBar(_progress: number): void {}
  flashFrame(_flag: boolean): void {}
}

// ── Other stubs ────────────────────────────────────────────────
export const dialog = {
  showOpenDialog: async () => ({ canceled: true, filePaths: [] }),
  showSaveDialog: async () => ({ canceled: true }),
  showMessageBox: async () => ({ response: 0 })
}

export const nativeTheme = new EventEmitter()
;(nativeTheme as any).shouldUseDarkColors = true

export const shell = {
  openExternal: async (_url: string) => {},
  openPath: async (_p: string) => '',
  showItemInFolder: (_p: string) => {}
}

export const clipboard = {
  readText: () => '',
  writeText: (_text: string) => {}
}

export const Menu = {
  buildFromTemplate: () => ({ popup: () => {} }),
  setApplicationMenu: () => {},
  getApplicationMenu: () => ({ popup: () => {} })
}

export const Tray = class extends EventEmitter {
  constructor() { super() }
  setToolTip() {}
  setContextMenu() {}
}

const mockSessionObject = {
  webRequest: {
    onBeforeSendHeaders: () => {}
  },
  getUserAgent: () => 'MockUA',
  setUserAgent: () => {},
  clearStorageData: async () => {},
  clearCache: async () => {},
  setPermissionRequestHandler: () => {},
  setPermissionCheckHandler: () => {},
  setDisplayMediaRequestHandler: () => {},
  setDevicePermissionHandler: () => {},
  addWordToSpellCheckerDictionary: () => {},
  removeListener: () => {},
  on: () => {}
}

export const session = {
  defaultSession: mockSessionObject,
  fromPartition: () => mockSessionObject
}

export const systemPreferences = {
  getUserDefault: () => null,
  subscribeNotification: () => {},
  unsubscribeNotification: () => {}
}

export const net = {
  isOnline: () => true
}

export const webContents = {
  getAllWebContents: () => [],
  getFocusedWebContents: () => null,
  fromId: () => null
}

export class Notification extends EventEmitter {
  constructor(_options?: any) {
    super()
  }
  show() {}
  close() {}
}

export class BaseWindow extends EventEmitter {
  constructor() {
    super()
  }
}

export class WebContentsView {
  webContents: any
  constructor() {
    this.webContents = {}
  }
}

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
  createFromPath: () => mockImage,
  createFromBuffer: () => mockImage,
  createFromDataURL: () => mockImage,
  createFromBitmap: () => mockImage
}

export const powerMonitor = new EventEmitter()

export const autoUpdater = new EventEmitter()

export const powerSaveBlocker = {
  start: () => 1,
  stop: () => {},
  isStarted: () => false
}

const electronMock = {
  app,
  safeStorage,
  ipcMain,
  BrowserWindow,
  dialog,
  nativeTheme,
  shell,
  clipboard,
  Menu,
  Tray,
  session,
  systemPreferences,
  net,
  powerMonitor,
  nativeImage,
  webContents,
  Notification,
  BaseWindow,
  WebContentsView,
  autoUpdater,
  powerSaveBlocker,
  screen: {
    getPrimaryDisplay: () => ({ workAreaSize: { width: 1920, height: 1080 } }),
    getAllDisplays: () => [],
    on: () => {},
    removeListener: () => {}
  }
}

export const screen = electronMock.screen

export default new Proxy(electronMock, {
  get(target, prop) {
    if (prop in target) {
      return target[prop as keyof typeof target]
    }
    console.warn(`[Electron Mock] Accessed unimplemented property: ${String(prop)}`)
    return {}
  }
})
