/**
 * Node.js Platform Adapter
 *
 * Factory cho IPlatformServices trong môi trường server (non-Electron).
 *
 * Usage in src/server/index.ts:
 *   import { createNodeAdapter } from '../platform/adapters/node'
 *   import { setPlatform } from '../platform/context'
 *   setPlatform(createNodeAdapter())
 */
import type { IPlatformServices } from '../../types'
import type { NodeAppOptions } from './app'
import { NodeApp } from './app'
import { NodeWindowManager } from './window'
import { NodeIpcBridge } from './ipc'
import { NodeSecureStorage } from './storage'
import { NodeSystemInfo } from './system'

export type { NodeAppOptions }
export { NodeApp } from './app'
export { NodeWindow, NodeWindowManager } from './window'
export { NodeIpcBridge } from './ipc'
export { NodeSecureStorage } from './storage'
export { NodeSystemInfo } from './system'

/**
 * Create a complete IPlatformServices for Node.js server mode.
 *
 * @param options - Optional configuration (mainly userDataPath)
 */
export function createNodeAdapter(options: NodeAppOptions = {}): IPlatformServices {
  const app = new NodeApp(options)
  const windowManager = new NodeWindowManager()
  const ipc = new NodeIpcBridge(windowManager)
  const storage = new NodeSecureStorage(app)
  const system = new NodeSystemInfo()

  return {
    mode: 'node',
    app,
    ipc,
    windowManager,
    storage,
    system
  }
}
