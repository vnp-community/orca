import { app } from 'electron'
import { join } from 'node:path'

// Why: the data-file path must not be a module-level constant. Module-level
// code runs at import time — before configureDevUserDataPath() redirects the
// userData path in index.ts — so a constant would capture the default (non-dev)
// path, causing dev and production instances to share the same file and silently
// overwrite each other.
//
// It also must not be resolved lazily on every call, because app.setName('Orca')
// runs before the Store constructor and would change the resolved path from
// lowercase 'orca' to uppercase 'Orca'. On case-sensitive filesystems (Linux)
// this would look in the wrong directory and lose existing user data.
//
// Solution: index.ts calls initDataPath() right after configureDevUserDataPath()
// but before app.setName(), capturing the correct path at the right moment.
let _dataFile: string | null = null
let _userDataDir: string | null = null

export function initDataPath(): void {
  // ORCA_DATA_DIR: allows headless/container deployments to redirect data storage.
  // Must be set before initDataPath() is called (i.e., before app.setName()).
  const userDataDir = process.env.ORCA_DATA_DIR ?? app.getPath('userData')
  _userDataDir = userDataDir
  _dataFile = join(userDataDir, 'orca-data.json')
}

// Not part of this module's public API (kept private, as in the original
// persistence.ts) — exported only so persistence.ts can import it for Store's
// internal use. It shares module-private _dataFile with initDataPath() and
// getCanonicalUserDataPath(), so it must not be split away from them: doing so
// would leave _dataFile permanently unset from this function's perspective and
// silently drop the ORCA_DATA_DIR override on every fallback call.
export function getDataFile(): string {
  if (!_dataFile) {
    // Safety fallback — should not be hit in normal startup.
    const userDataDir = app.getPath('userData')
    _userDataDir = userDataDir
    _dataFile = join(userDataDir, 'orca-data.json')
  }
  return _dataFile
}

/**
 * Return the userData directory captured at initDataPath() time, before
 * app.setName() can change how app.getPath('userData') resolves.
 *
 * Subsystems that must share storage with orca-data.json (mobile pairing's
 * DeviceRegistry, E2EE keypair, runtime metadata) read this instead of
 * resolving the path late, which on case-sensitive filesystems can land in a
 * different directory and lose paired devices across restarts/updates.
 */
export function getCanonicalUserDataPath(): string {
  if (!_userDataDir) {
    // Safety fallback — should not be hit in normal startup.
    _userDataDir = app.getPath('userData')
  }
  return _userDataDir
}
