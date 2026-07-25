# TASK-008: Tạo `createNodeAdapter()` factory + unit tests

**Source:** SOL-BE-002  
**Phase:** 1 | **Effort:** S (30–45 min)  
**Depends on:** TASK-004, TASK-005, TASK-006, TASK-007

---

## Objective

Tạo `src/platform/adapters/node/index.ts` — factory function `createNodeAdapter()` kết hợp tất cả các Node adapter components thành một `IPlatformServices` object. Đây là public entry point của NodeAdapter.

---

## Files to create

### 1. `src/platform/adapters/node/index.ts`

```typescript
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
```

### 2. `src/platform/adapters/node/__tests__/index.test.ts`

```typescript
import { describe, it, expect, afterEach } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { createNodeAdapter } from '../index'
import type { IPlatformServices } from '../../../types'
import { isAbsolute } from 'node:path'

const testPath = join(tmpdir(), `orca-adapter-test-${Date.now()}`)

afterEach(() => {
  if (existsSync(testPath)) rmSync(testPath, { recursive: true })
})

describe('createNodeAdapter()', () => {
  let platform: IPlatformServices

  beforeEach(() => {
    platform = createNodeAdapter({ userDataPath: testPath })
  })

  describe('mode', () => {
    it('mode is "node"', () => {
      expect(platform.mode).toBe('node')
    })
  })

  describe('app', () => {
    it('app is defined', () => {
      expect(platform.app).toBeDefined()
    })

    it('app.getPath("userData") returns testPath', () => {
      expect(platform.app.getPath('userData')).toBe(testPath)
    })

    it('app.whenReady() resolves', async () => {
      await expect(platform.app.whenReady()).resolves.toBeUndefined()
    })

    it('app.isPackaged is true', () => {
      expect(platform.app.isPackaged).toBe(true)
    })
  })

  describe('ipc', () => {
    it('ipc is defined', () => {
      expect(platform.ipc).toBeDefined()
    })

    it('ipc.handle + invoke roundtrip', async () => {
      const ipc = platform.ipc as any
      ipc.handle('test:ping', async () => 'pong')
      const result = await ipc.invoke('test:ping', 0)
      expect(result).toBe('pong')
    })
  })

  describe('windowManager', () => {
    it('windowManager is defined', () => {
      expect(platform.windowManager).toBeDefined()
    })

    it('windowManager.getAllWindows() returns []', () => {
      expect(platform.windowManager.getAllWindows()).toEqual([])
    })

    it('windowManager.createWindow() returns window with id', () => {
      const win = platform.windowManager.createWindow({})
      expect(win.id).toBeGreaterThan(0)
    })
  })

  describe('storage', () => {
    it('storage is defined', () => {
      expect(platform.storage).toBeDefined()
    })

    it('storage encrypt/decrypt roundtrip', () => {
      const enc = platform.storage.encryptString('secret')
      expect(platform.storage.decryptString(enc)).toBe('secret')
    })
  })

  describe('system', () => {
    it('system is defined', () => {
      expect(platform.system).toBeDefined()
    })

    it('system.getCpuCount() >= 1', () => {
      expect(platform.system.getCpuCount()).toBeGreaterThanOrEqual(1)
    })
  })

  describe('inter-component wiring', () => {
    it('ipc.sendToWindow routes to window', () => {
      const win = platform.windowManager.createWindow({})
      const received: any[] = []
      ;(win as any).onSend('push:ch', (args: any[]) => received.push(args))

      platform.ipc.sendToWindow(win.id, 'push:ch', 'payload')

      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['payload'])
    })

    it('ipc.sendToAll broadcasts to all windows', () => {
      const w1 = platform.windowManager.createWindow({})
      const w2 = platform.windowManager.createWindow({})
      const r1: any[] = []
      const r2: any[] = []
      ;(w1 as any).onSend('broadcast', (a: any[]) => r1.push(a))
      ;(w2 as any).onSend('broadcast', (a: any[]) => r2.push(a))

      platform.ipc.sendToAll('broadcast', 'msg')

      expect(r1).toHaveLength(1)
      expect(r2).toHaveLength(1)
    })
  })

  describe('multiple adapters', () => {
    it('each createNodeAdapter() returns independent instances', () => {
      const p1 = createNodeAdapter({ userDataPath: join(testPath, 'p1') })
      const p2 = createNodeAdapter({ userDataPath: join(testPath, 'p2') })

      expect(p1.app).not.toBe(p2.app)
      expect(p1.ipc).not.toBe(p2.ipc)
      expect(p1.windowManager).not.toBe(p2.windowManager)
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript
npx tsc --noEmit 2>&1 | grep "adapters/node" | head -20

# All adapter tests
npx vitest run src/platform/adapters/node/

# Verify no electron imports in entire platform dir
grep -r "from 'electron'" src/platform/
# Expected: empty
```

Expected: **50+ total tests pass** (combined from TASK-004 through TASK-008), 0 errors.

---

## Done criteria

- [x] `src/platform/adapters/node/index.ts` tạo thành công với `createNodeAdapter()`
- [x] Factory wires tất cả 5 components: `app`, `windowManager`, `ipc`, `storage`, `system`
- [x] `ipc.sendToWindow` → `window.send` routing hoạt động end-to-end
- [x] 15+ tests in `index.test.ts` pass
- [x] Không có `import 'electron'` trong bất kỳ file nào trong `src/platform/`
