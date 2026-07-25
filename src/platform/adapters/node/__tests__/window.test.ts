import { describe, it, expect, beforeEach, vi } from 'vitest'
import { NodeWindow, NodeWindowManager } from '../window'
import { runIWindowConformanceTests } from '../../../__tests__/interface-conformance'

// ── Conformance ──────────────────────────────────────────────────────────────
runIWindowConformanceTests(() => new NodeWindow(1))

// ── NodeWindow-specific ───────────────────────────────────────────────────────
describe('NodeWindow — specific behavior', () => {
  let win: NodeWindow

  beforeEach(() => {
    win = new NodeWindow(99)
  })

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

  beforeEach(() => {
    manager = new NodeWindowManager()
  })

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
