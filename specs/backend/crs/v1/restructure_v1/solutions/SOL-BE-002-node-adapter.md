# SOL-BE-002 — Node.js Adapter Implementation

**CR:** [CR-002](../../../../../docs/crs/v1/restructure_v1/CR-002-node-adapter.md)  
**TDD Refs:** TDD-01 (Architecture), TDD-02 (Main Process), TDD-06 (Persistence)  
**Approach:** Test-Driven — conformance tests từ SOL-BE-001

> **🏁 STATUS: ✅ COMPLETE — 2026-07-23**  
> All 6 AC passed | 125/125 adapter tests | NodeApp/Window/IpcBridge/Storage/System | No electron imports

---

## 1. Phân tích từ TDD

Từ **TDD-02 §1 (Startup Sequence)** và **TDD-06 (Persistence)**:

```
app.whenReady()
  ├─ initDataPath()    ← app.getPath('userData') phải trả về writable dir
  ├─ Store.init()      ← SQLite tại userData/db.sqlite
  └─ ...
```

Từ **TDD-04 §9 (Runtime Metadata)**:
```typescript
// Ghi ra file: ~/.config/orca/runtime/runtime.json
writeRuntimeMetadata({ authToken, wsPort, socketPath, pid })
// NodeApp phải cung cấp đúng userData path cho việc này
```

---

## 2. File Structure

```
src/platform/adapters/node/
├── index.ts             # Factory: createNodeAdapter()
├── app.ts               # NodeApp
├── window.ts            # NodeWindow, NodeWindowManager
├── ipc.ts               # NodeIpcBridge
├── storage.ts           # NodeSecureStorage
├── system.ts            # NodeSystemInfo
└── __tests__/
    ├── app.test.ts
    ├── window.test.ts
    ├── ipc.test.ts
    └── storage.test.ts
```

---

## 3. Test Specifications

### 3.1 `app.test.ts`

```typescript
// src/platform/adapters/node/__tests__/app.test.ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mkdirSync, rmSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { NodeApp } from '../app'

// Re-run IApp conformance suite
import { runIAppConformanceTests } from '../../../__tests__/interface-conformance'

const testDataPath = join(tmpdir(), `orca-test-${Date.now()}`)

describe('NodeApp', () => {
  let app: NodeApp
  
  beforeEach(() => {
    app = new NodeApp({ userDataPath: testDataPath })
  })
  
  afterEach(() => {
    if (existsSync(testDataPath)) {
      rmSync(testDataPath, { recursive: true })
    }
  })

  // Run conformance checks
  runIAppConformanceTests(() => new NodeApp({ userDataPath: testDataPath }))

  // NodeApp-specific tests
  describe('getPath("userData")', () => {
    it('returns the configured userDataPath', () => {
      expect(app.getPath('userData')).toBe(testDataPath)
    })

    it('creates userData directory if it does not exist', () => {
      const freshPath = join(tmpdir(), `orca-fresh-${Date.now()}`)
      new NodeApp({ userDataPath: freshPath })
      expect(existsSync(freshPath)).toBe(true)
      rmSync(freshPath, { recursive: true })
    })

    it('uses ORCA_USER_DATA_PATH env var when no option given', () => {
      const envPath = join(tmpdir(), 'orca-env-test')
      process.env.ORCA_USER_DATA_PATH = envPath
      const envApp = new NodeApp()
      expect(envApp.getPath('userData')).toBe(envPath)
      delete process.env.ORCA_USER_DATA_PATH
      if (existsSync(envPath)) rmSync(envPath, { recursive: true })
    })

    it('falls back to ~/.orca when neither option nor env var', () => {
      delete process.env.ORCA_USER_DATA_PATH
      const defaultApp = new NodeApp()
      expect(defaultApp.getPath('userData')).toContain('.orca')
    })
  })

  describe('getVersion()', () => {
    it('returns ORCA_VERSION env var when set', () => {
      process.env.ORCA_VERSION = '9.9.9-test'
      expect(app.getVersion()).toBe('9.9.9-test')
      delete process.env.ORCA_VERSION
    })

    it('returns fallback "0.0.0" when env var not set', () => {
      delete process.env.ORCA_VERSION
      expect(app.getVersion()).toBe('0.0.0')
    })
  })

  describe('whenReady()', () => {
    it('resolves immediately without waiting', async () => {
      const start = Date.now()
      await app.whenReady()
      expect(Date.now() - start).toBeLessThan(10) // < 10ms
    })

    it('can be awaited multiple times', async () => {
      await app.whenReady()
      await app.whenReady()  // should not throw
    })
  })

  describe('event system', () => {
    it('emits events via EventEmitter', () => {
      const handler = vi.fn()
      app.on('before-quit', handler)
      app.emit('before-quit')
      expect(handler).toHaveBeenCalledOnce()
    })

    it('off() unregisters listener', () => {
      const handler = vi.fn()
      app.on('before-quit', handler)
      app.off('before-quit', handler)
      app.emit('before-quit')
      expect(handler).not.toHaveBeenCalled()
    })
  })

  describe('Path mappings', () => {
    const pathNames = ['home', 'temp', 'exe', 'module', 'desktop',
      'documents', 'downloads', 'music', 'pictures', 'videos'] as const
    
    it.each(pathNames)('getPath("%s") returns absolute path', (name) => {
      const p = app.getPath(name as any)
      expect(typeof p).toBe('string')
      expect(p.length).toBeGreaterThan(0)
    })

    it('getPath(unknown) falls back to userData/unknown', () => {
      const p = app.getPath('unknown-dir' as any)
      expect(p).toContain('unknown-dir')
      expect(p.startsWith(testDataPath)).toBe(true)
    })
  })

  describe('No-ops', () => {
    it('setName() does not throw', () => {
      expect(() => app.setName('TestApp')).not.toThrow()
    })

    it('disableHardwareAcceleration() does not throw', () => {
      expect(() => app.disableHardwareAcceleration()).not.toThrow()
    })

    it('relaunch() emits warning but does not throw', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(() => app.relaunch()).not.toThrow()
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('no-op'))
      warn.mockRestore()
    })
  })
})
```

