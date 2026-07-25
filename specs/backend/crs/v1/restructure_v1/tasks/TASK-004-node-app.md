# TASK-004: Tạo `NodeApp` — IApp implementation

**Source:** SOL-BE-002  
**Phase:** 1 | **Effort:** S (45–60 min)  
**Depends on:** TASK-002, TASK-003

---

## Objective

Tạo `src/platform/adapters/node/app.ts` — implementation của `IApp` chạy thuần Node.js, không có Electron. Bao gồm cả unit test đầy đủ.

---

## Files to create

### 1. `src/platform/adapters/node/app.ts`

```typescript
import { EventEmitter } from 'node:events'
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
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
      case 'userData':   return this._userDataPath
      case 'appData':    return join(home, '.config')
      case 'home':       return home
      case 'temp':       return tmpdir()
      case 'exe':        return process.execPath
      case 'module':     return __dirname
      case 'desktop':    return join(home, 'Desktop')
      case 'documents':  return join(home, 'Documents')
      case 'downloads':  return join(home, 'Downloads')
      case 'music':      return join(home, 'Music')
      case 'pictures':   return join(home, 'Pictures')
      case 'videos':     return join(home, 'Videos')
      default:           return join(this._userDataPath, name)
    }
  }

  getAppPath(): string {
    return process.env.ORCA_APP_PATH ?? join(__dirname, '..', '..', '..', '..')
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

  // EventEmitter implements on/off/once/emit — no override needed
  // TypeScript needs explicit cast for return type compatibility
  on(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.on(event, listener)
  }

  off(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.off(event, listener)
  }

  once(event: AppEvent, listener: (...args: any[]) => void): this {
    return super.once(event, listener)
  }
}
```

### 2. `src/platform/adapters/node/__tests__/app.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir, homedir } from 'node:os'
import { NodeApp } from '../app'
import { runIAppConformanceTests } from '../../../__tests__/interface-conformance'

const testDataPath = join(tmpdir(), `orca-node-app-test-${Date.now()}`)

// Cleanup after each test group
afterEach(() => {
  if (existsSync(testDataPath)) rmSync(testDataPath, { recursive: true })
})

// ── Conformance ──────────────────────────────────────────────────────────────
runIAppConformanceTests(() => new NodeApp({ userDataPath: testDataPath }))

// ── NodeApp-specific ─────────────────────────────────────────────────────────
describe('NodeApp — specific behavior', () => {
  let app: NodeApp

  beforeEach(() => {
    app = new NodeApp({ userDataPath: testDataPath })
  })

  describe('userDataPath resolution', () => {
    it('uses option.userDataPath when provided', () => {
      expect(app.getPath('userData')).toBe(testDataPath)
    })

    it('creates userData directory if missing', () => {
      const freshPath = join(tmpdir(), `orca-fresh-${Date.now()}`)
      new NodeApp({ userDataPath: freshPath })
      expect(existsSync(freshPath)).toBe(true)
      rmSync(freshPath, { recursive: true })
    })

    it('uses ORCA_USER_DATA_PATH env var when no option', () => {
      const envPath = join(tmpdir(), `orca-env-${Date.now()}`)
      process.env.ORCA_USER_DATA_PATH = envPath
      const envApp = new NodeApp()
      expect(envApp.getPath('userData')).toBe(envPath)
      delete process.env.ORCA_USER_DATA_PATH
      if (existsSync(envPath)) rmSync(envPath, { recursive: true })
    })

    it('falls back to ~/.orca when no option and no env var', () => {
      delete process.env.ORCA_USER_DATA_PATH
      const defaultApp = new NodeApp()
      expect(defaultApp.getPath('userData')).toContain('.orca')
    })
  })

  describe('getVersion()', () => {
    it('returns ORCA_VERSION env var', () => {
      process.env.ORCA_VERSION = '9.9.9-test'
      expect(app.getVersion()).toBe('9.9.9-test')
      delete process.env.ORCA_VERSION
    })

    it('returns "0.0.0" as fallback', () => {
      delete process.env.ORCA_VERSION
      expect(app.getVersion()).toBe('0.0.0')
    })
  })

  describe('path mappings', () => {
    const paths = ['home', 'temp', 'desktop', 'documents', 'downloads'] as const
    it.each(paths)('getPath("%s") returns a string', (name) => {
      expect(typeof app.getPath(name)).toBe('string')
      expect(app.getPath(name).length).toBeGreaterThan(0)
    })

    it('getPath(unknown) returns path inside userData', () => {
      const p = app.getPath('my-custom-dir' as any)
      expect(p.startsWith(testDataPath)).toBe(true)
      expect(p).toContain('my-custom-dir')
    })
  })

  describe('event emission', () => {
    it('on/emit works via EventEmitter', () => {
      const handler = vi.fn()
      app.on('before-quit', handler)
      app.emit('before-quit')
      expect(handler).toHaveBeenCalledOnce()
    })

    it('off removes listener', () => {
      const handler = vi.fn()
      app.on('test-event', handler)
      app.off('test-event', handler)
      app.emit('test-event')
      expect(handler).not.toHaveBeenCalled()
    })
  })

  describe('no-ops', () => {
    it('relaunch() warns without throwing', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(() => app.relaunch()).not.toThrow()
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('no-op'))
      warn.mockRestore()
    })

    it('setName() does not throw', () => {
      expect(() => app.setName('Orca')).not.toThrow()
    })

    it('disableHardwareAcceleration() does not throw', () => {
      expect(() => app.disableHardwareAcceleration()).not.toThrow()
    })
  })

  describe('whenReady()', () => {
    it('resolves immediately', async () => {
      const start = Date.now()
      await app.whenReady()
      expect(Date.now() - start).toBeLessThan(10)
    })

    it('can be awaited multiple times', async () => {
      await app.whenReady()
      await expect(app.whenReady()).resolves.toBeUndefined()
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "adapters/node/app" | head -10

# Run tests
npx vitest run src/platform/adapters/node/__tests__/app.test.ts
```

Expected: **15+ tests pass**, 0 errors.

---

## Done criteria

- [x] `src/platform/adapters/node/app.ts` tạo thành công
- [x] `src/platform/adapters/node/__tests__/app.test.ts` tạo thành công
- [x] 15+ unit tests pass
- [x] Không có `import` từ `'electron'`
- [x] Conformance tests (`runIAppConformanceTests`) pass
