# TASK-005: Tạo `NodeWindow` + `NodeWindowManager`

**Source:** SOL-BE-002  
**Phase:** 1 | **Effort:** S (45–60 min)  
**Depends on:** TASK-002, TASK-003

---

## Objective

Tạo `src/platform/adapters/node/window.ts` — implementation của `IWindow` và `IWindowManager` dùng EventEmitter thuần. Không có DOM, không có Electron.

---

## Files to create

### 1. `src/platform/adapters/node/window.ts`

```typescript
import { EventEmitter } from 'node:events'
import type {
  IWindow,
  IWindowManager,
  WindowCreationOptions,
  WindowEvent
} from '../../window-interface'

/**
 * NodeWindow — IWindow implementation for Node.js server mode.
 *
 * Represents a "virtual window" — no actual GUI, but provides:
 * - Event emission (closed, focus, etc.)
 * - send() routing to WebSocket subscribers
 */
export class NodeWindow extends EventEmitter implements IWindow {
  readonly id: number
  private _destroyed = false
  private readonly _sendSubscribers = new Map<string, Set<(args: any[]) => void>>()

  constructor(id: number) {
    super()
    this.id = id
  }

  // ── State queries ──────────────────────────────────────────────────────────
  isDestroyed(): boolean { return this._destroyed }
  isMinimized(): boolean { return false }
  isMaximized(): boolean { return false }
  isFullScreen(): boolean { return false }
  isVisible(): boolean { return true }
  isFocused(): boolean { return true }

  // ── Actions ────────────────────────────────────────────────────────────────
  show(): void {}
  hide(): void {}
  focus(): void {}
  restore(): void {}

  close(): void {
    this.destroy()
  }

  destroy(): void {
    if (this._destroyed) return
    this._destroyed = true
    this._sendSubscribers.clear()
    this.emit('closed')
    this.removeAllListeners()
  }

  // ── Messaging ──────────────────────────────────────────────────────────────

  /**
   * Send a message to all subscribers on this channel.
   * In Node mode, this routes to WebSocket clients via WebIpcBridge.
   */
  send(channel: string, ...args: any[]): void {
    if (this._destroyed) return
    const subs = this._sendSubscribers.get(channel)
    if (subs) {
      for (const cb of subs) cb(args)
    }
  }

  /**
   * Subscribe to messages sent via send() on a specific channel.
   * Returns an unsubscribe function.
   * Used by WebIpcBridge to forward window messages to WebSocket clients.
   */
  onSend(channel: string, callback: (args: any[]) => void): () => void {
    let set = this._sendSubscribers.get(channel)
    if (!set) {
      set = new Set()
      this._sendSubscribers.set(channel, set)
    }
    set.add(callback)
    return () => set!.delete(callback)
  }

  // ── EventEmitter overrides for type compatibility ──────────────────────────
  on(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.on(event, listener)
  }
  once(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.once(event, listener)
  }
  off(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.off(event, listener)
  }
}

/**
 * NodeWindowManager — IWindowManager for Node.js server mode.
 *
 * Creates and tracks NodeWindow instances.
 * Each connected WebSocket client gets its own NodeWindow (virtual).
 */
export class NodeWindowManager implements IWindowManager {
  private readonly _windows = new Map<number, NodeWindow>()
  private _mainWindow: NodeWindow | null = null
  private _nextId = 1

  createWindow(_options: WindowCreationOptions = {}): NodeWindow {
    const win = new NodeWindow(this._nextId++)
    this._windows.set(win.id, win)
    win.once('closed', () => this._windows.delete(win.id))
    return win
  }

  getAllWindows(): IWindow[] {
    return [...this._windows.values()]
  }

  getFocusedWindow(): IWindow | null {
    return this._mainWindow
  }

  getMainWindow(): IWindow | null {
    return this._mainWindow
  }

  setMainWindow(window: IWindow | null): void {
    this._mainWindow = window as NodeWindow | null
  }

  /** Look up a window by id */
  getWindowById(id: number): NodeWindow | undefined {
    return this._windows.get(id)
  }
}
```

### 2. `src/platform/adapters/node/__tests__/window.test.ts`

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { NodeWindow, NodeWindowManager } from '../window'
import { runIWindowConformanceTests } from '../../../__tests__/interface-conformance'

// ── Conformance ──────────────────────────────────────────────────────────────
runIWindowConformanceTests(() => new NodeWindow(1))