### 3.2 `window.test.ts`

```typescript
// src/platform/adapters/node/__tests__/window.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { NodeWindow, NodeWindowManager } from '../window'
import { runIWindowConformanceTests } from '../../../__tests__/interface-conformance'

describe('NodeWindow', () => {
  // Run conformance checks
  runIWindowConformanceTests(() => new NodeWindow(1))

  let win: NodeWindow

  beforeEach(() => {
    win = new NodeWindow(42)
  })

  it('has the id provided in constructor', () => {
    expect(win.id).toBe(42)
  })

  describe('send()', () => {
    it('notifies onSend subscribers', () => {
      const callback = vi.fn()
      win.onSend('my:channel', callback)
      win.send('my:channel', 'hello', 'world')
      expect(callback).toHaveBeenCalledWith(['hello', 'world'])
    })

    it('does not call unsubscribed listeners', () => {
      const callback = vi.fn()
      const unsub = win.onSend('my:channel', callback)
      unsub()  // unsubscribe
      win.send('my:channel', 'hello')
      expect(callback).not.toHaveBeenCalled()
    })

    it('does not notify subscribers on different channel', () => {
      const callback = vi.fn()
      win.onSend('channel-A', callback)
      win.send('channel-B', 'data')
      expect(callback).not.toHaveBeenCalled()
    })

    it('silently ignores send after destroy', () => {
      const callback = vi.fn()
      win.onSend('ch', callback)
      win.destroy()
      expect(() => win.send('ch', 'data')).not.toThrow()
      // callback NOT called because subscribers cleared on destroy
    })
  })

  describe('destroy()', () => {
    it('sets isDestroyed() to true', () => {
      win.destroy()
      expect(win.isDestroyed()).toBe(true)
    })

    it('emits "closed" event', () => {
      const handler = vi.fn()
      win.on('closed', handler)
      win.destroy()
      expect(handler).toHaveBeenCalledOnce()
    })

    it('is idempotent — second destroy does not emit event twice', () => {
      const handler = vi.fn()
      win.on('closed', handler)
      win.destroy()
      win.destroy()
      expect(handler).toHaveBeenCalledOnce()
    })
  })
})

describe('NodeWindowManager', () => {
  let manager: NodeWindowManager

  beforeEach(() => {
    manager = new NodeWindowManager()
  })

  describe('createWindow()', () => {
    it('returns a window with positive id', () => {
      const win = manager.createWindow({})
      expect(win.id).toBeGreaterThan(0)
    })

    it('assigns unique ids to each window', () => {
      const w1 = manager.createWindow({})
      const w2 = manager.createWindow({})
      expect(w1.id).not.toBe(w2.id)
    })
  })

  describe('getAllWindows()', () => {
    it('returns empty array initially', () => {
      expect(manager.getAllWindows()).toHaveLength(0)
    })

    it('returns all created windows', () => {
      manager.createWindow({})
      manager.createWindow({})
      expect(manager.getAllWindows()).toHaveLength(2)
    })

    it('removes destroyed windows', () => {
      const win = manager.createWindow({})
      expect(manager.getAllWindows()).toHaveLength(1)
      win.destroy()
      expect(manager.getAllWindows()).toHaveLength(0)
    })
  })

  describe('getFocusedWindow()', () => {
    it('returns null when no main window set', () => {
      expect(manager.getFocusedWindow()).toBeNull()
    })

    it('returns mainWindow when set', () => {
      const win = manager.createWindow({})
      manager.setMainWindow(win)
      expect(manager.getFocusedWindow()).toBe(win)
    })
  })

  describe('setMainWindow()', () => {
    it('accepts null to clear main window', () => {
      const win = manager.createWindow({})
      manager.setMainWindow(win)
      manager.setMainWindow(null)
      expect(manager.getMainWindow()).toBeNull()
    })
  })
})
```

