# TASK-001: Tạo `src/platform/types.ts` + `src/platform/context.ts`

**Source:** SOL-BE-001  
**Phase:** 1 | **Effort:** XS (< 30 min)  
**Depends on:** —

---

## Objective

Tạo hai files nền tảng của platform abstraction layer:
1. `src/platform/types.ts` — định nghĩa `IPlatformServices` và `PlatformMode`
2. `src/platform/context.ts` — singleton pattern `setPlatform` / `getPlatform`

---

## Context cần đọc

- `src/platform/` (directory — chưa tồn tại, sẽ tạo mới)

---

## Files to create

### 1. `src/platform/types.ts`

```typescript
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
```

### 2. `src/platform/context.ts`

```typescript
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
```

### 3. `src/platform/__tests__/context.test.ts`

```typescript
import { describe, it, expect, beforeEach } from 'vitest'
import {
  setPlatform,
  getPlatform,
  isPlatformInitialized,
  _resetPlatformForTesting
} from '../context'
import type { IPlatformServices } from '../types'

function mockPlatform(): IPlatformServices {
  return {
    mode: 'node',
    app: {} as any,
    ipc: {} as any,
    windowManager: {} as any,
    storage: {} as any,
    system: {} as any
  }
}

describe('Platform Context', () => {
  beforeEach(() => {
    _resetPlatformForTesting()
  })

  it('isPlatformInitialized() returns false before setPlatform()', () => {
    expect(isPlatformInitialized()).toBe(false)
  })

  it('getPlatform() throws before setPlatform()', () => {
    expect(() => getPlatform()).toThrow('Platform not initialized')
  })

  it('setPlatform() + getPlatform() roundtrip', () => {
    const p = mockPlatform()
    setPlatform(p)
    expect(getPlatform()).toBe(p)
  })

  it('isPlatformInitialized() returns true after setPlatform()', () => {
    setPlatform(mockPlatform())
    expect(isPlatformInitialized()).toBe(true)
  })

  it('setPlatform() throws if called twice', () => {
    setPlatform(mockPlatform())
    expect(() => setPlatform(mockPlatform())).toThrow('Platform already initialized')
  })

  it('_resetPlatformForTesting() allows re-initialization', () => {
    setPlatform(mockPlatform())
    _resetPlatformForTesting()
    expect(isPlatformInitialized()).toBe(false)
    // Should not throw
    setPlatform(mockPlatform())
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "platform/" | head -20

# Run context tests
npx vitest run src/platform/__tests__/context.test.ts
```

Expected:
- Zero TypeScript errors in `src/platform/`
- 6/6 context tests pass

---

## Done criteria

- [x] `src/platform/types.ts` tồn tại, export `PlatformMode` và `IPlatformServices`
- [x] `src/platform/context.ts` tồn tại với 4 exports
- [x] `src/platform/__tests__/context.test.ts` pass 6 tests
- [x] Không có `import` nào từ `'electron'` trong 3 files trên
