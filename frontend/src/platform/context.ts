/**
 * Platform Context — Singleton accessor
 *
 * Call setPlatform() once at startup before loading any src/main/ code.
 * All subsequent calls to getPlatform() return the same instance.
 *
 * @module platform/context
 */

import type { IPlatformServices } from './types'

let _platform: IPlatformServices | null = null

/**
 * Initialize the platform singleton.
 * @throws Error if called more than once.
 */
export function setPlatform(services: IPlatformServices): void {
  if (_platform !== null) {
    throw new Error(
      '[Platform] Platform already initialized. setPlatform() must only be called once.'
    )
  }
  _platform = services
}

/**
 * Retrieve the current platform services.
 * @throws Error if called before setPlatform().
 */
export function getPlatform(): IPlatformServices {
  if (_platform === null) {
    throw new Error(
      '[Platform] Platform not initialized. Call setPlatform() before using getPlatform().'
    )
  }
  return _platform
}

/**
 * Check whether platform has been initialized.
 * Safe to call at any time — never throws.
 */
export function isPlatformInitialized(): boolean {
  return _platform !== null
}

/**
 * Reset platform singleton.
 * FOR TESTING ONLY — do not use in production code.
 * @internal
 */
export function _resetPlatformForTesting(): void {
  _platform = null
}