### 3.3 `ipc.test.ts`

```typescript
// src/platform/adapters/node/__tests__/ipc.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { NodeIpcBridge } from '../ipc'
import { NodeWindowManager } from '../window'
import { runIIpcBridgeConformanceTests } from '../../../__tests__/interface-conformance'

describe('NodeIpcBridge', () => {
  let manager: NodeWindowManager
  let ipc: NodeIpcBridge

  beforeEach(() => {
    manager = new NodeWindowManager()
    ipc = new NodeIpcBridge(manager)
  })

  // Run conformance checks
  runIIpcBridgeConformanceTests(() => {
    const m = new NodeWindowManager()
    return new NodeIpcBridge(m)
  })

  describe('invoke() — async handler', () => {
    it('passes args correctly to handler', async () => {
      ipc.handle('math:add', async (_e, a, b) => a + b)
      const result = await ipc.invoke('math:add', 0, 3, 4)
      expect(result).toBe(7)
    })

    it('propagates errors from handler', async () => {
      ipc.handle('bad:handler', async () => {
        throw new Error('handler error')
      })
      await expect(ipc.invoke('bad:handler', 0))
        .rejects.toThrow('handler error')
    })

    it('supports synchronous handler return value', async () => {
      ipc.handle('sync:handler', (_e) => 'sync-result')
      const result = await ipc.invoke('sync:handler', 0)
      expect(result).toBe('sync-result')
    })
  })

  describe('sendToWindow()', () => {
    it('routes message to correct window', () => {
      const win = manager.createWindow({}) as any
      manager.setMainWindow(win)
      
      const received: any[] = []
      win.onSend('test:channel', (args: any[]) => received.push(args))
      
      ipc.sendToWindow(win.id, 'test:channel', 'hello', 42)
      
      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['hello', 42])
    })

    it('is silent when window id not found', () => {
      expect(() => ipc.sendToWindow(9999, 'test:channel')).not.toThrow()
    })
  })

  describe('sendToAll()', () => {
    it('sends to all windows', () => {
      const w1 = manager.createWindow({}) as any
      const w2 = manager.createWindow({}) as any
      
      const received1: any[] = []
      const received2: any[] = []
      w1.onSend('broadcast', (args: any[]) => received1.push(args))
      w2.onSend('broadcast', (args: any[]) => received2.push(args))
      
      ipc.sendToAll('broadcast', 'msg')
      
      expect(received1).toHaveLength(1)
      expect(received2).toHaveLength(1)
    })
  })

  describe('IpcEvent.sender', () => {
    it('provides sender.send() that routes to the source window', async () => {
      const win = manager.createWindow({}) as any
      const received: any[] = []
      win.onSend('reply:channel', (args: any[]) => received.push(args))
      
      ipc.handle('test:with-reply', async (event) => {
        event.sender.send('reply:channel', 'reply-data')
        return 'ok'
      })
      
      await ipc.invoke('test:with-reply', win.id)
      
      expect(received).toHaveLength(1)
      expect(received[0]).toEqual(['reply-data'])
    })
  })
})
```

### 3.4 `storage.test.ts`

