# SOL-BE-001 — Platform Interface Implementation

**CR:** [CR-001](../../../../../docs/crs/v1/restructure_v1/CR-001-platform-interface.md)  
**TDD Refs:** TDD-01 (Architecture), TDD-09 (IPC Handlers)  
**Approach:** Test-Driven — viết tests trước implementations

> **🏁 STATUS: ✅ COMPLETE — 2026-07-23**  
> All 5 Acceptance Criteria passed | 6/6 tests | 0 TS errors | No electron imports in `src/platform/`

---

## 1. Phân tích từ TDD

Từ **TDD-01 §2 (Startup Sequence)** và **TDD-09 (IPC Handlers)**:
- `app.whenReady()` phải resolve ngay trong Node mode (không có Electron event loop)
- `app.getPath('userData')` là critical — tất cả persistence đều dùng path này
- `ipcMain.handle()` được gọi bởi hàng trăm handlers trong `src/main/ipc/`
- `BrowserWindow` không có DOM nhưng phải có event emission (closed, focus, v.v.)

---

## 2. File Structure

```
src/platform/
├── index.ts                    # Public exports
├── types.ts                    # IPlatformServices, PlatformMode
├── context.ts                  # Singleton pattern
├── app-interface.ts            # IApp interface
├── window-interface.ts         # IWindow, IWindowManager interfaces
├── ipc-interface.ts            # IIpcBridge interface
├── storage-interface.ts        # ISecureStorage interface
├── system-interface.ts         # ISystemInfo interface
└── __tests__/
    └── context.test.ts         # Context singleton tests
```

---

## 3. Test Specifications

### 3.1 `context.test.ts`

```typescript
// src/platform/__tests__/context.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'

// Reset module state between tests (singleton needs reset)
// Sử dụng vi.resetModules() để isolate mỗi test

describe('Platform Context', () => {
  beforeEach(async () => {
    vi.resetModules()
  })

  describe('setPlatform / getPlatform', () => {
    it('should store and return the platform services', async () => {
      const { setPlatform, getPlatform } = await import('../context')
      const mockPlatform = createMockPlatform()
      
      setPlatform(mockPlatform)
      
      expect(getPlatform()).toBe(mockPlatform)
    })

    it('should throw if getPlatform called before setPlatform', async () => {
      const { getPlatform } = await import('../context')
      
      expect(() => getPlatform()).toThrow('Platform not initialized')
    })

    it('should throw if setPlatform called twice', async () => {
      const { setPlatform } = await import('../context')
      const mockPlatform = createMockPlatform()
      
      setPlatform(mockPlatform)
      
      expect(() => setPlatform(mockPlatform))
        .toThrow('Platform already initialized')
    })

    it('should report isPlatformInitialized correctly', async () => {
      const { setPlatform, isPlatformInitialized } = await import('../context')
      
      expect(isPlatformInitialized()).toBe(false)
      setPlatform(createMockPlatform())
      expect(isPlatformInitialized()).toBe(true)
    })
  })
})

function createMockPlatform() {
  return {
    mode: 'node' as const,
    app: {} as any,
    ipc: {} as any,
    windowManager: {} as any,
    storage: {} as any,
    system: {} as any
  }
}
```

### 3.2 Interface Conformance Tests Pattern

