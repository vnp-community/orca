import { EventEmitter } from 'node:events'
import { mkdirSync } from 'node:fs'
import { join, resolve } from 'node:path'

import { homedir, tmpdir } from 'node:os'
import type { IApp, AppPathName, AppEvent } from '../../app-interface'

export interface NodeAppOptions {
  userDataPath?: string
}

/**
 * NodeApp — IApp implementation for Node.js server mode.
 *
 * Provides file system paths, version info, and lifecycle events
 * without any Electron dependency.
 */
export class NodeApp extends EventEmitter implements IApp {
  readonly isPackaged: boolean = true

  private readonly _userDataPath: string

  constructor(options: NodeAppOptions = {}) {
    super()
    this._userDataPath =
      options.userDataPath ??
      process.env.ORCA_USER_DATA_PATH ??
      join(homedir(), '.orca')

    // Ensure userData directory exists
    mkdirSync(this._userDataPath, { recursive: true })
  }

  getVersion(): string {
    return process.env.ORCA_VERSION ?? '0.0.0'
  }

  getPath(name: AppPathName): string {
    const home = homedir()
    switch (name) {
      case 'userData':
        return this._userDataPath
      case 'appData':
        return join(home, '.config')
      case 'home':
        return home
      case 'temp':
        return tmpdir()
      case 'exe':
        return process.execPath
      case 'module':
        return __dirname
      case 'desktop':
        return join(home, 'Desktop')
      case 'documents':
        return join(home, 'Documents')
      case 'downloads':
        return join(home, 'Downloads')
      case 'music':
        return join(home, 'Music')
      case 'pictures':
        return join(home, 'Pictures')
      case 'videos':
        return join(home, 'Videos')
      default:
        return join(this._userDataPath, name)
    }
  }

  getAppPath(): string {
    // Why: ORCA_APP_PATH allows explicit override (e.g. in Docker / tests).
    if (process.env.ORCA_APP_PATH) return resolve(process.env.ORCA_APP_PATH)
    // Why: In server mode, compiled output is at <appRoot>/out/server/index.js.
    // __dirname here is <appRoot>/out/server (or deeper in sub-modules).
    // We find the app root by walking up to the directory that contains `out/`.
    // Specifically: out/server → out → appRoot (2 levels up from out/server/).
    // Using require.main?.filename is more reliable than __dirname for the
    // root entry point, but __dirname in *this* file (node/app.ts) compiles to
    // out/platform/adapters/node → 4 levels up to appRoot.
    return resolve(__dirname, '..', '..', '..', '..')
  }

  async whenReady(): Promise<void> {
    // In Node mode, always "ready"
    return Promise.resolve()
  }

  quit(): void {
    this.emit('before-quit')
    this.emit('will-quit')
    process.exit(0)
  }

  exit(code = 0): void {
    process.exit(code)
  }

  relaunch(): void {
    console.warn('[NodeApp] relaunch() is a no-op in Node server mode')
  }

  setName(_name: string): void {
    // no-op in Node mode
  }

  disableHardwareAcceleration(): void {
    // no-op in Node mode
  }

  // EventEmitter implements on/off/once/emit — explicit overrides for type compat
  on(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.on(event, listener)
  }

  off(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.off(event, listener)
  }

  once(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.once(event, listener)
  }

  emit(event: AppEvent | string, ...args: any[]): boolean {
    return super.emit(event, ...args)
  }
}
