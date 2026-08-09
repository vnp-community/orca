/**
 * Interface Conformance Test Helpers
 *
 * Cung cấp các test suite để verify bất kỳ IApp, IWindow, IIpcBridge
 * implementation nào đều đúng theo contract.
 *
 * Sử dụng trong TASK-004, TASK-005, TASK-006 (NodeAdapter tests).
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { isAbsolute } from 'node:path'
import type { IApp } from '../app-interface'
import type { IWindow } from '../window-interface'
import type { IIpcBridge, IpcEvent, IpcListener } from '../ipc-interface'

// ─── IApp Conformance ───────────────────────────────────────────────────────

export function runIAppConformanceTests(factory: () => IApp): void {
  describe('IApp conformance', () => {
    let app: IApp

    beforeEach(() => {
      app = factory()
    })

    it('getVersion() returns a non-empty string', () => {
      expect(typeof app.getVersion()).toBe('string')
      expect(app.getVersion().length).toBeGreaterThan(0)
    })

    it('getPath("userData") returns absolute path', () => {
      expect(isAbsolute(app.getPath('userData'))).toBe(true)
    })

    it('getPath("home") returns absolute path', () => {
      expect(isAbsolute(app.getPath('home'))).toBe(true)
    })

    it('getPath("temp") returns absolute path', () => {
      expect(isAbsolute(app.getPath('temp'))).toBe(true)
    })

    it('isPackaged is a boolean', () => {
      expect(typeof app.isPackaged).toBe('boolean')
    })

    it('whenReady() resolves', async () => {
      await expect(app.whenReady()).resolves.toBeUndefined()
    })

    it('whenReady() resolves within 10ms (should be immediate)', async () => {
      const start = Date.now()
      await app.whenReady()
      expect(Date.now() - start).toBeLessThan(10)
    })

    it('on/off event subscription does not throw', () => {
      const handler = vi.fn()
      expect(() => app.on('quit', handler)).not.toThrow()
      expect(() => app.off('quit', handler)).not.toThrow()
    })

    it('relaunch() does not throw', () => {
      expect(() => app.relaunch()).not.toThrow()
    })

    it('setName() does not throw', () => {
      expect(() => app.setName('test')).not.toThrow()
    })

    it('disableHardwareAcceleration() does not throw', () => {
      expect(() => app.disableHardwareAcceleration()).not.toThrow()
    })
  })
}

// ─── IWindow Conformance ─────────────────────────────────────────────────────

export function runIWindowConformanceTests(factory: () => IWindow): void {
  describe('IWindow conformance', () => {
    let win: IWindow

    beforeEach(() => {
      win = factory()
    })

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

    it('isFullScreen() returns boolean', () => {
      expect(typeof win.isFullScreen()).toBe('boolean')
    })

    it('isVisible() returns boolean', () => {
      expect(typeof win.isVisible()).toBe('boolean')
    })

    it('isFocused() returns boolean', () => {
      expect(typeof win.isFocused()).toBe('boolean')
    })

    it('send() does not throw', () => {
      expect(() => win.send('test:channel', { x: 1 })).not.toThrow()
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

    it('double destroy() is safe (idempotent)', () => {
      win.destroy()
      expect(() => win.destroy()).not.toThrow()
    })

    it('"closed" event fires only once even on double destroy', () => {
      const handler = vi.fn()
      win.on('closed', handler)
      win.destroy()
      win.destroy()
      expect(handler).toHaveBeenCalledOnce()
    })
  })
}

// ─── IIpcBridge Conformance ───────────────────────────────────────────────────

/**
 * factory must return an IIpcBridge that also exposes
 * an `invoke(channel, windowId, ...args)` method (NodeIpcBridge-specific)
 */
export function runIIpcBridgeConformanceTests(
  factory: () => IIpcBridge & {
    invoke(channel: string, windowId: number, ...args: any[]): Promise<any>
    emit(channel: string, event: IpcEvent, ...args: any[]): boolean
  }
): void {
  describe('IIpcBridge conformance', () => {
    let ipc: ReturnType<typeof factory>

    beforeEach(() => {
      ipc = factory()
    })

    it('handle() + invoke() roundtrip', async () => {
      ipc.handle('test:double', async (_e: IpcEvent, n: number) => n * 2)
      const result = await ipc.invoke('test:double', 1, 21)
      expect(result).toBe(42)
    })

    it('invoke() rejects for unregistered channel', async () => {
      await expect(ipc.invoke('no:handler', 1))
        .rejects.toThrow('No IPC handler registered')
    })

    it('removeHandler() prevents further invocations', async () => {
      ipc.handle('test:remove', async () => 'ok')
      ipc.removeHandler('test:remove')
      await expect(ipc.invoke('test:remove', 1)).rejects.toThrow()
    })

    it('handle() with duplicate warns and overwrites', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      ipc.handle('test:dup', async () => 1)
      ipc.handle('test:dup', async () => 2)
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('Overwriting'))
      const result = await ipc.invoke('test:dup', 1)
      expect(result).toBe(2)
      warnSpy.mockRestore()
    })

    it('on() listener receives emitted events', () => {
      const listener: IpcListener = vi.fn()
      const event: IpcEvent = { sender: { id: 0, send: vi.fn() } }
      ipc.on('test:event', listener)
      ipc.emit('test:event', event, 'payload')
      expect(listener).toHaveBeenCalledWith(event, 'payload')
    })

    it('off() removes listener', () => {
      const listener: IpcListener = vi.fn()
      const event: IpcEvent = { sender: { id: 0, send: vi.fn() } }
      ipc.on('test:off', listener)
      ipc.off('test:off', listener)
      ipc.emit('test:off', event)
      expect(listener).not.toHaveBeenCalled()
    })
  })
}
