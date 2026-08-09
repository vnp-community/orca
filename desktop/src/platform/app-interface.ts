/**
 * IApp — abstraction over Electron's `app` module.
 * NodeAdapter implements this without any Electron dependency.
 */
export type IApp = {
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
  emit(event: AppEvent | string, ...args: any[]): boolean
}

export type AppPathName =
  | 'userData' | 'appData' | 'home' | 'temp'
  | 'exe' | 'module' | 'desktop' | 'documents'
  | 'downloads' | 'music' | 'pictures' | 'videos'
  | string // fallback

export type AppEvent =
  | 'ready' | 'quit' | 'before-quit' | 'will-quit'
  | 'activate' | 'open-url' | 'second-instance'
  | string
