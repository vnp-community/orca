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
  [key: string]: any // passthrough for Electron-specific options
}

/** IWindowManager — factory and registry for windows */
export interface IWindowManager {
  createWindow(options: WindowCreationOptions): IWindow
  getAllWindows(): IWindow[]
  getFocusedWindow(): IWindow | null
  getMainWindow(): IWindow | null
  setMainWindow(window: IWindow | null): void
}
