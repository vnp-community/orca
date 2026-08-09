/**
 * Platform Abstraction Layer — Core Types
 *
 * Defines the contract between Orca business logic and platform runtime.
 * Implementations: ElectronAdapter (desktop) and NodeAdapter (server).
 *
 * @module platform/types
 */

import type { IApp } from './app-interface'
import type { IIpcBridge } from './ipc-interface'
import type { IWindowManager } from './window-interface'
import type { ISecureStorage } from './storage-interface'
import type { ISystemInfo } from './system-interface'

/** Discriminator for the current runtime mode */
export type PlatformMode = 'electron' | 'node'

/** All platform services bundled into one injectable object */
export interface IPlatformServices {
  readonly mode: PlatformMode
  readonly app: IApp
  readonly ipc: IIpcBridge
  readonly windowManager: IWindowManager
  readonly storage: ISecureStorage
  readonly system: ISystemInfo
}