```typescript
// src/platform/adapters/node/__tests__/storage.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { rmSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { NodeSecureStorage } from '../storage'
import { NodeApp } from '../app'

describe('NodeSecureStorage', () => {
  const testDataPath = join(tmpdir(), `orca-storage-test-${Date.now()}`)
  let storage: NodeSecureStorage

  beforeEach(() => {
    const app = new NodeApp({ userDataPath: testDataPath })
    storage = new NodeSecureStorage(app)
  })

  afterEach(() => {
    if (existsSync(testDataPath)) {
      rmSync(testDataPath, { recursive: true })
    }
  })

  describe('isEncryptionAvailable()', () => {
    it('returns a boolean', () => {
      expect(typeof storage.isEncryptionAvailable()).toBe('boolean')
    })
  })

  describe('encrypt / decrypt roundtrip', () => {
    it('decryptString(encryptString(x)) === x for ASCII string', () => {
      const plaintext = 'hello-secret-value'
      const encrypted = storage.encryptString(plaintext)
      expect(storage.decryptString(encrypted)).toBe(plaintext)
    })

    it('decryptString(encryptString(x)) === x for Unicode string', () => {
      const plaintext = '秘密データ 🔐 мой пароль'
      const encrypted = storage.encryptString(plaintext)
      expect(storage.decryptString(encrypted)).toBe(plaintext)
    })

    it('encrypted value is a Buffer', () => {
      const encrypted = storage.encryptString('test')
      expect(encrypted).toBeInstanceOf(Buffer)
    })

    it('encrypted value differs from plaintext', () => {
      const plaintext = 'test-value'
      const encrypted = storage.encryptString(plaintext)
      expect(encrypted.toString('utf-8')).not.toBe(plaintext)
    })

    it('two encryptions of same plaintext produce different ciphertexts (random IV)', () => {
      const plaintext = 'same-value'
      const enc1 = storage.encryptString(plaintext)
      const enc2 = storage.encryptString(plaintext)
      expect(enc1).not.toEqual(enc2)
    })

    it('key is persistent across NodeSecureStorage instances', () => {
      const app = new NodeApp({ userDataPath: testDataPath })
      const storage2 = new NodeSecureStorage(app)
      
      const plaintext = 'persistent-test'
      const encrypted = storage.encryptString(plaintext)
      expect(storage2.decryptString(encrypted)).toBe(plaintext)
    })
  })
})
```

---

## 4. Implementation Checklist

### NodeApp
- [x] Extends `EventEmitter` để support `on/off/emit`
- [x] Constructor tạo userData directory nếu không tồn tại
- [x] `getPath('userData')` ưu tiên: option > env var > `~/.orca`
- [x] `whenReady()` trả về `Promise.resolve()` — **không async delay**
- [x] `quit()` emit `before-quit` và `will-quit` trước khi `process.exit(0)`
- [x] `isPackaged = true` (Node mode luôn là production mode)
- [x] `relaunch()` log warning, không crash

### NodeWindow
- [x] Extends `EventEmitter`
- [x] Constructor nhận `id: number`
- [x] `send()` notify tất cả onSend subscribers
- [x] `onSend()` trả về unsubscribe function
- [x] `destroy()` là idempotent, chỉ emit `closed` 1 lần
- [x] Sau `destroy()`, `send()` không notify subscribers

### NodeWindowManager
- [x] `createWindow()` dùng auto-incrementing id
- [x] Tự động remove window khỏi map khi window bị `destroy()`
- [x] `setMainWindow(null)` hợp lệ

### NodeIpcBridge
- [x] `handle()` log warning nếu overwriting
- [x] `invoke()` tạo `IpcEvent` với `sender.id = windowId`
- [x] `invoke()` throws `Error` (không phải string) cho unknown channel
- [x] `sendToAll()` không throw nếu không có window nào

### NodeSecureStorage
- [x] Key được persist trong `userData/.crypto/storage.key` với mode `0o600`
- [x] Dùng AES-256-GCM (authenticated encryption)
- [x] Random IV cho mỗi lần encrypt
- [x] `decryptString()` gracefully fallback nếu data không phải ciphertext
- [x] `isEncryptionAvailable()` trả về `false` nếu crypto init thất bại

---

## 5. Acceptance Criteria

| # | Criteria | Test |
|---|---------|------|
| AC-1 | NodeApp conformance tests pass | `app.test.ts` | ✅ |
| AC-2 | NodeWindow conformance tests pass | `window.test.ts` | ✅ |
| AC-3 | NodeIpcBridge conformance tests pass | `ipc.test.ts` | ✅ |
| AC-4 | Encrypt/decrypt roundtrip pass | `storage.test.ts` | ✅ |
| AC-5 | No `electron` import in any adapter file | grep check | ✅ |
| AC-6 | 100% public method coverage | vitest coverage | ✅ |

---

## 6. Migration Note

Sau khi NodeAdapter pass AC-1 đến AC-6, cập nhật `src/server/index.ts`:

```typescript
// TRƯỚC (workaround):
import electronMock from '../main/mocks/electron'
// ... module.Module.prototype.require hack

// SAU (clean):
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'

const adapter = createNodeAdapter()
setPlatform(adapter)
// electron alias trong vite.server.config.ts sẽ redirect
// sang src/platform/stubs/electron-node-wrapper.ts
```