// ── NodeWindow-specific ───────────────────────────────────────────────────────
describe('NodeWindow — specific behavior', () => {
  let win: NodeWindow

  beforeEach(() => { win = new NodeWindow(99) })

  it('has the id provided in constructor', () => {
    expect(win.id).toBe(99)
  })

  describe('send() + onSend()', () => {
    it('notifies subscribers on matching channel', () => {
      const cb = vi.fn()
      win.onSend('ch:test', cb)
      win.send('ch:test', 'hello', 42)
      expect(cb).toHaveBeenCalledWith(['hello', 42])
    })

    it('does not notify subscribers on different channel', () => {
      const cb = vi.fn()
      win.onSend('ch:A', cb)
      win.send('ch:B', 'data')
      expect(cb).not.toHaveBeenCalled()
    })

    it('unsubscribe fn stops notifications', () => {
      const cb = vi.fn()
      const unsub = win.onSend('ch:unsub', cb)
      unsub()
      win.send('ch:unsub', 'x')
      expect(cb).not.toHaveBeenCalled()
    })

    it('multiple subscribers all receive', () => {
      const cb1 = vi.fn()
      const cb2 = vi.fn()
      win.onSend('ch:multi', cb1)
      win.onSend('ch:multi', cb2)
      win.send('ch:multi', 'data')
      expect(cb1).toHaveBeenCalledOnce()
      expect(cb2).toHaveBeenCalledOnce()
    })

    it('send() after destroy() is silent', () => {
      const cb = vi.fn()
      win.onSend('ch:after-destroy', cb)
      win.destroy()
      expect(() => win.send('ch:after-destroy', 'x')).not.toThrow()
      expect(cb).not.toHaveBeenCalled()
    })
  })

  describe('destroy()', () => {
    it('clears all send subscribers', () => {
      const cb = vi.fn()
      win.onSend('ch:clear', cb)
      win.destroy()
      win.send('ch:clear', 'x')
      expect(cb).not.toHaveBeenCalled()
    })
  })
})

// ── NodeWindowManager ─────────────────────────────────────────────────────────
describe('NodeWindowManager', () => {
  let manager: NodeWindowManager

  beforeEach(() => { manager = new NodeWindowManager() })

  describe('createWindow()', () => {
    it('returns NodeWindow with positive id', () => {
      const w = manager.createWindow({})
      expect(w.id).toBeGreaterThan(0)
    })

    it('assigns unique ids', () => {
      const w1 = manager.createWindow({})
      const w2 = manager.createWindow({})
      expect(w1.id).not.toBe(w2.id)
    })
  })

  describe('getAllWindows()', () => {
    it('is empty initially', () => {
      expect(manager.getAllWindows()).toHaveLength(0)
    })

    it('lists created windows', () => {
      manager.createWindow({})
      manager.createWindow({})
      expect(manager.getAllWindows()).toHaveLength(2)
    })

    it('removes destroyed windows automatically', () => {
      const w = manager.createWindow({})
      w.destroy()
      expect(manager.getAllWindows()).toHaveLength(0)
    })
  })

  describe('mainWindow', () => {
    it('getFocusedWindow() returns null initially', () => {
      expect(manager.getFocusedWindow()).toBeNull()
    })

    it('getFocusedWindow() returns main window after setMainWindow()', () => {
      const w = manager.createWindow({})
      manager.setMainWindow(w)
      expect(manager.getFocusedWindow()).toBe(w)
    })

    it('setMainWindow(null) clears main window', () => {
      const w = manager.createWindow({})
      manager.setMainWindow(w)
      manager.setMainWindow(null)
      expect(manager.getMainWindow()).toBeNull()
    })
  })

  describe('getWindowById()', () => {
    it('returns window by id', () => {
      const w = manager.createWindow({})
      expect(manager.getWindowById(w.id)).toBe(w)
    })

    it('returns undefined for unknown id', () => {
      expect(manager.getWindowById(9999)).toBeUndefined()
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "adapters/node/window" | head -10
npx vitest run src/platform/adapters/node/__tests__/window.test.ts
```

Expected: **20+ tests pass**, 0 TypeScript errors.

---

## Done criteria

- [x] `src/platform/adapters/node/window.ts` tạo thành công
- [x] `NodeWindow` có `onSend()` method trả về unsubscribe function
- [x] `NodeWindowManager` có `getWindowById()` method
- [x] 20+ unit tests pass
- [x] Conformance tests pass