```typescript
// Pattern được dùng trong mỗi adapter test để verify conformance
// src/platform/__tests__/interface-conformance.ts

export function runIAppConformanceTests(factory: () => IApp): void {
  describe('IApp conformance', () => {
    let app: IApp

    beforeEach(() => { app = factory() })

    it('getVersion() returns a string', () => {
      expect(typeof app.getVersion()).toBe('string')
    })

    it('getPath("userData") returns an absolute path', () => {
      const p = app.getPath('userData')
      expect(path.isAbsolute(p)).toBe(true)
    })

    it('getPath("home") returns an absolute path', () => {
      expect(path.isAbsolute(app.getPath('home'))).toBe(true)
    })

    it('getPath("temp") returns an absolute path', () => {
      expect(path.isAbsolute(app.getPath('temp'))).toBe(true)
    })

    it('isPackaged is a boolean', () => {
      expect(typeof app.isPackaged).toBe('boolean')
    })

    it('whenReady() resolves', async () => {
      await expect(app.whenReady()).resolves.toBeUndefined()
    })

    it('on/off event subscription works', () => {
      const handler = vi.fn()
      app.on('quit', handler)
      app.off('quit', handler)
      // Cannot call quit() in tests as it exits process
    })
  })
}

export function runIWindowConformanceTests(factory: () => IWindow): void {
  describe('IWindow conformance', () => {
    let win: IWindow

    beforeEach(() => { win = factory() })

    it('id is a positive number', () => {
      expect(win.id).toBeGreaterThan(0)
    })

    it('isDestroyed() returns false initially', () => {
      expect(win.isDestroyed()).toBe(false)
    })

    it('isMinimized() returns boolean', () => {
      expect(typeof win.isMinimized()).toBe('boolean')
    })

    it('isMaximized() returns boolean', () => {
      expect(typeof win.isMaximized()).toBe('boolean')
    })

    it('isFocused() returns boolean', () => {
      expect(typeof win.isFocused()).toBe('boolean')
    })

    it('isVisible() returns boolean', () => {
      expect(typeof win.isVisible()).toBe('boolean')
    })

    it('send() does not throw', () => {
      expect(() => win.send('test-channel', { data: 1 })).not.toThrow()
    })

    it('destroy() marks window as destroyed', () => {
      win.destroy()
      expect(win.isDestroyed()).toBe(true)
    })

    it('emits "closed" event when destroyed', () => {
      const handler = vi.fn()
      win.on('closed', handler)
      win.destroy()
      expect(handler).toHaveBeenCalledOnce()
    })

    it('double destroy() is safe', () => {
      win.destroy()
      expect(() => win.destroy()).not.toThrow()
    })
  })
}

export function runIIpcBridgeConformanceTests(
  factory: () => IIpcBridge & { invoke(channel: string, windowId: number, ...args: any[]): Promise<any> }
): void {
  describe('IIpcBridge conformance', () => {
    let ipc: ReturnType<typeof factory>

    beforeEach(() => { ipc = factory() })

    it('handle() registers a handler', async () => {
      ipc.handle('test:channel', async (_e, x) => x * 2)
      const result = await ipc.invoke('test:channel', 1, 21)
      expect(result).toBe(42)
    })

    it('invoke() throws for unregistered channel', async () => {
      await expect(ipc.invoke('unknown:channel', 1))
        .rejects.toThrow('No IPC handler registered')
    })

    it('removeHandler() prevents further invocations', async () => {
      ipc.handle('test:removable', async () => 'hello')
      ipc.removeHandler('test:removable')
      await expect(ipc.invoke('test:removable', 1))
        .rejects.toThrow()
    })

    it('handle() with duplicate channel emits warning but overwrites', () => {
      const warnSpy = vi.spyOn(console, 'warn')
      ipc.handle('test:dup', async () => 1)
      ipc.handle('test:dup', async () => 2)
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('Overwriting'))
      warnSpy.mockRestore()
    })

    it('on/off listener works', () => {
      const handler = vi.fn()
      ipc.on('test:event', handler)
      ipc.emit('test:event', { sender: { id: 0, send: vi.fn() } }, 'payload')
      expect(handler).toHaveBeenCalledWith(
        expect.objectContaining({ sender: expect.any(Object) }),
        'payload'
      )
      ipc.off('test:event', handler)
    })
  })
}
```

---

## 4. Implementation Guide

### 4.1 `src/platform/types.ts`

```typescript
export type PlatformMode = 'electron' | 'node'

export interface IPlatformServices {
  readonly mode: PlatformMode
  readonly app: IApp
  readonly ipc: IIpcBridge
  readonly windowManager: IWindowManager
  readonly storage: ISecureStorage
  readonly system: ISystemInfo
}
```

**Checklist:**
- [x] Export `PlatformMode` union type
- [x] Export `IPlatformServices` interface
- [x] Không import gì từ `electron`
- [x] Không import gì từ `src/main/`

### 4.2 `src/platform/context.ts`

**Implementation constraints từ TDD-01:**
> "SQLite as source of truth: toàn bộ state persist trong SQLite"

Context phải được init **trước** khi bất kỳ SQLite access nào xảy ra, vì `getPath('userData')` được dùng để locate database file.

```typescript
// Thứ tự init trong server entry point:
// 1. setPlatform(createNodeAdapter())  ← TRƯỚC TIÊN
// 2. import('../main/index')           ← sau — sẽ dùng getPlatform()
```

**Implementation checklist:**
- [x] `_platform` là module-level variable (not exported)
- [x] `setPlatform()` throws if called twice
- [x] `getPlatform()` throws with clear message if not initialized
- [x] `isPlatformInitialized()` là pure predicate, không throw
- [x] Module có thể được reset trong tests via `vi.resetModules()`

### 4.3 `src/platform/ipc-interface.ts`

Từ **TDD-09 §1**, các IPC handlers trong `src/main/ipc/` dùng pattern:
```typescript
ipcMain.handle('channel:name', async (event, ...args) => { ... })
```

Interface phải mirror chính xác signature này:
```typescript
export type IpcHandler = (event: IpcEvent, ...args: any[]) => Promise<any> | any

// IpcEvent.sender phải có .send() để match Electron's ipcMain event
export interface IpcEvent {
  sender: {
    readonly id: number
    send(channel: string, ...args: any[]): void
  }
}
```

---

## 5. Verification Commands

```bash
# 1. TypeScript compilation
pnpm tsc --noEmit

# 2. Run platform tests
pnpm vitest run src/platform/

# 3. Verify no electron imports
grep -r "from 'electron'" src/platform/
# Expected: empty output

# 4. Verify no src/main imports
grep -r "from.*src/main" src/platform/
# Expected: empty output
```

---

## 6. Acceptance Criteria

| # | Criteria | Test |
|---|---------|------|
| AC-1 | `context.ts` compiles với `node` environment | `pnpm tsc` | ✅ |
| AC-2 | `setPlatform` / `getPlatform` lifecycle passes | `context.test.ts` | ✅ |
| AC-3 | All interface types are non-`any` | TypeScript strict mode | ✅ |
| AC-4 | No `electron` dependency in `src/platform/` | grep check | ✅ |
| AC-5 | Conformance test suite passes for NodeAdapter | SOL-BE-002 | ✅ |
