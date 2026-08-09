/**
 * Platform Abstraction Layer — Public API
 *
 * Import platform types and context from this entry point.
 * Do NOT import directly from sub-files in application code.
 */

export type { PlatformMode, IPlatformServices } from './types'
export type { IApp, AppPathName, AppEvent } from './app-interface'
export type {
  IWindow,
  IWindowManager,
  WindowCreationOptions,
  WindowEvent
} from './window-interface'
export type { IIpcBridge, IpcHandler, IpcListener, IpcEvent } from './ipc-interface'
export type { ISecureStorage } from './storage-interface'
export type { ISystemInfo } from './system-interface'
export {
  setPlatform,
  getPlatform,
  isPlatformInitialized,
  _resetPlatformForTesting
} from './context'
