/**
 * electron-web-stub.ts — No-op stub for web browser mode.
 *
 * In web mode (SPA served to a browser), Electron APIs are not available.
 * This stub ensures that any accidental `import { app } from 'electron'`
 * in shared code does not crash the web bundle.
 *
 * The real web frontend API contract is exposed via `window.api`
 * (provided by the WebSocket IPC layer, not by this stub).
 *
 * @see src/platform/stubs/electron-node-wrapper.ts for server-mode stub
 * @see src/platform/adapters/node/ for the NodeAdapter implementation
 */

export const app = {
  getVersion: (): string => '0.0.0',
  getPath: (_name: string): string => '',
  getAppPath: (): string => '',
  isPackaged: false as boolean,
  whenReady: (): Promise<void> => Promise.resolve(),
  quit: (): void => {},
  exit: (_code?: number): void => {},
  on: (): typeof app => app,
  off: (): typeof app => app,
  once: (): typeof app => app,
  removeListener: (): typeof app => app,
  emit: (): boolean => false
}

export const ipcMain = {
  handle: (_channel: string, _listener: any): void => {},
  removeHandler: (_channel: string): void => {},
  on: (_channel: string, _listener: any): typeof ipcMain => ipcMain,
  off: (_channel: string, _listener: any): typeof ipcMain => ipcMain,
  once: (_channel: string, _listener: any): typeof ipcMain => ipcMain,
  removeListener: (_channel: string, _listener: any): typeof ipcMain => ipcMain,
  emit: (): boolean => false
}

export class BrowserWindow {
  id: number = -1
  webContents = {
    id: -1,
    send: (_channel: string, ..._args: any[]): void => {},
    isDestroyed: (): boolean => true,
    getURL: (): string => '',
    getTitle: (): string => ''
  }

  constructor(_opts?: any) {}

  static getAllWindows(): BrowserWindow[] { return [] }
  static getFocusedWindow(): BrowserWindow | null { return null }
  static fromWebContents(_wc: any): BrowserWindow | null { return null }
  static fromId(_id: number): BrowserWindow | null { return null }

  on(): this { return this }
  off(): this { return this }
  once(): this { return this }
  show(): void {}
  hide(): void {}
  close(): void {}
  destroy(): void {}
  focus(): void {}
  restore(): void {}
  isDestroyed(): boolean { return true }
  isVisible(): boolean { return false }
  isMinimized(): boolean { return false }
  isMaximized(): boolean { return false }
  isFocused(): boolean { return false }
  isFullScreen(): boolean { return false }
  loadURL(_url: string): Promise<void> { return Promise.resolve() }
  loadFile(_file: string): Promise<void> { return Promise.resolve() }
  setBounds(_bounds: any): void {}
  getBounds() { return { x: 0, y: 0, width: 0, height: 0 } }
  send(_channel: string, ..._args: any[]): void {}
}

export const shell = {
  openExternal: (_url: string): Promise<void> => Promise.resolve(),
  openPath: (_path: string): Promise<string> => Promise.resolve(''),
  showItemInFolder: (_path: string): void => {}
}

export const dialog = {
  showOpenDialog: async (_win: any, _opts?: any) => ({ canceled: true, filePaths: [] }),
  showSaveDialog: async (_win: any, _opts?: any) => ({ canceled: true, filePath: '' }),
  showMessageBox: async (_win: any, _opts?: any) => ({ response: 0, checkboxChecked: false })
}

export const safeStorage = {
  isEncryptionAvailable: (): boolean => false,
  encryptString: (s: string): Buffer => Buffer.from(s, 'utf-8'),
  decryptString: (b: Buffer): string => b.toString('utf-8')
}

export const nativeTheme = {
  shouldUseDarkColors: false as boolean,
  themeSource: 'system' as string,
  on: (): void => {},
  off: (): void => {},
  emit: (): boolean => false
}

export const clipboard = {
  readText: (): string => '',
  writeText: (_text: string): void => {},
  readImage: (): null => null,
  writeImage: (_image: any): void => {},
  clear: (): void => {}
}

export const screen = {
  getPrimaryDisplay: () => ({
    workAreaSize: { width: 0, height: 0 },
    bounds: { x: 0, y: 0, width: 0, height: 0 },
    scaleFactor: 1
  }),
  getAllDisplays: (): any[] => []
}

export const net = {
  isOnline: (): boolean => typeof window !== 'undefined' ? window.navigator.onLine : true
}

export const session = {
  defaultSession: null as any,
  fromPartition: (_partition: string): any => null
}

export const powerMonitor = {
  on: (): void => {},
  off: (): void => {},
  getSystemIdleState: (_threshold: number): string => 'active',
  getSystemIdleTime: (): number => 0
}

export const systemPreferences = {
  getUserDefault: (_key: string, _type?: string): null => null,
  getMediaAccessStatus: (_type: string): string => 'granted',
  askForMediaAccess: async (_type: string): Promise<boolean> => true
}

export const autoUpdater = {
  on: (): void => {},
  off: (): void => {},
  checkForUpdates: (): void => {},
  setFeedURL: (): void => {}
}

export class Menu {
  static buildFromTemplate(_template: any[]): Menu { return new Menu() }
  static setApplicationMenu(_menu: any): void {}
  popup(_opts?: any): void {}
}

export class Notification {
  constructor(_options?: any) {}
  show(): void {}
  close(): void {}
  static isSupported(): boolean { return false }
  on(): this { return this }
}

export class Tray {
  constructor(_icon: any) {}
  setContextMenu(_menu: any): void {}
  setToolTip(_tip: string): void {}
  destroy(): void {}
  on(): this { return this }
}

export const globalShortcut = {
  register: (_accel: string, _callback: () => void): boolean => false,
  unregister: (_accel: string): void => {},
  unregisterAll: (): void => {},
  isRegistered: (_accel: string): boolean => false
}

const electronWebStub = {
  app, ipcMain, BrowserWindow, shell, dialog, safeStorage, nativeTheme,
  clipboard, screen, net, session, powerMonitor, systemPreferences,
  autoUpdater, Menu, Notification, Tray, globalShortcut
}

export default new Proxy(electronWebStub, {
  get(target, prop) {
    if (prop in target) {return target[prop as keyof typeof target]}
    // Silent in web mode — no warning needed
    return {}
  }
})
